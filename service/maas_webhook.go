package service

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

var (
	maasWebhookURL    string
	maasWebhookSecret string
	maasWebhookOnce   sync.Once
)

func initMaasWebhook() {
	maasWebhookURL = os.Getenv("MAAS_WEBHOOK_URL")
	maasWebhookSecret = os.Getenv("MAAS_WEBHOOK_SECRET")
}

// MaasChannelEvent 发送给 yudao-cloud maas-server 的渠道变更事件
type MaasChannelEvent struct {
	Event   string          `json:"event"`
	Channel MaasChannelInfo `json:"channel"`
}

// MaasChannelInfo 渠道简要信息
type MaasChannelInfo struct {
	Id       int    `json:"id"`
	Name     string `json:"name"`
	Type     int    `json:"type"`
	Models   string `json:"models"`
	Provider string `json:"provider"`
	Status   int    `json:"status"`
}

// NotifyMaasChannelChange 通知 yudao-cloud maas-server 渠道发生变更
func NotifyMaasChannelChange(event string, channel *model.Channel) {
	maasWebhookOnce.Do(initMaasWebhook)

	if maasWebhookURL == "" {
		return
	}

	channelInfo := MaasChannelInfo{
		Id:       channel.Id,
		Name:     channel.Name,
		Type:     channel.Type,
		Models:   channel.Models,
		Provider: getProviderName(channel.Type),
		Status:   channel.Status,
	}

	maasEvent := MaasChannelEvent{
		Event:   event,
		Channel: channelInfo,
	}

	payloadBytes, err := common.Marshal(maasEvent)
	if err != nil {
		common.SysError(fmt.Sprintf("MaaS Webhook 序列化失败: %v", err))
		return
	}

	signature := ""
	if maasWebhookSecret != "" {
		h := hmac.New(sha256.New, []byte(maasWebhookSecret))
		h.Write(payloadBytes)
		signature = hex.EncodeToString(h.Sum(nil))
	}

	req, err := http.NewRequest(http.MethodPost, maasWebhookURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		common.SysError(fmt.Sprintf("MaaS Webhook 创建请求失败: %v", err))
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Newapi-Signature", signature)
	req.Header.Set("X-Newapi-Event", event)

	client := GetHttpClient()
	resp, err := client.Do(req)
	if err != nil {
		common.SysError(fmt.Sprintf("MaaS Webhook 发送失败: %v", err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		common.SysError(fmt.Sprintf("MaaS Webhook 响应异常: status=%d", resp.StatusCode))
	} else {
		common.SysLog(fmt.Sprintf("MaaS Webhook 发送成功: event=%s, channel=%d", event, channel.Id))
	}
}

// getProviderName 根据 channel type 返回 provider 名称
func getProviderName(channelType int) string {
	providerMap := map[int]string{
		1: "openai", 3: "azure", 8: "custom", 14: "anthropic",
		15: "openai", 24: "gemini", 33: "aws",
	}
	if name, ok := providerMap[channelType]; ok {
		return name
	}
	return fmt.Sprintf("type_%d", channelType)
}
