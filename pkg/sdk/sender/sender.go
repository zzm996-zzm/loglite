package sender

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// ============================================================
// 日志条目抽象
// ============================================================

// LogEntry 日志条目（通用结构，用于发送）
// 这是 loglite 的标准日志格式
type LogEntry struct {
	// 必填字段
	ID        string    `json:"id"`        // UUID
	Timestamp time.Time `json:"timestamp"` // 时间戳
	Message   string    `json:"message"`   // 日志消息
	Level     string    `json:"level"`     // info/warn/error/debug
	Service   string    `json:"service"`   // 服务名称

	// 推荐字段（SDK自动填充或用户提供）
	TraceID   string `json:"trace_id,omitempty"`   // 追踪ID（分布式追踪）
	SpanID    string `json:"span_id,omitempty"`    // 跨度ID（分布式追踪）
	UserID    string `json:"user_id,omitempty"`    // 用户ID
	RequestID string `json:"request_id,omitempty"` // 请求ID
	IP        string `json:"ip,omitempty"`         // IP地址

	// 代码位置（SDK自动采集，解决「上下文不足」痛点）
	Caller     string `json:"caller,omitempty"`      // 调用位置 "main.go:42"
	Function   string `json:"function,omitempty"`    // 函数名 "main.HandleOrder"
	Package    string `json:"package,omitempty"`     // 包名 "github.com/xxx/service"
	StackTrace string `json:"stack_trace,omitempty"` // 错误堆栈（仅 error 级别）
	StackHash  string `json:"stack_hash,omitempty"`  // 堆栈指纹（用于错误聚合）

	// 任意字段
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// 系统字段（内部使用，不序列化）
	ReceivedAt time.Time `json:"-"` // 接收时间（服务端填充）
	StoredAt   time.Time `json:"-"` // 存储时间（服务端填充）
}

// LogRecord 日志记录接口
// 用于兼容不同日志框架（zap、zerolog、slog 等）
type LogRecord interface {
	// ToLogEntry 转换为 LogLite 标准格式
	ToLogEntry() *LogEntry
}

// LogAdapter 日志适配器接口
// 用于包装其他日志框架
type LogAdapter interface {
	// Adapt 将任意日志数据适配为 LogEntry
	Adapt(data interface{}) *LogEntry
}

// ============================================================
// 内置适配器
// ============================================================

// MapAdapter 将 map[string]interface{} 适配为 LogEntry
type MapAdapter struct {
	Service string // 默认服务名
}

func (a *MapAdapter) Adapt(data interface{}) *LogEntry {
	m, ok := data.(map[string]interface{})
	if !ok {
		return nil
	}

	entry := &LogEntry{
		Timestamp: time.Now(),
		Service:   a.Service,
		Metadata:  make(map[string]interface{}),
	}

	// 提取标准字段
	if v, ok := m["message"].(string); ok {
		entry.Message = v
		delete(m, "message")
	} else if v, ok := m["msg"].(string); ok {
		entry.Message = v
		delete(m, "msg")
	}

	if v, ok := m["level"].(string); ok {
		entry.Level = v
		delete(m, "level")
	}

	if v, ok := m["service"].(string); ok {
		entry.Service = v
		delete(m, "service")
	}

	if v, ok := m["caller"].(string); ok {
		entry.Caller = v
		delete(m, "caller")
	}

	if v, ok := m["function"].(string); ok {
		entry.Function = v
		delete(m, "function")
	}

	// 处理时间
	switch v := m["timestamp"].(type) {
	case time.Time:
		entry.Timestamp = v
		delete(m, "timestamp")
	case string:
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			entry.Timestamp = t
		}
		delete(m, "timestamp")
	}

	// 剩余字段放入 metadata
	for k, v := range m {
		entry.Metadata[k] = v
	}

	return entry
}

// RawLogEntry 直接实现 LogRecord 的 LogEntry
// 方便直接使用
func (e *LogEntry) ToLogEntry() *LogEntry {
	return e
}

// Reliability 可靠性级别
type Reliability int

const (
	// BestEffort 最高性能模式
	// - 内存缓冲区满了就丢弃
	// - 发送失败不重试
	// - 适合：开发环境、日志量大但允许丢失
	BestEffort Reliability = iota

	// Balanced 平衡模式
	// - 双缓冲 + 定期快照到磁盘
	// - 崩溃最多丢失一个快照周期（默认 5s）
	// - 发送失败写入降级文件
	// - 适合：生产环境、一般业务日志
	Balanced

	// Reliable WAL 模式
	// - 先写本地 WAL 文件，再异步发送
	// - 发送成功后才删除 WAL 记录
	// - 零丢失保证
	// - 适合：关键业务日志、审计日志
	Reliable
)

// Sender 日志发送器接口
type Sender interface {
	// Send 发送单条日志（异步，立即返回）
	Send(entry *LogEntry) error

	// SendRecord 发送实现了 LogRecord 接口的日志
	SendRecord(record LogRecord) error

	// SendMap 发送 map 格式的日志（便捷方法）
	SendMap(data map[string]interface{}) error

	// Flush 强制刷新缓冲区
	Flush() error

	// Close 关闭发送器，确保所有日志发送完成
	Close() error

	// Stats 获取统计信息
	Stats() *SenderStats
}

// SenderStats 发送器统计
type SenderStats struct {
	TotalSent     int64 // 总发送成功数
	TotalFailed   int64 // 总发送失败数
	TotalDropped  int64 // 总丢弃数
	BufferSize    int   // 当前缓冲区大小
	PendingCount  int   // 待发送数量
	LastSendTime  time.Time
	LastErrorTime time.Time
	LastError     string
}

// Config 发送器配置
type Config struct {
	Endpoint      string        // 服务端地址
	Service       string        // 服务名
	Reliability   Reliability   // 可靠性级别
	BatchSize     int           // 批量大小
	FlushInterval time.Duration // 刷新间隔
	BufferSize    int           // 缓冲区大小
	Timeout       time.Duration // HTTP 超时
	RetryCount    int           // 重试次数
	RetryInterval time.Duration // 重试间隔

	// Balanced 模式配置
	SnapshotInterval time.Duration // 快照间隔
	SnapshotDir      string        // 快照目录

	// Reliable 模式配置
	WALDir string // WAL 目录

	// 降级配置
	FallbackFile string // 降级文件路径
}

// DefaultConfig 默认配置
func DefaultConfig() Config {
	return Config{
		Endpoint:         "http://localhost:8081",
		Reliability:      BestEffort,
		BatchSize:        100,
		FlushInterval:    100 * time.Millisecond,
		BufferSize:       10000,
		Timeout:          5 * time.Second,
		RetryCount:       3,
		RetryInterval:    100 * time.Millisecond,
		SnapshotInterval: 5 * time.Second,
		SnapshotDir:      "./logs/snapshot",
		WALDir:           "./logs/wal",
	}
}

// ============================================================
// BaseSender - 基础发送器（提供通用方法）
// ============================================================

// BaseSender 基础发送器，提供 SendRecord 和 SendMap 的默认实现
type BaseSender struct {
	adapter *MapAdapter
	sender  Sender // 指向具体实现（用于调用 Send）
}

// InitBase 初始化基础发送器
func (b *BaseSender) InitBase(service string, sender Sender) {
	b.adapter = &MapAdapter{Service: service}
	b.sender = sender
}

// SendRecord 发送 LogRecord
func (b *BaseSender) SendRecord(record LogRecord) error {
	return b.sender.Send(record.ToLogEntry())
}

// SendMap 发送 map 格式日志
func (b *BaseSender) SendMap(data map[string]interface{}) error {
	entry := b.adapter.Adapt(data)
	if entry == nil {
		return nil
	}
	return b.sender.Send(entry)
}

// NewSender 创建发送器
func NewSender(cfg Config) Sender {
	switch cfg.Reliability {
	case BestEffort:
		return NewBestEffortSender(cfg)
	case Balanced:
		return NewBalancedSender(cfg)
	case Reliable:
		return NewReliableSender(cfg)
	default:
		return NewBestEffortSender(cfg)
	}
}

// ============================================================
// HTTP 发送工具（三种模式共用）
// ============================================================

// HTTPClient HTTP 发送客户端
type HTTPClient struct {
	endpoint string
	timeout  time.Duration
	mu       sync.Mutex
}

// NewHTTPClient 创建 HTTP 客户端
func NewHTTPClient(endpoint string, timeout time.Duration) *HTTPClient {
	return &HTTPClient{
		endpoint: endpoint,
		timeout:  timeout,
	}
}

// SendBatch 批量发送日志
func (c *HTTPClient) SendBatch(entries []*LogEntry) error {
	if len(entries) == 0 {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// 1. 序列化请求体
	body, err := json.Marshal(map[string]interface{}{"logs": entries})
	if err != nil {
		return fmt.Errorf("marshal logs: %w", err)
	}

	// 2. 构造 HTTP 请求
	url := c.endpoint + "/api/v1/logs/batch"
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// 3. 发送请求
	client := &http.Client{Timeout: c.timeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	// 4. 检查响应状态码
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server error: status=%d, body=%s", resp.StatusCode, string(bodyBytes))
	}

	// 5. 解析响应体（检查是否有错误信息）
	var result struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Error   string `json:"error,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		// 响应解析失败不影响发送（可能服务端没返回 JSON）
		return nil
	}

	if result.Code != 200 {
		return fmt.Errorf("server error: code=%d, message=%s, error=%s", result.Code, result.Message, result.Error)
	}

	return nil
}
