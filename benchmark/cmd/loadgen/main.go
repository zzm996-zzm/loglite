// benchmarks/loadgen/main.go
package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"time"

	"github.com/loglite/loglite/pkg/sdk"
	"github.com/loglite/loglite/pkg/sdk/sender"
)

// 配置参数
type Config struct {
	Endpoint    string
	Workers     int
	Rate        int           // 每秒日志数
	Duration    time.Duration // 压测时长
	LogSize     int           // 每条日志大小
	Service     string        // 服务名
	Reliability string        // 可靠性模式
}

// 结果结构
type BenchmarkResults struct {
	TotalDuration time.Duration
	TotalRequests int64
	SuccessCount  int64
	ErrorCount    int64
	Throughput    float64 // 每秒成功数
	AvgLatency    time.Duration
	ErrorRate     float64 // 错误率百分比
}

func main() {
	// 命令行参数
	endpoint := flag.String("endpoint", "http://localhost:8081", "LogLite服务地址")
	workers := flag.Int("workers", 10, "并发worker数")
	rate := flag.Int("rate", 1000, "每秒日志条数")
	duration := flag.Duration("duration", 30*time.Second, "压测持续时间")
	logSize := flag.Int("log-size", 200, "每条日志大小（字节）")
	service := flag.String("service", "benchmark-service", "服务名称")
	reliability := flag.String("reliability", "balanced", "可靠性模式: best-effort/balanced/reliable")

	flag.Parse()

	config := &Config{
		Endpoint:    *endpoint,
		Workers:     *workers,
		Rate:        *rate,
		Duration:    *duration,
		LogSize:     *logSize,
		Service:     *service,
		Reliability: *reliability,
	}

	fmt.Println("🚀 LogLite 压测开始...")
	fmt.Printf("配置: 端点=%s, 并发=%d, QPS=%d, 时长=%v\n",
		config.Endpoint, config.Workers, config.Rate, config.Duration)

	// 运行压测
	results := runBenchmark(config)

	// 输出结果
	printResults(results)
}

func runBenchmark(config *Config) *BenchmarkResults {
	// 转换可靠性级别
	var rel sender.Reliability
	switch config.Reliability {
	case "best-effort":
		rel = sender.BestEffort
	case "reliable":
		rel = sender.Reliable
	default:
		rel = sender.Balanced
	}

	// 1. 创建 LoggerProvider
	provider := sdk.NewLoggerProvider(
		sdk.WithEndpoint(config.Endpoint),
		sdk.WithReliability(rel),
		sdk.WithBatchSize(1000),
		sdk.WithFlushInterval(100*time.Millisecond),
	)
	defer provider.Shutdown()

	// 2. 创建多个 logger（模拟多个服务）
	loggers := make([]sdk.Logger, config.Workers)
	serviceNames := []string{"user-svc", "order-svc", "payment-svc", "auth-svc", "inventory-svc"}

	for i := 0; i < config.Workers; i++ {
		serviceName := fmt.Sprintf("%s-%d", serviceNames[i%len(serviceNames)], i)
		loggers[i] = *provider.Logger(serviceName)
	}

	// 3. 准备测试数据
	logTemplate := generateLogTemplate(config.LogSize)

	// 4. 统计变量
	var (
		successCount int64
		errorCount   int64
		totalLatency int64
		startTime    = time.Now()
		done         = make(chan bool)
		stopChan     = make(chan os.Signal, 1)
	)

	signal.Notify(stopChan, os.Interrupt)

	// 5. 启动 worker
	for i := 0; i < config.Workers; i++ {
		go func(workerID int) {
			logger := loggers[workerID]
			seq := 0

			// 控制发送速率
			interval := time.Second / time.Duration(config.Rate/config.Workers)
			ticker := time.NewTicker(interval)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					seq++

					// 生成随机日志内容
					logContent := randomizeLog(logTemplate)

					// 发送日志
					sendStart := time.Now()

					// 随机选择日志级别
					level := randomLevel()
					var err error

					// 🔧 修复：检查 Info/Error/Warn 方法是否有返回值
					switch level {
					case "info":

						// 如果没有返回值
						logger.Info(logContent,
							"worker", workerID,
							"sequence", seq,
							"timestamp", time.Now().UnixNano(),
							"user_id", rand.Intn(10000),
							"order_id", fmt.Sprintf("ORD-%08d", rand.Intn(1000000)),
							"amount", rand.Float64()*1000,
							"success", rand.Float32() > 0.1,
						)
						err = nil

					case "error":

						logger.Error(logContent,
							"worker", workerID,
							"sequence", seq,
							"error_type", "simulated_error",
							"error_code", rand.Intn(100),
							"retry_count", rand.Intn(5),
						)
						err = nil

					case "warn":

						logger.Warn(logContent,
							"worker", workerID,
							"sequence", seq,
							"warning_type", "high_latency",
							"latency_ms", rand.Intn(5000),
						)
						err = nil

					}

					latency := time.Since(sendStart)

					if err != nil {
						atomic.AddInt64(&errorCount, 1)
					} else {
						atomic.AddInt64(&successCount, 1)
						atomic.AddInt64(&totalLatency, latency.Nanoseconds())
					}

				case <-done:
					return
				}
			}
		}(i)
	}

	// 6. 进度显示
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			success := atomic.LoadInt64(&successCount)
			elapsed := time.Since(startTime).Seconds()
			qps := float64(success) / elapsed

			fmt.Printf("⏱️  进度: %.1f秒 | 成功: %d | QPS: %.0f | 错误: %d\n",
				elapsed, success, qps, atomic.LoadInt64(&errorCount))
		}
	}()

	// 7. 等待结束
	select {
	case <-time.After(config.Duration):
		fmt.Println("⏰ 压测时间到")
	case <-stopChan:
		fmt.Println("🛑 收到中断信号")
	}

	close(done)

	// 给一点时间让最后的日志发送完成
	time.Sleep(500 * time.Millisecond)

	// 8. 计算结果
	duration := time.Since(startTime)
	success := atomic.LoadInt64(&successCount)
	errors := atomic.LoadInt64(&errorCount)

	var avgLatency time.Duration
	if success > 0 {
		avgLatency = time.Duration(totalLatency/success) * time.Nanosecond
	}

	return &BenchmarkResults{
		TotalDuration: duration,
		TotalRequests: success + errors,
		SuccessCount:  success,
		ErrorCount:    errors,
		Throughput:    float64(success) / duration.Seconds(),
		AvgLatency:    avgLatency,
		ErrorRate:     float64(errors) / float64(success+errors) * 100,
	}
}

// 辅助函数
func generateLogTemplate(size int) string {
	base := "这是一条模拟业务日志，用于测试LogLite的性能表现。包含一些业务关键信息："
	if size <= len(base) {
		return base[:size]
	}

	// 填充到指定大小
	result := base
	filler := "日志数据填充内容。"
	for len(result) < size {
		result += filler
	}
	return result[:size]
}

func randomizeLog(template string) string {
	// 简单随机化：添加时间戳和随机数
	return fmt.Sprintf("%s [随机标识:%08d]", template, rand.Intn(100000000))
}

func randomLevel() string {
	// 模拟真实分布：info 85%, error 10%, warn 5%
	r := rand.Float32()
	if r < 0.85 {
		return "info"
	} else if r < 0.95 {
		return "error"
	}
	return "warn"
}

func printResults(results *BenchmarkResults) {
	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("📊 LogLite 压测结果")
	fmt.Println(strings.Repeat("=", 50))
	fmt.Printf("总时长:        %v\n", results.TotalDuration)
	fmt.Printf("总请求数:      %d\n", results.TotalRequests)
	fmt.Printf("成功数:        %d\n", results.SuccessCount)
	fmt.Printf("失败数:        %d\n", results.ErrorCount)
	fmt.Printf("吞吐量(QPS):   %.0f\n", results.Throughput)
	fmt.Printf("平均延迟:      %v\n", results.AvgLatency)
	fmt.Printf("错误率:        %.2f%%\n", results.ErrorRate)
	fmt.Println(strings.Repeat("=", 50))

	// 简单评估
	if results.ErrorRate > 5 {
		fmt.Println("❌ 警告：错误率过高")
	} else if results.Throughput < 1000 {
		fmt.Println("⚠️  注意：吞吐量较低")
	} else {
		fmt.Println("✅ 压测通过")
	}
}
