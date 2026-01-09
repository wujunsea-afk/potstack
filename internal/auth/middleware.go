package auth

import (
	"net/http"

	"potstack/config"

	"github.com/gin-gonic/gin"
)

// TokenAuthMiddleware 使用指定的令牌强制执行 Basic 认证
func TokenAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if config.PotStackToken == "" {
			// 如果未配置令牌，为了演示目的允许所有请求（但会发出警告）
			c.Next()
			return
		}

		user, password, hasAuth := c.Request.BasicAuth()
		// 验证令牌（作为用户名或密码均可接受）
		if !hasAuth || (user != config.PotStackToken && password != config.PotStackToken) {
			c.Header("WWW-Authenticate", `Basic realm="PotStack"`)
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		c.Next()
	}
}
