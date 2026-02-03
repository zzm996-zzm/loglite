package storage

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/loglite/loglite/internal/model"
)

// TestTieredStore_BasicOperations 测试基本读写操作
func TestTieredStore_BasicOperations(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "tiered_test_*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 创建分级存储
	cfg := Config{
		ConfigKeyPath:          tempDir,
		ConfigKeyHotThreshold:  "1s", // 使用短时间便于测试
		ConfigKeyWarmThreshold: "2s",
		ConfigKeyHotTTL:        "5s",
		ConfigKeyWarmTTL:       "10s",
		ConfigKeyColdTTL:       "15s",
		ConfigKeyMoverInterval: "1m", // 迁移间隔设长，手动控制
	}

	store, err := NewTieredStore(cfg)
	if err != nil {
		t.Fatalf("create tiered store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// 测试写入
	entry := &model.LogEntry{
		ID:        uuid.New().String(),
		Timestamp: time.Now(),
		Service:   "test-service",
		Level:     "info",
		Message:   "test message",
	}

	if err := store.Write(ctx, entry); err != nil {
		t.Fatalf("write: %v", err)
	}

	// 测试 Get
	found, err := store.Get(ctx, entry.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if found.ID != entry.ID {
		t.Errorf("expected ID %s, got %s", entry.ID, found.ID)
	}

	if found.Message != entry.Message {
		t.Errorf("expected message %s, got %s", entry.Message, found.Message)
	}

	// 测试 Stats
	stats, err := store.Stats(ctx)
	if err != nil {
		t.Fatalf("stats: %v", err)
	}

	if stats.Type != "tiered" {
		t.Errorf("expected type 'tiered', got %s", stats.Type)
	}

	if stats.TotalLogs < 1 {
		t.Errorf("expected at least 1 log, got %d", stats.TotalLogs)
	}

	// 测试 Health
	if err := store.Health(ctx); err != nil {
		t.Fatalf("health: %v", err)
	}
}

// TestTieredStore_BatchWrite 测试批量写入
func TestTieredStore_BatchWrite(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tiered_batch_test_*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewTieredStore(Config{
		ConfigKeyPath:          tempDir,
		ConfigKeyMoverInterval: "1m",
	})
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// 批量写入
	entries := make([]*model.LogEntry, 100)
	for i := range entries {
		entries[i] = &model.LogEntry{
			ID:        uuid.New().String(),
			Timestamp: time.Now(),
			Service:   "batch-service",
			Level:     "debug",
			Message:   "batch message",
		}
	}

	if err := store.WriteMany(ctx, entries); err != nil {
		t.Fatalf("write many: %v", err)
	}

	// 验证写入
	stats, _ := store.Stats(ctx)
	if stats.TotalLogs < 100 {
		t.Errorf("expected at least 100 logs, got %d", stats.TotalLogs)
	}
}

// TestTieredStore_Query 测试查询功能
func TestTieredStore_Query(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tiered_query_test_*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewTieredStore(Config{
		ConfigKeyPath:          tempDir,
		ConfigKeyMoverInterval: "1m",
	})
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// 写入不同服务的日志
	now := time.Now()
	services := []string{"api", "worker", "scheduler"}
	levels := []string{"info", "error", "warn"}

	for i := 0; i < 30; i++ {
		entry := &model.LogEntry{
			ID:        uuid.New().String(),
			Timestamp: now.Add(time.Duration(-i) * time.Minute),
			Service:   services[i%3],
			Level:     levels[i%3],
			Message:   "test message",
		}
		store.Write(ctx, entry)
	}

	// 测试服务过滤
	result, err := store.Query(ctx, &Query{
		Services: []string{"api"},
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}

	for _, e := range result.Entries {
		if e.Service != "api" {
			t.Errorf("expected service 'api', got %s", e.Service)
		}
	}

	// 测试级别过滤
	result, err = store.Query(ctx, &Query{
		Levels: []string{"error"},
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}

	for _, e := range result.Entries {
		if e.Level != "error" {
			t.Errorf("expected level 'error', got %s", e.Level)
		}
	}

	// 测试时间范围
	result, err = store.Query(ctx, &Query{
		From:  now.Add(-10 * time.Minute),
		To:    now,
		Limit: 50,
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}

	if len(result.Entries) == 0 {
		t.Error("expected non-empty result")
	}
}

// TestTieredStore_QueryRouting 测试查询路由
func TestTieredStore_QueryRouting(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tiered_routing_test_*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewTieredStore(Config{
		ConfigKeyPath:          tempDir,
		ConfigKeyHotThreshold:  "2h",
		ConfigKeyWarmThreshold: "24h",
		ConfigKeyMoverInterval: "1m",
	})
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer store.Close()

	tiered := store.(*TieredStore)

	// 测试查询路由逻辑
	now := time.Now()

	testCases := []struct {
		name          string
		query         *Query
		expectedTiers int // 预期查询的层数
	}{
		{
			name:          "no time range - all tiers",
			query:         &Query{},
			expectedTiers: 3,
		},
		{
			name: "recent data - hot only",
			query: &Query{
				From: now.Add(-1 * time.Hour),
				To:   now,
			},
			expectedTiers: 1,
		},
		{
			name: "old data - cold only",
			query: &Query{
				From: now.Add(-48 * time.Hour),
				To:   now.Add(-25 * time.Hour),
			},
			expectedTiers: 1,
		},
		{
			name: "cross hot and warm",
			query: &Query{
				From: now.Add(-5 * time.Hour),
				To:   now,
			},
			expectedTiers: 2,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tiers := tiered.routeQuery(tc.query)
			if len(tiers) != tc.expectedTiers {
				t.Errorf("expected %d tiers, got %d", tc.expectedTiers, len(tiers))
			}
		})
	}
}

// TestMover_Config 测试迁移器配置
func TestMover_Config(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "mover_test_*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewTieredStore(Config{
		ConfigKeyPath:           tempDir,
		ConfigKeyHotThreshold:   "100ms",
		ConfigKeyWarmThreshold:  "200ms",
		ConfigKeyMoverInterval:  "50ms",
		ConfigKeyMoverBatchSize: 100,
	})
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer store.Close()

	tiered := store.(*TieredStore)

	// 验证阈值配置
	if tiered.hotThreshold != 100*time.Millisecond {
		t.Errorf("expected hot threshold 100ms, got %v", tiered.hotThreshold)
	}

	if tiered.warmThreshold != 200*time.Millisecond {
		t.Errorf("expected warm threshold 200ms, got %v", tiered.warmThreshold)
	}

	// 验证迁移器配置
	if tiered.mover.cfg.BatchSize != 100 {
		t.Errorf("expected batch size 100, got %d", tiered.mover.cfg.BatchSize)
	}
}

// TestTieredCreator_Validate 测试配置验证
func TestTieredCreator_Validate(t *testing.T) {
	creator := &tieredCreator{}

	testCases := []struct {
		name      string
		cfg       Config
		expectErr bool
	}{
		{
			name:      "valid config",
			cfg:       Config{ConfigKeyPath: "/tmp/test"},
			expectErr: false,
		},
		{
			name:      "missing path",
			cfg:       Config{},
			expectErr: true,
		},
		{
			name: "invalid thresholds",
			cfg: Config{
				ConfigKeyPath:          "/tmp/test",
				ConfigKeyHotThreshold:  "25h", // > warm_threshold
				ConfigKeyWarmThreshold: "24h",
			},
			expectErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := creator.Validate(tc.cfg)
			if tc.expectErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// BenchmarkTieredStore_Write 写入性能测试
func BenchmarkTieredStore_Write(b *testing.B) {
	tempDir, _ := os.MkdirTemp("", "tiered_bench_*")
	defer os.RemoveAll(tempDir)

	store, _ := NewTieredStore(Config{
		ConfigKeyPath:          tempDir,
		ConfigKeyMoverInterval: "1h", // 禁用迁移
	})
	defer store.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		entry := &model.LogEntry{
			ID:        uuid.New().String(),
			Timestamp: time.Now(),
			Service:   "bench-service",
			Level:     "info",
			Message:   "benchmark message",
		}
		store.Write(ctx, entry)
	}
}

// BenchmarkTieredStore_Query 查询性能测试
func BenchmarkTieredStore_Query(b *testing.B) {
	tempDir, _ := os.MkdirTemp("", "tiered_bench_query_*")
	defer os.RemoveAll(tempDir)

	store, _ := NewTieredStore(Config{
		"path":           tempDir,
		"mover_interval": "1h",
	})
	defer store.Close()

	ctx := context.Background()

	// 预写入数据
	for i := 0; i < 10000; i++ {
		entry := &model.LogEntry{
			ID:        uuid.New().String(),
			Timestamp: time.Now(),
			Service:   "bench-service",
			Level:     "info",
			Message:   "benchmark message",
		}
		store.Write(ctx, entry)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Query(ctx, &Query{
			Services: []string{"bench-service"},
			Limit:    100,
		})
	}
}
