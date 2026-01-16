package main

import (
	"context"
	"crypto/tls"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"potstack/config"
	"potstack/internal/api"
	"potstack/internal/db"
	"potstack/internal/git"
	pothttps "potstack/internal/https"
	"potstack/internal/keeper"
	"potstack/internal/loader"
	"potstack/internal/resource"
	"potstack/internal/router"
	"potstack/internal/service"

	"github.com/gin-gonic/gin"
	_ "github.com/glebarez/go-sqlite" // 强制注册驱动
)

func main() {
	// 确保必要目录存在
	initDirectories()

	// 初始化日志
	initLogging()

	log.Println("Starting PotStack One...")

	// 初始化 HTTPS 配置
	templateFile := pothttps.GetTemplateFile()
	if err := pothttps.Init(config.HTTPSConfig, templateFile); err != nil {
		log.Printf("Warning: failed to init HTTPS config: %v", err)
	}

	// 启动配置热重载
	pothttps.StartWatcher(30 * time.Second)

	// 初始化数据库（延迟初始化，等待 potstack/repo.git 仓库存在）
	initDatabase()

	// 初始化 Services
	userService := service.NewUserService()
	repoService := service.NewRepoService()

	// 初始化动态路由器
	dynamicRouter := router.NewRouter(config.RepoDir)

	// 初始化 Keeper（Sandbox 管理器）
	sandboxManager := keeper.NewManager(config.RepoDir, dynamicRouter)

	// 创建用于优雅退出的 Context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动服务
	srvErrCh := make(chan error, 1)
	go func() {
		if err := runService(ctx, userService, repoService, dynamicRouter); err != nil {
			srvErrCh <- err
		}
	}()

	// 启动 Loader 初始化（异步，等待服务就绪）
	loaderDone := loader.StartAsync(userService, repoService, sandboxManager)

	// 等待 Loader 完成后启动 Keeper
	go func() {
		<-loaderDone
		sandboxManager.StartKeeper()
	}()

	// 信号处理
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		log.Printf("Received signal: %v. Shutting down...\n", sig)
	case err := <-srvErrCh:
		log.Printf("Service error: %v. Shutting down...\n", err)
	}

	cancel()

	// 关闭数据库
	db.Close()

	// 等待协程清理资源
	<-time.After(2 * time.Second)
	log.Println("PotStack exit.")
}

func initDirectories() {
	// 创建必要的目录
	dirs := []string{
		config.RepoDir,               // 仓库目录
		config.CertsDir,              // 证书目录
		filepath.Dir(config.LogFile), // 日志目录
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Printf("Warning: failed to create directory %s: %v", dir, err)
		}
	}
}

func initLogging() {
	if config.LogFile == "" {
		return
	}

	logFile, err := os.OpenFile(config.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Printf("Warning: failed to open log file: %v, using stdout", err)
		return
	}

	log.SetOutput(logFile)
	gin.DefaultWriter = logFile
	gin.DefaultErrorWriter = logFile
}

func initDatabase() {
	// 确保 potstack/repo.git/data 目录存在（直接创建，不等待 Loader）
	dbDir := filepath.Join(config.RepoDir, "potstack", "repo.git", "data")
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		log.Printf("Warning: failed to create db directory: %v", err)
		return
	}

	// 初始化数据库
	if err := db.Init(config.RepoDir); err != nil {
		log.Printf("Warning: failed to init database: %v", err)
		return
	}

	log.Println("Database initialized")
}

func runService(ctx context.Context, us service.IUserService, rs service.IRepoService, dynamicRouter *router.Router) error {
	// 设置 TLS（业务和管理端口共享）
	certManager := pothttps.NewManager()
	tlsConfig, err := certManager.Setup()
	if err != nil {
		log.Printf("TLS setup failed: %v, falling back to HTTP", err)
		tlsConfig = nil
	}

	if tlsConfig != nil {
		certManager.StartCertWatcher(30 * time.Second)
		certManager.StartRenewalChecker(1 * time.Minute)
	}

	// 启动三个端口
	go runBusinessService(ctx, dynamicRouter, tlsConfig)
	go runAdminService(ctx, dynamicRouter, tlsConfig)
	runInternalService(ctx, dynamicRouter) // 阻塞

	return nil
}

// runBusinessService 业务端口 (61080) - /web, /api, /cdn
func runBusinessService(ctx context.Context, dynamicRouter *router.Router, tlsConfig *tls.Config) {
	r := gin.Default()

	// CDN 静态资源
	r.GET("/cdn/*path", resource.CDNProcessor())

	// 动态路由：/api/{org}/{name}/*
	r.Any("/api/:org/:name/*path", func(c *gin.Context) {
		dynamicRouter.ServeHTTP(c.Writer, c.Request)
	})

	// 动态路由：/web/{org}/{name}/*
	r.Any("/web/:org/:name/*path", func(c *gin.Context) {
		dynamicRouter.ServeHTTP(c.Writer, c.Request)
	})

	// 健康检查
	r.GET("/health", api.HealthCheckHandler)

	srv := &http.Server{
		Addr:      ":" + config.HTTPPort,
		Handler:   r,
		TLSConfig: tlsConfig,
	}

	if tlsConfig != nil {
		log.Printf("Business HTTPS listening on :%s", config.HTTPPort)
		if err := srv.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			log.Printf("Business HTTPS server error: %v", err)
		}
	} else {
		log.Printf("Business HTTP listening on :%s", config.HTTPPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Business HTTP server error: %v", err)
		}
	}
}

// runAdminService 管理端口 (61081) - /health, /admin
func runAdminService(ctx context.Context, dynamicRouter *router.Router, tlsConfig *tls.Config) {
	r := gin.Default()

	// 健康检查
	r.GET("/health", api.HealthCheckHandler)

	// 动态路由：/admin/{org}/{name}/*
	r.Any("/admin/:org/:name/*path", func(c *gin.Context) {
		dynamicRouter.ServeHTTP(c.Writer, c.Request)
	})

	srv := &http.Server{
		Addr:      ":" + config.AdminPort,
		Handler:   r,
		TLSConfig: tlsConfig,
	}

	if tlsConfig != nil {
		log.Printf("Admin HTTPS listening on :%s", config.AdminPort)
		if err := srv.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			log.Printf("Admin HTTPS server error: %v", err)
		}
	} else {
		log.Printf("Admin HTTP listening on :%s", config.AdminPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Admin HTTP server error: %v", err)
		}
	}
}

// runInternalService 内部端口 (61082) - /pot, /repo, /refresh（HTTP only，无认证）
func runInternalService(ctx context.Context, dynamicRouter *router.Router) {
	r := gin.Default()

	// 刷新路由接口
	r.POST("/pot/potstack/router/refresh", router.RefreshHandler(dynamicRouter))

	// 动态路由：/pot/{org}/{name}/*
	r.Any("/pot/:org/:name/*path", func(c *gin.Context) {
		dynamicRouter.ServeHTTP(c.Writer, c.Request)
	})

	// Git Smart HTTP 协议（内部端口无认证）
	r.Any("/repo/:owner/:reponame/*action", git.SmartHTTPServer())

	// 健康检查
	r.GET("/health", api.HealthCheckHandler)

	srv := &http.Server{
		Addr:    ":" + config.InternalPort,
		Handler: r,
	}

	log.Printf("Internal HTTP listening on :%s", config.InternalPort)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("Internal HTTP server error: %v", err)
	}
}
