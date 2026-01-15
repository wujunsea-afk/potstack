package https

import (
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Config HTTPS 配置结构
type Config struct {
	Mode string     `yaml:"mode"` // http / https
	ACME ACMEConfig `yaml:"acme"`
}

// ACMEConfig ACME 自动证书配置
type ACMEConfig struct {
	Enabled           bool          `yaml:"enabled"`
	Domain            string        `yaml:"domain"`
	Challenge         string        `yaml:"challenge"` // http-01 / dns-01
	HTTP              HTTPChallenge `yaml:"http"`
	DNS               DNSChallenge  `yaml:"dns"`
	Directories       []string      `yaml:"directories"`
	RetryCount        int           `yaml:"retry_count"`
	RetryDelaySeconds int           `yaml:"retry_delay_seconds"`
	RenewBeforeDays   int           `yaml:"renew_before_days"`
	Email             string        `yaml:"email"`
}

// HTTPChallenge HTTP-01 挑战配置
type HTTPChallenge struct {
	Port int `yaml:"port"`
}

// DNSChallenge DNS-01 挑战配置
type DNSChallenge struct {
	Provider    string            `yaml:"provider"`
	Credentials map[string]string `yaml:"credentials"`
}

var (
	current    *Config
	mu         sync.RWMutex
	configPath string
	lastMod    time.Time
)

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Mode: "http",
		ACME: ACMEConfig{
			Enabled:           true,
			Domain:            "",
			Challenge:         "dns-01",
			HTTP:              HTTPChallenge{Port: 80},
			DNS:               DNSChallenge{Provider: "dnspod", Credentials: make(map[string]string)},
			Directories:       []string{"https://acme-v02.api.letsencrypt.org/directory"},
			RetryCount:        3,
			RetryDelaySeconds: 5,
			RenewBeforeDays:   30,
			Email:             "",
		},
	}
}

// Init 初始化 HTTPS 配置
// configFile: $DATA_DIR/https.yaml 的路径
// templateFile: 程序同目录的 https.yaml 模板路径
func Init(configFile, templateFile string) error {
	configPath = configFile

	// 如果配置文件不存在，从模板复制
	if !fileExists(configFile) {
		if err := copyTemplate(templateFile, configFile); err != nil {
			log.Printf("Warning: failed to copy template: %v, using defaults", err)
		}
	}

	return reload()
}

// copyTemplate 从模板复制配置文件
func copyTemplate(src, dst string) error {
	// 确保目标目录存在
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	if !fileExists(src) {
		// 模板不存在，创建默认配置
		return saveDefault(dst)
	}

	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	if err := os.WriteFile(dst, data, 0644); err != nil {
		return err
	}

	log.Printf("Copied config template to %s", dst)
	return nil
}

// saveDefault 保存默认配置到文件
func saveDefault(path string) error {
	cfg := DefaultConfig()
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	header := []byte(`# PotStack HTTPS 配置
# 修改后自动生效（约 30 秒）

`)
	if err := os.WriteFile(path, append(header, data...), 0644); err != nil {
		return err
	}

	log.Printf("Created default config: %s", path)
	return nil
}

// reload 重新加载配置
func reload() error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			mu.Lock()
			current = DefaultConfig()
			mu.Unlock()
			return nil
		}
		return err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return err
	}

	// 设置默认值
	if cfg.ACME.Challenge == "" {
		cfg.ACME.Challenge = "dns-01"
	}
	if cfg.ACME.RetryCount == 0 {
		cfg.ACME.RetryCount = 3
	}
	if cfg.ACME.RetryDelaySeconds == 0 {
		cfg.ACME.RetryDelaySeconds = 5
	}
	if cfg.ACME.RenewBeforeDays == 0 {
		cfg.ACME.RenewBeforeDays = 30
	}
	if len(cfg.ACME.Directories) == 0 {
		cfg.ACME.Directories = []string{"https://acme-v02.api.letsencrypt.org/directory"}
	}

	mu.Lock()
	current = &cfg
	mu.Unlock()

	log.Printf("HTTPS config loaded: mode=%s", cfg.Mode)
	return nil
}

// Get 获取当前配置（线程安全）
func Get() *Config {
	mu.RLock()
	defer mu.RUnlock()
	if current == nil {
		return DefaultConfig()
	}
	return current
}

// StartWatcher 启动配置文件监控，自动热重载
func StartWatcher(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			info, err := os.Stat(configPath)
			if err != nil {
				continue
			}
			if info.ModTime().After(lastMod) {
				lastMod = info.ModTime()
				if err := reload(); err != nil {
					log.Printf("Failed to reload HTTPS config: %v", err)
				} else {
					log.Println("HTTPS config reloaded")
				}
			}
		}
	}()
}

// IsHTTPS 返回是否启用 HTTPS
func IsHTTPS() bool {
	cfg := Get()
	return cfg.Mode == "https"
}

// NeedAutoCert 返回是否需要自动申请证书
func NeedAutoCert() bool {
	cfg := Get()
	return cfg.Mode == "https" && cfg.ACME.Enabled && cfg.ACME.Domain != ""
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
