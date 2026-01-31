package main

import (
	"fmt"
	"time"

	"github.com/loglite/loglite/pkg/sdk"
)

func main() {
	// 初始化 LogLite 客户端
	client := sdk.Init("demo-service",
		sdk.WithEndpoint("http://localhost:8080"),
		sdk.WithBatchSize(50),
		sdk.WithFlushInterval(100*time.Millisecond),
		sdk.WithCaller(true),
		sdk.WithStackTrace(true),
	)
	defer client.Close()

	// 设置为默认客户端（可选）
	sdk.SetDefault(client)

	// 基本使用
	client.Info("应用启动", "version", "1.0.0", "env", "development")
	client.Debug("调试信息", "config", map[string]string{"db": "localhost"})
	client.Warn("警告信息", "memory_usage", "85%")
	client.Error("错误信息", "error", "connection timeout", "retry", 3)

	// 使用 With 添加固定字段
	userLogger := client.With(map[string]interface{}{
		"user_id": "user-123",
		"role":    "admin",
	})
	userLogger.Info("用户操作", "action", "login")
	userLogger.Info("用户操作", "action", "view_dashboard")

	// 模拟业务场景
	for i := 0; i < 10; i++ {
		processOrder(client, fmt.Sprintf("ORD-%d", i+1))
	}

	// 等待日志发送完成
	fmt.Println("日志已发送，请在 Web UI 查看: http://localhost:8080")
}

func processOrder(client *sdk.Client, orderID string) {
	logger := client.With(map[string]interface{}{
		"order_id": orderID,
	})

	logger.Info("开始处理订单")

	// 模拟处理
	time.Sleep(10 * time.Millisecond)

	// 随机产生一些警告和错误
	if orderID == "ORD-3" {
		logger.Warn("订单处理警告", "reason", "库存不足")
	}
	if orderID == "ORD-7" {
		logger.Error("订单处理失败", "error", "支付超时")
		return
	}

	logger.Info("订单处理完成", "latency_ms", 10)
}
