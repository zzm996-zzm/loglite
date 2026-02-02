package model

import (
	"time"
)

// LogEntry 日志条目（核心数据模型）
// 这是 loglite 的标准日志格式，服务端和客户端统一使用
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

	// 任意字段（延迟分配：只在有字段时初始化 map）
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// 系统字段（内部使用，不序列化）
	ReceivedAt time.Time `json:"-"` // 接收时间（服务端填充）
	StoredAt   time.Time `json:"-"` // 存储时间（服务端填充）
}

// LogLevel 日志级别
type LogLevel string

const (
	LevelDebug LogLevel = "debug"
	LevelInfo  LogLevel = "info"
	LevelWarn  LogLevel = "warn"
	LevelError LogLevel = "error"
)

// ValidLevels 有效的日志级别
var ValidLevels = map[string]bool{
	"debug": true,
	"info":  true,
	"warn":  true,
	"error": true,
}

// LogError 日志错误类型
type LogError string

func (e LogError) Error() string {
	return string(e)
}

const (
	ErrEmptyMessage LogError = "message is required"
	ErrEmptyService LogError = "service is required"
)

// Validate 验证日志条目
func (e *LogEntry) Validate() error {
	if e.Message == "" {
		return ErrEmptyMessage
	}
	if e.Service == "" {
		return ErrEmptyService
	}
	if e.Level == "" {
		e.Level = "info"
	}
	if !ValidLevels[e.Level] {
		e.Level = "info"
	}
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}
	return nil
}

// SetMetadata 设置元数据字段（延迟分配）
func (e *LogEntry) SetMetadata(key string, value interface{}) {
	if e.Metadata == nil {
		e.Metadata = make(map[string]interface{}, 4)
	}
	e.Metadata[key] = value
}

// GetMetadata 获取元数据字段
func (e *LogEntry) GetMetadata(key string) (interface{}, bool) {
	if e.Metadata == nil {
		return nil, false
	}
	value, ok := e.Metadata[key]
	return value, ok
}

// DeleteMetadata 删除元数据字段，返回是否成功删除
func (e *LogEntry) DeleteMetadata(key string) bool {
	if e.Metadata == nil {
		return false
	}
	_, ok := e.Metadata[key]
	if ok {
		delete(e.Metadata, key)
	}
	return ok
}

// MetadataLen 返回元数据字段数量
func (e *LogEntry) MetadataLen() int {
	if e.Metadata == nil {
		return 0
	}
	return len(e.Metadata)
}

// RangeMetadata 遍历所有元数据字段
func (e *LogEntry) RangeMetadata(fn func(key string, value interface{}) bool) {
	if e.Metadata == nil {
		return
	}
	for k, v := range e.Metadata {
		if !fn(k, v) {
			return
		}
	}
}

// 使用 sonic 的默认序列化，无需自定义
// Metadata 字段使用 omitempty，如果为 nil 则不会序列化

// ToLogEntry 转换为 LogEntry（实现 LogRecord 接口）
func (e *LogEntry) ToLogEntry() *LogEntry {
	return e
}
