#!/bin/bash

# 并发性能分析脚本
# 用于分析并发测试的 pprof 文件

set -e

RESULTS_DIR="results/concurrent"

if [ ! -d "$RESULTS_DIR" ]; then
    echo "❌ 结果目录不存在，请先运行 bench_concurrent.sh"
    exit 1
fi

echo "🔍 并发性能分析"
echo ""

# 分析 CPU 性能
if [ -f "$RESULTS_DIR/cpu_concurrent.prof" ]; then
    echo "📊 CPU 性能分析 (并发测试):"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    go tool pprof -top -cum "$RESULTS_DIR/cpu_concurrent.prof" 2>/dev/null | head -30
    echo ""
fi

# 分析内存性能
if [ -f "$RESULTS_DIR/mem_concurrent.prof" ]; then
    echo "📊 内存性能分析 (并发测试):"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    go tool pprof -top -cum "$RESULTS_DIR/mem_concurrent.prof" 2>/dev/null | head -30
    echo ""
fi

# 分析 CPU 压力测试
if [ -f "$RESULTS_DIR/cpu_stress.prof" ]; then
    echo "📊 CPU 压力测试分析:"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    go tool pprof -top -cum "$RESULTS_DIR/cpu_stress.prof" 2>/dev/null | head -30
    echo ""
fi

echo "💡 使用以下命令进行交互式分析："
echo "  go tool pprof $RESULTS_DIR/cpu_concurrent.prof"
echo "  go tool pprof $RESULTS_DIR/mem_concurrent.prof"
echo "  go tool pprof $RESULTS_DIR/cpu_stress.prof"
echo ""
echo "💡 在 pprof 交互界面中可以使用："
echo "  top          - 查看最耗时的函数"
echo "  top10        - 查看前10个最耗时的函数"
echo "  list <func>  - 查看函数的具体代码"
echo "  web          - 生成调用图（需要 graphviz）"
