package model

import (
	"time"
)

// LogEntry 日志条目
type LogEntry struct {
	// 必填字段
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message"`
	Level     string    `json:"level"`
	Service   string    `json:"service"`

	// 推荐字段
	TraceID   string `json:"trace_id,omitempty"`
	SpanID    string `json:"span_id,omitempty"`
	UserID    string `json:"user_id,omitempty"`
	RequestID string `json:"request_id,omitempty"`
	IP        string `json:"ip,omitempty"`

	// 代码位置
	Caller     string `json:"caller,omitempty"`
	Function   string `json:"function,omitempty"`
	Package    string `json:"package,omitempty"`
	StackTrace string `json:"stack_trace,omitempty"`
	StackHash  string `json:"stack_hash,omitempty"`

	// 任意字段
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// 系统字段
	ReceivedAt time.Time `json:"-"`
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

// 错误定义
type LogError string

func (e LogError) Error() string {
	return string(e)
}

const (
	ErrEmptyMessage LogError = "message is required"
	ErrEmptyService LogError = "service is required"
)
