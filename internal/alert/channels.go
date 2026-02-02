package alert

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/bytedance/sonic"
)

// ============================================================
// 企业微信
// ============================================================

// WeComChannel 企业微信通知渠道
type WeComChannel struct {
	webhook string
	client  *http.Client
}

// NewWeComChannel 创建企业微信渠道
func NewWeComChannel(webhook string) *WeComChannel {
	return &WeComChannel{
		webhook: webhook,
		client:  &http.Client{Timeout: 5 * time.Second},
	}
}

func (c *WeComChannel) Name() string {
	return "wecom"
}

func (c *WeComChannel) Send(ctx context.Context, alert *Alert) error {
	// TODO: 实现企业微信消息发送
	// 消息格式参考：https://developer.work.weixin.qq.com/document/path/91770

	message := c.formatMessage(alert)

	body := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"content": message,
		},
	}

	return c.sendWebhook(ctx, body)
}

func (c *WeComChannel) Test(ctx context.Context) error {
	testAlert := &Alert{
		RuleName:  "测试",
		Severity:  SeverityInfo,
		Title:     "LogLite 告警测试",
		Message:   "这是一条测试消息",
		Timestamp: time.Now(),
	}
	return c.Send(ctx, testAlert)
}

func (c *WeComChannel) formatMessage(alert *Alert) string {
	// TODO: 美化消息格式
	return fmt.Sprintf(`## %s LogLite 告警
	
**告警名称**: %s
**告警级别**: %s
**告警时间**: %s

%s

[查看详情](%s)`,
		c.getSeverityEmoji(alert.Severity),
		alert.RuleName,
		alert.Severity,
		alert.Timestamp.Format("2006-01-02 15:04:05"),
		alert.Message,
		alert.DashboardURL,
	)
}

func (c *WeComChannel) getSeverityEmoji(severity Severity) string {
	switch severity {
	case SeverityCritical:
		return "🔴"
	case SeverityHigh:
		return "🟠"
	case SeverityMedium:
		return "🟡"
	case SeverityLow:
		return "🟢"
	default:
		return "ℹ️"
	}
}

func (c *WeComChannel) sendWebhook(ctx context.Context, body interface{}) error {
	data, err := sonic.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.webhook, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("webhook failed: status=%d", resp.StatusCode)
	}

	return nil
}

// ============================================================
// 飞书
// ============================================================

// FeishuChannel 飞书通知渠道
type FeishuChannel struct {
	webhook string
	atAll   bool
	client  *http.Client
}

// NewFeishuChannel 创建飞书渠道
func NewFeishuChannel(webhook string, atAll bool) *FeishuChannel {
	return &FeishuChannel{
		webhook: webhook,
		atAll:   atAll,
		client:  &http.Client{Timeout: 5 * time.Second},
	}
}

func (c *FeishuChannel) Name() string {
	return "feishu"
}

func (c *FeishuChannel) Send(ctx context.Context, alert *Alert) error {
	// TODO: 实现飞书消息发送
	// 消息格式参考：https://open.feishu.cn/document/client-docs/bot-v3/add-custom-bot

	message := c.formatMessage(alert)

	body := map[string]interface{}{
		"msg_type": "interactive",
		"card": map[string]interface{}{
			"header": map[string]interface{}{
				"title": map[string]string{
					"tag":     "plain_text",
					"content": alert.Title,
				},
				"template": c.getTemplateColor(alert.Severity),
			},
			"elements": []interface{}{
				map[string]interface{}{
					"tag":     "markdown",
					"content": message,
				},
			},
		},
	}

	return c.sendWebhook(ctx, body)
}

func (c *FeishuChannel) Test(ctx context.Context) error {
	testAlert := &Alert{
		RuleName:  "测试",
		Severity:  SeverityInfo,
		Title:     "LogLite 告警测试",
		Message:   "这是一条测试消息",
		Timestamp: time.Now(),
	}
	return c.Send(ctx, testAlert)
}

func (c *FeishuChannel) formatMessage(alert *Alert) string {
	// TODO: 美化消息格式
	return fmt.Sprintf(`**告警名称**: %s
**告警级别**: %s %s
**告警时间**: %s

%s

[查看详情](%s)`,
		alert.RuleName,
		c.getSeverityEmoji(alert.Severity),
		alert.Severity,
		alert.Timestamp.Format("2006-01-02 15:04:05"),
		alert.Message,
		alert.DashboardURL,
	)
}

func (c *FeishuChannel) getTemplateColor(severity Severity) string {
	switch severity {
	case SeverityCritical:
		return "red"
	case SeverityHigh:
		return "orange"
	case SeverityMedium:
		return "yellow"
	case SeverityLow:
		return "green"
	default:
		return "blue"
	}
}

func (c *FeishuChannel) getSeverityEmoji(severity Severity) string {
	switch severity {
	case SeverityCritical:
		return "🔴"
	case SeverityHigh:
		return "🟠"
	case SeverityMedium:
		return "🟡"
	case SeverityLow:
		return "🟢"
	default:
		return "ℹ️"
	}
}

func (c *FeishuChannel) sendWebhook(ctx context.Context, body interface{}) error {
	data, err := sonic.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.webhook, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("webhook failed: status=%d", resp.StatusCode)
	}

	return nil
}

// ============================================================
// 钉钉
// ============================================================

// DingTalkChannel 钉钉通知渠道
type DingTalkChannel struct {
	webhook string
	secret  string
	client  *http.Client
}

// NewDingTalkChannel 创建钉钉渠道
func NewDingTalkChannel(webhook, secret string) *DingTalkChannel {
	return &DingTalkChannel{
		webhook: webhook,
		secret:  secret,
		client:  &http.Client{Timeout: 5 * time.Second},
	}
}

func (c *DingTalkChannel) Name() string {
	return "dingtalk"
}

func (c *DingTalkChannel) Send(ctx context.Context, alert *Alert) error {
	// TODO: 实现钉钉消息发送（需要签名）
	// 消息格式参考：https://open.dingtalk.com/document/robots/custom-robot-access

	message := c.formatMessage(alert)

	body := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"title": alert.Title,
			"text":  message,
		},
	}

	// TODO: 添加签名逻辑
	url := c.webhook
	if c.secret != "" {
		url = c.signURL(url)
	}

	return c.sendWebhook(ctx, url, body)
}

func (c *DingTalkChannel) Test(ctx context.Context) error {
	testAlert := &Alert{
		RuleName:  "测试",
		Severity:  SeverityInfo,
		Title:     "LogLite 告警测试",
		Message:   "这是一条测试消息",
		Timestamp: time.Now(),
	}
	return c.Send(ctx, testAlert)
}

func (c *DingTalkChannel) formatMessage(alert *Alert) string {
	// TODO: 美化消息格式
	return fmt.Sprintf(`## %s LogLite 告警

**告警名称**: %s  
**告警级别**: %s  
**告警时间**: %s

%s

[查看详情](%s)`,
		c.getSeverityEmoji(alert.Severity),
		alert.RuleName,
		alert.Severity,
		alert.Timestamp.Format("2006-01-02 15:04:05"),
		alert.Message,
		alert.DashboardURL,
	)
}

func (c *DingTalkChannel) getSeverityEmoji(severity Severity) string {
	switch severity {
	case SeverityCritical:
		return "🔴"
	case SeverityHigh:
		return "🟠"
	case SeverityMedium:
		return "🟡"
	case SeverityLow:
		return "🟢"
	default:
		return "ℹ️"
	}
}

func (c *DingTalkChannel) signURL(url string) string {
	// TODO: 实现钉钉签名算法
	// https://open.dingtalk.com/document/robots/customize-robot-security-settings
	return url
}

func (c *DingTalkChannel) sendWebhook(ctx context.Context, url string, body interface{}) error {
	data, err := sonic.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("webhook failed: status=%d", resp.StatusCode)
	}

	return nil
}

// ============================================================
// 通用 Webhook
// ============================================================

// WebhookChannel 自定义 Webhook 渠道
type WebhookChannel struct {
	url     string
	method  string
	headers map[string]string
	client  *http.Client
}

// NewWebhookChannel 创建自定义 Webhook 渠道
func NewWebhookChannel(url, method string, headers map[string]string) *WebhookChannel {
	if method == "" {
		method = "POST"
	}
	return &WebhookChannel{
		url:     url,
		method:  method,
		headers: headers,
		client:  &http.Client{Timeout: 5 * time.Second},
	}
}

func (c *WebhookChannel) Name() string {
	return "webhook"
}

func (c *WebhookChannel) Send(ctx context.Context, alert *Alert) error {
	// TODO: 发送到自定义 Webhook
	data, err := sonic.Marshal(alert)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, c.method, c.url, bytes.NewReader(data))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook failed: status=%d", resp.StatusCode)
	}

	return nil
}

func (c *WebhookChannel) Test(ctx context.Context) error {
	testAlert := &Alert{
		RuleName:  "测试",
		Severity:  SeverityInfo,
		Title:     "LogLite 告警测试",
		Message:   "这是一条测试消息",
		Timestamp: time.Now(),
	}
	return c.Send(ctx, testAlert)
}
