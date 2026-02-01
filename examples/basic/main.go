package main

import (
	"time"

	"github.com/loglite/loglite/pkg/sdk"
	"github.com/loglite/loglite/pkg/sdk/sender"
)

func main() {
	// ============================================================
	// LogLite SDK 使用示例（OpenTelemetry 风格）
	// ============================================================

	// 1. 创建 LoggerProvider（重量级，只创建一次）
	provider := sdk.NewLoggerProvider(
		sdk.WithEndpoint("http://localhost:8081"),
		sdk.WithReliability(sender.Balanced),
	)
	defer provider.Shutdown()

	// 2. 获取 Logger（轻量级，可创建多个，共享 Provider）
	userLogger := provider.Logger("user-service")
	paymentLogger := provider.Logger("payment-service")
	orderLogger := provider.Logger("order-service")

	// 3. 使用
	userLogger.Info("用户登录", "user_id", 12345, "ip", "192.168.1.1")
	paymentLogger.Info("支付成功", "order_id", "ORD-001", "amount", 99.99)
	orderLogger.Info("订单创建", "order_id", "ORD-001", "items", 3)

	// ============================================================
	// 场景：不同业务需要不同可靠性
	// ============================================================

	// 关键业务使用 Reliable 模式
	criticalProvider := sdk.NewLoggerProvider(
		sdk.WithEndpoint("http://localhost:8081"),
		sdk.WithReliability(sender.Reliable),
		sdk.WithWALDir("./logs/critical-wal"),
	)
	defer criticalProvider.Shutdown()

	auditLogger := criticalProvider.Logger("audit-service")
	transactionLogger := criticalProvider.Logger("transaction-service")

	auditLogger.Info("用户权限变更", "user_id", 12345, "role", "admin")
	transactionLogger.Info("资金转账", "from", "A", "to", "B", "amount", 10000)

	// ============================================================
	// With：添加固定字段
	// ============================================================

	orderModuleLogger := orderLogger.With("module", "order", "version", "2.0")
	orderModuleLogger.Info("订单处理开始")
	orderModuleLogger.Warn("库存不足", "product_id", "PROD-001")
	orderModuleLogger.Error("订单创建失败", "error", "db connection timeout")

	// 等待日志发送完成
	time.Sleep(500 * time.Millisecond)
}
