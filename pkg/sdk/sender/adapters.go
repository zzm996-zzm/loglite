package sender

import (
	"time"
)

// ============================================================
// 日志框架适配器
// 用于将其他日志框架的输出转换为 LogLite 标准格式
// ============================================================

// ZapLogRecord zap 日志记录适配
// 将 zap 的 zapcore.Entry 转换为 LogEntry
//
// 使用方式（需要自己实现 zap Hook）：
//
//	type LogliteHook struct {
//	    sender sender.Sender
//	}
//
//	func (h *LogliteHook) Write(entry zapcore.Entry, fields []zapcore.Field) error {
//	    record := &sender.ZapLogRecord{
//	        Level:   entry.Level.String(),
//	        Time:    entry.Time,
//	        Message: entry.Message,
//	        Caller:  entry.Caller.String(),
//	        // fields 转换为 map...
//	    }
//	    return h.sender.SendRecord(record)
//	}
type ZapLogRecord struct {
	Level      string
	Time       time.Time
	Message    string
	Caller     string
	Function   string
	Package    string
	StackTrace string
	Fields     map[string]interface{}
}

func (r *ZapLogRecord) ToLogEntry() *LogEntry {
	entry := &LogEntry{
		Timestamp:  r.Time,
		Level:      r.Level,
		Message:    r.Message,
		Caller:     r.Caller,
		Function:   r.Function,
		Package:    r.Package,
		StackTrace: r.StackTrace,
	}

	// 从 Fields 中提取推荐字段
	if r.Fields != nil {
		if v, ok := r.Fields["trace_id"].(string); ok {
			entry.TraceID = v
		}
		if v, ok := r.Fields["span_id"].(string); ok {
			entry.SpanID = v
		}
		if v, ok := r.Fields["user_id"].(string); ok {
			entry.UserID = v
		}
		if v, ok := r.Fields["request_id"].(string); ok {
			entry.RequestID = v
		}
		if v, ok := r.Fields["ip"].(string); ok {
			entry.IP = v
		}

		// 剩余字段放入 metadata（零分配）
		for k, v := range r.Fields {
			switch k {
			case "trace_id", "span_id", "user_id", "request_id", "ip":
				// 已提取，跳过
			default:
				entry.SetMetadata(k, v)
			}
		}
	}

	return entry
}

// ZerologRecord zerolog 日志记录适配
// zerolog 输出 JSON，可以用 MapAdapter 处理
// 或者通过 Hook 方式捕获
//
// 使用方式：
//
//	type LogliteHook struct {
//	    sender sender.Sender
//	}
//
//	func (h *LogliteHook) Run(e *zerolog.Event, level zerolog.Level, msg string) {
//	    h.sender.SendMap(map[string]interface{}{
//	        "level":   level.String(),
//	        "message": msg,
//	        "time":    time.Now(),
//	    })
//	}
type ZerologRecord struct {
	Level   string
	Time    time.Time
	Message string
	Fields  map[string]interface{}
}

func (r *ZerologRecord) ToLogEntry() *LogEntry {
	entry := &LogEntry{
		Timestamp: r.Time,
		Level:     r.Level,
		Message:   r.Message,
	}

	// 从 fields 提取标准字段
	if r.Fields != nil {
		extractString := func(key string) string {
			if v, ok := r.Fields[key].(string); ok {
				return v
			}
			return ""
		}

		entry.TraceID = extractString("trace_id")
		entry.SpanID = extractString("span_id")
		entry.UserID = extractString("user_id")
		entry.RequestID = extractString("request_id")
		entry.IP = extractString("ip")
		entry.Caller = extractString("caller")
		entry.Function = extractString("function")
		entry.Package = extractString("package")
		entry.StackTrace = extractString("stack_trace")
		entry.StackHash = extractString("stack_hash")

		// 剩余字段放入 metadata（零分配）
		for k, v := range r.Fields {
			switch k {
			case "trace_id", "span_id", "user_id", "request_id", "ip",
				"caller", "function", "package", "stack_trace", "stack_hash":
				// 已提取，跳过
			default:
				entry.SetMetadata(k, v)
			}
		}
	}

	return entry
}

// SlogRecord slog (Go 1.21+) 日志记录适配
//
// 使用方式：
//
//	type LogliteHandler struct {
//	    sender sender.Sender
//	}
//
//	func (h *LogliteHandler) Handle(ctx context.Context, r slog.Record) error {
//	    fields := make(map[string]interface{})
//	    r.Attrs(func(a slog.Attr) bool {
//	        fields[a.Key] = a.Value.Any()
//	        return true
//	    })
//	    record := &sender.SlogRecord{
//	        Level:   r.Level.String(),
//	        Time:    r.Time,
//	        Message: r.Message,
//	        Fields:  fields,
//	    }
//	    return h.sender.SendRecord(record)
//	}
type SlogRecord struct {
	Level   string
	Time    time.Time
	Message string
	Source  string // file:line
	Fields  map[string]interface{}
}

func (r *SlogRecord) ToLogEntry() *LogEntry {
	entry := &LogEntry{
		Timestamp: r.Time,
		Level:     r.Level,
		Message:   r.Message,
		Caller:    r.Source,
	}

	// 从 Fields 中提取推荐字段
	if r.Fields != nil {
		extractString := func(key string) string {
			if v, ok := r.Fields[key].(string); ok {
				return v
			}
			return ""
		}

		entry.TraceID = extractString("trace_id")
		entry.SpanID = extractString("span_id")
		entry.UserID = extractString("user_id")
		entry.RequestID = extractString("request_id")
		entry.IP = extractString("ip")
		entry.Function = extractString("function")
		entry.Package = extractString("package")
		entry.StackTrace = extractString("stack_trace")
		entry.StackHash = extractString("stack_hash")

		// 剩余字段放入 metadata（零分配）
		for k, v := range r.Fields {
			switch k {
			case "trace_id", "span_id", "user_id", "request_id", "ip",
				"function", "package", "stack_trace", "stack_hash":
				// 已提取，跳过
			default:
				entry.SetMetadata(k, v)
			}
		}
	}

	return entry
}

// ============================================================
// 通用 JSON 适配器
// ============================================================

// JSONAdapter 将 JSON 字节适配为 LogEntry
// 适用于日志输出为 JSON 格式的场景
type JSONAdapter struct {
	Service string
}

// AdaptJSON 将 JSON map 转换为 LogEntry
// 支持常见字段名映射：
// - message/msg -> Message
// - level/lvl   -> Level
// - time/ts/@timestamp -> Timestamp
// - caller/source -> Caller
func (a *JSONAdapter) AdaptJSON(data map[string]interface{}) *LogEntry {
	entry := &LogEntry{
		Timestamp: time.Now(),
		Service:   a.Service,
		// metadata 零分配，只在有字段时使用
	}

	// Message 字段
	for _, key := range []string{"message", "msg", "Message", "MSG"} {
		if v, ok := data[key].(string); ok {
			entry.Message = v
			delete(data, key)
			break
		}
	}

	// Level 字段
	for _, key := range []string{"level", "lvl", "Level", "severity"} {
		if v, ok := data[key].(string); ok {
			entry.Level = v
			delete(data, key)
			break
		}
	}

	// Timestamp 字段
	for _, key := range []string{"time", "ts", "@timestamp", "timestamp", "Time"} {
		switch v := data[key].(type) {
		case time.Time:
			entry.Timestamp = v
			delete(data, key)
		case string:
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				entry.Timestamp = t
			} else if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
				entry.Timestamp = t
			}
			delete(data, key)
		case float64:
			// Unix timestamp (秒或毫秒)
			if v > 1e12 {
				entry.Timestamp = time.UnixMilli(int64(v))
			} else {
				entry.Timestamp = time.Unix(int64(v), 0)
			}
			delete(data, key)
		}
	}

	// Service 字段
	for _, key := range []string{"service", "Service", "app", "application"} {
		if v, ok := data[key].(string); ok {
			entry.Service = v
			delete(data, key)
			break
		}
	}

	// 推荐字段
	extractString := func(keys []string, setter func(string)) {
		for _, key := range keys {
			if v, ok := data[key].(string); ok {
				setter(v)
				delete(data, key)
				return
			}
		}
	}

	extractString([]string{"trace_id", "traceId", "TraceID"}, func(v string) { entry.TraceID = v })
	extractString([]string{"span_id", "spanId", "SpanID"}, func(v string) { entry.SpanID = v })
	extractString([]string{"user_id", "userId", "UserID"}, func(v string) { entry.UserID = v })
	extractString([]string{"request_id", "requestId", "RequestID"}, func(v string) { entry.RequestID = v })
	extractString([]string{"ip", "IP", "ipAddress"}, func(v string) { entry.IP = v })

	// 代码位置字段
	extractString([]string{"caller", "source", "Caller", "file"}, func(v string) { entry.Caller = v })
	extractString([]string{"function", "func", "Function", "method"}, func(v string) { entry.Function = v })
	extractString([]string{"package", "pkg", "Package"}, func(v string) { entry.Package = v })
	extractString([]string{"stack_trace", "stackTrace", "StackTrace", "stack"}, func(v string) { entry.StackTrace = v })
	extractString([]string{"stack_hash", "stackHash", "StackHash"}, func(v string) { entry.StackHash = v })

	// 剩余字段放入 Metadata（零分配）
	for k, v := range data {
		entry.SetMetadata(k, v)
	}

	return entry
}
