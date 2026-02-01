package gorm

import (
	"context"
	"fmt"
	"time"

	"github.com/loglite/loglite/pkg/sdk"
	"gorm.io/gorm/logger"
)

// ============================================================
// GORM Hook 框架
// 自动记录慢查询、错误日志、SQL 追踪
// ============================================================

// Logger GORM 日志记录器
type Logger struct {
	logger *sdk.Logger
	config Config
}

// Config 日志配置
type Config struct {
	SlowThreshold time.Duration // 慢查询阈值
	LogLevel      logger.LogLevel
	Colorful      bool
}

// Option 配置选项
type Option func(*Config)

// WithSlowThreshold 设置慢查询阈值
func WithSlowThreshold(d time.Duration) Option {
	return func(c *Config) {
		c.SlowThreshold = d
	}
}

// WithLogLevel 设置日志级别
func WithLogLevel(level logger.LogLevel) Option {
	return func(c *Config) {
		c.LogLevel = level
	}
}

// NewLogger 创建 GORM 日志记录器
func NewLogger(l *sdk.Logger, opts ...Option) *Logger {
	config := Config{
		SlowThreshold: 200 * time.Millisecond,
		LogLevel:      logger.Warn,
		Colorful:      false,
	}

	for _, opt := range opts {
		opt(&config)
	}

	return &Logger{
		logger: l,
		config: config,
	}
}

// LogMode 设置日志级别
func (l *Logger) LogMode(level logger.LogLevel) logger.Interface {
	newLogger := *l
	newLogger.config.LogLevel = level
	return &newLogger
}

// Info 记录 info 日志
func (l *Logger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.config.LogLevel >= logger.Info {
		l.logger.Info(fmt.Sprintf(msg, data...))
	}
}

// Warn 记录 warn 日志
func (l *Logger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.config.LogLevel >= logger.Warn {
		l.logger.Warn(fmt.Sprintf(msg, data...))
	}
}

// Error 记录 error 日志
func (l *Logger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.config.LogLevel >= logger.Error {
		l.logger.Error(fmt.Sprintf(msg, data...))
	}
}

// Trace 记录 SQL 执行日志
func (l *Logger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.config.LogLevel <= logger.Silent {
		return
	}

	elapsed := time.Since(begin)
	sql, rows := fc()

	// 记录错误
	if err != nil && l.config.LogLevel >= logger.Error {
		l.logger.Error("SQL Error",
			"sql", sql,
			"error", err.Error(),
			"elapsed_ms", float64(elapsed.Milliseconds()),
			"rows", rows,
		)
		return
	}

	// 记录慢查询
	if elapsed > l.config.SlowThreshold && l.config.LogLevel >= logger.Warn {
		l.logger.Warn("Slow SQL",
			"sql", sql,
			"elapsed_ms", float64(elapsed.Milliseconds()),
			"threshold_ms", float64(l.config.SlowThreshold.Milliseconds()),
			"rows", rows,
		)
		return
	}

	// 记录普通日志
	if l.config.LogLevel >= logger.Info {
		l.logger.Info("SQL",
			"sql", sql,
			"elapsed_ms", float64(elapsed.Milliseconds()),
			"rows", rows,
		)
	}
}

// ============================================================
// 使用示例
// ============================================================

// Example 使用示例
func Example() {
	// 1. 创建 LoggerProvider
	provider := sdk.NewLoggerProvider(
		sdk.WithEndpoint("http://localhost:8081"),
	)
	defer provider.Shutdown()

	// 2. 获取 Logger
	logger := provider.Logger("my-service")

	// 3. 创建 GORM Logger
	gormLogger := NewLogger(logger,
		WithSlowThreshold(200*time.Millisecond),
	)

	// 4. 配置 GORM
	// db, _ := gorm.Open(mysql.Open(dsn), &gorm.Config{
	//     Logger: gormLogger,
	// })

	// 自动记录：
	// - Slow SQL（慢查询）
	// - SQL Error（错误）
	// - SQL（普通查询，如果 LogLevel >= Info）
	_ = gormLogger
}
