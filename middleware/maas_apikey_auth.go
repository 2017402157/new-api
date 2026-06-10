package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

const (
	maaSKeyPrefix = "maas-"
	headerMaasKey  = "X-Maas-Key" // Alternative header for API key
)

// MaasApiKeyAuth MaaS 开发者 API Key 认证中间件
// 识别 Authorization: Bearer maas-xxxx 或 X-Maas-Key: maas-xxxx
// 如果是 MaaS Key，验证并设置 context；否则回退到 MaasJwtAuth/TokenAuth
func MaasApiKeyAuth() func(c *gin.Context) {
	return func(c *gin.Context) {
		// 1. 从 Authorization 或 X-Maas-Key 获取 key
		keyStr := extractMaasKey(c)
		if keyStr == "" {
			// 不是 MaaS Key，回退到 MaasJwtAuth
			MaasJwtAuth()(c)
			return
		}

		// 2. 提取 key 前缀用于查找
		keyPrefix := keyStr
		if len(keyStr) > 12 {
			keyPrefix = keyStr[:12]
		}

		if !strings.HasPrefix(keyPrefix, maaSKeyPrefix) {
			// 不是 MaaS Key 格式，回退
			MaasJwtAuth()(c)
			return
		}

		// 3. 验证 key（通过 yudao-cloud maas-server 查询）
		// 在 MVP 阶段，通过内部 API 验证；后续通过 Redis 缓存
		valid, tenantId, userId, modelWhitelist, err := service.VerifyMaasApiKey(keyStr)
		if err != nil || !valid {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"message": "Invalid MaaS API Key",
					"type":    "maas_auth_error",
				},
			})
			c.Abort()
			return
		}

		// 4. 设置 context
		c.Set("id", userId)
		c.Set("username", fmt.Sprintf("maas_key_%s", keyPrefix))
		c.Set("maas_tenant_id", tenantId)
		c.Set("maas_api_key", keyPrefix)
		c.Set("maas_model_whitelist", modelWhitelist)
		c.Set(string(constant.ContextKeyTokenGroup), "maas")
		c.Set(string(constant.ContextKeyUserGroup), "maas")
		c.Set("token_name", keyPrefix)
		c.Set("token_unlimited_quota", false)

		// 5. 检查模型白名单（如果有）
		if modelWhitelist != "" && modelWhitelist != "[]" {
			requestModel := common.GetContextKeyString(c, constant.ContextKeyOriginalModel)
			if requestModel != "" && !isModelAllowed(requestModel, modelWhitelist) {
				c.JSON(http.StatusForbidden, gin.H{
					"error": gin.H{
						"message": fmt.Sprintf("Model '%s' not allowed for this API Key", requestModel),
						"type":    "maas_model_forbidden",
					},
				})
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

func extractMaasKey(c *gin.Context) string {
	// Check X-Maas-Key header first
	key := c.GetHeader(headerMaasKey)
	if key != "" && strings.HasPrefix(key, maaSKeyPrefix) {
		return key
	}

	// Check Authorization: Bearer maas-xxxx
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
		bearer := strings.TrimPrefix(authHeader, "Bearer ")
		if strings.HasPrefix(bearer, maaSKeyPrefix) {
			return bearer
		}
	}

	return ""
}

func isModelAllowed(model string, whitelist string) bool {
	// Parse whitelist as JSON array and check if model is included
	// Simple check: if whitelist contains the model name substring
	return strings.Contains(whitelist, model)
}