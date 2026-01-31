package sender_test

import (
	"time"

	"github.com/loglite/loglite/pkg/sdk/sender"
)

// 示例：使用不同的可靠性级别
func Example_reliability() {
	// ============================================================
	// 模式 1: BestEffort - 最高性能
	// ============================================================
	// 适合场景：
	// - 开发/测试环境
	// - 日志量巨大，允许少量丢失
	// - Debug 级别日志
	bestEffort := sender.NewSender(sender.Config{
		Endpoint:      "http://localhost:8081",
		Service:       "my-service",
		Reliability:   sender.BestEffort,
		BatchSize:     100,
		FlushInterval: 100 * time.Millisecond,
		BufferSize:    10000, // 满了就丢
	})
	defer bestEffort.Close()

	// ============================================================
	// 模式 2: Balanced - 平衡模式
	// ============================================================
	// 适合场景：
	// - 生产环境
	// - 普通业务日志
	// - 可以容忍极端情况下丢失 5s 日志
	balanced := sender.NewSender(sender.Config{
		Endpoint:         "http://localhost:8081",
		Service:          "my-service",
		Reliability:      sender.Balanced,
		BatchSize:        100,
		FlushInterval:    100 * time.Millisecond,
		SnapshotInterval: 5 * time.Second, // 每 5s 快照
		SnapshotDir:      "./logs/snapshot",
		FallbackFile:     "./logs/fallback.log", // 发送失败降级
	})
	defer balanced.Close()

	// ============================================================
	// 模式 3: Reliable - 零丢失
	// ============================================================
	// 适合场景：
	// - 关键业务日志
	// - 审计日志
	// - 金融交易日志
	reliable := sender.NewSender(sender.Config{
		Endpoint:      "http://localhost:8081",
		Service:       "my-service",
		Reliability:   sender.Reliable,
		BatchSize:     100,
		FlushInterval: 100 * time.Millisecond,
		WALDir:        "./logs/wal", // WAL 目录
		RetryCount:    3,            // 重试 3 次
		RetryInterval: 100 * time.Millisecond,
	})
	defer reliable.Close()

	// 发送日志
	log := &sender.LogEntry{
		Timestamp: time.Now(),
		Message:   "test message",
		Level:     "info",
		Service:   "my-service",
	}

	bestEffort.Send(log)
	balanced.Send(log)
	reliable.Send(log)
}

// 示例：多种发送方式
func Example_sendMethods() {
	s := sender.NewSender(sender.DefaultConfig())
	defer s.Close()

	// ============================================================
	// 方式 1: 直接发送 LogEntry
	// ============================================================
	s.Send(&sender.LogEntry{
		Timestamp: time.Now(),
		Message:   "user login",
		Level:     "info",
		Service:   "auth-service",
		Metadata: map[string]interface{}{
			"user_id": 12345,
			"ip":      "192.168.1.1",
		},
	})

	// ============================================================
	// 方式 2: 发送 Map（灵活格式）
	// ============================================================
	// 适合与其他日志框架集成
	s.SendMap(map[string]interface{}{
		"message":  "order created",
		"level":    "info",
		"order_id": 67890,
		"amount":   99.99,
	})

	// ============================================================
	// 方式 3: 发送 LogRecord（自定义类型）
	// ============================================================
	// 适合包装 zap/zerolog/slog 等框架
	record := &sender.ZapLogRecord{
		Level:    "error",
		Time:     time.Now(),
		Message:  "database connection failed",
		Caller:   "db/pool.go:42",
		Function: "Connect",
		Fields: map[string]interface{}{
			"error": "timeout",
			"retry": 3,
		},
	}
	s.SendRecord(record)
}

// 示例：与 zap 集成（伪代码）
func Example_zapIntegration() {
	// 创建 sender
	s := sender.NewSender(sender.Config{
		Endpoint:    "http://localhost:8081",
		Service:     "my-service",
		Reliability: sender.Balanced,
	})
	defer s.Close()

	// ============================================================
	// 方式 1: 通过 zap WriteSyncer
	// ============================================================
	//
	// import "go.uber.org/zap/zapcore"
	//
	// type LogliteWriter struct {
	//     sender sender.Sender
	// }
	//
	// func (w *LogliteWriter) Write(p []byte) (n int, err error) {
	//     // 解析 JSON，发送到 loglite
	//     var data map[string]interface{}
	//     json.Unmarshal(p, &data)
	//     w.sender.SendMap(data)
	//     return len(p), nil
	// }
	//
	// func (w *LogliteWriter) Sync() error {
	//     return w.sender.Flush()
	// }
	//
	// // 创建 zap logger
	// core := zapcore.NewCore(
	//     zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
	//     &LogliteWriter{sender: s},
	//     zapcore.InfoLevel,
	// )
	// logger := zap.New(core)
	// logger.Info("hello loglite")

	// ============================================================
	// 方式 2: 通过 zap Hook
	// ============================================================
	//
	// type LogliteHook struct {
	//     sender sender.Sender
	// }
	//
	// func (h *LogliteHook) OnWrite(entry *buffer.Buffer) error {
	//     // 解析并发送
	//     return nil
	// }
}

// 示例：与 zerolog 集成（伪代码）
func Example_zerologIntegration() {
	// 创建 sender
	s := sender.NewSender(sender.DefaultConfig())
	defer s.Close()

	// ============================================================
	// zerolog 使用 Hook 方式
	// ============================================================
	//
	// import "github.com/rs/zerolog"
	//
	// type LogliteHook struct {
	//     sender sender.Sender
	// }
	//
	// func (h LogliteHook) Run(e *zerolog.Event, level zerolog.Level, msg string) {
	//     // zerolog 的 hook 在日志写入时触发
	//     h.sender.SendMap(map[string]interface{}{
	//         "level":   level.String(),
	//         "message": msg,
	//     })
	// }
	//
	// // 使用 hook
	// log := zerolog.New(os.Stdout).Hook(LogliteHook{sender: s})
	// log.Info().Str("user", "alice").Msg("logged in")
}

// 示例：与 slog 集成（Go 1.21+，伪代码）
func Example_slogIntegration() {
	// 创建 sender
	s := sender.NewSender(sender.DefaultConfig())
	defer s.Close()

	// ============================================================
	// slog 使用自定义 Handler
	// ============================================================
	//
	// import "log/slog"
	//
	// type LogliteHandler struct {
	//     sender  sender.Sender
	//     service string
	// }
	//
	// func (h *LogliteHandler) Handle(ctx context.Context, r slog.Record) error {
	//     fields := make(map[string]interface{})
	//     r.Attrs(func(a slog.Attr) bool {
	//         fields[a.Key] = a.Value.Any()
	//         return true
	//     })
	//
	//     record := &sender.SlogRecord{
	//         Level:   r.Level.String(),
	//         Time:    r.Time,
	//         Message: r.Message,
	//         Fields:  fields,
	//     }
	//     return h.sender.SendRecord(record)
	// }
	//
	// func (h *LogliteHandler) WithAttrs(attrs []slog.Attr) slog.Handler { ... }
	// func (h *LogliteHandler) WithGroup(name string) slog.Handler { ... }
	// func (h *LogliteHandler) Enabled(ctx context.Context, level slog.Level) bool { ... }
	//
	// // 使用
	// logger := slog.New(&LogliteHandler{sender: s, service: "my-app"})
	// logger.Info("hello", "user", "bob")
}
