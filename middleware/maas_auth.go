package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const (
	// yudao-cloud gateway 签发 JWT 后注入的 header 名
	headerMaasJWT = "X-Maas-JWT"

	// 环境变量：JWT 验证密钥（HS256，与 yudao-cloud gateway 的 maas.jwt.secret 一致）
	envMaasJwtSecret = "MAAS_JWT_SECRET"

	// 环境变量：系统级 API Key（sk-xxx），用于 MaaS JWT 认证时查找对应的 new-api Token
	// 在 MVP 阶段，所有通过网关 JWT 认证的请求共用此系统 Token
	envMaasSystemTokenKey = "MAAS_SYSTEM_TOKEN_KEY"
)

var (
	maasJwtSecret     []byte
	maasSystemTokenKey string
	maasJwtInitialized bool
)

func initMaasJwtConfig() {
	secret := common.GetEnvOrDefaultString(envMaasJwtSecret, "")
	if len(secret) < 32 {
		// HMAC-SHA256 要求至少 256 位 (32 bytes)
		common.SysError("MAAS_JWT_SECRET 未配置或过短（至少 32 字符），MaaS JWT 认证将不可用")
		return
	}
	maasJwtSecret = []byte(secret)
	maasSystemTokenKey = common.GetEnvOrDefaultString(envMaasSystemTokenKey, "")
	maasJwtInitialized = true
}

// MaasJwtAuth 是 MaaS JWT 认证中间件。
// 检测 X-Maas-JWT header，如果存在则验证 JWT 并设置 context；
// 如果不存在则调用原始 TokenAuth() 作为回退。
//
// 请求流程：
// 1. lobehub/前端 → yudao-cloud gateway（携带 yudao MD5 token）
// 2. gateway TokenAuthenticationFilter 解析 MD5 token → LoginUser
// 3. gateway MaasJwtSignFilter 签发 JWT → X-Maas-JWT header
// 4. gateway 路由转发到 new-api（RewritePath 去掉 /maas 前缀）
// 5. new-api MaasJwtAuth 验证 JWT，设置虚拟 Token context
func MaasJwtAuth() func(c *gin.Context) {
	return func(c *gin.Context) {
		if !maasJwtInitialized {
			initMaasJwtConfig()
		}

		jwtStr := c.GetHeader(headerMaasJWT)
		if jwtStr == "" || !maasJwtInitialized {
			// 无 JWT 或 JWT 未初始化 → 回退到原始 TokenAuth
			TokenAuth()(c)
			return
		}

		// 验证 JWT
		claims, err := verifyMaasJwt(jwtStr)
		if err != nil {
			common.SysLog(fmt.Sprintf("MaasJwtAuth: JWT 验证失败: %v", err))
			abortWithOpenAiMessage(c, http.StatusUnauthorized,
				common.TranslateMessage(c, i18n.MsgTokenInvalid))
			return
		}

		// JWT 验证成功，设置 context
		setupMaasJwtContext(c, claims)
		c.Next()
	}
}

// MaasJwtClaims 表示 MaaS JWT 的 claims 结构
type MaasJwtClaims struct {
	UserId       int64    `json:"user_id"`
	TenantId     int64    `json:"tenant_id"`
	UserType     int      `json:"user_type"`
	DeptId       int64    `json:"dept_id,omitempty"`
	DataScope    string   `json:"data_scope,omitempty"`
	DeptScopeIds string   `json:"dept_scope_ids,omitempty"`
	Permissions  []string `json:"permissions,omitempty"`
	Source       string   `json:"source"`
	ModelWhitelist string `json:"model_whitelist,omitempty"` // Path B (API Key) 时使用

	jwt.RegisteredClaims
}

// verifyMaasJwt 验证 MaaS JWT 签名和 claims
func verifyMaasJwt(tokenString string) (*MaasJwtClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &MaasJwtClaims{}, func(token *jwt.Token) (any, error) {
		// 确认签名算法为 HS256
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return maasJwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*MaasJwtClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token claims")
}

// setupMaasJwtContext 从 JWT claims 设置 gin context 变量
// 这些变量与 TokenAuth() 设置的变量一致，确保下游代码（Distribute 等）正常工作
func setupMaasJwtContext(c *gin.Context, claims *MaasJwtClaims) {
	// 设置用户信息
	c.Set("id", int(claims.UserId))
	c.Set("username", fmt.Sprintf("maas_user_%d", claims.UserId))

	// 尝试查找系统级 Token
	var token *model.Token
	var tokenErr error

	if maasSystemTokenKey != "" {
		// MVP: 使用系统级 Token
		key := maasSystemTokenKey
		if strings.HasPrefix(key, "sk-") {
			parts := strings.Split(key[3:], "-")
			key = parts[0]
		}
		token, tokenErr = model.GetTokenByKey(key, false)
		if tokenErr != nil {
			common.SysLog(fmt.Sprintf("MaasJwtAuth: 系统级 Token 查找失败: %v", tokenErr))
		}
	}

	if token != nil {
		// 使用系统 Token 的 ID 和 Key
		c.Set("token_id", token.Id)
		c.Set("token_key", token.Key)
		c.Set("token_name", token.Name)
		c.Set("token_unlimited_quota", token.UnlimitedQuota)
		if !token.UnlimitedQuota {
			c.Set("token_quota", token.RemainQuota)
		}
		c.Set("token_model_limit_enabled", token.ModelLimitsEnabled)
		c.Set("token_model_limit", token.GetModelLimitsMap())
	} else {
		// 无系统 Token → 创建虚拟 Token context
		// 使用 unlimited quota 以确保请求不被额度限制
		c.Set("token_id", 0)
		c.Set("token_key", "maas-virtual-token")
		c.Set("token_name", "MaaS Virtual Token")
		c.Set("token_unlimited_quota", true)
		c.Set("token_model_limit_enabled", false)
	}

	// 设置租户分组
	// JWT claims 中的 tenant_id 映射到 new-api 的 group
	tenantGroup := fmt.Sprintf("tenant_%d", claims.TenantId)
	common.SetContextKey(c, constant.ContextKeyTokenGroup, claims.Source)
	common.SetContextKey(c, constant.ContextKeyUsingGroup, tenantGroup)
	common.SetContextKey(c, constant.ContextKeyTokenCrossGroupRetry, false)

	// 设置 MaaS 特有的 context 变量（供后续 MaaS 中间件使用）
	c.Set("maas_tenant_id", claims.TenantId)
	c.Set("maas_dept_id", claims.DeptId)
	c.Set("maas_data_scope", claims.DataScope)
	c.Set("maas_permissions", claims.Permissions)
	c.Set("maas_source", claims.Source)
	c.Set("maas_model_whitelist", claims.ModelWhitelist)
}