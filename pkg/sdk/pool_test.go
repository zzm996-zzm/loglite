package sdk

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/loglite/loglite/pkg/sdk/sender"
)

// ============================================================
// 1. 基本功能测试
// ============================================================

// TestGetLogEntry 测试获取 LogEntry
func TestGetLogEntry(t *testing.T) {
	entry := GetLogEntry()

	if entry == nil {
		t.Fatal("GetLogEntry() 返回 nil")
	}

	// 验证所有字段都是零值
	if entry.ID != "" {
		t.Errorf("ID 应为空，实际为: %s", entry.ID)
	}
	if entry.Message != "" {
		t.Errorf("Message 应为空，实际为: %s", entry.Message)
	}
	if entry.Level != "" {
		t.Errorf("Level 应为空，实际为: %s", entry.Level)
	}
	if entry.Service != "" {
		t.Errorf("Service 应为空，实际为: %s", entry.Service)
	}
	if !entry.Timestamp.IsZero() {
		t.Errorf("Timestamp 应为零值，实际为: %v", entry.Timestamp)
	}
	// 验证动态字段为空
	if entry.FieldCount() != 0 {
		t.Errorf("FieldCount 应为 0，实际为: %d", entry.FieldCount())
	}
}

// TestPutLogEntry 测试归还 LogEntry
func TestPutLogEntry(t *testing.T) {
	// 1. 获取一个 entry 并填充数据
	entry := GetLogEntry()
	entry.ID = "test-id-123"
	entry.Message = "test message"
	entry.Level = "info"
	entry.Service = "test-service"
	entry.TraceID = "trace-123"
	entry.SpanID = "span-123"
	entry.UserID = "user-123"
	entry.RequestID = "req-123"
	entry.IP = "192.168.1.1"
	entry.Caller = "main.go:123"
	entry.Function = "main.test"
	entry.Package = "main"
	entry.StackTrace = "stack trace here"
	entry.StackHash = "hash123"
	entry.Timestamp = time.Now()
	entry.ReceivedAt = time.Now()
	entry.StoredAt = time.Now()
	entry.SetField("key1", "value1")
	entry.SetField("key2", 123)

	// 2. 归还到池中
	PutLogEntry(entry)

	// 3. 再次获取，验证字段已重置
	entry2 := GetLogEntry()

	// 验证所有字符串字段已清空
	if entry2.ID != "" {
		t.Errorf("ID 应为空，实际为: %s", entry2.ID)
	}
	if entry2.Message != "" {
		t.Errorf("Message 应为空，实际为: %s", entry2.Message)
	}
	if entry2.Level != "" {
		t.Errorf("Level 应为空，实际为: %s", entry2.Level)
	}
	if entry2.Service != "" {
		t.Errorf("Service 应为空，实际为: %s", entry2.Service)
	}
	if entry2.TraceID != "" {
		t.Errorf("TraceID 应为空，实际为: %s", entry2.TraceID)
	}

	// 验证时间字段已重置
	if !entry2.Timestamp.IsZero() {
		t.Errorf("Timestamp 应为零值，实际为: %v", entry2.Timestamp)
	}

	// 验证动态字段已清空
	if entry2.FieldCount() != 0 {
		t.Errorf("FieldCount 应为 0，实际为: %d", entry2.FieldCount())
	}
}

// TestPoolReuse 测试对象复用
func TestPoolReuse(t *testing.T) {
	// 获取并归还多次，验证不会出错
	for i := 0; i < 100; i++ {
		entry := GetLogEntry()
		entry.ID = "test-id"
		entry.Message = "test message"
		entry.SetField("iteration", i)
		PutLogEntry(entry)
	}
}

// ============================================================
// 2. 并发安全测试
// ============================================================

// TestPoolConcurrency 测试并发安全
func TestPoolConcurrency(t *testing.T) {
	var wg sync.WaitGroup
	workers := 100
	iterations := 1000

	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				entry := GetLogEntry()
				entry.ID = "test-id"
				entry.Message = "test message"
				entry.Level = "info"
				entry.Service = "test-service"
				entry.SetField("worker_id", workerID)
				entry.SetField("iteration", j)
				PutLogEntry(entry)
			}
		}(i)
	}
	wg.Wait()
}

// TestPoolDataRace 测试数据竞争（使用 -race 标志运行）
func TestPoolDataRace(t *testing.T) {
	var wg sync.WaitGroup
	workers := 10
	iterations := 100

	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				entry := GetLogEntry()

				// 填充各种字段
				entry.ID = "test-id"
				entry.Message = "test message"
				entry.Level = "info"
				entry.Service = "test-service"
				entry.TraceID = "trace-123"
				entry.SpanID = "span-123"
				entry.UserID = "user-123"
				entry.Timestamp = time.Now()
				entry.SetField("worker", workerID)
				entry.SetField("iter", j)

				// 归还
				PutLogEntry(entry)
			}
		}(i)
	}
	wg.Wait()
}

// ============================================================
// 3. 与 Logger 集成测试
// ============================================================

// mockSender 用于测试的 mock sender
type mockSender struct {
	mu      sync.Mutex
	entries []*sender.LogEntry
}

func (m *mockSender) Send(entry *sender.LogEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	// 深拷贝存储，避免引用被修改
	copied := entry.DeepCopy()
	m.entries = append(m.entries, &copied)
	return nil
}

func (m *mockSender) SendRecord(record sender.LogRecord) error {
	return m.Send(record.ToLogEntry())
}

func (m *mockSender) SendMap(data map[string]interface{}) error {
	return nil
}

func (m *mockSender) Flush() error {
	return nil
}

func (m *mockSender) Close() error {
	return nil
}

func (m *mockSender) Stats() *sender.SenderStats {
	return &sender.SenderStats{}
}

func (m *mockSender) getEntries() []*sender.LogEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.entries
}

// TestLoggerWithPool 测试 Logger 与 Pool 的集成
func TestLoggerWithPool(t *testing.T) {
	mock := &mockSender{}
	provider := NewLoggerProvider(WithSender(mock))
	defer provider.Shutdown()

	logger := provider.Logger("test-service")

	// 发送多条日志
	for i := 0; i < 100; i++ {
		logger.Info("test message", "count", i, "type", "test")
	}

	// 验证日志已发送
	entries := mock.getEntries()
	if len(entries) != 100 {
		t.Errorf("期望 100 条日志，实际 %d 条", len(entries))
	}

	// 验证日志内容
	for i, entry := range entries {
		if entry.Message != "test message" {
			t.Errorf("日志 %d: Message = %s, 期望 test message", i, entry.Message)
		}
		if entry.Level != "info" {
			t.Errorf("日志 %d: Level = %s, 期望 info", i, entry.Level)
		}
		if entry.Service != "test-service" {
			t.Errorf("日志 %d: Service = %s, 期望 test-service", i, entry.Service)
		}
	}
}

// TestLoggerWithPoolAndContext 测试带 context 的日志
func TestLoggerWithPoolAndContext(t *testing.T) {
	mock := &mockSender{}
	provider := NewLoggerProvider(WithSender(mock))
	defer provider.Shutdown()

	logger := provider.Logger("test-service")

	// 创建带 context 的 logger
	ctx := context.WithValue(context.Background(), TraceIDKey, "trace-123")
	ctx = context.WithValue(ctx, UserIDKey, "user-456")
	ctxLogger := logger.WithContext(ctx)

	// 发送日志
	ctxLogger.Info("test with context", "extra", "value")

	// 验证 context 字段
	entries := mock.getEntries()
	if len(entries) != 1 {
		t.Fatalf("期望 1 条日志，实际 %d 条", len(entries))
	}

	entry := entries[0]
	if entry.TraceID != "trace-123" {
		t.Errorf("TraceID = %s, 期望 trace-123", entry.TraceID)
	}
	if entry.UserID != "user-456" {
		t.Errorf("UserID = %s, 期望 user-456", entry.UserID)
	}
}

// ============================================================
// 4. 内存泄漏测试
// ============================================================

// TestPoolNoMemoryLeak 测试不会内存泄漏
func TestPoolNoMemoryLeak(t *testing.T) {
	// 这个测试主要用于配合 -memprofile 使用
	// 运行：go test -run TestPoolNoMemoryLeak -memprofile mem.prof

	for i := 0; i < 10000; i++ {
		entry := GetLogEntry()
		entry.ID = "test-id"
		entry.Message = "test message with some longer content to simulate real usage"
		entry.Level = "info"
		entry.Service = "test-service"
		entry.TraceID = "trace-123456789"
		entry.SetField("iteration", i)
		entry.SetField("data", "some test data")
		PutLogEntry(entry)
	}
}

// ============================================================
// 5. Benchmark
// ============================================================

// BenchmarkWithPool 使用 sync.Pool 的性能
func BenchmarkWithPool(b *testing.B) {
	var result *sender.LogEntry
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		entry := GetLogEntry()
		entry.ID = "test-id"
		entry.Message = "test message"
		entry.Level = "info"
		entry.Service = "test-service"
		entry.SetField("key1", "value1")
		entry.SetField("key2", 123)
		result = entry
		PutLogEntry(entry)
	}

	_ = result
}

// BenchmarkWithoutPool 不使用 sync.Pool 的性能
func BenchmarkWithoutPool(b *testing.B) {
	var result *sender.LogEntry
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		entry := &sender.LogEntry{
			ID:      "test-id",
			Message: "test message",
			Level:   "info",
			Service: "test-service",
		}
		entry.SetField("key1", "value1")
		entry.SetField("key2", 123)
		result = entry
	}

	_ = result
}

// BenchmarkWithPool_Parallel 并行使用 Pool
func BenchmarkWithPool_Parallel(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var result *sender.LogEntry
		for pb.Next() {
			entry := GetLogEntry()
			entry.ID = "test-id"
			entry.Message = "test message"
			entry.Level = "info"
			entry.Service = "test-service"
			entry.SetField("key1", "value1")
			entry.SetField("key2", 123)
			result = entry
			PutLogEntry(entry)
		}
		_ = result
	})
}
