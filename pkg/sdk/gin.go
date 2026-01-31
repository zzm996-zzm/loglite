package sdk

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// GinMiddleware Gin 日志中间件
func GinMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// 生成或提取 trace_id
		traceID := c.GetHeader("X-Trace-ID")
		if traceID == "" {
			traceID = uuid.New().String()
		}

		// 生成 request_id
		requestID := uuid.New().String()

		// 注入 context
		ctx := c.Request.Context()
		ctx = context.WithValue(ctx, TraceIDKey, traceID)
		ctx = context.WithValue(ctx, RequestIDKey, requestID)

		// 创建 Logger 并注入
		if defaultClient != nil {
			logger := defaultClient.WithContext(ctx)
			ctx = ToContext(ctx, logger)
		}

		c.Request = c.Request.WithContext(ctx)

		// 设置响应头
		c.Header("X-Trace-ID", traceID)
		c.Header("X-Request-ID", requestID)

		// 执行请求
		c.Next()

		// 记录请求日志
		if defaultClient != nil {
			latency := time.Since(start)
			status := c.Writer.Status()

			level := "info"
			if status >= 500 {
				level = "error"
			} else if status >= 400 {
				level = "warn"
			}

			defaultClient.log(level, "HTTP Request",
				"method", c.Request.Method,
				"path", c.Request.URL.Path,
				"query", c.Request.URL.RawQuery,
				"status", status,
				"latency_ms", latency.Milliseconds(),
				"client_ip", c.ClientIP(),
				"user_agent", c.Request.UserAgent(),
				"trace_id", traceID,
				"request_id", requestID,
			)
		}
	}
}

// GinRecovery Gin Panic 恢复中间件
func GinRecovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// 记录 panic
				if defaultClient != nil {
					logger := FromContext(c.Request.Context())
					logger.Error("Panic recovered",
						"error", err,
						"method", c.Request.Method,
						"path", c.Request.URL.Path,
					)

					// 强制刷新，确保 panic 日志发送
					defaultClient.Flush()
				}

				// 返回 500
				c.AbortWithStatus(500)
			}
		}()

		c.Next()
	}
}
