package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"

	"github.com/gin-gonic/gin"
)

// MaasTenantRateLimit MaaS 租户级限流中间件
// 在 MaasJwtAuth 之后使用，按 tenant_id 限流
func MaasTenantRateLimit() func(c *gin.Context) {
	return func(c *gin.Context) {
		tenantIdVal, exists := c.Get("maas_tenant_id")
		if !exists {
			c.Next()
			return
		}

		tenantId, ok := tenantIdVal.(int64)
		if !ok || tenantId == 0 {
			c.Next()
			return
		}

		if !common.RedisEnabled {
			c.Next()
			return
		}

		ctx := context.Background()
		rdb := common.RDB
		key := fmt.Sprintf("maas:ratelimit:%s", strconv.FormatInt(tenantId, 10))

		count, err := rdb.Incr(ctx, key).Result()
		if err != nil {
			c.Next()
			return
		}

		if count == 1 {
			rdb.Expire(ctx, key, time.Minute)
		}

		maxRPM := common.GetEnvOrDefault("MAAS_TENANT_MAX_RPM", 120)
		if count > int64(maxRPM) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": gin.H{
					"message": fmt.Sprintf("Tenant rate limit exceeded: %d requests per minute", maxRPM),
					"type":    "maas_rate_limit_error",
				},
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
