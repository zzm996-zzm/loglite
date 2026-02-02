package sender

import (
	"testing"
	"time"
)

// ============================================================
// 新方案 vs 旧方案 对比测试
// ============================================================

// BenchmarkNewEntry_SmallFields 测试新方案：≤8 字段（零额外分配）
func BenchmarkNewEntry_SmallFields(b *testing.B) {
	var result LogEntry

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		entry := LogEntry{
			ID:        "test-id",
			Timestamp: time.Now(),
			Message:   "test message",
			Level:     "info",
			Service:   "test-service",
		}
		// 4 个动态字段（使用 smallFields）
		entry.SetField("trace_id", "trace-123")
		entry.SetField("user_id", "user-456")
		entry.SetField("order_id", 12345)
		entry.SetField("amount", 99.99)

		result = entry
	}

	_ = result
}

// BenchmarkOldEntry_SmallFields 测试旧方案：map（3 次分配）
func BenchmarkOldEntry_SmallFields(b *testing.B) {
	type OldLogEntry struct {
		ID        string
		Timestamp time.Time
		Message   string
		Level     string
		Service   string
		Metadata  map[string]any
	}

	var result OldLogEntry

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		entry := OldLogEntry{
			ID:        "test-id",
			Timestamp: time.Now(),
			Message:   "test message",
			Level:     "info",
			Service:   "test-service",
			Metadata:  make(map[string]any),
		}
		entry.Metadata["trace_id"] = "trace-123"
		entry.Metadata["user_id"] = "user-456"
		entry.Metadata["order_id"] = 12345
		entry.Metadata["amount"] = 99.99

		result = entry
	}

	_ = result
}

// BenchmarkNewEntry_DeepCopy 测试新方案的深拷贝（零额外分配）
func BenchmarkNewEntry_DeepCopy(b *testing.B) {
	entry := LogEntry{
		ID:        "test-id",
		Timestamp: time.Now(),
		Message:   "test message",
		Level:     "info",
		Service:   "test-service",
	}
	entry.SetField("trace_id", "trace-123")
	entry.SetField("user_id", "user-456")
	entry.SetField("order_id", 12345)
	entry.SetField("amount", 99.99)

	var result LogEntry

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		result = entry.DeepCopy()
	}

	_ = result
}

// BenchmarkOldEntry_DeepCopy 测试旧方案的深拷贝（3 次分配）
func BenchmarkOldEntry_DeepCopy(b *testing.B) {
	type OldLogEntry struct {
		ID        string
		Timestamp time.Time
		Message   string
		Level     string
		Service   string
		Metadata  map[string]any
	}

	entry := OldLogEntry{
		ID:        "test-id",
		Timestamp: time.Now(),
		Message:   "test message",
		Level:     "info",
		Service:   "test-service",
		Metadata:  make(map[string]any),
	}
	entry.Metadata["trace_id"] = "trace-123"
	entry.Metadata["user_id"] = "user-456"
	entry.Metadata["order_id"] = 12345
	entry.Metadata["amount"] = 99.99

	var result OldLogEntry

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// 深拷贝需要拷贝 map
		copied := OldLogEntry{
			ID:        entry.ID,
			Timestamp: entry.Timestamp,
			Message:   entry.Message,
			Level:     entry.Level,
			Service:   entry.Service,
			Metadata:  make(map[string]any, len(entry.Metadata)),
		}
		for k, v := range entry.Metadata {
			copied.Metadata[k] = v
		}
		result = copied
	}

	_ = result
}

// BenchmarkNewEntry_ManyFields 测试新方案：>8 字段（使用 extraFields）
func BenchmarkNewEntry_ManyFields(b *testing.B) {
	var result LogEntry

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		entry := LogEntry{
			ID:        "test-id",
			Timestamp: time.Now(),
			Message:   "test message",
			Level:     "info",
			Service:   "test-service",
		}
		// 12 个字段（8 smallFields + 4 extraFields）
		for j := 0; j < 12; j++ {
			entry.SetField(string(rune('a'+j)), j)
		}

		result = entry
	}

	_ = result
}

// BenchmarkOldEntry_ManyFields 测试旧方案：>8 字段
func BenchmarkOldEntry_ManyFields(b *testing.B) {
	type OldLogEntry struct {
		ID        string
		Timestamp time.Time
		Message   string
		Level     string
		Service   string
		Metadata  map[string]any
	}

	var result OldLogEntry

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		entry := OldLogEntry{
			ID:        "test-id",
			Timestamp: time.Now(),
			Message:   "test message",
			Level:     "info",
			Service:   "test-service",
			Metadata:  make(map[string]any),
		}
		for j := 0; j < 12; j++ {
			entry.Metadata[string(rune('a'+j))] = j
		}

		result = entry
	}

	_ = result
}

// ============================================================
// 功能测试
// ============================================================

func TestLogEntry_SetField(t *testing.T) {
	entry := LogEntry{}

	// 测试 smallFields（前 8 个）
	for i := 0; i < 8; i++ {
		entry.SetField(string(rune('a'+i)), i)
	}
	if entry.smallCount != 8 {
		t.Errorf("smallCount = %d, want 8", entry.smallCount)
	}
	if len(entry.extraFields) != 0 {
		t.Errorf("extraFields len = %d, want 0", len(entry.extraFields))
	}

	// 测试 extraFields（9-16）
	for i := 8; i < 16; i++ {
		entry.SetField(string(rune('a'+i)), i)
	}
	if len(entry.extraFields) != 8 {
		t.Errorf("extraFields len = %d, want 8", len(entry.extraFields))
	}

	// 验证字段值
	for i := 0; i < 16; i++ {
		v, ok := entry.GetField(string(rune('a' + i)))
		if !ok {
			t.Errorf("field %c not found", rune('a'+i))
		}
		if v != i {
			t.Errorf("field %c = %v, want %d", rune('a'+i), v, i)
		}
	}
}

func TestLogEntry_DeepCopy(t *testing.T) {
	entry := LogEntry{
		ID:      "original",
		Message: "test",
	}
	entry.SetField("key1", "value1")
	entry.SetField("key2", 123)

	// 深拷贝
	copied := entry.DeepCopy()

	// 修改原始
	entry.ID = "modified"
	entry.SetField("key1", "modified")

	// 验证拷贝未受影响
	if copied.ID != "original" {
		t.Errorf("copied.ID = %s, want original", copied.ID)
	}
	v, _ := copied.GetField("key1")
	if v != "value1" {
		t.Errorf("copied key1 = %v, want value1", v)
	}
}

func TestLogEntry_Reset(t *testing.T) {
	entry := LogEntry{
		ID:      "test",
		Message: "test",
	}
	entry.SetField("key1", "value1")
	entry.SetField("key2", "value2")

	entry.Reset()

	if entry.ID != "" {
		t.Errorf("ID not reset")
	}
	if entry.smallCount != 0 {
		t.Errorf("smallCount = %d, want 0", entry.smallCount)
	}
	if entry.FieldCount() != 0 {
		t.Errorf("FieldCount = %d, want 0", entry.FieldCount())
	}
}
