package api

import (
	"errors"
	"fmt"
	"net/http"

	"potstack/internal/service"

	"github.com/gin-gonic/gin"
)

// CreateRepoHandler 处理 POST /api/v1/admin/users/:username/repos 请求
func (s *Server) CreateRepoHandler(c *gin.Context) {
	username := c.Param("username")
	var opt CreateRepoOption
	if err := c.ShouldBindJSON(&opt); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	repo, err := s.repoService.CreateRepo(c.Request.Context(), username, opt.Name)
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		} else if errors.Is(err, service.ErrRepoAlreadyExists) {
			c.JSON(http.StatusConflict, gin.H{"error": "repository already exists"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
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
func (s *Server) DeleteRepoHandler(c *gin.Context) {
	owner := c.Param("owner")
	repoName := c.Param("repo")

	if err := s.repoService.DeleteRepo(c.Request.Context(), owner, repoName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// GetRepoHandler 处理 GET /api/v1/repos/:owner/:repo 请求
func (s *Server) GetRepoHandler(c *gin.Context) {
	owner := c.Param("owner")
	repoName := c.Param("repo")

	repo, err := s.repoService.GetRepo(c.Request.Context(), owner, repoName)
	if err != nil {
		if errors.Is(err, service.ErrRepoNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "repository not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
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
