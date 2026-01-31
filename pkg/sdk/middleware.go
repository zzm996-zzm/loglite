package sdk

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// HTTPMiddleware 标准 HTTP 中间件
func HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// 生成或提取 trace_id
		traceID := r.Header.Get("X-Trace-ID")
		if traceID == "" {
			traceID = uuid.New().String()
		}

		// 生成 request_id
		requestID := uuid.New().String()

		// 注入 context
		ctx := r.Context()
		ctx = context.WithValue(ctx, TraceIDKey, traceID)
		ctx = context.WithValue(ctx, RequestIDKey, requestID)

		// 创建 Logger 并注入
		if defaultClient != nil {
			logger := defaultClient.WithContext(ctx)
			ctx = ToContext(ctx, logger)
		}

		// 设置响应头
		w.Header().Set("X-Trace-ID", traceID)
		w.Header().Set("X-Request-ID", requestID)

		// 包装 ResponseWriter 以获取状态码
		wrapped := &responseWriter{ResponseWriter: w, statusCode: 200}

		// 执行请求
		next.ServeHTTP(wrapped, r.WithContext(ctx))

		// 记录请求日志
		if defaultClient != nil {
			latency := time.Since(start)
			defaultClient.Info("HTTP Request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", wrapped.statusCode,
				"latency_ms", latency.Milliseconds(),
				"client_ip", getClientIP(r),
				"user_agent", r.UserAgent(),
				"trace_id", traceID,
				"request_id", requestID,
			)
		}
	})
}

// responseWriter 包装 ResponseWriter
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// getClientIP 获取客户端 IP
func getClientIP(r *http.Request) string {
	// 优先从 X-Forwarded-For 获取
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	return r.RemoteAddr
}

// RecoveryMiddleware Panic 恢复中间件
func RecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				// 记录 panic
				if defaultClient != nil {
					logger := FromContext(r.Context())
					logger.Error("Panic recovered",
						"error", err,
						"method", r.Method,
						"path", r.URL.Path,
					)
				}

				// 返回 500
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(w, r)
	})
}
