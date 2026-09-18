// Package notify 负责把告警事件推送到启用的 Webhook 渠道(飞书 / 钉钉 / 企业微信机器人)。
// 推送失败只记日志,不影响主流程。
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"gorm.io/gorm"

	"haproxy-webui/backend/internal/model"
)

// 告警种类。
const (
	KindReloadFailed  = "reload_failed"
	KindNodeDown      = "node_down"
	KindNodeRecovered = "node_recovered"
	KindBackendDown   = "backend_down"
	KindBackendUp     = "backend_up"
	KindVRRPChange    = "vrrp_change"
	KindTest          = "test"
)

var kindLabels = map[string]string{
	KindReloadFailed:  "reload 失败",
	KindNodeDown:      "连通性探测失败",
	KindNodeRecovered: "节点恢复",
	KindBackendDown:   "backend 全部 DOWN",
	KindBackendUp:     "backend 恢复",
	KindVRRPChange:    "主备切换",
	KindTest:          "测试消息",
}

// Alert 是一条待推送的告警事件。
type Alert struct {
	Kind     string // KindXxx 常量
	Instance string // 实例名
	Detail   string // 人读的事件说明
}

var pushClient = &http.Client{Timeout: 5 * time.Second}

// Text 渲染告警为纯文本消息体。
func (a Alert) Text() string {
	title := "告警"
	if a.Kind == KindNodeRecovered || a.Kind == KindBackendUp {
		title = "告警恢复"
	}
	return fmt.Sprintf("【HAProxy WebUI · %s】%s\n实例:%s\n详情:%s\n时间:%s",
		title, kindLabels[a.Kind], a.Instance, a.Detail,
		time.Now().Format("2006-01-02 15:04:05"))
}

// buildPayload 按渠道类型构造请求体。
func buildPayload(channelType, text string) (any, error) {
	switch channelType {
	case model.ChannelFeishu:
		return map[string]any{"msg_type": "text", "content": map[string]string{"text": text}}, nil
	case model.ChannelDingtalk, model.ChannelWecom:
		return map[string]any{"msgtype": "text", "text": map[string]string{"content": text}}, nil
	default:
		return nil, fmt.Errorf("unknown channel type %q", channelType)
	}
}

// SendChannel 向单个渠道推送,返回错误(供测试发送接口透出)。
func SendChannel(ctx context.Context, ch model.AlertChannel, text string) error {
	payload, err := buildPayload(ch.Type, text)
	if err != nil {
		return err
	}
	buf, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ch.WebhookURL, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := pushClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}

// Send 把告警推送到全部启用渠道;每个渠道独立超时、互不影响,失败仅记日志。
func Send(ctx context.Context, db *gorm.DB, a Alert) {
	var channels []model.AlertChannel
	if err := db.Where("enabled = ?", true).Find(&channels).Error; err != nil {
		log.Printf("notify: list channels: %v", err)
		return
	}
	if len(channels) == 0 {
		return
	}
	text := a.Text()
	for _, ch := range channels {
		if err := SendChannel(ctx, ch, text); err != nil {
			log.Printf("notify: channel %q(%s): %v", ch.Name, ch.Type, err)
		} else {
			log.Printf("notify: alert %s sent via %q", a.Kind, ch.Name)
		}
	}
}
