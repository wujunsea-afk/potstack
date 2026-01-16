package router

import (
	"net/http"

	"potstack/config"
	"potstack/internal/git"
	"potstack/internal/models"

	"github.com/gin-gonic/gin"
)

// RefreshHandler 刷新路由接口处理器
func RefreshHandler(dynamicRouter *Router) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Org  string `json:"org" binding:"required"`
			Name string `json:"name" binding:"required"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		// 1. 从 Git 读取 pot.yml
		var potCfg models.PotConfig
		if err := git.ReadPotYml(config.RepoDir, req.Org, req.Name, &potCfg); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "pot.yml not found"})
			return
		}

		// 2. 根据类型注册路由
		if potCfg.Type == "static" {
			// Static 类型直接注册
			if err := dynamicRouter.RegisterStatic(req.Org, req.Name, &potCfg); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		} else if potCfg.Type == "exe" {
			// Exe 类型需要检查运行状态
			if err := dynamicRouter.RegisterExe(req.Org, req.Name); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported pot type"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "ok", "org": req.Org, "name": req.Name})
	}
}
