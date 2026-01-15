package api

import (
	"errors"
	"net/http"

	"potstack/internal/service"

	"github.com/gin-gonic/gin"
)

// CreateUserHandler 处理 POST /api/v1/admin/users 请求
func (s *Server) CreateUserHandler(c *gin.Context) {
	var opt CreateUserOption
	if err := c.ShouldBindJSON(&opt); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 密码暂时留空或从 opt 获取（如果支持设置初始密码）
	user, err := s.userService.CreateUser(c.Request.Context(), opt.Username, opt.Email, "")
	if err != nil {
		if errors.Is(err, service.ErrUserAlreadyExists) {
			c.JSON(http.StatusConflict, gin.H{"error": "user already exists"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusCreated, User{
		ID:       user.ID,
		Username: user.Username,
		Email:    user.Email,
	})
}

// DeleteUserHandler 处理 DELETE /api/v1/admin/users/:username 请求
func (s *Server) DeleteUserHandler(c *gin.Context) {
	username := c.Param("username")

	if err := s.userService.DeleteUser(c.Request.Context(), username); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}
