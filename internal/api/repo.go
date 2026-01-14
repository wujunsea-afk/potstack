package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"potstack/config"
	"potstack/internal/db"
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

	// 获取用户
	user, err := db.GetUserByUsername(username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}
	if user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	// 检查仓库是否已存在
	existing, err := db.GetRepositoryByOwnerAndName(username, opt.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}
	if existing != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "repository already exists"})
		return
	}

	repoPath := filepath.Join(config.RepoDir, username, opt.Name+".git")

	// 创建父目录
	if err := os.MkdirAll(filepath.Dir(repoPath), 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create directory"})
		return
	}

	// 初始化 Git 仓库
	uuid, err := git.InitBare(repoPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 创建数据库记录
	repo, err := db.CreateRepository(user.ID, opt.Name, opt.Description, uuid)
	if err != nil {
		// 回滚
		os.RemoveAll(repoPath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create repository"})
		return
	}

	// 设置 CloneURL
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	repo.CloneURL = fmt.Sprintf("%s://%s/%s/%s.git", scheme, c.Request.Host, username, opt.Name)

	c.JSON(http.StatusCreated, repo)
}

// DeleteRepoHandler 处理 DELETE /api/v1/repos/:owner/:repo 请求
func DeleteRepoHandler(c *gin.Context) {
	owner := c.Param("owner")
	repoName := c.Param("repo")

	// 删除数据库记录
	if err := db.DeleteRepository(owner, repoName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete from database"})
		return
	}

	// 删除仓库目录
	repoPath := filepath.Join(config.RepoDir, owner, repoName+".git")
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

	// 从数据库获取
	repo, err := db.GetRepositoryByOwnerAndName(owner, repoName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}
	if repo == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "repository not found"})
		return
	}

	// 设置 CloneURL
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	repo.CloneURL = fmt.Sprintf("%s://%s/%s/%s.git", scheme, c.Request.Host, owner, repoName)

	c.JSON(http.StatusOK, repo)
}
