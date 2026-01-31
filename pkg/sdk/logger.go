package sdk

import (
	"context"
)

// Context Key 定义
type contextKey string

const (
	TraceIDKey   contextKey = "trace_id"
	RequestIDKey contextKey = "request_id"
	UserIDKey    contextKey = "user_id"
	LoggerKey    contextKey = "loglite_logger"
)

// Logger 带有预设字段的日志器
type Logger struct {
	client *Client
	fields map[string]interface{}
}

// Debug 记录 debug 日志
func (l *Logger) Debug(message string, keyvals ...interface{}) {
	l.log("debug", message, keyvals...)
}

// Info 记录 info 日志
func (l *Logger) Info(message string, keyvals ...interface{}) {
	l.log("info", message, keyvals...)
}

// Warn 记录 warn 日志
func (l *Logger) Warn(message string, keyvals ...interface{}) {
	l.log("warn", message, keyvals...)
}

// Error 记录 error 日志
func (l *Logger) Error(message string, keyvals ...interface{}) {
	l.log("error", message, keyvals...)
}

// With 添加更多字段
func (l *Logger) With(fields map[string]interface{}) *Logger {
	newFields := make(map[string]interface{})
	for k, v := range l.fields {
		newFields[k] = v
	}
	for k, v := range fields {
		newFields[k] = v
	}
	return &Logger{
		client: l.client,
		fields: newFields,
	}
}

// WithContext 从 Context 更新字段
func (l *Logger) WithContext(ctx context.Context) *Logger {
	newFields := make(map[string]interface{})
	for k, v := range l.fields {
		newFields[k] = v
	}

	if traceID, ok := ctx.Value(TraceIDKey).(string); ok {
		newFields["trace_id"] = traceID
	}
	if requestID, ok := ctx.Value(RequestIDKey).(string); ok {
		newFields["request_id"] = requestID
	}
	if userID, ok := ctx.Value(UserIDKey).(string); ok {
		newFields["user_id"] = userID
	}

	return &Logger{
		client: l.client,
		fields: newFields,
	}
}

// log 内部日志方法
func (l *Logger) log(level, message string, keyvals ...interface{}) {
	// 合并预设字段和传入字段
	merged := make([]interface{}, 0, len(l.fields)*2+len(keyvals))

	for k, v := range l.fields {
		merged = append(merged, k, v)
	}
	merged = append(merged, keyvals...)

	// 调整 caller depth
	l.client.callerDepth = 4
	defer func() { l.client.callerDepth = 3 }()

	l.client.log(level, message, merged...)
}

// FromContext 从 Context 获取 Logger
func FromContext(ctx context.Context) *Logger {
	if l, ok := ctx.Value(LoggerKey).(*Logger); ok {
		return l
	}
	// 返回一个默认的 Logger
	return defaultClient.WithContext(ctx)
}

// ToContext 将 Logger 存入 Context
func ToContext(ctx context.Context, l *Logger) context.Context {
	return context.WithValue(ctx, LoggerKey, l)
}

// 默认客户端
var defaultClient *Client

// SetDefault 设置默认客户端
func SetDefault(c *Client) {
	defaultClient = c
}

// Default 获取默认客户端
func Default() *Client {
	return defaultClient
}

// 包级别的便捷方法
func Debug(message string, keyvals ...interface{}) {
	if defaultClient != nil {
		defaultClient.Debug(message, keyvals...)
	}
}

func Info(message string, keyvals ...interface{}) {
	if defaultClient != nil {
		defaultClient.Info(message, keyvals...)
	}
}

func Warn(message string, keyvals ...interface{}) {
	if defaultClient != nil {
		defaultClient.Warn(message, keyvals...)
	}
}

func Error(message string, keyvals ...interface{}) {
	if defaultClient != nil {
		defaultClient.Error(message, keyvals...)
	}
}
