#!/bin/bash

# 并发性能测试脚本
# 用于测试 Balanced 模式在高并发场景下的性能

set -e

# 确保在正确的目录下运行
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

RESULTS_DIR="results/concurrent"
mkdir -p "$RESULTS_DIR"

echo "🚀 开始并发性能测试..."
echo ""

# 1. 测试不同并发度下的性能
echo "📊 测试 1: 不同并发度下的性能 (Balanced)"
go test -bench='BenchmarkBalanced_Concurrent' \
    -benchmem \
    -benchtime=3s \
    -cpuprofile="$RESULTS_DIR/cpu_concurrent.prof" \
    -memprofile="$RESULTS_DIR/mem_concurrent.prof" \
    . \
    | tee "$RESULTS_DIR/concurrent_results.txt"

echo ""
echo "✅ 并发度测试完成！结果保存在: $RESULTS_DIR/concurrent_results.txt"
echo ""

# 2. 对比不同模式的并发性能
echo "📊 测试 2: 不同可靠性模式的并发性能对比"
go test -bench='BenchmarkBalanced_ConcurrentModes' \
    -benchmem \
    -benchtime=3s \
    . \
    | tee "$RESULTS_DIR/modes_comparison.txt"

echo ""
echo "✅ 模式对比测试完成！结果保存在: $RESULTS_DIR/modes_comparison.txt"
echo ""

# 3. 测试吞吐量
echo "📊 测试 3: 不同并发度下的吞吐量"
go test -bench='BenchmarkBalanced_ConcurrentThroughput' \
    -benchmem \
    -benchtime=5s \
    . \
    | tee "$RESULTS_DIR/throughput_results.txt"

echo ""
echo "✅ 吞吐量测试完成！结果保存在: $RESULTS_DIR/throughput_results.txt"
echo ""

# 4. 测试延迟分布
echo "📊 测试 4: 并发写入延迟分布"
go test -bench='BenchmarkBalanced_ConcurrentLatency' \
    -benchmem \
    -benchtime=2s \
    . \
    | tee "$RESULTS_DIR/latency_results.txt"

echo ""
echo "✅ 延迟测试完成！结果保存在: $RESULTS_DIR/latency_results.txt"
echo ""

# 5. CPU 压力测试
echo "📊 测试 5: CPU 压力测试"
go test -bench='BenchmarkBalanced_ConcurrentCPU' \
    -benchmem \
    -benchtime=3s \
    -cpuprofile="$RESULTS_DIR/cpu_stress.prof" \
    . \
    | tee "$RESULTS_DIR/cpu_stress_results.txt"

echo ""
echo "✅ CPU 压力测试完成！结果保存在: $RESULTS_DIR/cpu_stress_results.txt"
echo ""

echo "📈 所有测试完成！"
echo ""
echo "📁 结果文件："
echo "  - $RESULTS_DIR/concurrent_results.txt      (并发度测试)"
echo "  - $RESULTS_DIR/modes_comparison.txt        (模式对比)"
echo "  - $RESULTS_DIR/throughput_results.txt      (吞吐量测试)"
echo "  - $RESULTS_DIR/latency_results.txt         (延迟测试)"
echo "  - $RESULTS_DIR/cpu_stress_results.txt      (CPU 压力测试)"
echo ""
echo "🔍 性能分析文件："
echo "  - $RESULTS_DIR/cpu_concurrent.prof         (CPU 性能分析)"
echo "  - $RESULTS_DIR/mem_concurrent.prof         (内存性能分析)"
echo "  - $RESULTS_DIR/cpu_stress.prof             (CPU 压力分析)"
echo ""
echo "💡 使用以下命令查看性能分析："
echo "  go tool pprof $RESULTS_DIR/cpu_concurrent.prof"
echo "  go tool pprof $RESULTS_DIR/mem_concurrent.prof"
