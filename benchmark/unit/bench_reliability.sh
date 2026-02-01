#!/bin/bash

# 专门测试可靠性模式的压测脚本

set -e

echo "🎯 LogLite 可靠性模式性能分析"
echo "================================"
echo ""

# 创建输出目录
mkdir -p results/reliability

# 1. 基准测试
echo "📊 Step 1: 运行基准测试..."
go test -bench=BenchmarkSDK_ReliabilityModes -benchmem -benchtime=10s | tee results/reliability/baseline.txt

echo ""
echo "✅ 基准测试完成！"
echo ""

# 2. 生成 CPU Profile（每种模式单独测试）
echo "🔥 Step 2: 生成 CPU Profile..."

echo "  - BestEffort 模式..."
go test -bench=BenchmarkSDK_ReliabilityModes/BestEffort \
    -cpuprofile=results/reliability/cpu_besteffort.prof \
    -benchtime=10s > /dev/null 2>&1

echo "  - Balanced 模式..."
go test -bench=BenchmarkSDK_ReliabilityModes/Balanced \
    -cpuprofile=results/reliability/cpu_balanced.prof \
    -benchtime=10s > /dev/null 2>&1

echo "  - Reliable 模式..."
go test -bench=BenchmarkSDK_ReliabilityModes/Reliable \
    -cpuprofile=results/reliability/cpu_reliable.prof \
    -benchtime=10s > /dev/null 2>&1

echo "✅ CPU Profile 生成完成！"
echo ""

# 3. 生成 Memory Profile
echo "💾 Step 3: 生成 Memory Profile..."

echo "  - BestEffort 模式..."
go test -bench=BenchmarkSDK_ReliabilityModes/BestEffort \
    -memprofile=results/reliability/mem_besteffort.prof \
    -benchtime=10s > /dev/null 2>&1

echo "  - Balanced 模式..."
go test -bench=BenchmarkSDK_ReliabilityModes/Balanced \
    -memprofile=results/reliability/mem_balanced.prof \
    -benchtime=10s > /dev/null 2>&1

echo "  - Reliable 模式..."
go test -bench=BenchmarkSDK_ReliabilityModes/Reliable \
    -memprofile=results/reliability/mem_reliable.prof \
    -benchtime=10s > /dev/null 2>&1

echo "✅ Memory Profile 生成完成！"
echo ""

# 4. 生成详细报告
echo "📈 Step 4: 生成分析报告..."

cat > results/reliability/ANALYSIS.md << 'EOF'
# 可靠性模式性能分析报告

## 📊 基准测试结果

查看 `baseline.txt` 文件获取详细数据。

关注指标：
- **ns/op**: 每次操作耗时（越低越好）
- **allocs/op**: 每次操作内存分配次数（越少越好）
- **B/op**: 每次操作分配的字节数（越少越好）

## 🔥 CPU Profile 分析

### BestEffort 模式
```bash
go tool pprof results/reliability/cpu_besteffort.prof
```

交互命令：
- `top10` - 查看 CPU 占用最高的 10 个函数
- `list <funcName>` - 查看具体函数的代码
- `web` - 生成调用图（需要 graphviz）

### Balanced 模式
```bash
go tool pprof results/reliability/cpu_balanced.prof
```

### Reliable 模式
```bash
go tool pprof results/reliability/cpu_reliable.prof
```

## 💾 Memory Profile 分析

### BestEffort 模式
```bash
go tool pprof results/reliability/mem_besteffort.prof
```

交互命令：
- `top10` - 查看内存分配最多的 10 个函数
- `list <funcName>` - 查看具体函数的代码
- `alloc_space` - 按总分配量排序
- `alloc_objects` - 按分配次数排序

### Balanced 模式
```bash
go tool pprof results/reliability/mem_balanced.prof
```

### Reliable 模式
```bash
go tool pprof results/reliability/mem_reliable.prof
```

## 🎯 优化建议

### 常见优化点

1. **减少内存分配**
   - 使用 `sync.Pool` 复用对象
   - 预分配切片容量
   - 避免不必要的字符串拼接

2. **优化字符串操作**
   - 使用 `strings.Builder` 代替 `+` 拼接
   - 使用 `strconv` 代替 `fmt.Sprintf`

3. **减少反射使用**
   - 类型断言代替反射
   - 预计算类型信息

4. **优化数据结构**
   - 使用更紧凑的数据结构
   - 减少指针间接访问

## 📝 分析步骤

1. 查看 `baseline.txt` 找出最慢的模式
2. 使用 pprof 分析该模式的 CPU 热点
3. 使用 pprof 分析该模式的内存热点
4. 针对性优化
5. 重新压测对比
EOF

echo "✅ 分析报告生成完成！"
echo ""

# 5. 生成对比图表（使用 pprof web）
echo "🌐 Step 5: 生成可视化分析..."
echo ""
echo "  你可以使用以下命令查看可视化分析："
echo ""
echo "  # CPU 火焰图（BestEffort）"
echo "  go tool pprof -http=:8080 results/reliability/cpu_besteffort.prof"
echo ""
echo "  # Memory 分析（Balanced）"
echo "  go tool pprof -http=:8081 results/reliability/mem_balanced.prof"
echo ""
echo "  # CPU 对比（Reliable）"
echo "  go tool pprof -http=:8082 results/reliability/cpu_reliable.prof"
echo ""

# 6. 快速分析（自动输出 top10）
echo "📋 Step 6: 快速分析 Top 10 热点..."
echo ""

echo "=== BestEffort CPU Top 10 ==="
go tool pprof -top -cum results/reliability/cpu_besteffort.prof 2>/dev/null | head -20
echo ""

echo "=== Balanced CPU Top 10 ==="
go tool pprof -top -cum results/reliability/cpu_balanced.prof 2>/dev/null | head -20
echo ""

echo "=== Reliable CPU Top 10 ==="
go tool pprof -top -cum results/reliability/cpu_reliable.prof 2>/dev/null | head -20
echo ""

echo "🎉 所有分析完成！"
echo ""
echo "📁 生成的文件："
echo "   - results/reliability/baseline.txt          # 基准测试结果"
echo "   - results/reliability/cpu_*.prof            # CPU Profile"
echo "   - results/reliability/mem_*.prof            # Memory Profile"
echo "   - results/reliability/ANALYSIS.md           # 分析指南"
echo ""
echo "🔍 下一步："
echo "   1. 查看 baseline.txt 对比三种模式的性能"
echo "   2. 找出最慢的模式"
echo "   3. 使用 pprof 分析该模式的热点"
echo "   4. 针对性优化代码"
echo ""
