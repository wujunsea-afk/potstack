package api

import (
	"net/http"
	"os"
	"path/filepath"

	"potstack/config"

	"github.com/gin-gonic/gin"
)

// CreateUserHandler 处理 POST /api/v1/admin/users 请求
func CreateUserHandler(c *gin.Context) {
	var opt CreateUserOption
	if err := c.ShouldBindJSON(&opt); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userPath := filepath.Join(config.RepoRoot, opt.Username)
	if err := os.MkdirAll(userPath, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user directory"})
		return
	}

	c.JSON(http.StatusCreated, User{
		Username: opt.Username,
		Email:    opt.Email,
	})
}

// DeleteUserHandler 处理 DELETE /api/v1/admin/users/:username 请求
func DeleteUserHandler(c *gin.Context) {
	username := c.Param("username")
	userPath := filepath.Join(config.RepoRoot, username)

	if err := os.RemoveAll(userPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete user"})
		return
	}

	c.Status(http.StatusNoContent)
}

// CreateOrgHandler 处理 POST /api/v1/admin/users/:owner/orgs 请求
func CreateOrgHandler(c *gin.Context) {
	var opt struct {
		Username string `json:"username" binding:"required"`
	}
	if err := c.ShouldBindJSON(&opt); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 在 Zero-DB 模式下，组织只是仓库根目录下的另一个目录。
	// 根据 Gogs 规范，组织通常是顶级实体，但由管理员创建。

	orgPath := filepath.Join(config.RepoRoot, opt.Username)
	if err := os.MkdirAll(orgPath, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create org directory"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"username": opt.Username,
	})
}

// DeleteOrgHandler 处理 DELETE /api/v1/orgs/:orgname 请求
func DeleteOrgHandler(c *gin.Context) {
	orgname := c.Param("orgname")
	orgPath := filepath.Join(config.RepoRoot, orgname)

	if err := os.RemoveAll(orgPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete org"})
		return
	}

	c.Status(http.StatusNoContent)
}
