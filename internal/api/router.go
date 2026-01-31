package api

import (
	"github.com/gin-gonic/gin"
	"github.com/loglite/loglite/internal/storage"
)

// TailHub 全局实例
var globalTailHub *TailHub

// SetupRouter 设置路由
func SetupRouter(store *storage.BadgerStore) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(corsMiddleware())

	handler := NewHandler(store)
	statsHandler := NewStatsHandler(store)

	// 初始化 TailHub
	globalTailHub = NewTailHub()
	tailHandler := NewTailHandler(globalTailHub)

	// 健康检查
	r.GET("/health", handler.Health)

	// API v1
	v1 := r.Group("/api/v1")
	{
		// 日志接收
		v1.POST("/logs", handler.ReceiveLog)
		v1.POST("/logs/batch", handler.ReceiveBatch)

		// 日志查询（支持自然语言）
		v1.GET("/query", handler.QueryLogs)
		v1.GET("/logs/:id", handler.GetLog)

		// 实时日志流
		v1.GET("/tail", tailHandler.HandleTail)

		// 统计
		v1.GET("/stats", handler.GetStats)
		v1.GET("/stats/errors", statsHandler.GetErrorStats)
		v1.GET("/stats/errors/trend", statsHandler.GetErrorTrend)
	}

	// Web UI (静态文件)
	r.GET("/", serveIndex)
	r.Static("/static", "./web/static")

	return r
}

// GetTailHub 获取 TailHub 实例（用于广播日志）
func GetTailHub() *TailHub {
	return globalTailHub
}

// corsMiddleware CORS 中间件
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// serveIndex 提供首页
func serveIndex(c *gin.Context) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.File("./web/templates/index.html")
}
