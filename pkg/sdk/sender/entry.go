package sender

import (
	"encoding/json"
	"time"
)

// ============================================================
// 优化版 LogEntry - 小对象优化（Small Object Optimization）
// ============================================================
//
// 设计思想：
// 1. 大多数日志只有 3-8 个自定义字段
// 2. 使用固定数组避免 map 的 3 次分配（header + bucket + ...）
// 3. 超过 8 个字段时才使用 slice（按需扩展）
// 4. map 作为最后手段（几乎不用）
//
// 内存布局：
// ┌─────────────────────────────────────────────────────────┐
// │                    LogEntry (栈/嵌入)                    │
// ├─────────────────────────────────────────────────────────┤
// │ 固定字段: ID, Timestamp, Message, Level, Service, ...   │
// ├─────────────────────────────────────────────────────────┤
// │ smallFields [8]FieldEntry  ← 直接嵌入，零分配！          │
// │ smallCount  int                                         │
// ├─────────────────────────────────────────────────────────┤
// │ extraFields []FieldEntry   ← 按需分配（>8 字段时）       │
// │ overflow    map[string]any ← 最后手段（几乎不用）        │
// └─────────────────────────────────────────────────────────┘
//
// 分配次数对比：
// - 原方案 (map):        3 次分配（结构体 + map header + bucket）
// - 新方案 (≤8 字段):    1 次分配（只有结构体）
// - 新方案 (>8 字段):    2 次分配（结构体 + slice）
// - 深拷贝:              0 次额外分配（值拷贝 fixed array）

// FieldEntry 字段条目（key-value 对）
type FieldEntry struct {
	Key   string
	Value any
}

// LogEntry 日志条目（优化版）
type LogEntry struct {
	// ============ 必填字段 ============
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message"`
	Level     string    `json:"level"`
	Service   string    `json:"service"`

	// ============ 推荐字段（SDK 自动填充或用户提供）============
	TraceID   string `json:"trace_id,omitempty"`
	SpanID    string `json:"span_id,omitempty"`
	UserID    string `json:"user_id,omitempty"`
	RequestID string `json:"request_id,omitempty"`
	IP        string `json:"ip,omitempty"`

	// ============ 代码位置（SDK 自动采集）============
	Caller     string `json:"caller,omitempty"`
	Function   string `json:"function,omitempty"`
	Package    string `json:"package,omitempty"`
	StackTrace string `json:"stack_trace,omitempty"`
	StackHash  string `json:"stack_hash,omitempty"`

	// ============ 动态字段（小对象优化）============
	// 固定数组：直接嵌入结构体，零额外分配
	smallFields [8]FieldEntry
	smallCount  int // 当前使用的字段数

	// 扩展 slice：超过 8 个字段时使用
	extraFields []FieldEntry

	// 溢出 map：极端情况（几乎不用）
	overflow map[string]any

	// ============ 系统字段（内部使用）============
	ReceivedAt time.Time `json:"-"`
	StoredAt   time.Time `json:"-"`
}

// ============================================================
// 字段操作方法
// ============================================================

// SetField 设置动态字段
// 优先使用 smallFields，超过 8 个使用 extraFields
func (e *LogEntry) SetField(key string, value any) {
	// 1. 检查是否已存在（更新）
	for i := 0; i < e.smallCount; i++ {
		if e.smallFields[i].Key == key {
			e.smallFields[i].Value = value
			return
		}
	}
	for i := range e.extraFields {
		if e.extraFields[i].Key == key {
			e.extraFields[i].Value = value
			return
		}
	}
	if e.overflow != nil {
		if _, exists := e.overflow[key]; exists {
			e.overflow[key] = value
			return
		}
	}

	// 2. 新增字段
	if e.smallCount < 8 {
		// 使用固定数组（零分配）
		e.smallFields[e.smallCount] = FieldEntry{Key: key, Value: value}
		e.smallCount++
	} else if len(e.extraFields) < 16 {
		// 使用扩展 slice（一次分配）
		e.extraFields = append(e.extraFields, FieldEntry{Key: key, Value: value})
	} else {
		// 使用 map（极端情况）
		if e.overflow == nil {
			e.overflow = make(map[string]any)
		}
		e.overflow[key] = value
	}
}

// GetField 获取动态字段
func (e *LogEntry) GetField(key string) (any, bool) {
	for i := 0; i < e.smallCount; i++ {
		if e.smallFields[i].Key == key {
			return e.smallFields[i].Value, true
		}
	}
	for i := range e.extraFields {
		if e.extraFields[i].Key == key {
			return e.extraFields[i].Value, true
		}
	}
	if e.overflow != nil {
		if v, ok := e.overflow[key]; ok {
			return v, ok
		}
	}
	return nil, false
}

// FieldCount 返回动态字段数量
func (e *LogEntry) FieldCount() int {
	count := e.smallCount + len(e.extraFields)
	if e.overflow != nil {
		count += len(e.overflow)
	}
	return count
}

// RangeFields 遍历所有动态字段
func (e *LogEntry) RangeFields(fn func(key string, value any) bool) {
	for i := 0; i < e.smallCount; i++ {
		if !fn(e.smallFields[i].Key, e.smallFields[i].Value) {
			return
		}
	}
	for i := range e.extraFields {
		if !fn(e.extraFields[i].Key, e.extraFields[i].Value) {
			return
		}
	}
	if e.overflow != nil {
		for k, v := range e.overflow {
			if !fn(k, v) {
				return
			}
		}
	}
}

// ============================================================
// 重置和拷贝
// ============================================================

// Reset 重置 LogEntry（用于 sync.Pool）
func (e *LogEntry) Reset() {
	// 重置固定字段
	e.ID = ""
	e.Timestamp = time.Time{}
	e.Message = ""
	e.Level = ""
	e.Service = ""
	e.TraceID = ""
	e.SpanID = ""
	e.UserID = ""
	e.RequestID = ""
	e.IP = ""
	e.Caller = ""
	e.Function = ""
	e.Package = ""
	e.StackTrace = ""
	e.StackHash = ""
	e.ReceivedAt = time.Time{}
	e.StoredAt = time.Time{}

	// 重置 smallFields（只需清零 count，数组内容无需清理）
	e.smallCount = 0

	// 清空 extraFields（保留 slice 容量）
	e.extraFields = e.extraFields[:0]

	// 清空 overflow map
	if e.overflow != nil {
		for k := range e.overflow {
			delete(e.overflow, k)
		}
	}
}

// DeepCopy 深拷贝 LogEntry
// 关键优化：smallFields 是值类型数组，自动值拷贝！
func (e *LogEntry) DeepCopy() LogEntry {
	copied := LogEntry{
		// 固定字段：直接值拷贝
		ID:         e.ID,
		Timestamp:  e.Timestamp,
		Message:    e.Message,
		Level:      e.Level,
		Service:    e.Service,
		TraceID:    e.TraceID,
		SpanID:     e.SpanID,
		UserID:     e.UserID,
		RequestID:  e.RequestID,
		IP:         e.IP,
		Caller:     e.Caller,
		Function:   e.Function,
		Package:    e.Package,
		StackTrace: e.StackTrace,
		StackHash:  e.StackHash,
		ReceivedAt: e.ReceivedAt,
		StoredAt:   e.StoredAt,

		// smallFields：值类型数组，自动拷贝！无需分配！
		smallFields: e.smallFields,
		smallCount:  e.smallCount,
	}

	// extraFields：需要拷贝 slice（只有 >8 字段时才分配）
	if len(e.extraFields) > 0 {
		copied.extraFields = make([]FieldEntry, len(e.extraFields))
		copy(copied.extraFields, e.extraFields)
	}

	// overflow：需要拷贝 map（几乎不会执行）
	if len(e.overflow) > 0 {
		copied.overflow = make(map[string]any, len(e.overflow))
		for k, v := range e.overflow {
			copied.overflow[k] = v
		}
	}

	return copied
}

// ============================================================
// JSON 序列化
// ============================================================

// MarshalJSON 自定义 JSON 序列化
// 将 smallFields + extraFields + overflow 合并为 "metadata" 字段
func (e *LogEntry) MarshalJSON() ([]byte, error) {
	// 构建临时结构用于序列化
	type Alias LogEntry // 避免递归调用

	// 收集所有动态字段到 metadata map
	var metadata map[string]any
	fieldCount := e.FieldCount()

	if fieldCount > 0 {
		metadata = make(map[string]any, fieldCount)
		e.RangeFields(func(key string, value any) bool {
			metadata[key] = value
			return true
		})
	}

	// 构建输出结构
	output := struct {
		ID         string         `json:"id"`
		Timestamp  time.Time      `json:"timestamp"`
		Message    string         `json:"message"`
		Level      string         `json:"level"`
		Service    string         `json:"service"`
		TraceID    string         `json:"trace_id,omitempty"`
		SpanID     string         `json:"span_id,omitempty"`
		UserID     string         `json:"user_id,omitempty"`
		RequestID  string         `json:"request_id,omitempty"`
		IP         string         `json:"ip,omitempty"`
		Caller     string         `json:"caller,omitempty"`
		Function   string         `json:"function,omitempty"`
		Package    string         `json:"package,omitempty"`
		StackTrace string         `json:"stack_trace,omitempty"`
		StackHash  string         `json:"stack_hash,omitempty"`
		Metadata   map[string]any `json:"metadata,omitempty"`
	}{
		ID:         e.ID,
		Timestamp:  e.Timestamp,
		Message:    e.Message,
		Level:      e.Level,
		Service:    e.Service,
		TraceID:    e.TraceID,
		SpanID:     e.SpanID,
		UserID:     e.UserID,
		RequestID:  e.RequestID,
		IP:         e.IP,
		Caller:     e.Caller,
		Function:   e.Function,
		Package:    e.Package,
		StackTrace: e.StackTrace,
		StackHash:  e.StackHash,
		Metadata:   metadata,
	}

	return json.Marshal(output)
}

// ============================================================
// 接口实现
// ============================================================

// ToLogEntry 实现 LogRecord 接口
func (e *LogEntry) ToLogEntry() *LogEntry {
	return e
}
