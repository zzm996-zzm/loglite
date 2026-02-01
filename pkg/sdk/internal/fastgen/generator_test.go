package fastgen

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

// ============================================================
// 功能测试
// ============================================================

func TestNewID(t *testing.T) {
	g := NewGenerator()
	defer g.Close()
	
	// 生成 1000 个 ID，检查唯一性
	ids := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		id := g.NewID()
		if len(id) != 32 {
			t.Errorf("ID 长度错误: got %d, want 32", len(id))
		}
		if ids[id] {
			t.Errorf("重复 ID: %s", id)
		}
		ids[id] = true
	}
}

func TestNewShortID(t *testing.T) {
	g := NewGenerator()
	defer g.Close()
	
	// 生成 1000 个短 ID，检查唯一性
	ids := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		id := g.NewShortID()
		if len(id) != 24 {
			t.Errorf("短 ID 长度错误: got %d, want 24", len(id))
		}
		if ids[id] {
			t.Errorf("重复 ID: %s", id)
		}
		ids[id] = true
	}
}

func TestCachedTime(t *testing.T) {
	g := NewGenerator()
	defer g.Close()
	
	// 等待一次时间更新
	time.Sleep(5 * time.Millisecond)
	
	t1 := g.CachedTime()
	time.Sleep(2 * time.Millisecond)
	t2 := g.CachedTime()
	
	// 缓存时间应该更新了（误差在 3ms 内）
	diff := t2.Sub(t1)
	if diff < 0 || diff > 5*time.Millisecond {
		t.Errorf("时间缓存异常: diff=%v", diff)
	}
}

// ============================================================
// 性能测试
// ============================================================

// BenchmarkNewID 测试 FastGenerator.NewID() 性能
func BenchmarkNewID(b *testing.B) {
	g := NewGenerator()
	defer g.Close()
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_ = g.NewID()
	}
	
	b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "ids/sec")
}

// BenchmarkNewShortID 测试 FastGenerator.NewShortID() 性能
func BenchmarkNewShortID(b *testing.B) {
	g := NewGenerator()
	defer g.Close()
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_ = g.NewShortID()
	}
	
	b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "ids/sec")
}

// BenchmarkCachedTime 测试 FastGenerator.CachedTime() 性能
func BenchmarkCachedTime(b *testing.B) {
	g := NewGenerator()
	defer g.Close()
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_ = g.CachedTime()
	}
	
	b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "ops/sec")
}

// ============================================================
// 对比测试（vs 标准库）
// ============================================================

// BenchmarkUUID_New 测试 uuid.New() 性能（对比基准）
func BenchmarkUUID_New(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_ = uuid.New()
	}
	
	b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "ids/sec")
}

// BenchmarkUUID_NewString 测试 uuid.New().String() 性能（对比基准）
func BenchmarkUUID_NewString(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_ = uuid.New().String()
	}
	
	b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "ids/sec")
}

// BenchmarkTimeNow 测试 time.Now() 性能（对比基准）
func BenchmarkTimeNow(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_ = time.Now()
	}
	
	b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "ops/sec")
}

// ============================================================
// 并发测试
// ============================================================

// BenchmarkNewID_Parallel 并发测试 ID 生成
func BenchmarkNewID_Parallel(b *testing.B) {
	g := NewGenerator()
	defer g.Close()
	
	b.ResetTimer()
	b.ReportAllocs()
	
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = g.NewID()
		}
	})
	
	b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "ids/sec")
}

// BenchmarkUUID_Parallel 并发测试 UUID 生成（对比）
func BenchmarkUUID_Parallel(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()
	
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = uuid.New().String()
		}
	})
	
	b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "ids/sec")
}

// BenchmarkCachedTime_Parallel 并发测试时间缓存
func BenchmarkCachedTime_Parallel(b *testing.B) {
	g := NewGenerator()
	defer g.Close()
	
	b.ResetTimer()
	b.ReportAllocs()
	
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = g.CachedTime()
		}
	})
	
	b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "ops/sec")
}

// BenchmarkTimeNow_Parallel 并发测试 time.Now()（对比）
func BenchmarkTimeNow_Parallel(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()
	
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = time.Now()
		}
	})
	
	b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "ops/sec")
}
