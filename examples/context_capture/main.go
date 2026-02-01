package main

import (
	"fmt"
	"time"

	"github.com/loglite/loglite/pkg/sdk"
)

// 全局 Provider 和 Logger
var (
	provider *sdk.LoggerProvider
	logger   *sdk.Logger
)

// ============================================================
// 演示：自动上下文采集
// ============================================================

func main() {
	// 1. 创建 LoggerProvider
	provider = sdk.NewLoggerProvider(
		sdk.WithEndpoint("http://localhost:8080"),
		sdk.WithCaller(true),     // 启用调用位置采集
		sdk.WithStackTrace(true), // 启用堆栈采集（error 级别）
	)
	defer provider.Shutdown()

	// 2. 获取 Logger
	logger = provider.Logger("context-demo")

	// 3. 设置为默认 Logger（可选，用于中间件）
	sdk.SetDefaultLogger(logger)

	fmt.Println("=== 演示自动上下文采集 ===")
	fmt.Println()

	// ============================================================
	// 场景 1: Info 日志 - 自动采集 Caller, Function, Package
	// ============================================================

	fmt.Println("1. Info 日志（自动采集调用位置）")
	logger.Info("用户登录成功",
		"user_id", 12345,
		"ip", "192.168.1.100",
	)

	// 生成的日志会包含：
	// {
	//   "message": "用户登录成功",
	//   "level": "info",
	//   "caller": "main.go:XX",        ← 自动采集
	//   "function": "main",            ← 自动采集
	//   "package": "main",             ← 自动采集
	//   "metadata": {
	//     "user_id": 12345,
	//     "ip": "192.168.1.100"
	//   }
	// }

	time.Sleep(100 * time.Millisecond)

	// ============================================================
	// 场景 2: Error 日志 - 自动采集完整堆栈 + 指纹
	// ============================================================

	fmt.Println("2. Error 日志（自动采集堆栈和指纹）")

	// 模拟多层调用
	processOrder()

	time.Sleep(100 * time.Millisecond)

	// ============================================================
	// 场景 3: 结构化日志 - 保留上下文
	// ============================================================

	fmt.Println("3. 结构化日志（链式调用保留上下文）")

	orderLogger := logger.With(
		"module", "order",
		"version", "1.0.0",
	)

	orderLogger.Info("订单创建成功",
		"order_id", "ORD-2024-001",
		"amount", 99.99,
	)

	// 生成的日志会包含：
	// {
	//   "caller": "main.go:XX",        ← 自动采集
	//   "function": "main",            ← 自动采集
	//   "metadata": {
	//     "module": "order",           ← 预设字段
	//     "version": "1.0.0",          ← 预设字段
	//     "order_id": "ORD-2024-001",  ← 动态字段
	//     "amount": 99.99              ← 动态字段
	//   }
	// }

	time.Sleep(100 * time.Millisecond)

	// ============================================================
	// 场景 4: 错误聚合演示
	// ============================================================

	fmt.Println("4. 错误聚合（相同错误生成相同指纹）")

	// 模拟同一个错误在不同地方发生
	simulateDatabaseError("user-service")
	simulateDatabaseError("order-service")
	simulateDatabaseError("payment-service")

	// 服务端会根据 stack_hash 自动聚合这些错误
	// stack_hash 相同 → 同一类错误
	// 即使发生在不同的服务中，也能识别为同一个问题

	time.Sleep(200 * time.Millisecond)

	// ============================================================
	// 场景 5: 多服务共享 Provider
	// ============================================================

	fmt.Println("5. 多服务共享 Provider")

	// 同一个 Provider，不同的 service
	userLogger := provider.Logger("user-service")
	paymentLogger := provider.Logger("payment-service")

	userLogger.Info("用户注册成功", "user_id", 12345)
	paymentLogger.Info("支付成功", "order_id", "ORD-001", "amount", 99.99)

	time.Sleep(100 * time.Millisecond)

	fmt.Println("\n✅ 所有日志已发送，查看服务端输出")
}

// processOrder 模拟多层调用
func processOrder() {
	validateOrder()
}

func validateOrder() {
	checkInventory()
}

func checkInventory() {
	// 这里发生错误
	logger.Error("库存不足",
		"product_id", "PROD-001",
		"required", 10,
		"available", 3,
	)

	// 生成的日志会包含完整的调用链：
	// {
	//   "message": "库存不足",
	//   "level": "error",
	//   "caller": "main.go:XX",                    ← 自动采集
	//   "function": "checkInventory",              ← 自动采集
	//   "package": "main",                         ← 自动采集
	//   "stack_trace": "main.checkInventory\n      ← 完整堆栈
	//                   \tmain.go:XX\n
	//                   main.validateOrder\n
	//                   \tmain.go:YY\n
	//                   main.processOrder\n
	//                   \tmain.go:ZZ\n
	//                   main.main\n
	//                   \tmain.go:WW\n",
	//   "stack_hash": "a1b2c3d4e5f6g7h8",         ← 错误指纹
	//   "metadata": {
	//     "product_id": "PROD-001",
	//     "required": 10,
	//     "available": 3
	//   }
	// }
}

// simulateDatabaseError 模拟数据库错误
func simulateDatabaseError(serviceName string) {
	connectDatabase(serviceName)
}

func connectDatabase(serviceName string) {
	queryDatabase(serviceName)
}

func queryDatabase(serviceName string) {
	logger.Error("数据库连接超时",
		"service", serviceName,
		"timeout", "5s",
	)

	// 由于调用链相同（simulateDatabaseError → connectDatabase → queryDatabase）
	// 即使 serviceName 不同，stack_hash 也会相同
	// 服务端可以将这些错误聚合在一起：
	//
	// 错误类型: 数据库连接超时
	// stack_hash: x1y2z3a4b5c6d7e8
	// 发生次数: 3
	// 发生服务:
	//   - user-service
	//   - order-service
	//   - payment-service
}
