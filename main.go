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
	"potstack/internal/git"
	"potstack/internal/router"

	"github.com/gin-gonic/gin"
)

func main() {
	// 初始化日志
	if config.LogFile != "" {
		logDir := filepath.Dir(config.LogFile)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			log.Fatalf("Failed to create log directory %s: %v", logDir, err)
		}

		logFile, err := os.OpenFile(config.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
		if err != nil {
			log.Fatalf("Failed to open log file: %v", err)
		}
		// 诊断：直接将日志写入文件，而不是同时写入标准错误输出
		log.SetOutput(logFile)
		gin.DefaultWriter = logFile
		gin.DefaultErrorWriter = logFile
	}

	log.Println("Starting PotStack One...")

	// 1. 创建用于优雅退出的 Context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.Printf("PotStack listening on port: %s", config.HTTPPort)

	// 2. 启动 HTTP 服务 (Goroutine A)
	srvErrCh := make(chan error, 1)
	go func() {
		if err := runService(ctx); err != nil {
			srvErrCh <- err
		}
	}()

	// 3. 信号处理
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		log.Printf("Received signal: %v. Shutting down...\n", sig)
	case err := <-srvErrCh:
		log.Printf("Service error: %v. Shutting down...\n", err)
	}

	cancel()

	// 等待协程清理资源
	<-time.After(2 * time.Second)
	log.Println("Shutdown timeout (or explicit wait).")

	log.Println("PotStack exit.")
}

func runService(ctx context.Context) error {
	r := gin.Default()

	// API 路由组
	v1 := r.Group("/api/v1")
	{
		// 管理员用户管理 (受保护)
		admin := v1.Group("/admin")
		admin.Use(auth.TokenAuthMiddleware())
		{
			admin.POST("/users", api.CreateUserHandler)
			admin.DELETE("/users/:username", api.DeleteUserHandler)
			admin.POST("/users/:username/repos", api.CreateRepoHandler)
			admin.POST("/users/:username/orgs", api.CreateOrgHandler)
		}

		// 组织管理
		v1.DELETE("/orgs/:orgname", api.DeleteOrgHandler)

		// 仓库管理
		v1.GET("/repos/:owner/:repo", api.GetRepoHandler)
		v1.DELETE("/repos/:username/:reponame", api.DeleteRepoHandler)
	}

	// 统一资源路由
	r.GET("/uri/*path", auth.TokenAuthMiddleware(), router.ResourceProcessor())
	r.Any("/att/*path", auth.TokenAuthMiddleware(), router.ATTProcessor())
	r.GET("/cdn/*path", router.CDNProcessor()) // 公开
	r.Any("/web/*path", router.WebProcessor())

	// Git Smart HTTP 协议 (受保护)
	r.Any("/:owner/:reponame/*action", auth.TokenAuthMiddleware(), git.SmartHTTPServer())

	// 内部路由 (示例)
	r.GET("/health", api.HealthCheckHandler)

	srv := &http.Server{
		Addr:    ":" + config.HTTPPort,
		Handler: r,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	<-ctx.Done()
	log.Println("Stopping HTTP service...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	return srv.Shutdown(shutdownCtx)
}
