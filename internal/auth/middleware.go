package auth

import (
	"net/http"
	"strings"

	"potstack/config"

	"github.com/gin-gonic/gin"
)

// TokenAuthMiddleware 令牌认证中间件
// 支持两种认证方式：
// 1. Token 方式: Authorization: token <TOKEN>
// 2. Basic Auth 方式: Authorization: Basic base64(TOKEN:) 或 base64(:TOKEN)
func TokenAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if config.PotStackToken == "" {
			// 如果未配置令牌，允许所有请求（仅用于开发）
			c.Next()
			return
		}

		// 尝试 Token 方式认证
		authHeader := c.GetHeader("Authorization")
		if strings.HasPrefix(authHeader, "token ") {
			token := strings.TrimPrefix(authHeader, "token ")
			if token == config.PotStackToken {
				c.Next()
				return
			}
		}

		// 尝试 Basic Auth 方式认证
		user, password, hasAuth := c.Request.BasicAuth()
		if hasAuth && (user == config.PotStackToken || password == config.PotStackToken) {
			c.Next()
			return
		}

		// 认证失败
		c.Header("WWW-Authenticate", `Basic realm="PotStack"`)
		c.AbortWithStatus(http.StatusUnauthorized)
	}
}
