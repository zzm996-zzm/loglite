#!/bin/bash

# LogLite 压测运行脚本

set -e

echo "🚀 LogLite 性能压测"
echo "===================="
echo ""

# 创建输出目录
mkdir -p results

# 1. 运行所有压测
echo "📊 运行所有压测..."
go test -bench=. -benchmem -benchtime=5s | tee results/all.txt

echo ""
echo "✅ 压测完成！结果保存在 results/all.txt"
echo ""

# 2. 生成 CPU Profile
echo "🔥 生成 CPU Profile..."
go test -bench=BenchmarkSDK_Info -cpuprofile=results/cpu.prof -benchtime=10s

echo "✅ CPU Profile 保存在 results/cpu.prof"
echo "   查看: go tool pprof results/cpu.prof"
echo ""

# 3. 生成 Memory Profile
echo "💾 生成 Memory Profile..."
go test -bench=BenchmarkSDK_Info -memprofile=results/mem.prof -benchtime=10s

echo "✅ Memory Profile 保存在 results/mem.prof"
echo "   查看: go tool pprof results/mem.prof"
echo ""

# 4. 对比不同可靠性模式
echo "⚖️  对比不同可靠性模式..."
go test -bench=BenchmarkSDK_ReliabilityModes -benchmem | tee results/reliability.txt

echo ""
echo "✅ 可靠性模式对比保存在 results/reliability.txt"
echo ""

# 5. 对比不同批量大小
echo "📦 对比不同批量大小..."
go test -bench=BenchmarkStorage_BatchWrite -benchmem | tee results/batch.txt

echo ""
echo "✅ 批量大小对比保存在 results/batch.txt"
echo ""

# 6. 并发性能测试
echo "🔀 并发性能测试..."
go test -bench=BenchmarkSDK_Parallel -benchmem -cpu=1,2,4,8 | tee results/parallel.txt

echo ""
echo "✅ 并发测试结果保存在 results/parallel.txt"
echo ""

echo "🎉 所有压测完成！"
echo ""
echo "📁 结果文件："
echo "   - results/all.txt         # 所有压测结果"
echo "   - results/cpu.prof        # CPU Profile"
echo "   - results/mem.prof        # Memory Profile"
echo "   - results/reliability.txt # 可靠性模式对比"
echo "   - results/batch.txt       # 批量大小对比"
echo "   - results/parallel.txt    # 并发测试"
echo ""
echo "📊 分析建议："
echo "   1. 查看 all.txt 找出最慢的操作"
echo "   2. 使用 pprof 分析 CPU 和内存热点"
echo "   3. 对比不同配置的性能差异"
echo ""
