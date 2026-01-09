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
)

func init() {
	RepoRoot = os.Getenv("POTSTACK_REPO_ROOT")
	if RepoRoot == "" {
		RepoRoot = "data"
	}

	HTTPPort = os.Getenv("POTSTACK_HTTP_PORT")
	if HTTPPort == "" {
		HTTPPort = "61080"
	}

	PotStackToken = os.Getenv("POTSTACK_TOKEN")

	LogFile = os.Getenv("POTSTACK_LOG_FILE")
	if LogFile == "" {
		LogFile = "./log/potstack.log"
	}
}
