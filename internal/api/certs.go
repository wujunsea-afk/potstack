package api

import (
	"net/http"

	"potstack/internal/https"

	"github.com/gin-gonic/gin"
)

// CertInfoHandler 获取证书信息
// GET /api/v1/admin/certs/info
func CertInfoHandler(c *gin.Context) {
	manager := https.NewManager()
	info, err := manager.GetCertInfo()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, info)
}

// CertRenewHandler 强制续签证书
// POST /api/v1/admin/certs/renew
func CertRenewHandler(c *gin.Context) {
	manager := https.NewManager()
	archiveDir, err := manager.ForceRenew()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	response := gin.H{
		"success": true,
		"message": "Certificate renewed successfully",
	}
	if archiveDir != "" {
		response["archived_to"] = archiveDir
	}

	c.JSON(http.StatusOK, response)
}
