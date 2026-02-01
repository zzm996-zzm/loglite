package sender

import (
	"testing"
	"time"
)

// BenchmarkCopyEntry_Small 测试小 Metadata 的深拷贝（2 个元素）
func BenchmarkCopyEntry_Small(b *testing.B) {
	entry := &LogEntry{
		ID:        "test-id-123",
		Timestamp: time.Now(),
		Message:   "test message",
		Level:     "info",
		Service:   "test-service",
		Metadata:  make(map[string]interface{}),
	}
	entry.Metadata["key1"] = "value1"
	entry.Metadata["key2"] = "value2"
	
	var result *LogEntry // 防止编译器优化
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		// 模拟 BestEffortSender.copyEntry
		copied := &LogEntry{
			ID:        entry.ID,
			Timestamp: entry.Timestamp,
			Message:   entry.Message,
			Level:     entry.Level,
			Service:   entry.Service,
			Metadata:  make(map[string]interface{}, len(entry.Metadata)),
		}
		
		for k, v := range entry.Metadata {
			copied.Metadata[k] = v
		}
		
		result = copied // 防止编译器优化掉 copied
	}
	
	_ = result
}

// BenchmarkCopyEntry_Medium 测试中等 Metadata 的深拷贝（5 个元素）
func BenchmarkCopyEntry_Medium(b *testing.B) {
	entry := &LogEntry{
		ID:        "test-id-123",
		Timestamp: time.Now(),
		Message:   "test message",
		Level:     "info",
		Service:   "test-service",
		Metadata:  make(map[string]interface{}),
	}
	for i := 0; i < 5; i++ {
		entry.Metadata[string(rune('a'+i))] = i
	}
	
	var result *LogEntry
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		copied := &LogEntry{
			ID:        entry.ID,
			Timestamp: entry.Timestamp,
			Message:   entry.Message,
			Level:     entry.Level,
			Service:   entry.Service,
			Metadata:  make(map[string]interface{}, len(entry.Metadata)),
		}
		
		for k, v := range entry.Metadata {
			copied.Metadata[k] = v
		}
		
		result = copied
	}
	
	_ = result
}

// BenchmarkCopyEntry_Large 测试大 Metadata 的深拷贝（20 个元素）
func BenchmarkCopyEntry_Large(b *testing.B) {
	entry := &LogEntry{
		ID:        "test-id-123",
		Timestamp: time.Now(),
		Message:   "test message",
		Level:     "info",
		Service:   "test-service",
		Metadata:  make(map[string]interface{}),
	}
	for i := 0; i < 20; i++ {
		entry.Metadata[string(rune('a'+i))] = i
	}
	
	var result *LogEntry
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		copied := &LogEntry{
			ID:        entry.ID,
			Timestamp: entry.Timestamp,
			Message:   entry.Message,
			Level:     entry.Level,
			Service:   entry.Service,
			Metadata:  make(map[string]interface{}, len(entry.Metadata)),
		}
		
		for k, v := range entry.Metadata {
			copied.Metadata[k] = v
		}
		
		result = copied
	}
	
	_ = result
}

// BenchmarkCopyEntry_WithoutCapacity 测试不预分配容量（对比）
func BenchmarkCopyEntry_WithoutCapacity(b *testing.B) {
	entry := &LogEntry{
		ID:        "test-id-123",
		Timestamp: time.Now(),
		Message:   "test message",
		Level:     "info",
		Service:   "test-service",
		Metadata:  make(map[string]interface{}),
	}
	entry.Metadata["key1"] = "value1"
	entry.Metadata["key2"] = "value2"
	
	var result *LogEntry
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		copied := &LogEntry{
			ID:        entry.ID,
			Timestamp: entry.Timestamp,
			Message:   entry.Message,
			Level:     entry.Level,
			Service:   entry.Service,
			Metadata:  make(map[string]interface{}), // 不预分配容量
		}
		
		for k, v := range entry.Metadata {
			copied.Metadata[k] = v  // 可能触发扩容
		}
		
		result = copied
	}
	
	_ = result
}
