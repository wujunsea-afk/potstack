package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"potstack/config"
	"potstack/internal/git"

	"github.com/gin-gonic/gin"
)

// CreateRepoHandler 处理 POST /api/v1/admin/users/:username/repos 请求
func CreateRepoHandler(c *gin.Context) {
	username := c.Param("username")
	var opt CreateRepoOption
	if err := c.ShouldBindJSON(&opt); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	repoPath := filepath.Join(config.RepoRoot, username, opt.Name+".git")

	// 如果目录不存在，创建父目录
	if err := os.MkdirAll(filepath.Dir(repoPath), 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user directory"})
		return
	}

	uuid, err := git.InitBare(repoPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	repo := Repository{
		Name:     opt.Name,
		FullName: username + "/" + opt.Name,
		Owner: &User{
			Username: username,
		},
		UUID:     uuid,
		CloneURL: fmt.Sprintf("http://%s/%s/%s.git", c.Request.Host, username, opt.Name),
	}

	c.JSON(http.StatusCreated, repo)
}

// DeleteRepoHandler 处理 DELETE /api/v1/repos/:username/:reponame 请求
func DeleteRepoHandler(c *gin.Context) {
	username := c.Param("username")
	reponame := c.Param("reponame")

	repoPath := filepath.Join(config.RepoRoot, username, reponame+".git")

	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		c.Status(http.StatusNoContent)
		return
	}

	if err := os.RemoveAll(repoPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete repository"})
		return
	}

	c.Status(http.StatusNoContent)
}

// GetRepoHandler 处理 GET /api/v1/repos/:owner/:repo 请求
func GetRepoHandler(c *gin.Context) {
	owner := c.Param("owner")
	repoName := c.Param("repo")

	repoPath := filepath.Join(config.RepoRoot, owner, repoName+".git")

	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "repository not found"})
		return
	}

	// 读取 UUID
	uuidBytes, _ := os.ReadFile(filepath.Join(repoPath, "uuid"))
	uuid := string(uuidBytes)

	repo := Repository{
		Name:     repoName,
		FullName: owner + "/" + repoName,
		Owner: &User{
			Username: owner,
		},
		UUID:     uuid,
		CloneURL: fmt.Sprintf("http://%s/%s/%s.git", c.Request.Host, owner, repoName),
	}

	c.JSON(http.StatusOK, repo)
}
