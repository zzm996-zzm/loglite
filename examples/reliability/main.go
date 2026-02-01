package main

import (
	"fmt"
	"time"

	"github.com/loglite/loglite/pkg/sdk"
	"github.com/loglite/loglite/pkg/sdk/sender"
)

// ============================================================
// 可靠性级别演示（OpenTelemetry 风格 API）
// ============================================================

func main() {
	fmt.Println("=== LogLite 可靠性级别演示 ===\n")

	// ============================================================
	// 1. BestEffort - 高性能（开发/测试环境）
	// ============================================================
	fmt.Println("【1】BestEffort 模式（高性能，允许丢失）")

	bestEffortProvider := sdk.NewLoggerProvider(
		sdk.WithEndpoint("http://localhost:8081"),
		sdk.WithReliability(sender.BestEffort),
		sdk.WithBufferSize(10000),
	)
	defer bestEffortProvider.Shutdown()

	devLogger := bestEffortProvider.Logger("dev-service")

	for i := 0; i < 100; i++ {
		devLogger.Debug("debug日志", "index", i)
	}

	fmt.Println("  ✓ 发送 100 条 debug 日志")
	fmt.Println("  特点: Ring Buffer，满了就丢弃")
	fmt.Println()

	// ============================================================
	// 2. Balanced - 平衡模式（生产环境默认）
	// ============================================================
	fmt.Println("【2】Balanced 模式（平衡性能和可靠性）")

	balancedProvider := sdk.NewLoggerProvider(
		sdk.WithEndpoint("http://localhost:8081"),
		sdk.WithReliability(sender.Balanced),
		sdk.WithFallbackFile("./logs/fallback.log"),
		sdk.WithSnapshotDir("./logs/snapshot"),
		sdk.WithSnapshotInterval(5*time.Second),
	)
	defer balancedProvider.Shutdown()

	// 多服务共享同一个 Provider
	userLogger := balancedProvider.Logger("user-service")
	orderLogger := balancedProvider.Logger("order-service")

	userLogger.Info("用户登录", "user_id", 12345)
	orderLogger.Warn("库存不足", "product_id", "PROD-001")
	orderLogger.Error("支付失败", "order_id", "ORD-001", "error", "timeout")

	fmt.Println("  ✓ 发送 3 条日志（2个服务共享 Provider）")
	fmt.Println("  特点: Double Buffer + 定期快照 + 降级文件")
	fmt.Println("  崩溃恢复: 最多丢失 5 秒数据")
	fmt.Println()

	// ============================================================
	// 3. Reliable - 零丢失（关键业务）
	// ============================================================
	fmt.Println("【3】Reliable 模式（零丢失，WAL）")

	reliableProvider := sdk.NewLoggerProvider(
		sdk.WithEndpoint("http://localhost:8081"),
		sdk.WithReliability(sender.Reliable),
		sdk.WithWALDir("./logs/wal"),
		sdk.WithRetry(3, 100*time.Millisecond),
	)
	defer reliableProvider.Shutdown()

	paymentLogger := reliableProvider.Logger("payment-service")
	auditLogger := reliableProvider.Logger("audit-service")

	paymentLogger.Info("订单创建", "order_id", "ORD-2024-001", "amount", 999.99)
	paymentLogger.Info("支付成功", "order_id", "ORD-2024-001", "transaction_id", "TXN-123")
	auditLogger.Info("权限变更", "user_id", "admin", "action", "grant_role")

	fmt.Println("  ✓ 发送 3 条关键日志（2个服务共享 Provider）")
	fmt.Println("  特点: Write-Ahead Log，先写磁盘再返回")
	fmt.Println("  崩溃恢复: 零丢失，重启后自动重发")
	fmt.Println()

	// ============================================================
	// 4. 性能对比测试
	// ============================================================
	fmt.Println("【4】性能对比测试")

	testCount := 1000

	// BestEffort
	start := time.Now()
	for i := 0; i < testCount; i++ {
		devLogger.Info("performance test", "index", i)
	}
	bestEffortTime := time.Since(start)

	// Balanced
	start = time.Now()
	for i := 0; i < testCount; i++ {
		userLogger.Info("performance test", "index", i)
	}
	balancedTime := time.Since(start)

	// Reliable
	start = time.Now()
	for i := 0; i < testCount; i++ {
		paymentLogger.Info("performance test", "index", i)
	}
	reliableTime := time.Since(start)

	fmt.Printf("  BestEffort:  %d 条 / %v = %.0f ops/s\n",
		testCount, bestEffortTime, float64(testCount)/bestEffortTime.Seconds())
	fmt.Printf("  Balanced:    %d 条 / %v = %.0f ops/s\n",
		testCount, balancedTime, float64(testCount)/balancedTime.Seconds())
	fmt.Printf("  Reliable:    %d 条 / %v = %.0f ops/s\n",
		testCount, reliableTime, float64(testCount)/reliableTime.Seconds())
	fmt.Println()

	// ============================================================
	// 5. 场景推荐
	// ============================================================
	fmt.Println("【5】使用场景推荐")
	fmt.Println()
	fmt.Println("  场景                           推荐模式      理由")
	fmt.Println("  ─────────────────────────────────────────────────────")
	fmt.Println("  开发/测试环境                  BestEffort    性能最高")
	fmt.Println("  普通业务日志（生产）           Balanced      平衡")
	fmt.Println("  关键业务日志                   Reliable      零丢失")
	fmt.Println("  订单/支付日志                  Reliable      审计要求")
	fmt.Println("  Debug 日志                     BestEffort    量大允许丢")
	fmt.Println("  用户行为日志                   Balanced      重要但不绝对")
	fmt.Println()

	// 等待发送完成
	fmt.Println("等待日志发送完成...")
	time.Sleep(1 * time.Second)

	fmt.Println("\n✅ 所有示例完成")
}
