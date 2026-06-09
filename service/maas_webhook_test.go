package service

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
)

func TestGetProviderName(t *testing.T) {
	tests := []struct {
		channelType int
		expected   string
	}{
		{1, "openai"},
		{3, "azure"},
		{14, "anthropic"},
		{24, "gemini"},
		{33, "aws"},
		{999, "type_999"},
	}

	for _, tt := range tests {
		result := getProviderName(tt.channelType)
		assert.Equal(t, tt.expected, result)
	}
}

func TestNotifyMaasChannelChange_NoWebhookURL(t *testing.T) {
	// Without MAAS_WEBHOOK_URL set, should return without error
	channel := &model.Channel{
		Id:     1,
		Name:   "test-channel",
		Type:   1,
		Models: "gpt-4o",
		Status: 1,
	}
	// Should not panic
	NotifyMaasChannelChange("channel_create", channel)
}
