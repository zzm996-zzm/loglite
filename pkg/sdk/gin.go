package sdk

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// GinMiddleware Gin 日志中间件
// 需要先设置 defaultLogger: sdk.SetDefaultLogger(logger)
func GinMiddleware() gin.HandlerFunc {
	return GinMiddlewareWithLogger(nil)
}

// GinMiddlewareWithLogger 使用指定 Logger 的 Gin 中间件
func GinMiddlewareWithLogger(logger *Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// 使用指定的 logger 或 defaultLogger
		l := logger
		if l == nil {
			l = defaultLogger
		}

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
		if l != nil {
			ctxLogger := l.WithContext(ctx)
			ctx = ToContext(ctx, ctxLogger)
		}

		c.Request = c.Request.WithContext(ctx)

		// 设置响应头
		c.Header("X-Trace-ID", traceID)
		c.Header("X-Request-ID", requestID)

		// 执行请求
		c.Next()

		// 记录请求日志
		if l != nil {
			latency := time.Since(start)
			status := c.Writer.Status()

			level := "info"
			if status >= 500 {
				level = "error"
			} else if status >= 400 {
				level = "warn"
			}

			switch level {
			case "info":
				l.Info("HTTP Request",
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
			case "warn":
				l.Warn("HTTP Request",
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
			case "error":
				l.Error("HTTP Request",
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
}

// GinRecovery Gin Panic 恢复中间件
func GinRecovery() gin.HandlerFunc {
	return GinRecoveryWithLogger(nil)
}

// GinRecoveryWithLogger 使用指定 Logger 的 Panic 恢复中间件
func GinRecoveryWithLogger(logger *Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// 使用指定的 logger 或 defaultLogger
				l := logger
				if l == nil {
					l = defaultLogger
				}

				// 记录 panic
				if l != nil {
					ctxLogger := FromContext(c.Request.Context())
					if ctxLogger != nil {
						ctxLogger.Error("Panic recovered",
							"error", err,
							"method", c.Request.Method,
							"path", c.Request.URL.Path,
						)
					} else {
						l.Error("Panic recovered",
							"error", err,
							"method", c.Request.Method,
							"path", c.Request.URL.Path,
						)
					}

					// 强制刷新
					l.Flush()
				}

				// 返回 500
				c.AbortWithStatus(500)
			}
		}()

		c.Next()
	}
}
