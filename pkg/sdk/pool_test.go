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
	
	// 验证 Metadata 已初始化
	if entry.Metadata == nil {
		t.Error("Metadata 未初始化")
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
	entry.Metadata["key1"] = "value1"
	entry.Metadata["key2"] = 123
	
	// 记录 Metadata 的长度（map 没有容量概念，只能看长度）
	oldLen := len(entry.Metadata)
	
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
	if entry2.SpanID != "" {
		t.Errorf("SpanID 应为空，实际为: %s", entry2.SpanID)
	}
	if entry2.UserID != "" {
		t.Errorf("UserID 应为空，实际为: %s", entry2.UserID)
	}
	if entry2.RequestID != "" {
		t.Errorf("RequestID 应为空，实际为: %s", entry2.RequestID)
	}
	if entry2.IP != "" {
		t.Errorf("IP 应为空，实际为: %s", entry2.IP)
	}
	if entry2.Caller != "" {
		t.Errorf("Caller 应为空，实际为: %s", entry2.Caller)
	}
	if entry2.Function != "" {
		t.Errorf("Function 应为空，实际为: %s", entry2.Function)
	}
	if entry2.Package != "" {
		t.Errorf("Package 应为空，实际为: %s", entry2.Package)
	}
	if entry2.StackTrace != "" {
		t.Errorf("StackTrace 应为空，实际为: %s", entry2.StackTrace)
	}
	if entry2.StackHash != "" {
		t.Errorf("StackHash 应为空，实际为: %s", entry2.StackHash)
	}
	
	// 验证时间字段已重置
	if !entry2.Timestamp.IsZero() {
		t.Errorf("Timestamp 应为零值，实际为: %v", entry2.Timestamp)
	}
	if !entry2.ReceivedAt.IsZero() {
		t.Errorf("ReceivedAt 应为零值，实际为: %v", entry2.ReceivedAt)
	}
	if !entry2.StoredAt.IsZero() {
		t.Errorf("StoredAt 应为零值，实际为: %v", entry2.StoredAt)
	}
	
	// 验证 Metadata 已清空
	if len(entry2.Metadata) != 0 {
		t.Errorf("Metadata 应为空，实际长度为: %d, 内容: %v", len(entry2.Metadata), entry2.Metadata)
	}
	
	// 提示：由于是 map，底层容量无法直接查看，但通过 sync.Pool 复用对象可以减少内存分配
	_ = oldLen // 避免未使用变量警告
}

// TestPoolReuse 测试对象复用
func TestPoolReuse(t *testing.T) {
	// 获取一个 entry
	entry1 := GetLogEntry()
	entry1.ID = "first"
	
	// 记录地址
	ptr1 := entry1
	
	// 归还
	PutLogEntry(entry1)
	
	// 再次获取
	entry2 := GetLogEntry()
	ptr2 := entry2
	
	// 在某些情况下，应该能复用同一个对象
	// 但由于 sync.Pool 的实现细节，不一定每次都复用
	// 所以这里只是记录，不做强制断言
	if ptr1 == ptr2 {
		t.Logf("✅ 对象被成功复用（地址相同）")
	} else {
		t.Logf("⚠️  对象未复用（地址不同），这在 sync.Pool 中是正常的")
	}
	
	// 但必须保证新获取的 entry 是干净的
	if entry2.ID != "" {
		t.Errorf("复用的 entry 字段未清空: ID=%s", entry2.ID)
	}
}

// ============================================================
// 2. 并发安全测试
// ============================================================

// TestPoolConcurrency 测试并发安全性
func TestPoolConcurrency(t *testing.T) {
	const goroutines = 100
	const iterations = 1000
	
	var wg sync.WaitGroup
	wg.Add(goroutines)
	
	// 启动多个 goroutine 并发使用 pool
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			
			for j := 0; j < iterations; j++ {
				// 获取 entry
				entry := GetLogEntry()
				
				// 填充数据
				entry.ID = "test-id"
				entry.Message = "test message"
				entry.Metadata["key"] = "value"
				
				// 模拟一些处理
				time.Sleep(time.Microsecond)
				
				// 归还 entry
				PutLogEntry(entry)
			}
		}(i)
	}
	
	wg.Wait()
	t.Logf("✅ 并发测试通过：%d goroutines × %d iterations = %d 次操作", 
		goroutines, iterations, goroutines*iterations)
}

// TestPoolDataRace 测试数据竞争（使用 go test -race 运行）
func TestPoolDataRace(t *testing.T) {
	const goroutines = 10
	var wg sync.WaitGroup
	wg.Add(goroutines)
	
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			
			entry := GetLogEntry()
			entry.Metadata["test"] = "value"
			PutLogEntry(entry)
		}()
	}
	
	wg.Wait()
}

// ============================================================
// 3. 实际场景测试（Logger 集成）
// ============================================================

// TestLoggerWithPool 测试 Logger 使用 pool 的正确性
func TestLoggerWithPool(t *testing.T) {
	// 创建一个 mock sender 来验证数据
	mockSender := &mockSender{
		entries: make([]*sender.LogEntry, 0),
	}
	
	// 创建 provider 和 logger
	provider := &LoggerProvider{
		sender:       mockSender,
		enableCaller: false,
		enableStack:  false,
	}
	
	logger := provider.Logger("test-service")
	
	// 记录多条日志
	logger.Info("message 1", "key1", "value1")
	logger.Warn("message 2", "key2", "value2")
	logger.Error("message 3", "key3", "value3")
	
	// 验证发送的数据
	if len(mockSender.entries) != 3 {
		t.Fatalf("应该发送 3 条日志，实际发送了 %d 条", len(mockSender.entries))
	}
	
	// 验证每条日志的数据正确性
	tests := []struct {
		idx     int
		level   string
		message string
		key     string
		value   string
	}{
		{0, "info", "message 1", "key1", "value1"},
		{1, "warn", "message 2", "key2", "value2"},
		{2, "error", "message 3", "key3", "value3"},
	}
	
	for _, tt := range tests {
		entry := mockSender.entries[tt.idx]
		
		if entry.Level != tt.level {
			t.Errorf("日志 %d: Level 错误，期望 %s，实际 %s", tt.idx, tt.level, entry.Level)
		}
		if entry.Message != tt.message {
			t.Errorf("日志 %d: Message 错误，期望 %s，实际 %s", tt.idx, tt.message, entry.Message)
		}
		if entry.Service != "test-service" {
			t.Errorf("日志 %d: Service 错误，期望 test-service，实际 %s", tt.idx, entry.Service)
		}
		if entry.ID == "" {
			t.Errorf("日志 %d: ID 为空", tt.idx)
		}
		if entry.Timestamp.IsZero() {
			t.Errorf("日志 %d: Timestamp 为零值", tt.idx)
		}
		if val, ok := entry.Metadata[tt.key]; !ok || val != tt.value {
			t.Errorf("日志 %d: Metadata[%s] 错误，期望 %v，实际 %v", tt.idx, tt.key, tt.value, val)
		}
	}
	
	// 验证没有数据污染（前一条日志的数据不会影响后一条）
	for i := 0; i < len(mockSender.entries)-1; i++ {
		e1 := mockSender.entries[i]
		e2 := mockSender.entries[i+1]
		
		if e1.ID == e2.ID {
			t.Errorf("日志 %d 和 %d 的 ID 相同: %s", i, i+1, e1.ID)
		}
		if e1.Message == e2.Message {
			t.Errorf("日志 %d 和 %d 的 Message 相同: %s", i, i+1, e1.Message)
		}
	}
}

// TestLoggerWithPoolAndContext 测试 Logger 使用 pool + context 的正确性
func TestLoggerWithPoolAndContext(t *testing.T) {
	mockSender := &mockSender{
		entries: make([]*sender.LogEntry, 0),
	}
	
	provider := &LoggerProvider{
		sender:       mockSender,
		enableCaller: false,
		enableStack:  false,
	}
	
	logger := provider.Logger("test-service")
	
	// 使用 WithContext
	ctx := context.Background()
	ctx = context.WithValue(ctx, TraceIDKey, "trace-123")
	ctx = context.WithValue(ctx, RequestIDKey, "req-456")
	
	ctxLogger := logger.WithContext(ctx)
	ctxLogger.Info("test with context", "user", "alice")
	
	// 验证 context 数据正确写入
	if len(mockSender.entries) != 1 {
		t.Fatalf("应该发送 1 条日志，实际发送了 %d 条", len(mockSender.entries))
	}
	
	entry := mockSender.entries[0]
	
	if entry.TraceID != "trace-123" {
		t.Errorf("TraceID 错误，期望 trace-123，实际 %s", entry.TraceID)
	}
	if entry.RequestID != "req-456" {
		t.Errorf("RequestID 错误，期望 req-456，实际 %s", entry.RequestID)
	}
	if val, ok := entry.Metadata["user"]; !ok || val != "alice" {
		t.Errorf("Metadata[user] 错误，期望 alice，实际 %v", val)
	}
}

// ============================================================
// 4. 内存泄漏测试
// ============================================================

// TestPoolNoMemoryLeak 测试 pool 不会导致内存泄漏
func TestPoolNoMemoryLeak(t *testing.T) {
	// 创建大量 entry 并归还
	for i := 0; i < 10000; i++ {
		entry := GetLogEntry()
		
		// 填充大量数据
		entry.ID = "very-long-id-" + string(make([]byte, 1000))
		entry.Message = "very long message " + string(make([]byte, 1000))
		
		// 填充大量 Metadata
		for j := 0; j < 100; j++ {
			entry.Metadata[string(rune(j))] = make([]byte, 100)
		}
		
		// 归还（应该清空引用，避免内存泄漏）
		PutLogEntry(entry)
	}
	
	// 再次获取，验证是干净的
	entry := GetLogEntry()
	if entry.ID != "" {
		t.Errorf("ID 应为空（可能存在内存泄漏）")
	}
	if len(entry.Metadata) != 0 {
		t.Errorf("Metadata 应为空（可能存在内存泄漏）")
	}
}

// ============================================================
// 5. 性能基准测试
// ============================================================

// BenchmarkWithPool 测试使用 pool 的性能
func BenchmarkWithPool(b *testing.B) {
	b.ReportAllocs()
	
	var result *sender.LogEntry
	
	for i := 0; i < b.N; i++ {
		entry := GetLogEntry()
		entry.ID = "test-id"
		entry.Message = "test message"
		entry.Level = "info"
		entry.Service = "test-service"
		entry.Metadata["key1"] = "value1"
		entry.Metadata["key2"] = "value2"
		result = entry // 防止编译器优化
		PutLogEntry(entry)
	}
	
	_ = result // 使用 result
}

// BenchmarkWithoutPool 测试不使用 pool 的性能（对比）
func BenchmarkWithoutPool(b *testing.B) {
	b.ReportAllocs()
	
	var result *sender.LogEntry
	
	for i := 0; i < b.N; i++ {
		entry := &sender.LogEntry{
			ID:       "test-id",
			Message:  "test message",
			Level:    "info",
			Service:  "test-service",
			Metadata: make(map[string]interface{}),
		}
		entry.Metadata["key1"] = "value1"
		entry.Metadata["key2"] = "value2"
		result = entry // 防止编译器优化
	}
	
	_ = result // 使用 result
}

// BenchmarkWithPool_Parallel 测试并发场景下使用 pool 的性能
func BenchmarkWithPool_Parallel(b *testing.B) {
	b.ReportAllocs()
	
	b.RunParallel(func(pb *testing.PB) {
		var result *sender.LogEntry
		for pb.Next() {
			entry := GetLogEntry()
			entry.ID = "test-id"
			entry.Message = "test message"
			entry.Metadata["key"] = "value"
			result = entry // 防止编译器优化
			PutLogEntry(entry)
		}
		_ = result
	})
}

// ============================================================
// Mock Sender (用于测试)
// ============================================================

type mockSender struct {
	mu      sync.Mutex
	entries []*sender.LogEntry
}

func (m *mockSender) Send(entry *sender.LogEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// 深拷贝 entry（因为 pool 会复用）
	copied := &sender.LogEntry{
		ID:          entry.ID,
		Timestamp:   entry.Timestamp,
		Message:     entry.Message,
		Level:       entry.Level,
		Service:     entry.Service,
		TraceID:     entry.TraceID,
		SpanID:      entry.SpanID,
		UserID:      entry.UserID,
		RequestID:   entry.RequestID,
		IP:          entry.IP,
		Caller:      entry.Caller,
		Function:    entry.Function,
		Package:     entry.Package,
		StackTrace:  entry.StackTrace,
		StackHash:   entry.StackHash,
		ReceivedAt:  entry.ReceivedAt,
		StoredAt:    entry.StoredAt,
		Metadata:    make(map[string]interface{}),
	}
	
	for k, v := range entry.Metadata {
		copied.Metadata[k] = v
	}
	
	m.entries = append(m.entries, copied)
	return nil
}

func (m *mockSender) SendRecord(record sender.LogRecord) error {
	// 简单实现，测试中不使用
	return nil
}

func (m *mockSender) SendMap(data map[string]interface{}) error {
	// 简单实现，测试中不使用
	return nil
}

func (m *mockSender) Flush() error {
	return nil
}

func (m *mockSender) Close() error {
	return nil
}

func (m *mockSender) Stats() *sender.SenderStats {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	return &sender.SenderStats{
		TotalSent:    int64(len(m.entries)),
		TotalFailed:  0,
		TotalDropped: 0,
		BufferSize:   0,
		PendingCount: 0,
	}
}
