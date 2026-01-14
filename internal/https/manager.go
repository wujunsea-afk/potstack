package https

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"potstack/config"

	"golang.org/x/crypto/acme/autocert"
)

// Manager 证书管理器
type Manager struct {
	certFile string
	keyFile  string
	certsDir string

	mu   sync.RWMutex
	cert *tls.Certificate

	autocertManager *autocert.Manager
}

// NewManager 创建证书管理器
func NewManager() *Manager {
	return &Manager{
		certFile: config.CertFile,
		keyFile:  config.KeyFile,
		certsDir: config.CertsDir,
	}
}

// Setup 根据配置设置 TLS
// 返回 TLSConfig（如果启用 HTTPS）或 nil（HTTP 模式）
func (m *Manager) Setup() (*tls.Config, error) {
	cfg := Get()

	if cfg.Mode != "https" {
		log.Println("Mode: HTTP")
		return nil, nil
	}

	// 确保证书目录存在
	if err := os.MkdirAll(m.certsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create certs dir: %w", err)
	}

	// 检查现有证书
	if m.certValid() {
		log.Println("Using existing certificate")
		return m.loadCertConfig()
	}

	// 需要申请证书
	if !cfg.ACME.Enabled {
		return nil, fmt.Errorf("HTTPS enabled but no certificate and ACME disabled")
	}

	if cfg.ACME.Domain == "" {
		return nil, fmt.Errorf("HTTPS enabled but no certificate and ACME domain not set")
	}

	// 根据挑战类型设置
	switch cfg.ACME.Challenge {
	case "http-01":
		return m.setupHTTP01(cfg)
	case "dns-01":
		return m.setupDNS01(cfg)
	default:
		return nil, fmt.Errorf("unknown challenge type: %s", cfg.ACME.Challenge)
	}
}

// setupHTTP01 设置 HTTP-01 挑战
func (m *Manager) setupHTTP01(cfg *Config) (*tls.Config, error) {
	log.Printf("Setting up HTTP-01 challenge for domain: %s", cfg.ACME.Domain)

	m.autocertManager = &autocert.Manager{
		Prompt:      autocert.AcceptTOS,
		HostPolicy:  autocert.HostWhitelist(cfg.ACME.Domain),
		Cache:       autocert.DirCache(m.certsDir),
		Email:       cfg.ACME.Email,
		RenewBefore: time.Duration(cfg.ACME.RenewBeforeDays) * 24 * time.Hour,
	}

	// 启动 HTTP-01 挑战监听器
	port := cfg.ACME.HTTP.Port
	if port == 0 {
		port = 80
	}

	go func() {
		addr := fmt.Sprintf(":%d", port)
		log.Printf("Starting HTTP-01 challenge listener on %s", addr)
		if err := http.ListenAndServe(addr, m.autocertManager.HTTPHandler(nil)); err != nil {
			log.Printf("HTTP-01 listener error: %v", err)
		}
	}()

	return &tls.Config{
		GetCertificate: m.autocertManager.GetCertificate,
	}, nil
}

// setupDNS01 设置 DNS-01 挑战
func (m *Manager) setupDNS01(cfg *Config) (*tls.Config, error) {
	log.Printf("Setting up DNS-01 challenge for domain: %s", cfg.ACME.Domain)

	// 使用 ACME 客户端申请证书
	client := NewACMEClient(cfg, m.certsDir, m.certFile, m.keyFile)
	if err := client.ObtainCertificate(); err != nil {
		return nil, fmt.Errorf("failed to obtain certificate: %w", err)
	}

	// 加载新申请的证书
	return m.loadCertConfig()
}

// certValid 检查证书是否存在且有效
func (m *Manager) certValid() bool {
	if !fileExists(m.certFile) || !fileExists(m.keyFile) {
		return false
	}

	// 解析证书
	cert, err := m.parseCertFile()
	if err != nil {
		log.Printf("Failed to parse certificate: %v", err)
		return false
	}

	// 检查是否过期
	if time.Now().After(cert.NotAfter) {
		log.Println("Certificate has expired")
		return false
	}

	// 检查域名是否匹配配置
	cfg := Get()
	if cfg.ACME.Enabled && cfg.ACME.Domain != "" {
		if !m.certMatchesDomain(cert, cfg.ACME.Domain) {
			log.Printf("Certificate domain mismatch: cert=%v, config=%s", cert.DNSNames, cfg.ACME.Domain)
			return false
		}
	}

	// 检查是否即将过期（提前续签）
	renewBefore := time.Duration(cfg.ACME.RenewBeforeDays) * 24 * time.Hour
	if time.Until(cert.NotAfter) < renewBefore {
		log.Printf("Certificate expires in %d days, needs renewal", int(time.Until(cert.NotAfter).Hours()/24))
		// 返回 true 仍使用现有证书，但在后台触发续签
		go m.renewWithBackup()
	}

	return true
}

// parseCertFile 解析证书文件
func (m *Manager) parseCertFile() (*x509.Certificate, error) {
	certPEM, err := os.ReadFile(m.certFile)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM")
	}

	return x509.ParseCertificate(block.Bytes)
}

// certMatchesDomain 检查证书是否匹配指定域名
func (m *Manager) certMatchesDomain(cert *x509.Certificate, domain string) bool {
	// 检查 Common Name
	if cert.Subject.CommonName == domain {
		return true
	}
	// 检查 SAN (Subject Alternative Names)
	for _, san := range cert.DNSNames {
		if san == domain {
			return true
		}
	}
	return false
}

// renewInBackground 后台续签证书（无备份，用于定时检查）
func (m *Manager) renewInBackground() {
	m.doRenew(false)
}

// renewWithBackup 带备份的续签
func (m *Manager) renewWithBackup() {
	m.doRenew(true)
}

// ForceRenew 强制续签（API 调用）
func (m *Manager) ForceRenew() (string, error) {
	archiveDir, err := m.archiveCurrent()
	if err != nil {
		log.Printf("Warning: failed to archive current cert: %v", err)
	}

	cfg := Get()
	if !cfg.ACME.Enabled {
		return "", fmt.Errorf("ACME not enabled")
	}

	var renewErr error
	switch cfg.ACME.Challenge {
	case "dns-01":
		client := NewACMEClient(cfg, m.certsDir, m.certFile, m.keyFile)
		renewErr = client.ObtainCertificate()
	case "http-01":
		return "", fmt.Errorf("HTTP-01 renewal is handled automatically by autocert")
	default:
		return "", fmt.Errorf("unknown challenge type: %s", cfg.ACME.Challenge)
	}

	if renewErr != nil {
		return "", renewErr
	}

	// 热重载
	cert, err := tls.LoadX509KeyPair(m.certFile, m.keyFile)
	if err != nil {
		return "", err
	}

	m.mu.Lock()
	m.cert = &cert
	m.mu.Unlock()

	log.Println("Certificate force renewed successfully")
	return archiveDir, nil
}

// doRenew 执行续签
func (m *Manager) doRenew(withBackup bool) {
	cfg := Get()
	if !cfg.ACME.Enabled || cfg.ACME.Domain == "" {
		log.Println("ACME not enabled, skipping renewal")
		return
	}

	log.Println("Starting certificate renewal...")

	// 备份
	if withBackup {
		archiveDir, err := m.archiveCurrent()
		if err != nil {
			log.Printf("Warning: failed to archive current cert: %v", err)
		} else if archiveDir != "" {
			log.Printf("Archived current certificate to: %s", archiveDir)
		}
	}

	// 续签
	var err error
	switch cfg.ACME.Challenge {
	case "dns-01":
		client := NewACMEClient(cfg, m.certsDir, m.certFile, m.keyFile)
		err = client.ObtainCertificate()
	case "http-01":
		log.Println("HTTP-01 renewal is handled automatically by autocert")
		return
	default:
		log.Printf("Unknown challenge type for renewal: %s", cfg.ACME.Challenge)
		return
	}

	if err != nil {
		log.Printf("Renewal failed: %v", err)
		return
	}

	// 热重载
	cert, err := tls.LoadX509KeyPair(m.certFile, m.keyFile)
	if err != nil {
		log.Printf("Failed to reload renewed certificate: %v", err)
		return
	}

	m.mu.Lock()
	m.cert = &cert
	m.mu.Unlock()

	log.Println("Certificate renewed successfully")
}

// archiveCurrent 备份当前证书
func (m *Manager) archiveCurrent() (string, error) {
	if !fileExists(m.certFile) {
		return "", nil // 没有现有证书，无需备份
	}

	archiveDir := filepath.Join(m.certsDir, "archive", time.Now().Format("20060102-150405"))
	if err := os.MkdirAll(archiveDir, 0755); err != nil {
		return "", err
	}

	// 复制证书和私钥
	if err := copyFile(m.certFile, filepath.Join(archiveDir, "cert.pem")); err != nil {
		return "", err
	}
	if err := copyFile(m.keyFile, filepath.Join(archiveDir, "key.pem")); err != nil {
		return "", err
	}

	log.Printf("Archived certificate to: %s", archiveDir)
	return archiveDir, nil
}

// copyFile 复制文件
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0600)
}

// GetCertInfo 获取证书信息
func (m *Manager) GetCertInfo() (map[string]interface{}, error) {
	if !fileExists(m.certFile) {
		return nil, fmt.Errorf("certificate file not found")
	}

	cert, err := m.parseCertFile()
	if err != nil {
		return nil, err
	}

	cfg := Get()
	remaining := time.Until(cert.NotAfter)
	renewBefore := time.Duration(cfg.ACME.RenewBeforeDays) * 24 * time.Hour

	return map[string]interface{}{
		"domain":         cert.DNSNames,
		"issuer":         cert.Issuer.CommonName,
		"not_before":     cert.NotBefore.Format(time.RFC3339),
		"not_after":      cert.NotAfter.Format(time.RFC3339),
		"remaining_days": int(remaining.Hours() / 24),
		"needs_renewal":  remaining < renewBefore,
	}, nil
}

// loadCertConfig 加载证书配置
func (m *Manager) loadCertConfig() (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(m.certFile, m.keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load certificate: %w", err)
	}

	m.mu.Lock()
	m.cert = &cert
	m.mu.Unlock()

	return &tls.Config{
		GetCertificate: m.getCertificate,
	}, nil
}

// getCertificate 获取证书（支持热重载）
func (m *Manager) getCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.cert == nil {
		return nil, fmt.Errorf("no certificate loaded")
	}
	return m.cert, nil
}

// StartCertWatcher 启动证书文件监控
func (m *Manager) StartCertWatcher(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		var lastMod time.Time
		for range ticker.C {
			info, err := os.Stat(m.certFile)
			if err != nil {
				continue
			}
			if info.ModTime().After(lastMod) {
				lastMod = info.ModTime()
				cert, err := tls.LoadX509KeyPair(m.certFile, m.keyFile)
				if err != nil {
					log.Printf("Failed to reload certificate: %v", err)
					continue
				}
				m.mu.Lock()
				m.cert = &cert
				m.mu.Unlock()
				log.Println("Certificate reloaded")
			}
		}
	}()
}

// StartRenewalChecker 启动证书续签检查器
// checkInterval: 检查间隔（建议 12 小时）
func (m *Manager) StartRenewalChecker(checkInterval time.Duration) {
	go func() {
		// 首次延迟 1 分钟执行，避免启动时立即检查
		time.Sleep(1 * time.Minute)

		ticker := time.NewTicker(checkInterval)
		defer ticker.Stop()

		for {
			if m.needsRenewal() {
				log.Println("Certificate needs renewal, starting background renewal...")
				m.renewInBackground()
			}

			<-ticker.C
		}
	}()
	log.Printf("Certificate renewal checker started, interval: %v", checkInterval)
}

// needsRenewal 检查是否需要续签
func (m *Manager) needsRenewal() bool {
	cfg := Get()
	if !cfg.ACME.Enabled {
		return false // ACME 未启用，不需要自动续签
	}

	if !fileExists(m.certFile) {
		log.Println("Certificate file not found, renewal needed")
		return true
	}

	certPEM, err := os.ReadFile(m.certFile)
	if err != nil {
		log.Printf("Failed to read certificate: %v", err)
		return true
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		log.Println("Failed to decode certificate PEM")
		return true
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		log.Printf("Failed to parse certificate: %v", err)
		return true
	}

	// 计算剩余有效期
	remaining := time.Until(cert.NotAfter)
	remainingDays := int(remaining.Hours() / 24)

	// 检查是否已过期
	if remaining <= 0 {
		log.Printf("Certificate has expired")
		return true
	}

	// 检查是否在续签窗口内
	renewBefore := time.Duration(cfg.ACME.RenewBeforeDays) * 24 * time.Hour
	if remaining < renewBefore {
		log.Printf("Certificate expires in %d days (< %d days), renewal needed",
			remainingDays, cfg.ACME.RenewBeforeDays)
		return true
	}

	log.Printf("Certificate valid for %d more days, no renewal needed", remainingDays)
	return false
}

// GetTemplateFile 获取程序同目录的模板文件路径
func GetTemplateFile() string {
	exePath, err := os.Executable()
	if err != nil {
		return "https.yaml.example"
	}
	return filepath.Join(filepath.Dir(exePath), "https.yaml.example")
}
