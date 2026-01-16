package git

import (
	"io"
	"path/filepath"

	gitlib "github.com/go-git/go-git/v5"
	"gopkg.in/yaml.v3"
)

// ReadFileFromHead 从 Git 仓库 HEAD 读取文件内容
func ReadFileFromHead(bareRepoPath, filePath string) ([]byte, error) {
	r, err := gitlib.PlainOpen(bareRepoPath)
	if err != nil {
		return nil, err
	}

	headRef, err := r.Head()
	if err != nil {
		return nil, err
	}

	commit, err := r.CommitObject(headRef.Hash())
	if err != nil {
		return nil, err
	}

	tree, err := commit.Tree()
	if err != nil {
		return nil, err
	}

	file, err := tree.File(filePath)
	if err != nil {
		return nil, err
	}

	reader, err := file.Reader()
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	return io.ReadAll(reader)
}

// ReadPotYml 从 Git 仓库根目录读取 pot.yml
func ReadPotYml(repoRoot, org, name string, cfg interface{}) error {
	bareRepoPath := filepath.Join(repoRoot, org, name+".git")
	data, err := ReadFileFromHead(bareRepoPath, "pot.yml")
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, cfg)
}
