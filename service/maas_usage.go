package service

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const (
	// Redis key patterns for MaaS usage tracking
	maasUsageKeyPattern     = "maas:usage:%s:%s:%s" // maas:usage:{tenant_id}:{model}:{date}
	maasDailyKeyPattern     = "maas:daily:%s:%s"     // maas:daily:{tenant_id}:{date}
	maasTenantQuotaKey      = "maas:quota:%s"        // maas:quota:{tenant_id}
)

// MaasUsageRecord 一次 API 调用的用量记录
type MaasUsageRecord struct {
	TenantId    int64  `json:"tenant_id"`
	UserId      int    `json:"user_id"`
	Model       string `json:"model"`
	QuotaUsed   int    `json:"quota_used"`
	PromptTokens int   `json:"prompt_tokens"`
	CompTokens  int    `json:"completion_tokens"`
	RequestType string `json:"request_type"` // chat, embedding, etc.
}

// MaasUsageStats 租户的用量统计
type MaasUsageStats struct {
	TenantId      int64  `json:"tenant_id"`
	Model         string `json:"model"`
	Date          string `json:"date"`
	RequestCount  int64  `json:"request_count"`
	QuotaUsed     int64  `json:"quota_used"`
	PromptTokens  int64  `json:"prompt_tokens"`
	CompTokens    int64  `json:"completion_tokens"`
}

// RecordMaasUsage 记录一次 MaaS API 用量到 Redis
func RecordMaasUsage(record *MaasUsageRecord) error {
	if !common.RedisEnabled {
		return nil // Redis 不可用时跳过实时统计
	}

	ctx := context.Background()
	rdb := common.RDB
	date := time.Now().Format("2006-01-02")

	// 按 tenant_id + model + date 聚合
	usageKey := fmt.Sprintf(maasUsageKeyPattern,
		strconv.FormatInt(record.TenantId, 10),
		record.Model,
		date,
	)

	// 按 tenant_id + date 汇总
	dailyKey := fmt.Sprintf(maasDailyKeyPattern,
		strconv.FormatInt(record.TenantId, 10),
		date,
	)

	pipe := rdb.Pipeline()
	// usageKey: hash with fields request_count, quota_used, prompt_tokens, comp_tokens
	pipe.HIncrBy(ctx, usageKey, "request_count", 1)
	pipe.HIncrBy(ctx, usageKey, "quota_used", int64(record.QuotaUsed))
	pipe.HIncrBy(ctx, usageKey, "prompt_tokens", int64(record.PromptTokens))
	pipe.HIncrBy(ctx, usageKey, "comp_tokens", int64(record.CompTokens))
	pipe.Expire(ctx, usageKey, 90*24*time.Hour) // 保留 90 天

	// dailyKey: hash with fields total_requests, total_quota, total_prompt, total_comp
	pipe.HIncrBy(ctx, dailyKey, "total_requests", 1)
	pipe.HIncrBy(ctx, dailyKey, "total_quota", int64(record.QuotaUsed))
	pipe.HIncrBy(ctx, dailyKey, "total_prompt", int64(record.PromptTokens))
	pipe.HIncrBy(ctx, dailyKey, "total_comp", int64(record.CompTokens))
	pipe.Expire(ctx, dailyKey, 90*24*time.Hour)

	_, err := pipe.Exec(ctx)
	return err
}

// GetMaasUsageStats 获取指定租户在某天的用量统计
func GetMaasUsageStats(tenantId int64, date string) ([]MaasUsageStats, error) {
	if !common.RedisEnabled {
		return nil, fmt.Errorf("Redis not available")
	}

	ctx := context.Background()
	rdb := common.RDB

	// 先获取每日汇总
	dailyKey := fmt.Sprintf(maasDailyKeyPattern,
		strconv.FormatInt(tenantId, 10),
		date,
	)
	dailyResult, err := rdb.HGetAll(ctx, dailyKey).Result()
	if err != nil {
		return nil, err
	}

	var stats []MaasUsageStats
	if len(dailyResult) > 0 {
		summary := MaasUsageStats{
			TenantId: tenantId,
			Model:    "*",
			Date:     date,
		}
		if v, ok := dailyResult["total_requests"]; ok {
			summary.RequestCount, _ = strconv.ParseInt(v, 10, 64)
		}
		if v, ok := dailyResult["total_quota"]; ok {
			summary.QuotaUsed, _ = strconv.ParseInt(v, 10, 64)
		}
		if v, ok := dailyResult["total_prompt"]; ok {
			summary.PromptTokens, _ = strconv.ParseInt(v, 10, 64)
		}
		if v, ok := dailyResult["total_comp"]; ok {
			summary.CompTokens, _ = strconv.ParseInt(v, 10, 64)
		}
		stats = append(stats, summary)
	}

	return stats, nil
}

// CheckMaasTenantQuota 检查租户配额是否充足
func CheckMaasTenantQuota(tenantId int64, requiredQuota int) (bool, int, error) {
	if !common.RedisEnabled {
		return true, 0, nil // Redis 不可用时允许通过
	}

	ctx := context.Background()
	rdb := common.RDB

	quotaKey := fmt.Sprintf(maasTenantQuotaKey, strconv.FormatInt(tenantId, 10))
	remainingStr, err := rdb.Get(ctx, quotaKey).Result()
	if err != nil {
		// key 不存在，说明无配额限制
		return true, -1, nil
	}

	remaining, err := strconv.Atoi(remainingStr)
	if err != nil {
		return true, -1, nil
	}

	if remaining < requiredQuota {
		return false, remaining, nil
	}

	return true, remaining, nil
}

// DeductMaasTenantQuota 扣减租户配额
func DeductMaasTenantQuota(tenantId int64, quota int) error {
	if !common.RedisEnabled {
		return nil
	}

	ctx := context.Background()
	rdb := common.RDB

	quotaKey := fmt.Sprintf(maasTenantQuotaKey, strconv.FormatInt(tenantId, 10))
	err := rdb.DecrBy(ctx, quotaKey, int64(quota)).Err()
	return err
}

// SetMaasTenantQuota 设置租户配额
func SetMaasTenantQuota(tenantId int64, quota int) error {
	if !common.RedisEnabled {
		return nil
	}

	ctx := context.Background()
	rdb := common.RDB

	quotaKey := fmt.Sprintf(maasTenantQuotaKey, strconv.FormatInt(tenantId, 10))
	return rdb.Set(ctx, quotaKey, quota, 0).Err()
}
