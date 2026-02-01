package sender

import (
	"testing"
)

// BenchmarkMapOnly_WithCapacity 只测试 map 分配（预分配容量）
func BenchmarkMapOnly_WithCapacity(b *testing.B) {
	var result map[string]interface{}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		m := make(map[string]interface{}, 2)  // 预分配容量 2
		m["key1"] = "value1"
		m["key2"] = "value2"
		result = m
	}
	
	_ = result
}

// BenchmarkMapOnly_WithoutCapacity 只测试 map 分配（不预分配）
func BenchmarkMapOnly_WithoutCapacity(b *testing.B) {
	var result map[string]interface{}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		m := make(map[string]interface{})  // 不预分配
		m["key1"] = "value1"
		m["key2"] = "value2"
		result = m
	}
	
	_ = result
}

// BenchmarkStructOnly 只测试结构体分配
func BenchmarkStructOnly(b *testing.B) {
	var result *LogEntry
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		entry := &LogEntry{
			ID:      "test",
			Message: "test",
		}
		result = entry
	}
	
	_ = result
}

// BenchmarkStructWithMap 测试结构体 + map
func BenchmarkStructWithMap(b *testing.B) {
	var result *LogEntry
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		entry := &LogEntry{
			ID:       "test",
			Message:  "test",
			Metadata: make(map[string]interface{}, 2),
		}
		entry.Metadata["key1"] = "value1"
		entry.Metadata["key2"] = "value2"
		result = entry
	}
	
	_ = result
}
