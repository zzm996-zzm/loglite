// benchmarks/unit/concurrent_bench_test.go
package unit

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/loglite/loglite/pkg/sdk"
	"github.com/loglite/loglite/pkg/sdk/sender"
)

// cleanupTestFiles 清理测试文件，确保每次测试都是干净的环境
func cleanupTestFiles(baseDir string) {
	// 清理快照文件
	snapshotDir := baseDir + "/snapshot"
	os.RemoveAll(snapshotDir)
	os.MkdirAll(snapshotDir, 0755)
	os.Remove(filepath.Join(snapshotDir, "buffer.snapshot"))
	os.Remove(filepath.Join(snapshotDir, "buffer.snapshot.tmp"))

	// 清理降级文件
	os.Remove(baseDir + "/fallback.log")

	// 清理 WAL 文件
	walDir := baseDir + "/wal"
	os.RemoveAll(walDir)
	os.MkdirAll(walDir, 0755)

	// 清理 WAL 相关文件
	os.Remove(filepath.Join(walDir, "wal.log"))
	os.Remove(filepath.Join(walDir, "sent.log"))
	os.Remove(filepath.Join(walDir, "seq.meta"))
	os.Remove(filepath.Join(walDir, "seq.meta.tmp"))
}

// BenchmarkBalanced_Concurrent 测试 Balanced 模式在不同并发度下的性能
func BenchmarkBalanced_Concurrent(b *testing.B) {
	concurrencyLevels := []int{1, 2, 4, 8, 16, 32, 64}

	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("Concurrency-%d", concurrency), func(b *testing.B) {
			tempDir := b.TempDir()
			cleanupTestFiles(tempDir)

			provider := sdk.NewLoggerProvider(
				sdk.WithEndpoint("http://localhost:8081"),
				sdk.WithReliability(sender.Balanced),
				sdk.WithSnapshotDir(tempDir+"/snapshot"),
				sdk.WithFallbackFile(tempDir+"/fallback.log"),
			)
			defer provider.Shutdown()

			logger := provider.Logger("concurrent-bench")

			// 每个 goroutine 的工作量
			workPerGoroutine := b.N / concurrency
			if workPerGoroutine == 0 {
				workPerGoroutine = 1
			}

			var wg sync.WaitGroup
			var totalOps int64

			b.ResetTimer()
			b.ReportAllocs()

			start := time.Now()
			for i := 0; i < concurrency; i++ {
				wg.Add(1)
				go func(id int) {
					defer wg.Done()
					for j := 0; j < workPerGoroutine; j++ {
						logger.Info("并发测试日志",
							"goroutine_id", id,
							"iteration", j,
							"timestamp", time.Now().UnixNano(),
						)
						atomic.AddInt64(&totalOps, 1)
					}
				}(i)
			}
			wg.Wait()
			duration := time.Since(start)

			b.StopTimer()

			// 报告额外指标
			ops := atomic.LoadInt64(&totalOps)
			b.ReportMetric(float64(ops)/duration.Seconds(), "logs/sec")
			b.ReportMetric(float64(duration.Nanoseconds())/float64(ops), "ns/log")
		})
	}
}

// BenchmarkBalanced_ConcurrentModes 对比不同可靠性模式的并发性能
func BenchmarkBalanced_ConcurrentModes(b *testing.B) {
	modes := []struct {
		name string
		mode sender.Reliability
	}{
		{"BestEffort", sender.BestEffort},
		{"Balanced", sender.Balanced},
		{"Reliable", sender.Reliable},
	}

	concurrency := 16 // 固定并发度

	for _, mode := range modes {
		b.Run(mode.name, func(b *testing.B) {
			tempDir := b.TempDir()
			cleanupTestFiles(tempDir)

			provider := sdk.NewLoggerProvider(
				sdk.WithEndpoint("http://localhost:8081"),
				sdk.WithReliability(mode.mode),
				sdk.WithWALDir(tempDir+"/wal"),
				sdk.WithSnapshotDir(tempDir+"/snapshot"),
				sdk.WithFallbackFile(tempDir+"/fallback.log"),
			)
			defer provider.Shutdown()

			logger := provider.Logger("concurrent-mode-bench")

			workPerGoroutine := b.N / concurrency
			if workPerGoroutine == 0 {
				workPerGoroutine = 1
			}

			var wg sync.WaitGroup
			var totalOps int64

			b.ResetTimer()
			b.ReportAllocs()

			start := time.Now()
			for i := 0; i < concurrency; i++ {
				wg.Add(1)
				go func(id int) {
					defer wg.Done()
					for j := 0; j < workPerGoroutine; j++ {
						logger.Info("并发模式测试",
							"mode", mode.name,
							"goroutine_id", id,
							"iteration", j,
						)
						atomic.AddInt64(&totalOps, 1)
					}
				}(i)
			}
			wg.Wait()
			duration := time.Since(start)

			b.StopTimer()

			ops := atomic.LoadInt64(&totalOps)
			b.ReportMetric(float64(ops)/duration.Seconds(), "logs/sec")
			b.ReportMetric(float64(duration.Nanoseconds())/float64(ops), "ns/log")
		})
	}
}

// BenchmarkBalanced_ConcurrentLatency 测试并发写入的延迟分布
func BenchmarkBalanced_ConcurrentLatency(b *testing.B) {
	concurrency := 16
	tempDir := b.TempDir()
	cleanupTestFiles(tempDir)

	provider := sdk.NewLoggerProvider(
		sdk.WithEndpoint("http://localhost:8081"),
		sdk.WithReliability(sender.Balanced),
		sdk.WithSnapshotDir(tempDir+"/snapshot"),
		sdk.WithFallbackFile(tempDir+"/fallback.log"),
	)
	defer provider.Shutdown()

	logger := provider.Logger("latency-bench")

	workPerGoroutine := b.N / concurrency
	if workPerGoroutine == 0 {
		workPerGoroutine = 1
	}

	var wg sync.WaitGroup
	latencies := make([]time.Duration, 0, b.N)
	var mu sync.Mutex

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < workPerGoroutine; j++ {
				start := time.Now()
				logger.Info("延迟测试",
					"goroutine_id", id,
					"iteration", j,
				)
				latency := time.Since(start)

				mu.Lock()
				latencies = append(latencies, latency)
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()

	b.StopTimer()

	// 计算延迟统计
	if len(latencies) > 0 {
		var sum time.Duration
		var max, min time.Duration = latencies[0], latencies[0]
		for _, lat := range latencies {
			sum += lat
			if lat > max {
				max = lat
			}
			if lat < min {
				min = lat
			}
		}
		avg := sum / time.Duration(len(latencies))
		b.ReportMetric(float64(avg.Nanoseconds()), "avg_latency_ns")
		b.ReportMetric(float64(max.Nanoseconds()), "max_latency_ns")
		b.ReportMetric(float64(min.Nanoseconds()), "min_latency_ns")
	}
}

// BenchmarkBalanced_ConcurrentThroughput 测试不同并发度下的吞吐量
func BenchmarkBalanced_ConcurrentThroughput(b *testing.B) {
	concurrencyLevels := []int{1, 4, 8, 16, 32, 64, 128}

	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("Throughput-%d", concurrency), func(b *testing.B) {
			tempDir := b.TempDir()
			cleanupTestFiles(tempDir)

			provider := sdk.NewLoggerProvider(
				sdk.WithEndpoint("http://localhost:8081"),
				sdk.WithReliability(sender.Balanced),
				sdk.WithSnapshotDir(tempDir+"/snapshot"),
				sdk.WithFallbackFile(tempDir+"/fallback.log"),
			)
			defer provider.Shutdown()

			logger := provider.Logger("throughput-bench")

			// 固定总工作量，测试不同并发度下的吞吐量
			totalWork := 10000
			workPerGoroutine := totalWork / concurrency
			if workPerGoroutine == 0 {
				workPerGoroutine = 1
			}

			var wg sync.WaitGroup
			var totalOps int64

			b.ResetTimer()
			b.ReportAllocs()

			start := time.Now()
			for i := 0; i < concurrency; i++ {
				wg.Add(1)
				go func(id int) {
					defer wg.Done()
					for j := 0; j < workPerGoroutine; j++ {
						logger.Info("吞吐量测试",
							"goroutine_id", id,
							"iteration", j,
						)
						atomic.AddInt64(&totalOps, 1)
					}
				}(i)
			}
			wg.Wait()
			duration := time.Since(start)

			b.StopTimer()

			ops := atomic.LoadInt64(&totalOps)
			throughput := float64(ops) / duration.Seconds()
			b.ReportMetric(throughput, "logs/sec")
			b.ReportMetric(float64(concurrency), "goroutines")
		})
	}
}

// BenchmarkBalanced_ConcurrentCPU 测试 CPU 使用情况
func BenchmarkBalanced_ConcurrentCPU(b *testing.B) {
	concurrency := runtime.GOMAXPROCS(0) * 2 // 2倍 CPU 核心数
	tempDir := b.TempDir()
	cleanupTestFiles(tempDir)

	provider := sdk.NewLoggerProvider(
		sdk.WithEndpoint("http://localhost:8081"),
		sdk.WithReliability(sender.Balanced),
		sdk.WithSnapshotDir(tempDir+"/snapshot"),
		sdk.WithFallbackFile(tempDir+"/fallback.log"),
	)
	defer provider.Shutdown()

	logger := provider.Logger("cpu-bench")

	workPerGoroutine := b.N / concurrency
	if workPerGoroutine == 0 {
		workPerGoroutine = 1
	}

	var wg sync.WaitGroup
	var totalOps int64

	b.ResetTimer()
	b.ReportAllocs()

	start := time.Now()
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < workPerGoroutine; j++ {
				logger.Info("CPU压力测试",
					"goroutine_id", id,
					"iteration", j,
				)
				atomic.AddInt64(&totalOps, 1)
			}
		}(i)
	}
	wg.Wait()
	duration := time.Since(start)

	b.StopTimer()

	ops := atomic.LoadInt64(&totalOps)
	b.ReportMetric(float64(ops)/duration.Seconds(), "logs/sec")
	b.ReportMetric(float64(concurrency), "goroutines")
}
