// benchmarks/unit/storage_bench_test.go
package unit

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/loglite/loglite/internal/storage"
	"github.com/loglite/loglite/pkg/sdk/sender"
)

// BenchmarkStorage_Write 测试存储写入性能
func BenchmarkStorage_Write(b *testing.B) {
	tempDir := b.TempDir()

	store, err := storage.NewBadger(
		tempDir,
		storage.WithConfig(storage.Config{
			"ttl": 168 * time.Hour,
		}),
	)
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()

	// ✅ 修复：正确生成 ID
	logs := make([]*sender.LogEntry, b.N)
	for i := 0; i < b.N; i++ {
		logs[i] = &sender.LogEntry{
			ID:        fmt.Sprintf("test-%d", i),
			Timestamp: time.Now(),
			Message:   "测试日志消息",
			Level:     "info",
			Service:   "test-service",
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		if err := store.Write(ctx, logs[i]); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkStorage_BatchWrite 测试批量写入性能
func BenchmarkStorage_BatchWrite(b *testing.B) {
	tempDir := b.TempDir()

	store, err := storage.NewBadger(
		tempDir,
		storage.WithConfig(storage.Config{
			"ttl": 168 * time.Hour,
		}),
	)
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()

	// 测试不同的批量大小
	batchSizes := []int{10, 50, 100, 500, 1000}

	for _, batchSize := range batchSizes {
		b.Run(fmt.Sprintf("BatchSize_%d", batchSize), func(b *testing.B) {
			batchCount := b.N / batchSize
			if batchCount == 0 {
				batchCount = 1
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < batchCount; i++ {
				logs := make([]*sender.LogEntry, batchSize)
				for j := 0; j < batchSize; j++ {
					logs[j] = &sender.LogEntry{
						ID:        fmt.Sprintf("batch-%d-%d", i, j),
						Timestamp: time.Now(),
						Message:   "批量测试日志",
						Level:     "info",
						Service:   "test-service",
					}
				}
				if err := store.WriteMany(ctx, logs); err != nil {
					b.Fatal(err)
				}
			}

			// 报告吞吐量
			b.ReportMetric(float64(batchCount*batchSize)/b.Elapsed().Seconds(), "logs/sec")
		})
	}
}

// BenchmarkStorage_Read 测试读取性能
func BenchmarkStorage_Read(b *testing.B) {
	tempDir := b.TempDir()

	store, err := storage.NewBadger(
		tempDir,
		storage.WithConfig(storage.Config{
			"ttl": 168 * time.Hour,
		}),
	)
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()

	// 先写入测试数据
	const dataSize = 10000
	keys := make([]string, dataSize)
	for i := 0; i < dataSize; i++ {
		key := fmt.Sprintf("read-test-%d", i)
		keys[i] = key
		log := &sender.LogEntry{
			ID:        key,
			Timestamp: time.Now(),
			Message:   "读取测试日志",
			Level:     "info",
			Service:   "test-service",
		}
		if err := store.Write(ctx, log); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := store.Get(ctx, keys[i%dataSize])
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkStorage_QueryByTime 测试时间范围查询
func BenchmarkStorage_QueryByTime(b *testing.B) {
	tempDir := b.TempDir()

	store, err := storage.NewBadger(
		tempDir,
		storage.WithConfig(storage.Config{
			"ttl": 168 * time.Hour,
		}),
	)
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()

	// 写入大量测试数据
	const dataSize = 10000
	baseTime := time.Now().Add(-24 * time.Hour)
	for i := 0; i < dataSize; i++ {
		log := &sender.LogEntry{
			ID:        fmt.Sprintf("query-test-%d", i),
			Timestamp: baseTime.Add(time.Duration(i) * time.Second),
			Message:   "查询测试日志",
			Level:     "info",
			Service:   "test-service",
		}
		if err := store.Write(ctx, log); err != nil {
			b.Fatal(err)
		}
	}

	// 测试不同的查询范围
	tests := []struct {
		name      string
		timeRange time.Duration
		limit     int
	}{
		{"1Hour_Limit100", 1 * time.Hour, 100},
		{"6Hour_Limit100", 6 * time.Hour, 100},
		{"24Hour_Limit100", 24 * time.Hour, 100},
		{"24Hour_Limit1000", 24 * time.Hour, 1000},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			startTime := baseTime
			endTime := baseTime.Add(tt.timeRange)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				result, err := store.Query(ctx, &storage.Query{
					From:  startTime,
					To:    endTime,
					Limit: tt.limit,
				})
				if err != nil {
					b.Fatal(err)
				}
				// 防止编译器优化掉查询
				_ = result.Entries
			}
		})
	}
}

// BenchmarkStorage_QueryByService 测试按服务查询
func BenchmarkStorage_QueryByService(b *testing.B) {
	tempDir := b.TempDir()

	store, err := storage.NewBadger(
		tempDir,
		storage.WithConfig(storage.Config{
			"ttl": 168 * time.Hour,
		}),
	)
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()

	// 写入多个服务的数据
	services := []string{"user-service", "order-service", "payment-service"}
	for i := 0; i < 10000; i++ {
		log := &sender.LogEntry{
			ID:        fmt.Sprintf("service-test-%d", i),
			Timestamp: time.Now(),
			Message:   "服务测试日志",
			Level:     "info",
			Service:   services[i%len(services)],
		}
		if err := store.Write(ctx, log); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		service := services[i%len(services)]
		result, err := store.Query(ctx, &storage.Query{
			Services: []string{service},
			From:     time.Now().Add(-24 * time.Hour),
			To:       time.Now(),
			Limit:    100,
		})
		if err != nil {
			b.Fatal(err)
		}
		_ = result.Entries
	}
}

// BenchmarkStorage_QueryByLevel 测试按级别查询
func BenchmarkStorage_QueryByLevel(b *testing.B) {
	tempDir := b.TempDir()

	store, err := storage.NewBadger(
		tempDir,
		storage.WithConfig(storage.Config{
			"ttl": 168 * time.Hour,
		}),
	)
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()

	// 写入不同级别的日志
	levels := []string{"debug", "info", "warn", "error"}
	for i := 0; i < 10000; i++ {
		log := &sender.LogEntry{
			ID:        fmt.Sprintf("level-test-%d", i),
			Timestamp: time.Now(),
			Message:   "级别测试日志",
			Level:     levels[i%len(levels)],
			Service:   "test-service",
		}
		if err := store.Write(ctx, log); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		level := levels[i%len(levels)]
		result, err := store.Query(ctx, &storage.Query{
			Levels: []string{level},
			From:   time.Now().Add(-24 * time.Hour),
			To:     time.Now(),
			Limit:  100,
		})
		if err != nil {
			b.Fatal(err)
		}
		_ = result.Entries
	}
}

// BenchmarkStorage_ConcurrentWrite 测试并发写入
func BenchmarkStorage_ConcurrentWrite(b *testing.B) {
	tempDir := b.TempDir()

	store, err := storage.NewBadger(
		tempDir,
		storage.WithConfig(storage.Config{
			"ttl": 168 * time.Hour,
		}),
	)
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			log := &sender.LogEntry{
				ID:        fmt.Sprintf("concurrent-%d-%d", b.N, i),
				Timestamp: time.Now(),
				Message:   "并发写入测试",
				Level:     "info",
				Service:   "test-service",
			}
			if err := store.Write(ctx, log); err != nil {
				b.Fatal(err)
			}
			i++
		}
	})
}
