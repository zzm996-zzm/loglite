package storage

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/loglite/loglite/internal/model"
)

// 分层时间阈值（可通过配置覆盖）
const (
	DefaultHotThreshold  = 2 * time.Hour  // Hot层：最近2小时
	DefaultWarmThreshold = 24 * time.Hour // Warm层：1-8小时
	// Cold层：24小时以上，直到TTL过期
)

// 配置键常量（避免字符串硬编码，提高类型安全性）
const (
	ConfigKeyPath = "path" // 存储路径

	ConfigKeyHotThreshold  = "hot_threshold"  // Hot → Warm 时间阈值
	ConfigKeyWarmThreshold = "warm_threshold" // Warm → Cold 时间阈值

	ConfigKeyHotTTL  = "hot_ttl"  // Hot 层 TTL
	ConfigKeyWarmTTL = "warm_ttl" // Warm 层 TTL
	ConfigKeyColdTTL = "cold_ttl" // Cold 层 TTL

	ConfigKeyMoverInterval  = "mover_interval"   // 迁移检查间隔
	ConfigKeyMoverBatchSize = "mover_batch_size" // 每批迁移数量
)

// TieredStore 分级存储
// 设计思想：
// - Hot：最近数据，高频访问，快速读写
// - Warm：中期数据，偶尔访问
// - Cold：历史数据，低频访问，可压缩
type TieredStore struct {
	hot  Store // 热数据层
	warm Store // 温数据层
	cold Store // 冷数据层

	// 配置
	cfg           Config
	hotThreshold  time.Duration // Hot→Warm 时间阈值
	warmThreshold time.Duration // Warm→Cold 时间阈值

	// 后台任务
	mover   *Mover
	stopper chan struct{}

	// 状态
	mu      sync.RWMutex
	stats   Stats
	startAt time.Time
	closed  bool
}

// NewTieredStore 创建分级存储
func NewTieredStore(cfg Config) (Store, error) {
	basePath := cfg.GetString(ConfigKeyPath, "./data/tiered")

	// 获取时间阈值配置
	hotThreshold := cfg.GetDuration(ConfigKeyHotThreshold, DefaultHotThreshold)
	warmThreshold := cfg.GetDuration(ConfigKeyWarmThreshold, DefaultWarmThreshold)

	// 获取各层 TTL
	hotTTL := cfg.GetDuration(ConfigKeyHotTTL, hotThreshold*2)    // 默认: Hot层保留4小时
	warmTTL := cfg.GetDuration(ConfigKeyWarmTTL, warmThreshold*2) // 默认: Warm层保留48小时
	coldTTL := cfg.GetDuration(ConfigKeyColdTTL, 7*24*time.Hour)  // 默认: Cold层保留7天

	// 创建三个 Badger 实例，各自独立的目录
	hotCfg := Config{
		ConfigKeyPath: filepath.Join(basePath, "hot"),
		"ttl":         hotTTL,
	}
	warmCfg := Config{
		ConfigKeyPath: filepath.Join(basePath, "warm"),
		"ttl":         warmTTL,
	}
	coldCfg := Config{
		ConfigKeyPath: filepath.Join(basePath, "cold"),
		"ttl":         coldTTL,
	}

	// 创建各层存储
	hot, err := NewBadgerStore(hotCfg)
	if err != nil {
		return nil, fmt.Errorf("create hot tier: %w", err)
	}

	warm, err := NewBadgerStore(warmCfg)
	if err != nil {
		hot.Close()
		return nil, fmt.Errorf("create warm tier: %w", err)
	}

	cold, err := NewBadgerStore(coldCfg)
	if err != nil {
		hot.Close()
		warm.Close()
		return nil, fmt.Errorf("create cold tier: %w", err)
	}

	t := &TieredStore{
		hot:           hot,
		warm:          warm,
		cold:          cold,
		cfg:           cfg,
		hotThreshold:  hotThreshold,
		warmThreshold: warmThreshold,
		stopper:       make(chan struct{}),
		startAt:       time.Now(),
	}

	// 创建并启动数据迁移器
	t.mover = NewMover(t, MoverConfig{
		HotThreshold:  hotThreshold,
		WarmThreshold: warmThreshold,
		Interval:      cfg.GetDuration(ConfigKeyMoverInterval, 5*time.Minute),
		BatchSize:     cfg.GetInt(ConfigKeyMoverBatchSize, 1000),
	})
	t.mover.Start()

	return t, nil
}

// Write 写入日志（总是写入 Hot 层）
func (t *TieredStore) Write(ctx context.Context, entry *model.LogEntry) error {
	t.mu.RLock()
	if t.closed {
		t.mu.RUnlock()
		return fmt.Errorf("store is closed")
	}
	t.mu.RUnlock()

	// 确保时间戳
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	return t.hot.Write(ctx, entry)
}

// WriteMany 批量写入（总是写入 Hot 层）
func (t *TieredStore) WriteMany(ctx context.Context, entries []*model.LogEntry) error {
	t.mu.RLock()
	if t.closed {
		t.mu.RUnlock()
		return fmt.Errorf("store is closed")
	}
	t.mu.RUnlock()

	// 确保所有条目都有时间戳
	now := time.Now()
	for _, entry := range entries {
		if entry.Timestamp.IsZero() {
			entry.Timestamp = now
		}
	}

	return t.hot.WriteMany(ctx, entries)
}

// Query 查询日志（智能路由到对应层）
func (t *TieredStore) Query(ctx context.Context, q *Query) (*Result, error) {
	start := time.Now()

	t.mu.RLock()
	if t.closed {
		t.mu.RUnlock()
		return nil, fmt.Errorf("store is closed")
	}
	t.mu.RUnlock()

	// 确定需要查询哪些层
	tiers := t.routeQuery(q)

	// 并行查询各层
	results := t.parallelQuery(ctx, q, tiers)

	// 合并结果
	merged := t.mergeResults(results, q)
	merged.Took = time.Since(start)

	return merged, nil
}

// routeQuery 根据查询条件决定需要查询哪些层
func (t *TieredStore) routeQuery(q *Query) []Store {
	now := time.Now()
	var tiers []Store

	// 计算时间边界
	hotBoundary := now.Add(-t.hotThreshold)   // 2小时前
	warmBoundary := now.Add(-t.warmThreshold) // 24小时前

	// 如果没有指定时间范围，查询所有层
	if q.From.IsZero() && q.To.IsZero() {
		return []Store{t.hot, t.warm, t.cold}
	}

	// 根据 From/To 决定查询哪些层
	queryFrom := q.From
	queryTo := q.To
	if queryTo.IsZero() {
		queryTo = now
	}

	// Hot 层：包含 hotBoundary 之后的数据
	if queryTo.After(hotBoundary) {
		tiers = append(tiers, t.hot)
	}

	// Warm 层：包含 warmBoundary ~ hotBoundary 之间的数据
	if queryFrom.Before(hotBoundary) && queryTo.After(warmBoundary) {
		tiers = append(tiers, t.warm)
	}

	// Cold 层：包含 warmBoundary 之前的数据
	if queryFrom.Before(warmBoundary) {
		tiers = append(tiers, t.cold)
	}

	// 安全检查：至少查询一层
	if len(tiers) == 0 {
		tiers = append(tiers, t.hot)
	}

	return tiers
}

// parallelQuery 并行查询多个层
func (t *TieredStore) parallelQuery(ctx context.Context, q *Query, tiers []Store) []*Result {
	if len(tiers) == 1 {
		// 只有一层，直接查询
		result, err := tiers[0].Query(ctx, q)
		if err != nil {
			return nil
		}
		return []*Result{result}
	}

	// 多层并行查询
	var wg sync.WaitGroup
	results := make([]*Result, len(tiers))

	for i, tier := range tiers {
		wg.Add(1)
		go func(idx int, store Store) {
			defer wg.Done()

			result, err := store.Query(ctx, q)
			if err != nil {
				return
			}
			results[idx] = result
		}(i, tier)
	}

	wg.Wait()
	return results
}

// mergeResults 合并多层查询结果
func (t *TieredStore) mergeResults(results []*Result, q *Query) *Result {
	merged := &Result{
		Entries: make([]*model.LogEntry, 0),
	}

	// 收集所有条目
	for _, r := range results {
		if r != nil {
			merged.Entries = append(merged.Entries, r.Entries...)
			merged.Total += r.Total
		}
	}

	// 按时间排序
	t.sortEntries(merged.Entries, q.SortBy)

	// 应用分页
	if q.Offset > 0 && q.Offset < len(merged.Entries) {
		merged.Entries = merged.Entries[q.Offset:]
	}

	if q.Limit > 0 && q.Limit < len(merged.Entries) {
		merged.Entries = merged.Entries[:q.Limit]
		merged.HasMore = true
	}

	return merged
}

// sortEntries 排序日志条目
func (t *TieredStore) sortEntries(entries []*model.LogEntry, sortBy string) {
	if len(entries) <= 1 {
		return
	}

	// 使用快排
	switch sortBy {
	case "time_asc":
		quickSort(entries, 0, len(entries)-1, func(a, b *model.LogEntry) bool {
			return a.Timestamp.Before(b.Timestamp)
		})
	default: // time_desc 或默认
		quickSort(entries, 0, len(entries)-1, func(a, b *model.LogEntry) bool {
			return a.Timestamp.After(b.Timestamp)
		})
	}
}

// quickSort 快速排序（避免引入 sort 包的额外开销）
func quickSort(arr []*model.LogEntry, low, high int, less func(a, b *model.LogEntry) bool) {
	if low < high {
		pivot := partition(arr, low, high, less)
		quickSort(arr, low, pivot-1, less)
		quickSort(arr, pivot+1, high, less)
	}
}

func partition(arr []*model.LogEntry, low, high int, less func(a, b *model.LogEntry) bool) int {
	pivot := arr[high]
	i := low - 1

	for j := low; j < high; j++ {
		if less(arr[j], pivot) {
			i++
			arr[i], arr[j] = arr[j], arr[i]
		}
	}

	arr[i+1], arr[high] = arr[high], arr[i+1]
	return i + 1
}

// Get 通过 ID 获取日志（按 Hot → Warm → Cold 顺序查找）
func (t *TieredStore) Get(ctx context.Context, id string) (*model.LogEntry, error) {
	t.mu.RLock()
	if t.closed {
		t.mu.RUnlock()
		return nil, fmt.Errorf("store is closed")
	}
	t.mu.RUnlock()

	// 优先从 Hot 层查找（大概率是最近的数据）
	if entry, err := t.hot.Get(ctx, id); err == nil {
		return entry, nil
	}

	// 然后从 Warm 层查找
	if entry, err := t.warm.Get(ctx, id); err == nil {
		return entry, nil
	}

	// 最后从 Cold 层查找
	if entry, err := t.cold.Get(ctx, id); err == nil {
		return entry, nil
	}

	return nil, fmt.Errorf("entry not found: %s", id)
}

// Stats 获取统计信息
func (t *TieredStore) Stats(ctx context.Context) (*Stats, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.closed {
		return nil, fmt.Errorf("store is closed")
	}

	// 并行获取各层统计
	var wg sync.WaitGroup
	var hotStats, warmStats, coldStats *Stats
	var hotErr, warmErr, coldErr error

	wg.Add(3)

	go func() {
		defer wg.Done()
		hotStats, hotErr = t.hot.Stats(ctx)
	}()

	go func() {
		defer wg.Done()
		warmStats, warmErr = t.warm.Stats(ctx)
	}()

	go func() {
		defer wg.Done()
		coldStats, coldErr = t.cold.Stats(ctx)
	}()

	wg.Wait()

	// 合并统计
	stats := &Stats{
		Type:    "tiered",
		Uptime:  time.Since(t.startAt),
		Healthy: true,
	}

	if hotErr == nil && hotStats != nil {
		stats.TotalLogs += hotStats.TotalLogs
		stats.TotalSize += hotStats.TotalSize
		stats.HotStats = &TierStats{
			Count: hotStats.TotalLogs,
			Size:  hotStats.TotalSize,
			TTL:   t.hotThreshold * 2,
		}
	}

	if warmErr == nil && warmStats != nil {
		stats.TotalLogs += warmStats.TotalLogs
		stats.TotalSize += warmStats.TotalSize
		stats.WarmStats = &TierStats{
			Count: warmStats.TotalLogs,
			Size:  warmStats.TotalSize,
			TTL:   t.warmThreshold * 2,
		}
	}

	if coldErr == nil && coldStats != nil {
		stats.TotalLogs += coldStats.TotalLogs
		stats.TotalSize += coldStats.TotalSize
		stats.ColdStats = &TierStats{
			Count: coldStats.TotalLogs,
			Size:  coldStats.TotalSize,
			TTL:   t.cfg.GetDuration(ConfigKeyColdTTL, 7*24*time.Hour),
		}
	}

	// 健康状态
	stats.Healthy = (hotErr == nil && warmErr == nil && coldErr == nil)

	return stats, nil
}

// Close 关闭存储
func (t *TieredStore) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return nil
	}

	t.closed = true

	// 停止迁移器
	if t.mover != nil {
		t.mover.Stop()
	}

	// 关闭信号
	close(t.stopper)

	// 关闭各层存储
	var errs []error

	if err := t.hot.Close(); err != nil {
		errs = append(errs, fmt.Errorf("close hot: %w", err))
	}

	if err := t.warm.Close(); err != nil {
		errs = append(errs, fmt.Errorf("close warm: %w", err))
	}

	if err := t.cold.Close(); err != nil {
		errs = append(errs, fmt.Errorf("close cold: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("close tiered store: %v", errs)
	}

	return nil
}

// Cleanup 清理旧数据
func (t *TieredStore) Cleanup(ctx context.Context, before time.Time) (int64, error) {
	t.mu.RLock()
	if t.closed {
		t.mu.RUnlock()
		return 0, fmt.Errorf("store is closed")
	}
	t.mu.RUnlock()

	var total int64

	// 清理各层
	if count, err := t.hot.Cleanup(ctx, before); err == nil {
		total += count
	}

	if count, err := t.warm.Cleanup(ctx, before); err == nil {
		total += count
	}

	if count, err := t.cold.Cleanup(ctx, before); err == nil {
		total += count
	}

	return total, nil
}

// Health 健康检查
func (t *TieredStore) Health(ctx context.Context) error {
	t.mu.RLock()
	if t.closed {
		t.mu.RUnlock()
		return fmt.Errorf("store is closed")
	}
	t.mu.RUnlock()

	// 检查各层健康状态
	if err := t.hot.Health(ctx); err != nil {
		return fmt.Errorf("hot tier unhealthy: %w", err)
	}

	if err := t.warm.Health(ctx); err != nil {
		return fmt.Errorf("warm tier unhealthy: %w", err)
	}

	if err := t.cold.Health(ctx); err != nil {
		return fmt.Errorf("cold tier unhealthy: %w", err)
	}

	return nil
}

// Delete 通过ID删除单条记录（在所有层中查找并删除）
func (t *TieredStore) Delete(ctx context.Context, id string) error {
	t.mu.RLock()
	if t.closed {
		t.mu.RUnlock()
		return fmt.Errorf("store is closed")
	}
	t.mu.RUnlock()

	// 检查上下文
	if err := ctx.Err(); err != nil {
		return err
	}

	// 按优先级顺序查找并删除：Hot → Warm → Cold
	// 数据通常只在其中一层，找到后立即删除并返回
	tiers := []Store{t.hot, t.warm, t.cold}
	tierNames := []string{"hot", "warm", "cold"}

	for i, tier := range tiers {
		err := tier.Delete(ctx, id)
		if err == nil {
			// 成功删除，返回
			return nil
		}

		// 如果是"未找到"错误，继续查找下一层
		// 其他错误（如数据库错误）应该立即返回
		if !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "entry not found") {
			return fmt.Errorf("delete from %s tier: %w", tierNames[i], err)
		}
	}

	// 所有层都没找到
	return fmt.Errorf("entry not found: %s", id)
}

// DeleteMany 批量删除（在所有层中查找并删除）
func (t *TieredStore) DeleteMany(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	t.mu.RLock()
	if t.closed {
		t.mu.RUnlock()
		return fmt.Errorf("store is closed")
	}
	t.mu.RUnlock()

	// 检查上下文
	if err := ctx.Err(); err != nil {
		return err
	}

	// 按优先级顺序在各层中删除
	// 对于批量删除，我们尝试从所有层删除（因为数据可能分散在不同层）
	tiers := []Store{t.hot, t.warm, t.cold}
	var allErrors []error

	// 遍历每一层，尝试删除所有 ID
	// 注意：由于数据可能分散在不同层，我们允许部分成功
	for _, tier := range tiers {
		err := tier.DeleteMany(ctx, ids)
		if err != nil {
			// 记录错误但继续处理其他层
			allErrors = append(allErrors, err)
		}
	}

	// 如果所有层都失败，返回错误
	// 否则返回 nil（允许部分成功）
	if len(allErrors) == len(tiers) {
		return fmt.Errorf("failed to delete from all tiers: %v", allErrors)
	}

	return nil
}

// 内部方法：供 Mover 使用

// GetHotStore 获取 Hot 层（供迁移使用）
func (t *TieredStore) GetHotStore() Store {
	return t.hot
}

// GetWarmStore 获取 Warm 层（供迁移使用）
func (t *TieredStore) GetWarmStore() Store {
	return t.warm
}

// GetColdStore 获取 Cold 层（供迁移使用）
func (t *TieredStore) GetColdStore() Store {
	return t.cold
}
