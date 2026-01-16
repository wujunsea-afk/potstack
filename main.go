package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"potstack/config"
	"potstack/internal/api"
	"potstack/internal/auth"
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
	r := gin.Default()
	_ = api.NewServer(us, rs) // 保留以备后用

	// CDN 静态资源
	r.GET("/cdn/*path", resource.CDNProcessor())

	// ⚠️ 重要：特殊路径必须在动态路由前注册
	r.POST("/pot/potstack/router/refresh", router.RefreshHandler(dynamicRouter))

	// 动态路由：/pot/{org}/{name}/* -> 去掉 /pot/{org}/{name}
	r.Any("/pot/:org/:name/*path", func(c *gin.Context) {
		dynamicRouter.ServeHTTP(c.Writer, c.Request)
	})

	// 动态路由：/api/{org}/{name}/* -> 去掉 /{org}/{name}
	r.Any("/api/:org/:name/*path", func(c *gin.Context) {
		dynamicRouter.ServeHTTP(c.Writer, c.Request)
	})

	// 动态路由：/web/{org}/{name}/* -> 去掉 /{org}/{name}
	r.Any("/web/:org/:name/*path", func(c *gin.Context) {
		dynamicRouter.ServeHTTP(c.Writer, c.Request)
	})

	// Git Smart HTTP 协议 (受保护)
	r.Any("/repo/:owner/:reponame/*action", auth.TokenAuthMiddleware(), git.SmartHTTPServer())

	// 健康检查
	r.GET("/health", api.HealthCheckHandler)

	// 未注册路由走动态路由器
	r.NoRoute(func(c *gin.Context) {
		dynamicRouter.ServeHTTP(c.Writer, c.Request)
	})

	// 设置 TLS
	certManager := pothttps.NewManager()
	tlsConfig, err := certManager.Setup()
	if err != nil {
		log.Printf("TLS setup failed: %v, falling back to HTTP", err)
		tlsConfig = nil
	}

	var srv *http.Server

	if tlsConfig != nil {
		// HTTPS 模式
		srv = &http.Server{
			Addr:      ":" + config.HTTPPort,
			Handler:   r,
			TLSConfig: tlsConfig,
		}
		log.Printf("HTTPS listening on :%s", config.HTTPPort)

		// 启动证书热重载
		certManager.StartCertWatcher(30 * time.Second)

		// 启动定时续签检查（每 12 小时检查一次）
		// certManager.StartRenewalChecker(12 * time.Hour)
		certManager.StartRenewalChecker(1 * time.Minute) // 测试用

		go func() {
			if err := srv.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
				log.Fatalf("HTTPS server error: %v", err)
			}
		}()
	} else {
		// HTTP 模式
		srv = &http.Server{
			Addr:    ":" + config.HTTPPort,
			Handler: r,
		}
		log.Printf("HTTP listening on :%s", config.HTTPPort)

		go func() {
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("HTTP server error: %v", err)
			}
		}()
	}

	<-ctx.Done()
	log.Println("Stopping service...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	return srv.Shutdown(shutdownCtx)
}
