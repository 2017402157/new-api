package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMaasUsageRecord_KeyPattern(t *testing.T) {
	record := &MaasUsageRecord{
		TenantId:     1,
		UserId:       100,
		Model:        "gpt-4o",
		QuotaUsed:    500,
		PromptTokens: 1000,
		CompTokens:   500,
		RequestType:  "chat",
	}

	// Just verify the record struct is valid
	assert.Equal(t, int64(1), record.TenantId)
	assert.Equal(t, "gpt-4o", record.Model)
	assert.Equal(t, 500, record.QuotaUsed)
}

func TestCheckMaasTenantQuota_NoRedis(t *testing.T) {
	// When Redis is not enabled, should allow through
	allowed, remaining, err := CheckMaasTenantQuota(1, 100)
	assert.NoError(t, err)
	assert.True(t, allowed)
	assert.Equal(t, 0, remaining)
}

func TestRecordMaasUsage_NoRedis(t *testing.T) {
	// When Redis is not enabled, should return nil
	record := &MaasUsageRecord{
		TenantId:     1,
		Model:        "gpt-4o",
		QuotaUsed:    500,
		PromptTokens: 1000,
		CompTokens:   500,
	}
	err := RecordMaasUsage(record)
	assert.NoError(t, err)
}

func TestSetMaasTenantQuota_NoRedis(t *testing.T) {
	err := SetMaasTenantQuota(1, 10000)
	assert.NoError(t, err)
}
