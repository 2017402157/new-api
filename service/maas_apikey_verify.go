package service

import (
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
)

var (
	maasServerURL   string
	maasServerURLOnce sync.Once
)

func getMaasServerURL() string {
	maasServerURLOnce.Do(func() {
		maasServerURL = os.Getenv("MAAS_SERVER_URL")
		if maasServerURL == "" {
			maasServerURL = "http://maas-server:48080"
		}
	})
	return maasServerURL
}

// VerifyMaasApiKey 通过 yudao-cloud maas-server 验证 MaaS API Key
// Returns: (valid, tenantId, userId, modelWhitelist, error)
func VerifyMaasApiKey(keyStr string) (bool, int64, int, string, error) {
	// 在 MVP 阶段，通过内部 API 验证
	// 后续通过 Redis 缓存优化
	url := fmt.Sprintf("%s/admin-api/maas/api-key/verify", getMaasServerURL())

	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false, 0, 0, "", err
	}

	req.Header.Set("X-Maas-Key", keyStr)
	req.Header.Set("Content-Type", "application/json")

	// 内部服务间调用，使用 JWT 或共享密钥
	internalSecret := os.Getenv("MAAS_INTERNAL_SECRET")
	if internalSecret != "" {
		req.Header.Set("X-Internal-Secret", internalSecret)
	}

	resp, err := client.Do(req)
	if err != nil {
		common.SysError(fmt.Sprintf("MaaS API Key 验证请求失败: %v", err))
		// 降级：如果 maas-server 不可达，允许请求通过（避免级联故障）
		return true, 0, 0, "", nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		// 解析响应获取 tenant_id, user_id, model_whitelist
		// MVP 阶段简化处理
		return true, 0, 0, "", nil
	}

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
		return false, 0, 0, "", nil
	}

	// 其他错误降级
	return true, 0, 0, "", nil
}