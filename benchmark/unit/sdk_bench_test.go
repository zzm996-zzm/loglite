// benchmarks/unit/sdk_bench_test.go
package unit

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/loglite/loglite/pkg/sdk"
	"github.com/loglite/loglite/pkg/sdk/sender"
)

// BenchmarkSDK_Info 测试 Info 方法基础性能
func BenchmarkSDK_Info(b *testing.B) {
	provider := sdk.NewLoggerProvider(
		sdk.WithEndpoint("http://localhost:8081"),
		sdk.WithReliability(sender.Balanced),
	)
	defer provider.Shutdown()

	logger := provider.Logger("bench-service")

	// 预生成测试数据
	testData := []struct {
		message string
		fields  []interface{}
	}{
		{"用户登录", []interface{}{"user_id", 12345, "ip", "192.168.1.1"}},
		{"订单创建", []interface{}{"order_id", "ORD-001", "amount", 99.99}},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		data := testData[i%len(testData)]
		logger.Info(data.message, data.fields...)
	}
}

// BenchmarkSDK_Error 测试 Error 方法性能（包含堆栈采集）
func BenchmarkSDK_Error(b *testing.B) {
	provider := sdk.NewLoggerProvider(
		sdk.WithEndpoint("http://localhost:8081"),
		sdk.WithReliability(sender.Balanced),
		sdk.WithStackTrace(true), // 开启堆栈采集
	)
	defer provider.Shutdown()

	logger := provider.Logger("bench-service")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		logger.Error("数据库连接失败",
			"error", "connection timeout",
			"retry_count", 3,
			"service", "mysql",
		)
	}
}

// BenchmarkSDK_With 测试 With 方法性能
func BenchmarkSDK_With(b *testing.B) {
	provider := sdk.NewLoggerProvider(
		sdk.WithEndpoint("http://localhost:8081"),
		sdk.WithReliability(sender.Balanced),
	)
	defer provider.Shutdown()

	logger := provider.Logger("bench-service")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		subLogger := logger.With(
			"trace_id", "trace-123",
			"request_id", "req-456",
			"user_id", 789,
		)
		subLogger.Info("请求处理", "path", "/api/users", "method", "GET")
	}
}

// BenchmarkSDK_WithContext 测试 WithContext 性能
func BenchmarkSDK_WithContext(b *testing.B) {
	provider := sdk.NewLoggerProvider(
		sdk.WithEndpoint("http://localhost:8081"),
		sdk.WithReliability(sender.Balanced),
	)
	defer provider.Shutdown()

	logger := provider.Logger("bench-service")

	ctx := context.WithValue(context.Background(), sdk.TraceIDKey, "trace-123")
	ctx = context.WithValue(ctx, sdk.UserIDKey, "user-456")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ctxLogger := logger.WithContext(ctx)
		ctxLogger.Info("API调用", "endpoint", "/api/orders")
	}
}

// BenchmarkSDK_Parallel 并发性能测试
func BenchmarkSDK_Parallel(b *testing.B) {
	provider := sdk.NewLoggerProvider(
		sdk.WithEndpoint("http://localhost:8081"),
		sdk.WithReliability(sender.Balanced),
	)
	defer provider.Shutdown()

	// ✅ 修复：在外面创建 logger，避免重复创建
	logger := provider.Logger("bench-service")

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			logger.Info("并发日志",
				"goroutine", i,
				"timestamp", time.Now().UnixNano(),
			)
			i++
		}
	})
}

// BenchmarkSDK_ReliabilityModes 测试不同可靠性模式性能
func BenchmarkSDK_ReliabilityModes(b *testing.B) {
	tests := []struct {
		name string
		mode sender.Reliability
	}{
		{"BestEffort", sender.BestEffort},
		{"Balanced", sender.Balanced},
		{"Reliable", sender.Reliable},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			provider := sdk.NewLoggerProvider(
				sdk.WithEndpoint("http://localhost:8081"),
				sdk.WithReliability(tt.mode),
				sdk.WithWALDir(b.TempDir()+"/wal"), // 为 Reliable 模式提供临时目录
			)
			defer provider.Shutdown()

			logger := provider.Logger("bench-service")

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				logger.Info("性能测试", "mode", tt.name, "index", i)
			}
		})
	}
}

// BenchmarkSDK_CallerCapture 测试调用位置采集的性能开销
func BenchmarkSDK_CallerCapture(b *testing.B) {
	tests := []struct {
		name         string
		enableCaller bool
	}{
		{"WithoutCaller", false},
		{"WithCaller", true},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			provider := sdk.NewLoggerProvider(
				sdk.WithEndpoint("http://localhost:8081"),
				sdk.WithReliability(sender.BestEffort),
				sdk.WithCaller(tt.enableCaller),
			)
			defer provider.Shutdown()

			logger := provider.Logger("bench-service")

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				logger.Info("测试日志", "index", i)
			}
		})
	}
}

// BenchmarkSDK_FieldCount 测试不同字段数量的性能影响
func BenchmarkSDK_FieldCount(b *testing.B) {
	provider := sdk.NewLoggerProvider(
		sdk.WithEndpoint("http://localhost:8081"),
		sdk.WithReliability(sender.BestEffort),
	)
	defer provider.Shutdown()

	logger := provider.Logger("bench-service")

	tests := []struct {
		name   string
		fields []interface{}
	}{
		{"0Fields", []interface{}{}},
		{"2Fields", []interface{}{"k1", "v1"}},
		{"4Fields", []interface{}{"k1", "v1", "k2", "v2"}},
		{"8Fields", []interface{}{"k1", "v1", "k2", "v2", "k3", "v3", "k4", "v4"}},
		{"16Fields", []interface{}{"k1", "v1", "k2", "v2", "k3", "v3", "k4", "v4", "k5", "v5", "k6", "v6", "k7", "v7", "k8", "v8"}},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				logger.Info("测试消息", tt.fields...)
			}
		})
	}
}

// BenchmarkSDK_MultiService 测试多服务共享 Provider 的性能
func BenchmarkSDK_MultiService(b *testing.B) {
	provider := sdk.NewLoggerProvider(
		sdk.WithEndpoint("http://localhost:8081"),
		sdk.WithReliability(sender.Balanced),
	)
	defer provider.Shutdown()

	// 创建多个 service 的 logger
	loggers := make([]*sdk.Logger, 10)
	for i := 0; i < 10; i++ {
		loggers[i] = provider.Logger(fmt.Sprintf("service-%d", i))
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		logger := loggers[i%len(loggers)]
		logger.Info("多服务日志", "index", i)
	}
}
