package loader

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"potstack/config"
	"potstack/internal/keeper"
	"potstack/internal/service"
)

// StartAsync 异步启动 Loader 初始化流程
// 返回 channel，完成时关闭
func StartAsync(us service.IUserService, rs service.IRepoService, sm *keeper.SandboxManager) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		l := initLoader(us, rs)
		sm.SetPotProvider(l)
		log.Println("Loader: initialization completed")
		close(done)
	}()
	return done
}

func initLoader(us service.IUserService, rs service.IRepoService) *Loader {
	// 构建服务 URL（使用内部端口，无需 HTTPS）
	serviceURL := fmt.Sprintf("http://localhost:%s", config.InternalPort)

	// HTTP Client（跳过 TLS 验证，保留以防未来需要）
	httpClient := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	// 等待服务就绪
	waitForService(httpClient, serviceURL)

	// 确保基础包存在
	basePackPath := filepath.Join(config.DataDir, "potstack-base.zip")
	ensureBasePack(basePackPath)

	// 创建并初始化 Loader
	cfg := &Config{
		PotStackURL:  serviceURL,
		Token:        config.PotStackToken,
		BasePackPath: basePackPath,
		HTTPClient:   httpClient,
	}

	l := New(cfg, us, rs)
	if err := l.Initialize(); err != nil {
		log.Fatalf("Loader: initialization failed: %v", err)
	}

	return l
}

func waitForService(client *http.Client, serviceURL string) {
	log.Println("Loader: waiting for service to be ready...")
	maxRetries := 600
	for i := 0; i < maxRetries; i++ {
		resp, err := client.Get(serviceURL + "/health")
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			log.Println("Loader: service is ready")
			return
		}
		if i%30 == 0 && i > 0 {
			log.Printf("Loader: still waiting... (%ds/%ds)", i, maxRetries)
		}
		time.Sleep(1 * time.Second)
	}
}

func ensureBasePack(targetPath string) {
	if _, err := os.Stat(targetPath); err == nil {
		return
	}

	exePath, err := os.Executable()
	if err != nil {
		return
	}
	srcPath := filepath.Join(filepath.Dir(exePath), "potstack-base.zip")

	if _, err := os.Stat(srcPath); os.IsNotExist(err) {
		return
	}

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
	return destFile.Sync()
}
