package config

import (
	"os"
	"path/filepath"
)

var (
	// 核心配置（环境变量）
	RepoRoot      string // 数据根目录
	HTTPPort      string // 服务端口
	PotStackToken string // 鉴权令牌
)

// 派生路径（基于 RepoRoot）
var (
	LogFile     string // $REPO_ROOT/log/potstack.log
	CertsDir    string // $REPO_ROOT/certs/
	CertFile    string // $REPO_ROOT/certs/cert.pem
	KeyFile     string // $REPO_ROOT/certs/key.pem
	HTTPSConfig string // $REPO_ROOT/https.yaml
	RepoDir     string // $REPO_ROOT/repo/ (仓库根目录)
)

func init() {
	RepoRoot = getEnv("POTSTACK_REPO_ROOT", "data")
	HTTPPort = getEnv("POTSTACK_HTTP_PORT", "61080")
	PotStackToken = os.Getenv("POTSTACK_TOKEN")

	// 派生路径
	LogFile = filepath.Join(RepoRoot, "log", "potstack.log")
	CertsDir = filepath.Join(RepoRoot, "certs")
	CertFile = filepath.Join(CertsDir, "cert.pem")
	KeyFile = filepath.Join(CertsDir, "key.pem")
	HTTPSConfig = filepath.Join(RepoRoot, "https.yaml")
	RepoDir = filepath.Join(RepoRoot, "repo")
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
