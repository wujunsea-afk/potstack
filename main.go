package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
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
	"potstack/internal/loader"
	"potstack/internal/router"
	"potstack/internal/service"

	"github.com/gin-gonic/gin"
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

	// 创建用于优雅退出的 Context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动服务
	srvErrCh := make(chan error, 1)
	go func() {
		if err := runService(ctx, userService, repoService); err != nil {
			srvErrCh <- err
		}
	}()

	// 启动 Loader 初始化（异步，等待服务就绪）
	go initLoader(userService, repoService)

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

func runService(ctx context.Context, us service.IUserService, rs service.IRepoService) error {
	r := gin.Default()
	server := api.NewServer(us, rs)

	// API 路由组
	v1 := r.Group("/api/v1")
	{
		// 管理员接口 (受保护)
		admin := v1.Group("/admin")
		admin.Use(auth.TokenAuthMiddleware())
		{
			admin.POST("/users", server.CreateUserHandler)
			admin.DELETE("/users/:username", server.DeleteUserHandler)
			admin.POST("/users/:username/repos", server.CreateRepoHandler)

			// 证书管理
			admin.GET("/certs/info", api.CertInfoHandler)
			admin.POST("/certs/renew", api.CertRenewHandler)
		}

		// 仓库管理
		repos := v1.Group("/repos")
		repos.Use(auth.TokenAuthMiddleware())
		{
			repos.GET("/:owner/:repo", server.GetRepoHandler)
			repos.DELETE("/:owner/:repo", server.DeleteRepoHandler)

			// 协作者管理 (Gogs 兼容)
			repos.GET("/:owner/:repo/collaborators", server.ListCollaboratorsHandler)
			repos.GET("/:owner/:repo/collaborators/:collaborator", server.CheckCollaboratorHandler)
			repos.PUT("/:owner/:repo/collaborators/:collaborator", server.AddCollaboratorHandler)
			repos.DELETE("/:owner/:repo/collaborators/:collaborator", server.RemoveCollaboratorHandler)
		}
	}

	// 统一资源路由
	r.GET("/uri/*path", auth.TokenAuthMiddleware(), router.ResourceProcessor())
	r.Any("/att/*path", auth.TokenAuthMiddleware(), router.ATTProcessor())
	r.GET("/cdn/*path", router.CDNProcessor())
	r.Any("/web/*path", router.WebProcessor())

	// Git Smart HTTP 协议 (受保护)
	r.Any("/:owner/:reponame/*action", auth.TokenAuthMiddleware(), git.SmartHTTPServer())

	// 健康检查
	r.GET("/health", api.HealthCheckHandler)

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

// initLoader 初始化 Loader 模块
func initLoader(us service.IUserService, rs service.IRepoService) {
	// 构建服务 URL（内部通信）
	scheme := "http"
	if pothttps.IsHTTPS() {
		scheme = "https"
	}
	serviceURL := fmt.Sprintf("%s://localhost:%s", scheme, config.HTTPPort)

	// 自定义 HTTP Client 以跳过 TLS 验证（仅用于本地 Loader）
	httpClient := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	// 等待服务就绪 (Git Push 需要 HTTP 服务)
	// 注意：ACME 申请证书可能需要几分钟，这里需要足够的等待时间
	log.Println("Loader: waiting for service to be ready...")
	maxRetries := 600 // 10分钟
	for i := 0; i < maxRetries; i++ {
		resp, err := httpClient.Get(serviceURL + "/health")
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			log.Println("Loader: service is ready")
			break
		}

		if i%30 == 0 && i > 0 {
			log.Printf("Loader: still waiting for service... (%ds/%ds)", i, maxRetries)
		}
		time.Sleep(1 * time.Second)
	}

	// 确保基础包存在（尝试自动从程序目录复制）
	basePackPath := filepath.Join(config.DataDir, "potstack-base.zip")
	ensureBasePack(basePackPath)

	// 创建 Loader 配置
	loaderCfg := &loader.Config{
		PotStackURL:  serviceURL,
		Token:        config.PotStackToken,
		BasePackPath: basePackPath,
		HTTPClient:   httpClient, // 需要让 Loader 支持自定义 Client
	}

	// 执行初始化
	l := loader.New(loaderCfg, us, rs)
	if err := l.Initialize(); err != nil {
		log.Fatalf("Loader: initialization failed: %v", err)
	}

	log.Println("Loader: initialization completed")
}

// ensureBasePack 自动分发基础包
func ensureBasePack(targetPath string) {
	if _, err := os.Stat(targetPath); err == nil {
		return // 已存在
	}

	// 尝试寻找源文件 (程序同级目录)
	exePath, err := os.Executable()
	if err != nil {
		return
	}
	srcPath := filepath.Join(filepath.Dir(exePath), "potstack-base.zip")

	if _, err := os.Stat(srcPath); os.IsNotExist(err) {
		// log.Printf("Base pack source not found at %s, skipping auto-copy", srcPath)
		return
	}

	// 执行复制
	if err := copyFile(srcPath, targetPath); err != nil {
		log.Printf("Warning: failed to copy base pack: %v", err)
	} else {
		log.Printf("Auto-deployed base pack to %s", targetPath)
	}
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return err
	}
	// 确保写入磁盘
	return destFile.Sync()
}
