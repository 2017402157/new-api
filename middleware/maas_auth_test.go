package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
)

const testJwtSecret = "test-secret-that-is-at-least-32-characters-long!!"

func init() {
	gin.SetMode(gin.TestMode)
}

// createTestMaasJwt 创建测试用 MaaS JWT
func createTestMaasJwt(claims MaasJwtClaims, secret []byte) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

func TestVerifyMaasJwt_Valid(t *testing.T) {
	maasJwtSecret = []byte(testJwtSecret)
	maasJwtInitialized = true

	claims := MaasJwtClaims{
		UserId:   1,
		TenantId: 100,
		UserType: 1,
		DeptId:   10,
		Source:   "md5_token",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "maas-gateway",
			Subject:   "1",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(2 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	tokenStr, err := createTestMaasJwt(claims, maasJwtSecret)
	assert.NoError(t, err)

	parsedClaims, err := verifyMaasJwt(tokenStr)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), parsedClaims.UserId)
	assert.Equal(t, int64(100), parsedClaims.TenantId)
	assert.Equal(t, int64(10), parsedClaims.DeptId)
	assert.Equal(t, "md5_token", parsedClaims.Source)
}

func TestVerifyMaasJwt_Expired(t *testing.T) {
	maasJwtSecret = []byte(testJwtSecret)
	maasJwtInitialized = true

	claims := MaasJwtClaims{
		UserId:   1,
		TenantId: 100,
		Source:   "md5_token",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)), // 已过期
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-3 * time.Hour)),
		},
	}

	tokenStr, err := createTestMaasJwt(claims, maasJwtSecret)
	assert.NoError(t, err)

	_, err = verifyMaasJwt(tokenStr)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "token is expired")
}

func TestVerifyMaasJwt_WrongSecret(t *testing.T) {
	maasJwtSecret = []byte(testJwtSecret)
	maasJwtInitialized = true

	claims := MaasJwtClaims{
		UserId:   1,
		TenantId: 100,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(2 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	wrongSecret := []byte("wrong-secret-that-is-at-least-32-characters-long!!")
	tokenStr, err := createTestMaasJwt(claims, wrongSecret)
	assert.NoError(t, err)

	_, err = verifyMaasJwt(tokenStr)
	assert.Error(t, err)
}

func TestMaasJwtAuth_WithValidJwt(t *testing.T) {
	maasJwtSecret = []byte(testJwtSecret)
	maasJwtInitialized = true

	claims := MaasJwtClaims{
		UserId:       1,
		TenantId:     100,
		UserType:     1,
		DeptId:       10,
		DataScope:    "1",
		Source:       "md5_token",
		Permissions:  []string{"read", "write"},
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(2 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	tokenStr, err := createTestMaasJwt(claims, maasJwtSecret)
	assert.NoError(t, err)

	// 创建测试路由
	r := gin.New()
	r.Use(MaasJwtAuth())
	r.GET("/v1/chat/completions", func(c *gin.Context) {
		// 验证 context 变量设置正确
		assert.Equal(t, 1, c.GetInt("id"))
		assert.Equal(t, true, c.GetBool("token_unlimited_quota"))
		assert.Equal(t, int64(100), c.GetInt64("maas_tenant_id"))
		assert.Equal(t, int64(10), c.GetInt64("maas_dept_id"))
		assert.Equal(t, "1", c.GetString("maas_data_scope"))
		assert.Equal(t, "md5_token", c.GetString("maas_source"))
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/chat/completions", nil)
	req.Header.Set(headerMaasJWT, tokenStr)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMaasJwtAuth_WithInvalidJwt(t *testing.T) {
	maasJwtSecret = []byte(testJwtSecret)
	maasJwtInitialized = true

	r := gin.New()
	r.Use(MaasJwtAuth())
	r.GET("/v1/chat/completions", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/chat/completions", nil)
	req.Header.Set(headerMaasJWT, "invalid-jwt-token")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestMaasJwtAuth_NoJwt_FallbackToTokenAuth(t *testing.T) {
	// 当没有 X-Maas-JWT header 时，应该回退到 TokenAuth
	// TokenAuth 没有 sk-xxx key 会返回 401
	maasJwtSecret = []byte(testJwtSecret)
	maasJwtInitialized = true

	r := gin.New()
	r.Use(MaasJwtAuth())
	r.GET("/v1/chat/completions", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/chat/completions", nil)
	// 不设置 X-Maas-JWT header
	r.ServeHTTP(w, req)

	// TokenAuth 在无 Authorization header 时返回 401
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestMaasJwtAuth_WithPermissions(t *testing.T) {
	maasJwtSecret = []byte(testJwtSecret)
	maasJwtInitialized = true

	claims := MaasJwtClaims{
		UserId:      42,
		TenantId:    200,
		Permissions: []string{"model:read", "model:write", "admin"},
		Source:      "md5_token",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(2 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	tokenStr, err := createTestMaasJwt(claims, maasJwtSecret)
	assert.NoError(t, err)

	r := gin.New()
	r.Use(MaasJwtAuth())
	r.GET("/v1/chat/completions", func(c *gin.Context) {
		permissions, exists := c.Get("maas_permissions")
		assert.True(t, exists)
		perms := permissions.([]string)
		assert.Len(t, perms, 3)
		assert.Contains(t, perms, "model:read")
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/chat/completions", nil)
	req.Header.Set(headerMaasJWT, tokenStr)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSetupMaasJwtContext_TenantGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	claims := &MaasJwtClaims{
		UserId:   1,
		TenantId: 100,
		Source:   "md5_token",
	}

	setupMaasJwtContext(c, claims)

	// 验证 tenant group 格式
	usingGroup := c.GetString(string(constant.ContextKeyUsingGroup))
	assert.Equal(t, "tenant_100", usingGroup)

	// 验证 MaaS 特有 context
	assert.Equal(t, int64(100), c.GetInt64("maas_tenant_id"))
	assert.Equal(t, "md5_token", c.GetString("maas_source"))
}

func TestSetupMaasJwtContext_VirtualToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	claims := &MaasJwtClaims{
		UserId:   1,
		TenantId: 100,
		Source:   "md5_token",
	}

	// 不设置 MAAS_SYSTEM_TOKEN_KEY，使用虚拟 Token
	maasSystemTokenKey = ""
	setupMaasJwtContext(c, claims)

	// 验证虚拟 Token context
	assert.Equal(t, 0, c.GetInt("token_id"))
	assert.Equal(t, "maas-virtual-token", c.GetString("token_key"))
	assert.Equal(t, true, c.GetBool("token_unlimited_quota"))
	assert.Equal(t, false, c.GetBool("token_model_limit_enabled"))
}

func TestMaasJwtClaims_WithModelWhitelist(t *testing.T) {
	maasJwtSecret = []byte(testJwtSecret)
	maasJwtInitialized = true

	claims := MaasJwtClaims{
		UserId:         1,
		TenantId:       100,
		Source:         "api_key",
		ModelWhitelist: "gpt-4,claude-3",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(2 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	tokenStr, err := createTestMaasJwt(claims, maasJwtSecret)
	assert.NoError(t, err)

	parsedClaims, err := verifyMaasJwt(tokenStr)
	assert.NoError(t, err)
	assert.Equal(t, "gpt-4,claude-3", parsedClaims.ModelWhitelist)
	assert.Equal(t, "api_key", parsedClaims.Source)
}

func TestInitMaasJwtConfig(t *testing.T) {
	// 测试：密钥过短时初始化失败
	maasJwtInitialized = false
	maasJwtSecret = nil

	os.Setenv("MAAS_JWT_SECRET", "short")
	initMaasJwtConfig()
	assert.False(t, maasJwtInitialized)

	// 测试：密钥足够长时初始化成功
	os.Setenv("MAAS_JWT_SECRET", testJwtSecret)
	initMaasJwtConfig()
	assert.True(t, maasJwtInitialized)
	assert.Equal(t, []byte(testJwtSecret), maasJwtSecret)

	// 清理
	os.Unsetenv("MAAS_JWT_SECRET")
}

// 验证 JWT 签发与验证的端到端流程
// 模拟 yudao-cloud gateway 签发 → new-api 验证
func TestMaasJwt_EndToEnd(t *testing.T) {
	sharedSecret := "e2e-test-secret-that-is-at-least-32-characters-long!!"
	maasJwtSecret = []byte(sharedSecret)
	maasJwtInitialized = true

	// 模拟 gateway 签发 JWT（与 MaasJwtSignFilter.java 逻辑一致）
	claims := MaasJwtClaims{
		UserId:       123,
		TenantId:     456,
		UserType:     1,
		DeptId:       789,
		DataScope:    "2",
		DeptScopeIds: "10,20,30",
		Permissions:  []string{"model:read"},
		Source:       "md5_token",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "maas-gateway",
			Subject:   "123",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(2 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	tokenStr, err := createTestMaasJwt(claims, maasJwtSecret)
	assert.NoError(t, err)

	// 模拟 new-api 验证 JWT
	parsedClaims, err := verifyMaasJwt(tokenStr)
	assert.NoError(t, err)

	// 验证所有 claims
	assert.Equal(t, int64(123), parsedClaims.UserId)
	assert.Equal(t, int64(456), parsedClaims.TenantId)
	assert.Equal(t, 1, parsedClaims.UserType)
	assert.Equal(t, int64(789), parsedClaims.DeptId)
	assert.Equal(t, "2", parsedClaims.DataScope)
	assert.Equal(t, "10,20,30", parsedClaims.DeptScopeIds)
	assert.Equal(t, []string{"model:read"}, parsedClaims.Permissions)
	assert.Equal(t, "md5_token", parsedClaims.Source)
	assert.Equal(t, "maas-gateway", parsedClaims.Issuer)
	assert.Equal(t, "123", parsedClaims.Subject)

	// 验证 context 设置
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	setupMaasJwtContext(c, parsedClaims)

	assert.Equal(t, 123, c.GetInt("id"))
	assert.Equal(t, int64(456), c.GetInt64("maas_tenant_id"))
	assert.Equal(t, int64(789), c.GetInt64("maas_dept_id"))
	assert.Equal(t, "2", c.GetString("maas_data_scope"))
	assert.Equal(t, "md5_token", c.GetString("maas_source"))
	assert.Equal(t, "tenant_456", c.GetString(string(constant.ContextKeyUsingGroup)))
}

func TestMain(m *testing.M) {
	// 确保测试环境变量
	common.RedisEnabled = false
	// 避免 model 包初始化数据库连接
	os.Setenv("MAAS_JWT_SECRET", testJwtSecret)
	code := m.Run()
	os.Unsetenv("MAAS_JWT_SECRET")
	os.Exit(code)
}

// 确保 fmt import 被使用（TestMaasJwtAuth_WithValidJwt 等测试间接使用）
var _ = fmt.Sprintf