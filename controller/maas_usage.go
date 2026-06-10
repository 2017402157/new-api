package controller

import (
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// GetMaasUsageStats 获取租户用量统计
func GetMaasUsageStats(c *gin.Context) {
	tenantIdStr := c.Query("tenant_id")
	if tenantIdStr == "" {
		// 尝试从 MaaS JWT context 获取
		tenantId, exists := c.Get("maas_tenant_id")
		if !exists {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "tenant_id is required",
			})
			return
		}
		tenantIdStr = strconv.FormatInt(tenantId.(int64), 10)
	}

	tenantId, err := strconv.ParseInt(tenantIdStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "invalid tenant_id",
		})
		return
	}

	date := c.DefaultQuery("date", time.Now().Format("2006-01-02"))

	stats, err := service.GetMaasUsageStats(tenantId, date)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}

// SetMaasTenantQuota 设置租户配额
func SetMaasTenantQuota(c *gin.Context) {
	var req struct {
		TenantId int64 `json:"tenant_id"`
		Quota    int   `json:"quota"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	if req.TenantId == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "tenant_id is required",
		})
		return
	}

	err := service.SetMaasTenantQuota(req.TenantId, req.Quota)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}
