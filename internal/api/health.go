package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HealthCheckHandler 返回服务的健康状态
func HealthCheckHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "UP",
		"service": "potstack",
	})
}
