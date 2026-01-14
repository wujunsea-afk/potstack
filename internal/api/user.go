package api

import (
	"net/http"
	"os"
	"path/filepath"

	"potstack/config"
	"potstack/internal/db"

	"github.com/gin-gonic/gin"
)

// CreateUserHandler 处理 POST /api/v1/admin/users 请求
func CreateUserHandler(c *gin.Context) {
	var opt CreateUserOption
	if err := c.ShouldBindJSON(&opt); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 检查用户是否已存在
	existing, err := db.GetUserByUsername(opt.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}
	if existing != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "user already exists"})
		return
	}

	// 创建用户目录
	userPath := filepath.Join(config.RepoDir, opt.Username)
	if err := os.MkdirAll(userPath, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user directory"})
		return
	}

	// 创建数据库记录
	user, err := db.CreateUser(opt.Username, opt.Email, "")
	if err != nil {
		// 回滚目录创建
		os.RemoveAll(userPath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
		return
	}

	c.JSON(http.StatusCreated, User{
		ID:       user.ID,
		Username: user.Username,
		Email:    user.Email,
	})
}

// DeleteUserHandler 处理 DELETE /api/v1/admin/users/:username 请求
func DeleteUserHandler(c *gin.Context) {
	username := c.Param("username")

	// 删除数据库记录（会级联删除相关仓库和协作者）
	if err := db.DeleteUser(username); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete user from database"})
		return
	}

	// 删除用户目录
	userPath := filepath.Join(config.RepoDir, username)
	if err := os.RemoveAll(userPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete user directory"})
		return
	}

	c.Status(http.StatusNoContent)
}
