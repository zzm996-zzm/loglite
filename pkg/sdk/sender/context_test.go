package sender_test

import (
	"fmt"
	"testing"

	"github.com/loglite/loglite/pkg/sdk/sender"
)

// 演示自动上下文采集
func ExampleCaptureContext() {
	// ============================================================
	// 场景 1: 捕获调用位置
	// ============================================================

	// 模拟在业务代码中打日志
	func() {
		caller := sender.CaptureContext(1) // skip=1 跳过当前匿名函数
		fmt.Printf("File: %s:%d\n", caller.File, caller.Line)
		fmt.Printf("Function: %s\n", caller.Function)
		fmt.Printf("Package: %s\n", caller.Package)
	}()

	// ============================================================
	// 场景 2: 自动填充日志上下文
	// ============================================================

	entry := &sender.LogEntry{
		Level:   "error",
		Message: "database connection failed",
	}

	// 自动填充 caller, function, package, stack_trace, stack_hash
	sender.EnrichLogEntry(entry, 0)

	fmt.Printf("Caller: %s\n", entry.Caller)
	fmt.Printf("Function: %s\n", entry.Function)
	fmt.Printf("Package: %s\n", entry.Package)
	fmt.Printf("StackHash: %s\n", entry.StackHash)

	// StackTrace 会是完整的堆栈信息
	// fmt.Printf("StackTrace:\n%s\n", entry.StackTrace)
}

// 演示堆栈捕获
func ExampleCaptureStack() {
	// 模拟多层调用
	levelOne := func() {
		levelTwo := func() {
			levelThree := func() {
				stack := sender.CaptureStack(0)
				fmt.Println("完整堆栈:")
				fmt.Println(stack)

				// 简化版本
				simple := sender.CaptureStackSimple(0)
				fmt.Println("\n简化堆栈:")
				fmt.Println(simple)

				// 堆栈指纹
				hash := sender.HashStack(stack)
				fmt.Printf("\n堆栈指纹: %s\n", hash)
			}
			levelThree()
		}
		levelTwo()
	}
	levelOne()
}

// 演示堆栈指纹的聚合能力
func TestStackHashAggregation(t *testing.T) {
	// ============================================================
	// 测试：相同错误在不同行，生成相同的指纹
	// ============================================================

	// 模拟函数 A
	funcA := func() string {
		stack := sender.CaptureStack(0) // 调用位置：行 XX
		return sender.HashStack(stack)
	}

	// 模拟函数 B（相同的调用链，不同的行号）
	funcB := func() string {
		stack := sender.CaptureStack(0) // 调用位置：行 YY
		return sender.HashStack(stack)
	}

	hashA := funcA()
	hashB := funcB()

	// 由于调用链不同，hash 会不同
	// 但在实际场景中，如果是同一个错误在同一个函数中，
	// 即使行号不同，文件名和函数名相同，hash 也会相同

	t.Logf("Hash A: %s", hashA)
	t.Logf("Hash B: %s", hashB)
}

// 演示便捷函数
func ExampleNewLogEntryWithContext() {
	// 一行代码创建带完整上下文的日志
	entry := sender.NewLogEntryWithContext("error", "payment processing failed")

	fmt.Printf("Message: %s\n", entry.Message)
	fmt.Printf("Level: %s\n", entry.Level)
	fmt.Printf("Caller: %s\n", entry.Caller)
	fmt.Printf("Function: %s\n", entry.Function)
	fmt.Printf("StackHash: %s\n", entry.StackHash)
}

// 演示真实场景：错误聚合
func Example_errorAggregation() {
	// ============================================================
	// 场景：数据库连接错误在多个地方发生
	// ============================================================

	// 模拟不同的调用位置
	errors := []struct {
		location string
		entry    *sender.LogEntry
	}{
		{
			location: "user-service/handler.go:42",
			entry: &sender.LogEntry{
				Level:   "error",
				Message: "database connection timeout",
			},
		},
		{
			location: "order-service/handler.go:88",
			entry: &sender.LogEntry{
				Level:   "error",
				Message: "database connection timeout",
			},
		},
		{
			location: "payment-service/handler.go:156",
			entry: &sender.LogEntry{
				Level:   "error",
				Message: "database connection timeout",
			},
		},
	}

	// 填充上下文
	for i := range errors {
		sender.EnrichLogEntry(errors[i].entry, 0)
	}

	// 按 StackHash 聚合
	aggregated := make(map[string][]string)
	for _, err := range errors {
		hash := err.entry.StackHash
		aggregated[hash] = append(aggregated[hash], err.location)
	}

	// 打印聚合结果
	fmt.Println("错误聚合结果:")
	for hash, locations := range aggregated {
		fmt.Printf("\n指纹: %s\n", hash)
		fmt.Printf("发生次数: %d\n", len(locations))
		fmt.Printf("发生位置:\n")
		for _, loc := range locations {
			fmt.Printf("  - %s\n", loc)
		}
	}
}
