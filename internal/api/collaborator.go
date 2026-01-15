package api

import (
	"errors"
	"net/http"

	"potstack/internal/service"

	"github.com/gin-gonic/gin"
)

// AddCollaboratorOption 添加协作者请求参数
type AddCollaboratorOption struct {
	Permission string `json:"permission"` // read / write / admin
}

// ListCollaboratorsHandler 列出仓库的所有协作者
func (s *Server) ListCollaboratorsHandler(c *gin.Context) {
	owner := c.Param("owner")
	repoName := c.Param("repo")

	collabs, err := s.repoService.ListCollaborators(c.Request.Context(), owner, repoName)
	if err != nil {
		if errors.Is(err, service.ErrRepoNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "repository not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, collabs)
}

// CheckCollaboratorHandler 判断是否为协作者
func (s *Server) CheckCollaboratorHandler(c *gin.Context) {
	owner := c.Param("owner")
	repoName := c.Param("repo")
	collaborator := c.Param("collaborator")

	isCollab, err := s.repoService.IsCollaborator(c.Request.Context(), owner, repoName, collaborator)
	if err != nil {
		if errors.Is(err, service.ErrRepoNotFound) { // IsCollaborator 内部调用了 GetRepo
			c.JSON(http.StatusNotFound, gin.H{"error": "repository not found"})
			return
		}
		// 用户未找到通常是 false，这里如果 IsCollaborator 返回 err 说明是系统错误
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if isCollab {
		c.Status(http.StatusNoContent)
	} else {
		c.Status(http.StatusNotFound)
	}
}

// AddCollaboratorHandler 添加协作者
func (s *Server) AddCollaboratorHandler(c *gin.Context) {
	owner := c.Param("owner")
	repoName := c.Param("repo")
	collaborator := c.Param("collaborator")

	// 解析请求参数
	var opt AddCollaboratorOption
	if err := c.ShouldBindJSON(&opt); err != nil {
		// 使用默认权限
		opt.Permission = "write"
	}

	// 验证权限值
	if opt.Permission == "" {
		opt.Permission = "write"
	}
	if opt.Permission != "read" && opt.Permission != "write" && opt.Permission != "admin" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid permission, must be read/write/admin"})
		return
	}

	if err := s.repoService.AddCollaborator(c.Request.Context(), owner, repoName, collaborator, opt.Permission); err != nil {
		if errors.Is(err, service.ErrRepoNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "repository not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.Status(http.StatusNoContent)
}

// RemoveCollaboratorHandler 移除协作者
func (s *Server) RemoveCollaboratorHandler(c *gin.Context) {
	owner := c.Param("owner")
	repoName := c.Param("repo")
	collaborator := c.Param("collaborator")

	if err := s.repoService.RemoveCollaborator(c.Request.Context(), owner, repoName, collaborator); err != nil {
		if errors.Is(err, service.ErrRepoNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "repository not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.Status(http.StatusNoContent)
}
