// benchmarks/unit/query_bench_test.go
package unit

import (
	"testing"

	"github.com/loglite/loglite/internal/query"
)

// BenchmarkNaturalQuery_Simple 测试简单自然语言查询
func BenchmarkNaturalQuery_Simple(b *testing.B) {
	q := query.NewNaturalQueryParser()

	queries := []string{
		"今天的错误日志",
		"用户登录失败的日志",
		"支付服务的警告",
		"数据库连接超时",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		queryStr := queries[i%len(queries)]
		q.Parse(queryStr)
	}
}

// BenchmarkNaturalQuery_Complex 测试复杂自然语言查询
func BenchmarkNaturalQuery_Complex(b *testing.B) {
	q := query.NewNaturalQueryParser()

	queries := []string{
		"今天下午3点到5点之间用户服务所有的错误日志",
		"昨天订单量大于100的用户支付成功记录",
		"上周数据库慢查询中响应时间超过1秒的记录",
		"最近1小时API网关返回状态码500的请求",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		queryStr := queries[i%len(queries)]
		q.Parse(queryStr)
	}
}

// BenchmarkQuery_Parse 测试查询解析性能
func BenchmarkQuery_Parse(b *testing.B) {
	q := query.NewNaturalQueryParser()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		queryStr := "今天的错误日志 from user-service level error"
		q.Parse(queryStr)
	}
}

// BenchmarkQuery_TimeExtraction 测试时间提取性能
func BenchmarkQuery_TimeExtraction(b *testing.B) {
	q := query.NewNaturalQueryParser()

	queries := []string{
		"今天的日志",
		"昨天下午3点的日志",
		"最近1小时的日志",
		"上周一到周五的日志",
		"2024-01-01到2024-01-31的日志",
	}

	for _, queryStr := range queries {
		b.Run(queryStr, func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				q.Parse(queryStr)
			}
		})
	}
}

// BenchmarkQuery_ServiceExtraction 测试服务名提取性能
func BenchmarkQuery_ServiceExtraction(b *testing.B) {
	q := query.NewNaturalQueryParser()

	queries := []string{
		"user-service的日志",
		"来自payment-service的错误",
		"order-service和cart-service的警告",
		"api-gateway的所有日志",
	}

	for _, queryStr := range queries {
		b.Run(queryStr, func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				q.Parse(queryStr)
			}
		})
	}
}

// BenchmarkQuery_LevelExtraction 测试日志级别提取性能
func BenchmarkQuery_LevelExtraction(b *testing.B) {
	q := query.NewNaturalQueryParser()

	queries := []string{
		"错误日志",
		"警告和错误",
		"info级别的日志",
		"debug日志",
	}

	for _, queryStr := range queries {
		b.Run(queryStr, func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				q.Parse(queryStr)
			}
		})
	}
}

// BenchmarkQuery_KeywordExtraction 测试关键词提取性能
func BenchmarkQuery_KeywordExtraction(b *testing.B) {
	q := query.NewNaturalQueryParser()

	queries := []string{
		"包含'数据库'的日志",
		"用户登录失败",
		"支付超时的错误",
		"API调用慢查询",
	}

	for _, queryStr := range queries {
		b.Run(queryStr, func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				q.Parse(queryStr)
			}
		})
	}
}

// BenchmarkQuery_FullParse 测试完整解析流程
func BenchmarkQuery_FullParse(b *testing.B) {
	q := query.NewNaturalQueryParser()

	// 模拟真实的复杂查询
	complexQuery := "今天下午2点到4点之间user-service和payment-service的错误日志，包含'超时'关键词"

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		params := q.Parse(complexQuery)
		// 防止编译器优化
		_ = params
	}
}

// BenchmarkQuery_ParserCreation 测试 Parser 创建开销
func BenchmarkQuery_ParserCreation(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		q := query.NewNaturalQueryParser()
		_ = q
	}
}

// BenchmarkQuery_CachedParser 测试复用 Parser 的性能
func BenchmarkQuery_CachedParser(b *testing.B) {
	// 预创建 Parser
	q := query.NewNaturalQueryParser()

	queries := []string{
		"今天的错误日志",
		"昨天的警告",
		"user-service的info日志",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		queryStr := queries[i%len(queries)]
		params := q.Parse(queryStr)
		_ = params
	}
}
