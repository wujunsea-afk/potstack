package config

import (
	"os"
)

var (
	// RepoRoot is the physical root directory for repositories
	RepoRoot string

	HTTPPort string

	// PotStackToken is the required token for authenticating administrative and git requests
	PotStackToken string

	// LogFile path for logging
	LogFile string
	
	// EnableHTTPS controls whether HTTPS is enabled
	EnableHTTPS bool
	
	// CertFile specifies the path to the TLS certificate file
	CertFile string
	
	// KeyFile specifies the path to the TLS private key file
	KeyFile string
)

func init() {
	RepoRoot = getEnv("POTSTACK_REPO_ROOT", "data")
	HTTPPort = getEnv("POTSTACK_HTTP_PORT", "61080")
	PotStackToken = os.Getenv("POTSTACK_TOKEN")
	LogFile = getEnv("POTSTACK_LOG_FILE", "./log/potstack.log")
	
	// HTTPS Configuration
	EnableHTTPS = getEnv("POTSTACK_ENABLE_HTTPS", "false") == "true"
	CertFile = getEnv("POTSTACK_CERT_FILE", "./cert.pem")
	KeyFile = getEnv("POTSTACK_KEY_FILE", "./key.pem")
}

// getEnv fetches an environment variable or returns a default value.
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

