package loader

import (
	"os"
	"path/filepath"
	"strings"

	"potstack/config"
	"potstack/internal/keeper"
)

// GetInstalledPots 实现 keeper.PotProvider 接口
// 遍历仓库目录，返回所有已安装的 Pot
func (l *Loader) GetInstalledPots() []keeper.PotURI {
	var list []keeper.PotURI

	entries, err := os.ReadDir(config.RepoDir)
	if err != nil {
		return list
	}

	for _, orgDir := range entries {
		if !orgDir.IsDir() {
			continue
		}
		orgPath := filepath.Join(config.RepoDir, orgDir.Name())
		repos, err := os.ReadDir(orgPath)
		if err != nil {
			continue
		}

		for _, repoDir := range repos {
			if !repoDir.IsDir() || !strings.HasSuffix(repoDir.Name(), ".git") {
				continue
			}
			name := strings.TrimSuffix(repoDir.Name(), ".git")
			list = append(list, keeper.PotURI{
				Org:  orgDir.Name(),
				Name: name,
			})
		}
	}
	return list
}
