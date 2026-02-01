package sdk

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/loglite/loglite/pkg/sdk/sender"
)

// ============================================================
// LoggerProvider - 重量级组件（类似 OpenTelemetry 的 LoggerProvider）
// 负责：配置、Sender 管理、资源管理
// 一个应用通常只创建一个 Provider
// ============================================================

// LoggerProvider 日志提供者（重量级）
type LoggerProvider struct {
	sender       sender.Sender // 底层发送器
	enableCaller bool          // 是否启用调用位置采集
	enableStack  bool          // 是否启用堆栈采集
	callerDepth  int           // 调用栈深度
}

// ProviderOption Provider 配置选项
type ProviderOption func(*providerBuilder)

// providerBuilder Provider 构建器
type providerBuilder struct {
	senderConfig sender.Config
	enableCaller bool
	enableStack  bool
	customSender sender.Sender
}

// ============================================================
// Provider 配置选项
// ============================================================

// WithEndpoint 设置服务端地址
func WithEndpoint(endpoint string) ProviderOption {
	return func(b *providerBuilder) {
		b.senderConfig.Endpoint = endpoint
	}
}

// WithReliability 设置可靠性级别
func WithReliability(level sender.Reliability) ProviderOption {
	return func(b *providerBuilder) {
		b.senderConfig.Reliability = level
	}
}

// WithSender 自定义 Sender（高级用法）
func WithSender(s sender.Sender) ProviderOption {
	return func(b *providerBuilder) {
		b.customSender = s
	}
}

// WithBatchSize 设置批量大小
func WithBatchSize(size int) ProviderOption {
	return func(b *providerBuilder) {
		b.senderConfig.BatchSize = size
	}
}

// WithFlushInterval 设置刷新间隔
func WithFlushInterval(d time.Duration) ProviderOption {
	return func(b *providerBuilder) {
		b.senderConfig.FlushInterval = d
	}
}

// WithBufferSize 设置缓冲区大小
func WithBufferSize(size int) ProviderOption {
	return func(b *providerBuilder) {
		b.senderConfig.BufferSize = size
	}
}

// WithHTTPTimeout 设置 HTTP 超时
func WithHTTPTimeout(d time.Duration) ProviderOption {
	return func(b *providerBuilder) {
		b.senderConfig.Timeout = d
	}
}

// WithFallbackFile 设置降级文件（Balanced 模式）
func WithFallbackFile(path string) ProviderOption {
	return func(b *providerBuilder) {
		b.senderConfig.FallbackFile = path
	}
}

// WithSnapshotDir 设置快照目录（Balanced 模式）
func WithSnapshotDir(dir string) ProviderOption {
	return func(b *providerBuilder) {
		b.senderConfig.SnapshotDir = dir
	}
}

// WithSnapshotInterval 设置快照间隔（Balanced 模式）
func WithSnapshotInterval(d time.Duration) ProviderOption {
	return func(b *providerBuilder) {
		b.senderConfig.SnapshotInterval = d
	}
}

// WithWALDir 设置 WAL 目录（Reliable 模式）
func WithWALDir(dir string) ProviderOption {
	return func(b *providerBuilder) {
		b.senderConfig.WALDir = dir
	}
}

// WithRetry 设置重试配置
func WithRetry(count int, interval time.Duration) ProviderOption {
	return func(b *providerBuilder) {
		b.senderConfig.RetryCount = count
		b.senderConfig.RetryInterval = interval
	}
}

// WithCaller 启用调用位置采集
func WithCaller(enable bool) ProviderOption {
	return func(b *providerBuilder) {
		b.enableCaller = enable
	}
}

// WithStackTrace 启用堆栈采集（仅 error 级别）
func WithStackTrace(enable bool) ProviderOption {
	return func(b *providerBuilder) {
		b.enableStack = enable
	}
}

// ============================================================
// Provider 创建和管理
// ============================================================

// NewLoggerProvider 创建 LoggerProvider（重量级，只创建一次）
//
// 使用示例：
//
//	provider := sdk.NewLoggerProvider(
//	    sdk.WithEndpoint("http://loglite:8080"),
//	    sdk.WithReliability(sender.Reliable),
//	)
//	defer provider.Shutdown()
//
//	userLogger := provider.Logger("user-service")
//	paymentLogger := provider.Logger("payment-service")
func NewLoggerProvider(opts ...ProviderOption) *LoggerProvider {
	// 默认配置
	builder := &providerBuilder{
		senderConfig: sender.Config{
			Endpoint:         "http://localhost:8080",
			Reliability:      sender.Balanced,
			BatchSize:        100,
			FlushInterval:    100 * time.Millisecond,
			BufferSize:       10000,
			Timeout:          5 * time.Second,
			RetryCount:       3,
			RetryInterval:    100 * time.Millisecond,
			SnapshotInterval: 5 * time.Second,
			SnapshotDir:      "./logs/snapshot",
			WALDir:           "./logs/wal",
		},
		enableCaller: true,
		enableStack:  true,
	}

	// 应用选项
	for _, opt := range opts {
		opt(builder)
	}

	// 创建 Provider
	p := &LoggerProvider{
		enableCaller: builder.enableCaller,
		enableStack:  builder.enableStack,
		callerDepth:  4, // Provider -> Logger -> log -> captureContext
	}

	// 创建 Sender
	if builder.customSender != nil {
		p.sender = builder.customSender
	} else {
		p.sender = sender.NewSender(builder.senderConfig)
		if p.sender == nil {
			fmt.Println("[SDK] Sender init failed, fallback to BestEffort mode")
			fallbackCfg := builder.senderConfig
			fallbackCfg.Reliability = sender.BestEffort
			p.sender = sender.NewBestEffortSender(fallbackCfg)
		}
	}

	return p
}

// Logger 获取指定 service 的 Logger（轻量级，可创建多个）
//
// 使用示例：
//
//	userLogger := provider.Logger("user-service")
//	paymentLogger := provider.Logger("payment-service")
func (p *LoggerProvider) Logger(service string) *Logger {
	return &Logger{
		provider: p,
		service:  service,
		fields:   make(map[string]interface{}),
	}
}

// Shutdown 关闭 Provider（释放资源）
func (p *LoggerProvider) Shutdown() error {
	if p.sender != nil {
		return p.sender.Close()
	}
	return nil
}

// Flush 强制刷新所有缓冲区
func (p *LoggerProvider) Flush() error {
	if p.sender != nil {
		return p.sender.Flush()
	}
	return nil
}

// ============================================================
// Context Key 定义
// ============================================================

type contextKey string

const (
	TraceIDKey   contextKey = "trace_id"
	RequestIDKey contextKey = "request_id"
	UserIDKey    contextKey = "user_id"
	LoggerKey    contextKey = "loglite_logger"
)

// ============================================================
// Logger - 轻量级组件（类似 OpenTelemetry 的 Logger）
// 绑定 service，可以创建很多个
// ============================================================

// Logger 日志记录器（轻量级）
type Logger struct {
	provider *LoggerProvider        // 指向 Provider
	service  string                 // 服务名
	fields   map[string]interface{} // 固定字段
}

// ============================================================
// Logger 日志方法
// ============================================================

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

// With 创建带有额外字段的子 Logger
//
// 使用示例：
//
//	orderLogger := userLogger.With("module", "order", "version", "1.0")
//	orderLogger.Info("创建订单")
func (l *Logger) With(keyvals ...interface{}) *Logger {
	// 复制现有字段
	newFields := make(map[string]interface{})
	for k, v := range l.fields {
		newFields[k] = v
	}

	// 添加新字段
	for i := 0; i < len(keyvals)-1; i += 2 {
		if key, ok := keyvals[i].(string); ok {
			newFields[key] = keyvals[i+1]
		}
	}

	return &Logger{
		provider: l.provider,
		service:  l.service,
		fields:   newFields,
	}
}

// WithContext 从 Context 创建子 Logger
func (l *Logger) WithContext(ctx context.Context) *Logger {
	newLogger := &Logger{
		provider: l.provider,
		service:  l.service,
		fields:   make(map[string]interface{}),
	}

	// 复制现有字段
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}

	// 从 context 提取字段
	if traceID, ok := ctx.Value(TraceIDKey).(string); ok {
		newLogger.fields["trace_id"] = traceID
	}
	if requestID, ok := ctx.Value(RequestIDKey).(string); ok {
		newLogger.fields["request_id"] = requestID
	}
	if userID, ok := ctx.Value(UserIDKey).(string); ok {
		newLogger.fields["user_id"] = userID
	}

	return newLogger
}

// ============================================================
// Logger 内部方法
// ============================================================

func (l *Logger) log(level, message string, keyvals ...interface{}) {
	p := l.provider

	// 1. 构造 LogEntry
	entry := &sender.LogEntry{
		ID:        uuid.New().String(),
		Timestamp: time.Now(),
		Message:   message,
		Level:     level,
		Service:   l.service,
		Metadata:  make(map[string]interface{}),
	}

	// 2. 添加固定字段
	for k, v := range l.fields {
		switch k {
		case "trace_id":
			entry.TraceID = fmt.Sprintf("%v", v)
		case "span_id":
			entry.SpanID = fmt.Sprintf("%v", v)
		case "request_id":
			entry.RequestID = fmt.Sprintf("%v", v)
		case "user_id":
			entry.UserID = fmt.Sprintf("%v", v)
		case "ip":
			entry.IP = fmt.Sprintf("%v", v)
		default:
			entry.Metadata[k] = v
		}
	}

	// 3. 解析 keyvals
	for i := 0; i < len(keyvals)-1; i += 2 {
		key, ok := keyvals[i].(string)
		if !ok {
			continue
		}
		value := keyvals[i+1]

		switch key {
		case "trace_id":
			entry.TraceID = fmt.Sprintf("%v", value)
		case "span_id":
			entry.SpanID = fmt.Sprintf("%v", value)
		case "request_id":
			entry.RequestID = fmt.Sprintf("%v", value)
		case "user_id":
			entry.UserID = fmt.Sprintf("%v", value)
		case "ip":
			entry.IP = fmt.Sprintf("%v", value)
		default:
			entry.Metadata[key] = value
		}
	}

	// 4. 自动采集调用上下文
	if p.enableCaller {
		if caller := captureContext(p.callerDepth); caller != nil {
			entry.Caller = fmt.Sprintf("%s:%d", caller.File, caller.Line)
			entry.Function = caller.Function
			entry.Package = caller.Package
		}
	}

	// 5. error 级别自动采集堆栈
	if level == "error" && p.enableStack {
		entry.StackTrace = captureStack(p.callerDepth)
		entry.StackHash = hashStack(entry.StackTrace)
	}

	// 6. 发送
	p.sender.Send(entry)
}

// Flush 强制刷新缓冲区
func (l *Logger) Flush() error {
	return l.provider.Flush()
}

// ============================================================
// 全局 Provider 和便捷函数
// ============================================================

var (
	globalProvider *LoggerProvider
	defaultLogger  *Logger
)

// SetGlobalProvider 设置全局 Provider
func SetGlobalProvider(p *LoggerProvider) {
	globalProvider = p
}

// GlobalProvider 获取全局 Provider
func GlobalProvider() *LoggerProvider {
	return globalProvider
}

// Global 获取全局 Logger（需要指定 service）
func Global(service string) *Logger {
	if globalProvider == nil {
		return nil
	}
	return globalProvider.Logger(service)
}

// SetDefaultLogger 设置默认 Logger（用于中间件）
func SetDefaultLogger(l *Logger) {
	defaultLogger = l
}

// DefaultLogger 获取默认 Logger
func DefaultLogger() *Logger {
	return defaultLogger
}

// FromContext 从 Context 获取 Logger
func FromContext(ctx context.Context) *Logger {
	if l, ok := ctx.Value(LoggerKey).(*Logger); ok {
		return l
	}
	return nil
}

// ToContext 将 Logger 存入 Context
func ToContext(ctx context.Context, l *Logger) context.Context {
	return context.WithValue(ctx, LoggerKey, l)
}
