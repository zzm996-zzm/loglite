package model

import (
	"strings"
	"time"
)

// JSONTime 自定义时间类型，用于 JSON 序列化
type JSONTime time.Time

// MarshalJSON 序列化为友好格式
func (t JSONTime) MarshalJSON() ([]byte, error) {
	stamp := time.Time(t).Format("2006-01-02 15:04:05")
	return []byte(`"` + stamp + `"`), nil
}

// UnmarshalJSON 反序列化，支持多种格式
func (t *JSONTime) UnmarshalJSON(data []byte) error {
	s := strings.Trim(string(data), `"`)
	if s == "" || s == "null" {
		*t = JSONTime(time.Time{})
		return nil
	}

	// 支持多种格式
	formats := []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05.999999999Z07:00",
		time.RFC3339,
		time.RFC3339Nano,
	}

	for _, format := range formats {
		if parsed, err := time.ParseInLocation(format, s, time.Local); err == nil {
			*t = JSONTime(parsed)
			return nil
		}
	}

	// 默认尝试
	parsed, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		*t = JSONTime(time.Now())
		return nil
	}
	*t = JSONTime(parsed)
	return nil
}

// Time 转换为 time.Time
func (t JSONTime) Time() time.Time {
	return time.Time(t)
}

// IsZero 检查是否为零值
func (t JSONTime) IsZero() bool {
	return time.Time(t).IsZero()
}

// LogEntry 日志条目
type LogEntry struct {
	// 必填字段
	ID        string   `json:"id"`
	Timestamp JSONTime `json:"timestamp"`
	Message   string   `json:"message"`
	Level     string   `json:"level"`
	Service   string   `json:"service"`

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
		e.Timestamp = JSONTime(time.Now())
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
