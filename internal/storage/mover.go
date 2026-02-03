package storage

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// MoverConfig 迁移器配置
type MoverConfig struct {
	HotThreshold  time.Duration // Hot → Warm 时间阈值
	WarmThreshold time.Duration // Warm → Cold 时间阈值
	Interval      time.Duration // 迁移检查间隔
	BatchSize     int           // 每批迁移数量
}

// MoverStats 迁移统计
type MoverStats struct {
	HotToWarm  int64     // Hot → Warm 迁移数量
	WarmToCold int64     // Warm → Cold 迁移数量
	LastRun    time.Time // 最后运行时间
	Errors     int64     // 错误次数
}

// Mover 数据迁移器
// 负责将过期数据从高层迁移到低层
type Mover struct {
	tiered *TieredStore
	cfg    MoverConfig

	// 状态
	running atomic.Bool
	stats   MoverStats
	mu      sync.RWMutex

	// 控制
	stopper chan struct{}
	done    chan struct{}
}

// NewMover 创建迁移器
func NewMover(tiered *TieredStore, cfg MoverConfig) *Mover {
	// 设置默认值
	if cfg.Interval == 0 {
		cfg.Interval = 5 * time.Minute
	}
	if cfg.BatchSize == 0 {
		cfg.BatchSize = 1000
	}
	if cfg.HotThreshold == 0 {
		cfg.HotThreshold = DefaultHotThreshold
	}
	if cfg.WarmThreshold == 0 {
		cfg.WarmThreshold = DefaultWarmThreshold
	}

	return &Mover{
		tiered:  tiered,
		cfg:     cfg,
		stopper: make(chan struct{}),
		done:    make(chan struct{}),
	}
}

// Start 启动迁移器
func (m *Mover) Start() {
	if m.running.Swap(true) {
		return // 已经在运行
	}

	go m.run()
}

// Stop 停止迁移器
func (m *Mover) Stop() {
	if !m.running.Swap(false) {
		return // 已经停止
	}

	close(m.stopper)

	// 等待完成，最多等待 30 秒
	select {
	case <-m.done:
	case <-time.After(30 * time.Second):
		log.Println("[Mover] Stop timeout")
	}
}

// Stats 获取统计信息
func (m *Mover) Stats() MoverStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats
}

// run 运行迁移循环
func (m *Mover) run() {
	defer close(m.done)

	ticker := time.NewTicker(m.cfg.Interval)
	defer ticker.Stop()

	// 启动时先执行一次
	m.migrate()

	for {
		select {
		case <-m.stopper:
			return
		case <-ticker.C:
			m.migrate()
		}
	}
}

// migrate 执行一次迁移
func (m *Mover) migrate() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	m.mu.Lock()
	m.stats.LastRun = time.Now()
	m.mu.Unlock()

	// 迁移 Hot → Warm
	hotMoved, err := m.migrateHotToWarm(ctx)
	if err != nil {
		log.Printf("[Mover] Hot→Warm error: %v", err)
		atomic.AddInt64(&m.stats.Errors, 1)
	} else if hotMoved > 0 {
		atomic.AddInt64(&m.stats.HotToWarm, int64(hotMoved))
		log.Printf("[Mover] Hot→Warm: moved %d entries", hotMoved)
	}

	// 迁移 Warm → Cold
	warmMoved, err := m.migrateWarmToCold(ctx)
	if err != nil {
		log.Printf("[Mover] Warm→Cold error: %v", err)
		atomic.AddInt64(&m.stats.Errors, 1)
	} else if warmMoved > 0 {
		atomic.AddInt64(&m.stats.WarmToCold, int64(warmMoved))
		log.Printf("[Mover] Warm→Cold: moved %d entries", warmMoved)
	}
}

// migrateHotToWarm 迁移 Hot 层过期数据到 Warm 层
func (m *Mover) migrateHotToWarm(ctx context.Context) (int, error) {
	threshold := time.Now().Add(-m.cfg.HotThreshold)

	// 查询 Hot 层中过期的数据
	q := &Query{
		To:     threshold,
		Limit:  m.cfg.BatchSize,
		SortBy: "time_asc", // 先迁移最旧的
	}

	result, err := m.tiered.hot.Query(ctx, q)
	if err != nil {
		return 0, err
	}

	if len(result.Entries) == 0 {
		return 0, nil
	}

	// 写入 Warm 层
	if err := m.tiered.warm.WriteMany(ctx, result.Entries); err != nil {
		return 0, err
	}

	//从Hot层删除
	ids := make([]string, 0, len(result.Entries))
	for _, entry := range result.Entries {
		ids = append(ids, entry.ID)
	}

	// 根据id批量删除
	err = m.tiered.hot.DeleteMany(ctx, ids)
	if err != nil {
		log.Printf("[Error] Hot→Warm: clean %d entries . error:%v", len(ids), err)
	}

	return len(result.Entries), nil
}

// migrateWarmToCold 迁移 Warm 层过期数据到 Cold 层
func (m *Mover) migrateWarmToCold(ctx context.Context) (int, error) {
	threshold := time.Now().Add(-m.cfg.WarmThreshold)

	// 查询 Warm 层中过期的数据
	q := &Query{
		To:     threshold,
		Limit:  m.cfg.BatchSize,
		SortBy: "time_asc",
	}

	result, err := m.tiered.warm.Query(ctx, q)
	if err != nil {
		return 0, err
	}

	if len(result.Entries) == 0 {
		return 0, nil
	}

	// 写入 Cold 层
	if err := m.tiered.cold.WriteMany(ctx, result.Entries); err != nil {
		return 0, err
	}

	ids := make([]string, 0, len(result.Entries))
	for _, entry := range result.Entries {
		ids = append(ids, entry.ID)
	}

	// 根据id批量删除
	err = m.tiered.warm.DeleteMany(ctx, ids)
	if err != nil {
		log.Printf("[Error] Warm→Cold: clean %d entries . error:%v", len(ids), err)
	}

	return len(result.Entries), nil
}

// ForceRun 强制运行一次迁移（用于测试或手动触发）
func (m *Mover) ForceRun() {
	m.migrate()
}
