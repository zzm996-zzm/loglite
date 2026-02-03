package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/loglite/loglite/pkg/sdk"
	"github.com/loglite/loglite/pkg/sdk/sender"
)

func main() {
	fmt.Println("=== LogLite 负载测试脚本 ===")
	fmt.Println("使用 Balanced 模式插入 10,000 条日志")
	fmt.Println()

	// 创建 LoggerProvider（Balanced 模式）
	provider := sdk.NewLoggerProvider(
		sdk.WithEndpoint("http://localhost:8081"),
		sdk.WithReliability(sender.Balanced),
		sdk.WithBatchSize(100),
		sdk.WithFlushInterval(100*time.Millisecond),
		sdk.WithSnapshotDir("./logs/snapshot"),
		sdk.WithFallbackFile("./logs/fallback.log"),
	)
	defer provider.Shutdown()

	// 定义服务和场景
	services := []string{
		"user-service",
		"payment-service",
		"order-service",
		"inventory-service",
		"notification-service",
		"auth-service",
		"api-gateway",
		"search-service",
	}

	// 创建每个服务的 logger
	loggers := make(map[string]*sdk.Logger)
	for _, service := range services {
		loggers[service] = provider.Logger(service)
	}

	// 日志模板数据
	userActions := []string{
		"用户登录", "用户注册", "用户登出", "修改密码", "更新资料",
		"查看订单", "添加购物车", "提交订单", "取消订单", "申请退款",
	}
	paymentActions := []string{
		"支付成功", "支付失败", "退款处理", "支付超时", "余额不足",
		"支付回调", "对账完成", "风控拦截", "支付重试", "订单支付",
	}
	orderActions := []string{
		"订单创建", "订单支付", "订单发货", "订单完成", "订单取消",
		"订单退款", "订单超时", "库存扣减", "库存回退", "订单查询",
	}
	inventoryActions := []string{
		"库存查询", "库存扣减", "库存回退", "库存预警", "库存同步",
		"补货完成", "库存盘点", "库存调整", "缺货通知", "库存锁定",
	}
	notificationActions := []string{
		"发送短信", "发送邮件", "推送通知", "消息队列", "通知失败",
		"通知成功", "模板渲染", "渠道切换", "重试发送", "批量发送",
	}
	authActions := []string{
		"登录验证", "Token 生成", "Token 刷新", "权限检查", "会话过期",
		"密码重置", "账号锁定", "登录失败", "登录成功", "登出清理",
	}
	gatewayActions := []string{
		"请求路由", "限流触发", "认证通过", "认证失败", "请求转发",
		"响应缓存", "熔断开启", "熔断关闭", "超时处理", "重试请求",
	}
	searchActions := []string{
		"搜索请求", "索引更新", "查询优化", "缓存命中", "缓存未命中",
		"搜索结果", "搜索超时", "索引重建", "分词处理", "相关性计算",
	}

	actionMap := map[string][]string{
		"user-service":         userActions,
		"payment-service":      paymentActions,
		"order-service":         orderActions,
		"inventory-service":     inventoryActions,
		"notification-service": notificationActions,
		"auth-service":          authActions,
		"api-gateway":           gatewayActions,
		"search-service":        searchActions,
	}

	// 错误消息模板
	errorMessages := []string{
		"数据库连接超时",
		"Redis 连接失败",
		"第三方 API 调用失败",
		"参数验证失败",
		"权限不足",
		"资源不存在",
		"服务不可用",
		"网络超时",
		"内存不足",
		"磁盘空间不足",
	}

	// 用户 ID 池
	userIDs := make([]int, 100)
	for i := range userIDs {
		userIDs[i] = 10000 + i
	}

	// 订单 ID 池
	orderIDs := make([]string, 1000)
	for i := range orderIDs {
		orderIDs[i] = fmt.Sprintf("ORD-%06d", i+1)
	}

	// 产品 ID 池
	productIDs := make([]string, 500)
	for i := range productIDs {
		productIDs[i] = fmt.Sprintf("PROD-%04d", i+1)
	}

	// IP 地址池
	ipAddresses := []string{
		"192.168.1.1", "192.168.1.2", "10.0.0.1", "10.0.0.2",
		"172.16.0.1", "172.16.0.2", "203.0.113.1", "203.0.113.2",
	}

	// 并发写入
	const totalLogs = 10000
	const workers = 10
	logsPerWorker := totalLogs / workers

	var wg sync.WaitGroup
	startTime := time.Now()

	fmt.Printf("开始插入 %d 条日志，使用 %d 个并发 worker...\n", totalLogs, workers)
	fmt.Println()

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			rand.Seed(time.Now().UnixNano() + int64(workerID))

			for i := 0; i < logsPerWorker; i++ {
				// 随机选择服务
				service := services[rand.Intn(len(services))]
				logger := loggers[service]
				actions := actionMap[service]

				// 随机选择日志级别（70% info, 20% warn, 10% error）
				level := rand.Float64()
				var logFunc func(string, ...interface{})
				var message string

				if level < 0.7 {
					// Info 日志
					logFunc = logger.Info
					message = actions[rand.Intn(len(actions))]
				} else if level < 0.9 {
					// Warn 日志
					logFunc = logger.Warn
					message = actions[rand.Intn(len(actions))] + " - 警告"
				} else {
					// Error 日志
					logFunc = logger.Error
					message = errorMessages[rand.Intn(len(errorMessages))]
				}

				// 根据服务生成不同的字段
				switch service {
				case "user-service":
					logFunc(message,
						"user_id", userIDs[rand.Intn(len(userIDs))],
						"ip", ipAddresses[rand.Intn(len(ipAddresses))],
						"user_agent", "Mozilla/5.0",
						"session_id", fmt.Sprintf("sess-%d", rand.Intn(10000)),
					)
				case "payment-service":
					logFunc(message,
						"order_id", orderIDs[rand.Intn(len(orderIDs))],
						"amount", rand.Float64()*1000+10,
						"payment_method", []string{"alipay", "wechat", "credit_card"}[rand.Intn(3)],
						"transaction_id", fmt.Sprintf("TXN-%d", rand.Intn(100000)),
					)
				case "order-service":
					logFunc(message,
						"order_id", orderIDs[rand.Intn(len(orderIDs))],
						"user_id", userIDs[rand.Intn(len(userIDs))],
						"product_id", productIDs[rand.Intn(len(productIDs))],
						"quantity", rand.Intn(10)+1,
						"total_amount", rand.Float64()*500+50,
					)
				case "inventory-service":
					logFunc(message,
						"product_id", productIDs[rand.Intn(len(productIDs))],
						"warehouse_id", fmt.Sprintf("WH-%d", rand.Intn(10)+1),
						"quantity", rand.Intn(1000),
						"operation", []string{"deduct", "restore", "adjust"}[rand.Intn(3)],
					)
				case "notification-service":
					logFunc(message,
						"user_id", userIDs[rand.Intn(len(userIDs))],
						"channel", []string{"sms", "email", "push"}[rand.Intn(3)],
						"template_id", fmt.Sprintf("TMP-%d", rand.Intn(50)+1),
						"status", []string{"sent", "failed", "pending"}[rand.Intn(3)],
					)
				case "auth-service":
					logFunc(message,
						"user_id", userIDs[rand.Intn(len(userIDs))],
						"ip", ipAddresses[rand.Intn(len(ipAddresses))],
						"token_type", []string{"access", "refresh"}[rand.Intn(2)],
						"expires_in", rand.Intn(3600)+1800,
					)
				case "api-gateway":
					logFunc(message,
						"path", []string{"/api/users", "/api/orders", "/api/products", "/api/payments"}[rand.Intn(4)],
						"method", []string{"GET", "POST", "PUT", "DELETE"}[rand.Intn(4)],
						"status_code", []int{200, 201, 400, 401, 404, 500}[rand.Intn(6)],
						"response_time_ms", rand.Intn(500)+10,
					)
				case "search-service":
					logFunc(message,
						"query", []string{"手机", "电脑", "耳机", "键盘", "鼠标"}[rand.Intn(5)],
						"results_count", rand.Intn(1000),
						"search_time_ms", rand.Intn(200)+10,
						"index_version", fmt.Sprintf("v%d.%d", rand.Intn(3)+1, rand.Intn(10)),
					)
				}

				// 每 1000 条打印进度
				if (i+1)%1000 == 0 {
					fmt.Printf("Worker %d: 已插入 %d 条日志\n", workerID, i+1)
				}

				// 随机延迟，模拟真实场景
				time.Sleep(time.Duration(rand.Intn(10)) * time.Millisecond)
			}
		}(w)
	}

	wg.Wait()

	elapsed := time.Since(startTime)
	fmt.Println()
	fmt.Printf("✅ 完成！共插入 %d 条日志\n", totalLogs)
	fmt.Printf("⏱️  耗时: %v\n", elapsed)
	fmt.Printf("📊 平均速度: %.0f 条/秒\n", float64(totalLogs)/elapsed.Seconds())
	fmt.Println()
	fmt.Println("等待日志发送完成...")
	time.Sleep(2 * time.Second)
	fmt.Println("✅ 测试完成！")
}
