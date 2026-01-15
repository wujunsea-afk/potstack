package config

import (
	"os"
	"path/filepath"
)

var (
	// 核心配置（环境变量）
	DataDir       string // 数据根目录
	HTTPPort      string // 服务端口
	PotStackToken string // 鉴权令牌
)

// 派生路径（基于 DataDir）
var (
	LogFile     string // $DATA_DIR/log/potstack.log
	CertsDir    string // $DATA_DIR/certs/
	CertFile    string // $DATA_DIR/certs/cert.pem
	KeyFile     string // $DATA_DIR/certs/key.pem
	HTTPSConfig string // $DATA_DIR/https.yaml
	RepoDir     string // $DATA_DIR/repo/ (仓库根目录)
)

func init() {
	DataDir = getEnv("POTSTACK_DATA_DIR", "data")
	HTTPPort = getEnv("POTSTACK_HTTP_PORT", "61080")
	PotStackToken = os.Getenv("POTSTACK_TOKEN")

	// 派生路径
	LogFile = filepath.Join(DataDir, "log", "potstack.log")
	CertsDir = filepath.Join(DataDir, "certs")
	CertFile = filepath.Join(CertsDir, "cert.pem")
	KeyFile = filepath.Join(CertsDir, "key.pem")
	HTTPSConfig = filepath.Join(DataDir, "https.yaml")
	RepoDir = filepath.Join(DataDir, "repo")
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
