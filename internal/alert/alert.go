package alert

import (
	"context"
	"time"
)

// ============================================================
// 告警系统框架
// 解决「报错发企业微信/飞书」痛点
// ============================================================

// AlertManager 告警管理器
type AlertManager struct {
	rules    []*Rule
	channels map[string]Channel
	enabled  bool
}

// Rule 告警规则
type Rule struct {
	Name        string        // 规则名称
	Description string        // 规则描述
	Condition   string        // 触发条件（DSL）
	Severity    Severity      // 告警级别
	Channels    []string      // 通知渠道
	Cooldown    time.Duration // 冷却时间
	Enabled     bool          // 是否启用
	
	// 内部状态
	lastTrigger time.Time // 上次触发时间
}

// Severity 告警级别
type Severity string

const (
	SeverityCritical Severity = "critical" // 🔴 严重
	SeverityHigh     Severity = "high"     // 🟠 高
	SeverityMedium   Severity = "medium"   // 🟡 中
	SeverityLow      Severity = "low"      // 🟢 低
	SeverityInfo     Severity = "info"     // ℹ️ 信息
)

// Alert 告警事件
type Alert struct {
	RuleName    string                 // 规则名称
	Severity    Severity               // 告警级别
	Title       string                 // 告警标题
	Message     string                 // 告警消息
	Details     map[string]interface{} // 详细信息
	Timestamp   time.Time              // 触发时间
	DashboardURL string                // 查看详情 URL
}

// Channel 通知渠道接口
type Channel interface {
	// Name 渠道名称
	Name() string
	
	// Send 发送告警
	Send(ctx context.Context, alert *Alert) error
	
	// Test 测试连接
	Test(ctx context.Context) error
}

// ============================================================
// 核心方法（框架）
// ============================================================

// NewAlertManager 创建告警管理器
func NewAlertManager(config *Config) *AlertManager {
	return &AlertManager{
		rules:    make([]*Rule, 0),
		channels: make(map[string]Channel),
		enabled:  config.Enabled,
	}
}

// RegisterChannel 注册通知渠道
func (m *AlertManager) RegisterChannel(name string, channel Channel) {
	m.channels[name] = channel
}

// AddRule 添加告警规则
func (m *AlertManager) AddRule(rule *Rule) {
	m.rules = append(m.rules, rule)
}

// Check 检查告警规则（定期调用）
// TODO: 实现规则引擎
func (m *AlertManager) Check(ctx context.Context) error {
	// 1. 遍历所有规则
	// 2. 评估条件是否满足
	// 3. 触发告警
	// 4. 发送通知
	return nil
}

// Trigger 触发告警
func (m *AlertManager) Trigger(ctx context.Context, rule *Rule, alert *Alert) error {
	if !m.enabled || !rule.Enabled {
		return nil
	}
	
	// 检查冷却时间
	if time.Since(rule.lastTrigger) < rule.Cooldown {
		return nil
	}
	
	// 发送到所有配置的渠道
	for _, channelName := range rule.Channels {
		if channel, ok := m.channels[channelName]; ok {
			if err := channel.Send(ctx, alert); err != nil {
				// 记录错误但继续发送其他渠道
				continue
			}
		}
	}
	
	// 更新触发时间
	rule.lastTrigger = time.Now()
	
	return nil
}

// Start 启动告警管理器
func (m *AlertManager) Start(ctx context.Context) error {
	// TODO: 启动定期检查协程
	return nil
}

// Stop 停止告警管理器
func (m *AlertManager) Stop() error {
	return nil
}

// ============================================================
// 配置
// ============================================================

// Config 告警配置
type Config struct {
	Enabled  bool                      // 是否启用
	Channels map[string]*ChannelConfig // 渠道配置
	Rules    []*RuleConfig             // 规则配置
}

// ChannelConfig 渠道配置
type ChannelConfig struct {
	Type    string                 // 渠道类型：wecom/feishu/dingtalk/email/webhook
	Webhook string                 // Webhook URL
	Secret  string                 // 签名密钥
	Params  map[string]interface{} // 其他参数
}

// RuleConfig 规则配置
type RuleConfig struct {
	Name        string        // 规则名称
	Description string        // 规则描述
	Condition   string        // 触发条件
	Severity    string        // 告警级别
	Channels    []string      // 通知渠道
	Cooldown    time.Duration // 冷却时间
	Enabled     bool          // 是否启用
}
