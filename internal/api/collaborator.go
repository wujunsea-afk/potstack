package api

import (
	"net/http"

	"potstack/internal/db"

	"github.com/gin-gonic/gin"
)

// AddCollaboratorOption 添加协作者请求参数
type AddCollaboratorOption struct {
	Permission string `json:"permission"` // read / write / admin
}

// ListCollaboratorsHandler 列出仓库的所有协作者
// GET /api/v1/repos/:owner/:repo/collaborators
func ListCollaboratorsHandler(c *gin.Context) {
	owner := c.Param("owner")
	repoName := c.Param("repo")

	// 获取仓库
	repo, err := db.GetRepositoryByOwnerAndName(owner, repoName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}
	if repo == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "repository not found"})
		return
	}

	// 获取协作者列表
	collaborators, err := db.GetCollaborators(repo.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get collaborators"})
		return
	}

	// 转换为响应格式
	var response []*db.CollaboratorResponse
	for _, collab := range collaborators {
		if resp := collab.ToResponse(); resp != nil {
			response = append(response, resp)
		}
	}

	if response == nil {
		response = []*db.CollaboratorResponse{}
	}

	c.JSON(http.StatusOK, response)
}

// CheckCollaboratorHandler 判断是否为协作者
// GET /api/v1/repos/:owner/:repo/collaborators/:collaborator
func CheckCollaboratorHandler(c *gin.Context) {
	owner := c.Param("owner")
	repoName := c.Param("repo")
	collaborator := c.Param("collaborator")

	// 获取仓库
	repo, err := db.GetRepositoryByOwnerAndName(owner, repoName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}
	if repo == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "repository not found"})
		return
	}

	// 获取用户
	user, err := db.GetUserByUsername(collaborator)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}
	if user == nil {
		c.Status(http.StatusNotFound)
		return
	}

	// 检查是否为协作者
	isCollab, err := db.IsCollaborator(repo.ID, user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}

	if isCollab {
		c.Status(http.StatusNoContent)
	} else {
		c.Status(http.StatusNotFound)
	}
}

// AddCollaboratorHandler 添加协作者
// PUT /api/v1/repos/:owner/:repo/collaborators/:collaborator
func AddCollaboratorHandler(c *gin.Context) {
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

	// 获取仓库
	repo, err := db.GetRepositoryByOwnerAndName(owner, repoName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}
	if repo == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "repository not found"})
		return
	}

	// 获取或创建用户
	user, err := db.GetOrCreateUser(collaborator, "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get or create user"})
		return
	}

	// 添加协作者
	if err := db.AddCollaborator(repo.ID, user.ID, opt.Permission); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to add collaborator"})
		return
	}

	c.Status(http.StatusNoContent)
}

// RemoveCollaboratorHandler 移除协作者
// DELETE /api/v1/repos/:owner/:repo/collaborators/:collaborator
func RemoveCollaboratorHandler(c *gin.Context) {
	owner := c.Param("owner")
	repoName := c.Param("repo")
	collaborator := c.Param("collaborator")

	// 获取仓库
	repo, err := db.GetRepositoryByOwnerAndName(owner, repoName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}
	if repo == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "repository not found"})
		return
	}

	// 获取用户
	user, err := db.GetUserByUsername(collaborator)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}
	if user == nil {
		c.Status(http.StatusNoContent)
		return
	}

	// 移除协作者
	if err := db.RemoveCollaborator(repo.ID, user.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to remove collaborator"})
		return
	}

	c.Status(http.StatusNoContent)
}
