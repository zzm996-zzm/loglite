package main

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/loglite/loglite/pkg/sdk"
)

func main() {
	// 初始化 LogLite 客户端
	client := sdk.Init("gin-demo",
		sdk.WithEndpoint("http://localhost:8080"),
	)
	defer client.Close()

	// 设置为默认客户端
	sdk.SetDefault(client)

	// 创建 Gin 引擎
	r := gin.New()

	// 使用 LogLite 中间件
	r.Use(sdk.GinMiddleware())  // 请求日志
	r.Use(sdk.GinRecovery())    // Panic 恢复

	// 路由
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "Hello, LogLite!",
		})
	})

	r.GET("/api/users/:id", func(c *gin.Context) {
		// 从 context 获取 logger（自动带上 trace_id, request_id）
		logger := sdk.FromContext(c.Request.Context())

		userID := c.Param("id")
		logger.Info("查询用户", "user_id", userID)

		// 模拟业务逻辑
		time.Sleep(50 * time.Millisecond)

		c.JSON(http.StatusOK, gin.H{
			"id":   userID,
			"name": "John Doe",
		})
	})

	r.POST("/api/orders", func(c *gin.Context) {
		logger := sdk.FromContext(c.Request.Context())

		var req struct {
			ProductID string  `json:"product_id"`
			Quantity  int     `json:"quantity"`
			Amount    float64 `json:"amount"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			logger.Warn("参数错误", "error", err.Error())
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		logger.Info("创建订单",
			"product_id", req.ProductID,
			"quantity", req.Quantity,
			"amount", req.Amount,
		)

		// 模拟订单处理
		orderID := "ORD-" + time.Now().Format("20060102150405")
		time.Sleep(100 * time.Millisecond)

		logger.Info("订单创建成功", "order_id", orderID)

		c.JSON(http.StatusOK, gin.H{
			"order_id": orderID,
			"status":   "created",
		})
	})

	r.GET("/api/panic", func(c *gin.Context) {
		// 测试 panic 恢复
		panic("intentional panic for testing")
	})

	// 启动服务
	client.Info("Gin 服务启动", "port", 9090)
	r.Run(":9090")
}
