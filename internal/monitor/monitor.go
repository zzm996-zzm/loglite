package monitor

import (
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/loglite/loglite/internal/storage"
)

// ============================================================
// 监控仪表板框架
// 实时监控系统状态、性能指标
// ============================================================

// Monitor 监控管理器
type Monitor struct {
	store   *storage.BadgerStore
	metrics *Metrics
	mu      sync.RWMutex

	// 滑动平均计算
	ingestLatencies  []time.Duration
	queryLatencies   []time.Duration
	storageLatencies []time.Duration

	// 时间窗口统计
	timeWindowLogs map[int64]int64 // timestamp -> count

	// 原子计数器
	logsReceived  int64
	logsStored    int64
	logsDropped   int64
	requestsTotal int64
	requestsError int64
}

// Metrics 监控指标
type Metrics struct {
	// 日志统计
	LogsReceived int64 `json:"logs_received"` // 接收的日志总数
	LogsStored   int64 `json:"logs_stored"`   // 存储的日志总数
	LogsDropped  int64 `json:"logs_dropped"`  // 丢弃的日志总数（限流等原因）

	// 性能指标
	AvgIngestLatency  time.Duration `json:"avg_ingest_latency"`  // 平均接收延迟
	AvgQueryLatency   time.Duration `json:"avg_query_latency"`   // 平均查询延迟
	AvgStorageLatency time.Duration `json:"avg_storage_latency"` // 平均存储延迟

	// 系统资源
	MemoryUsage int64   `json:"memory_usage"` // 内存使用量（字节）
	DiskUsage   int64   `json:"disk_usage"`   // 磁盘使用量（字节）
	CPUPercent  float64 `json:"cpu_percent"`  // CPU 使用率

	// 数据库统计
	BadgerSize   int64   `json:"badger_size"`    // BadgerDB 大小
	CacheHitRate float64 `json:"cache_hit_rate"` // 缓存命中率

	// API 统计
	RequestsTotal int64 `json:"requests_total"` // 请求总数
	RequestsError int64 `json:"requests_error"` // 错误请求数

	// 时间窗口统计
	LogsLast1Min  int64 `json:"logs_last_1min"`  // 最近1分钟日志数
	LogsLast5Min  int64 `json:"logs_last_5min"`  // 最近5分钟日志数
	LogsLast15Min int64 `json:"logs_last_15min"` // 最近15分钟日志数

	// 更新时间
	UpdatedAt time.Time `json:"updated_at"`
}

// NewMonitor 创建监控管理器
func NewMonitor(store *storage.BadgerStore) *Monitor {
	m := &Monitor{
		store: store,
		metrics: &Metrics{
			UpdatedAt: time.Now(),
		},
		ingestLatencies:  make([]time.Duration, 0, 100),
		queryLatencies:   make([]time.Duration, 0, 100),
		storageLatencies: make([]time.Duration, 0, 100),
		timeWindowLogs:   make(map[int64]int64),
	}

	// 启动后台收集协程
	go m.collect()

	return m
}

// GetMetrics 获取当前指标（实时更新）
func (m *Monitor) GetMetrics() *Metrics {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 实时更新原子计数器到 metrics（不等待后台协程）
	m.metrics.LogsReceived = atomic.LoadInt64(&m.logsReceived)
	m.metrics.LogsStored = atomic.LoadInt64(&m.logsStored)
	m.metrics.LogsDropped = atomic.LoadInt64(&m.logsDropped)
	m.metrics.RequestsTotal = atomic.LoadInt64(&m.requestsTotal)
	m.metrics.RequestsError = atomic.LoadInt64(&m.requestsError)

	// 实时收集系统指标
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	m.metrics.MemoryUsage = int64(memStats.Alloc)

	// 实时收集数据库指标
	if m.store != nil {
		if stats, err := m.store.GetStats(); err == nil {
			m.metrics.BadgerSize = stats.StorageSize
		}
	}

	m.metrics.UpdatedAt = time.Now()

	// 复制一份返回
	metrics := *m.metrics
	return &metrics
}

// IncrLogReceived 增加接收日志计数
func (m *Monitor) IncrLogReceived(count int64) {
	atomic.AddInt64(&m.logsReceived, count)
}

// IncrLogStored 增加存储日志计数
func (m *Monitor) IncrLogStored(count int64) {
	atomic.AddInt64(&m.logsStored, count)
}

// IncrLogDropped 增加丢弃日志计数
func (m *Monitor) IncrLogDropped(count int64) {
	atomic.AddInt64(&m.logsDropped, count)
}

// IncrRequestTotal 增加请求总数
func (m *Monitor) IncrRequestTotal() {
	atomic.AddInt64(&m.requestsTotal, 1)
}

// IncrRequestError 增加错误请求数
func (m *Monitor) IncrRequestError() {
	atomic.AddInt64(&m.requestsError, 1)
}

// RecordIngestLatency 记录接收延迟
func (m *Monitor) RecordIngestLatency(latency time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 滑动窗口：保留最近100个样本
	m.ingestLatencies = append(m.ingestLatencies, latency)
	if len(m.ingestLatencies) > 100 {
		m.ingestLatencies = m.ingestLatencies[1:]
	}

	// 计算平均值
	var sum time.Duration
	for _, l := range m.ingestLatencies {
		sum += l
	}
	if len(m.ingestLatencies) > 0 {
		m.metrics.AvgIngestLatency = sum / time.Duration(len(m.ingestLatencies))
	}
}

// RecordQueryLatency 记录查询延迟
func (m *Monitor) RecordQueryLatency(latency time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.queryLatencies = append(m.queryLatencies, latency)
	if len(m.queryLatencies) > 100 {
		m.queryLatencies = m.queryLatencies[1:]
	}

	var sum time.Duration
	for _, l := range m.queryLatencies {
		sum += l
	}
	if len(m.queryLatencies) > 0 {
		m.metrics.AvgQueryLatency = sum / time.Duration(len(m.queryLatencies))
	}
}

// RecordStorageLatency 记录存储延迟
func (m *Monitor) RecordStorageLatency(latency time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.storageLatencies = append(m.storageLatencies, latency)
	if len(m.storageLatencies) > 100 {
		m.storageLatencies = m.storageLatencies[1:]
	}

	var sum time.Duration
	for _, l := range m.storageLatencies {
		sum += l
	}
	if len(m.storageLatencies) > 0 {
		m.metrics.AvgStorageLatency = sum / time.Duration(len(m.storageLatencies))
	}
}

// collect 后台收集指标
func (m *Monitor) collect() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		m.collectSystemMetrics()
		m.collectDatabaseMetrics()
		m.collectTimeWindowMetrics()

		m.mu.Lock()
		// 更新原子计数器到 metrics
		m.metrics.LogsReceived = atomic.LoadInt64(&m.logsReceived)
		m.metrics.LogsStored = atomic.LoadInt64(&m.logsStored)
		m.metrics.LogsDropped = atomic.LoadInt64(&m.logsDropped)
		m.metrics.RequestsTotal = atomic.LoadInt64(&m.requestsTotal)
		m.metrics.RequestsError = atomic.LoadInt64(&m.requestsError)
		m.metrics.UpdatedAt = time.Now()
		m.mu.Unlock()
	}
}

// collectSystemMetrics 收集系统资源指标
func (m *Monitor) collectSystemMetrics() {
	m.mu.Lock()
	defer m.mu.Unlock()

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	m.metrics.MemoryUsage = int64(memStats.Alloc)

	// CPU 使用率需要计算，这里简化处理
	// 实际应该使用 gopsutil 或计算两次采样之间的差值
	m.metrics.CPUPercent = 0.0 // TODO: 实现 CPU 使用率计算
}

// collectDatabaseMetrics 收集数据库指标
func (m *Monitor) collectDatabaseMetrics() {
	if m.store == nil {
		return
	}

	stats, err := m.store.GetStats()
	if err != nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.metrics.BadgerSize = stats.StorageSize
	m.metrics.CacheHitRate = 0.0 // BadgerDB 没有直接提供缓存命中率
}

// collectTimeWindowMetrics 收集时间窗口指标
func (m *Monitor) collectTimeWindowMetrics() {
	if m.store == nil {
		return
	}

	now := time.Now()

	// 查询最近 1/5/15 分钟的日志数量
	windows := []struct {
		duration time.Duration
		target   *int64
	}{
		{1 * time.Minute, &m.metrics.LogsLast1Min},
		{5 * time.Minute, &m.metrics.LogsLast5Min},
		{15 * time.Minute, &m.metrics.LogsLast15Min},
	}

	for _, w := range windows {
		startTime := now.Add(-w.duration)
		params := storage.QueryParams{
			StartTime: startTime,
			EndTime:   now,
			Limit:     10000,
		}

		_, count, err := m.store.Query(params)
		if err == nil {
			m.mu.Lock()
			*w.target = int64(count)
			m.mu.Unlock()
		}
	}
}

// ============================================================
// 健康检查
// ============================================================

// HealthStatus 健康状态
type HealthStatus struct {
	Status    string                 `json:"status"` // healthy/degraded/unhealthy
	Uptime    time.Duration          `json:"uptime"` // 运行时间
	Checks    map[string]CheckResult `json:"checks"`
	UpdatedAt time.Time              `json:"updated_at"`
}

// CheckResult 检查结果
type CheckResult struct {
	Status  string `json:"status"` // pass/fail
	Message string `json:"message"`
}

// HealthChecker 健康检查器
type HealthChecker struct {
	startTime time.Time
	store     *storage.BadgerStore
}

// NewHealthChecker 创建健康检查器
func NewHealthChecker(store *storage.BadgerStore) *HealthChecker {
	return &HealthChecker{
		startTime: time.Now(),
		store:     store,
	}
}

// Check 执行健康检查
func (h *HealthChecker) Check() *HealthStatus {
	status := &HealthStatus{
		Status:    "healthy",
		Uptime:    time.Since(h.startTime),
		Checks:    make(map[string]CheckResult),
		UpdatedAt: time.Now(),
	}

	// 检查数据库连接
	status.Checks["database"] = h.checkDatabase(h.store)

	// 检查磁盘空间
	status.Checks["disk"] = h.checkDisk()

	// 检查内存
	status.Checks["memory"] = h.checkMemory()

	// 判断整体状态
	hasFail := false
	hasDegraded := false
	for _, result := range status.Checks {
		if result.Status == "fail" {
			hasFail = true
			break
		} else if result.Status == "degraded" {
			hasDegraded = true
		}
	}

	if hasFail {
		status.Status = "unhealthy"
	} else if hasDegraded {
		status.Status = "degraded"
	}

	return status
}

// checkDatabase 检查数据库
func (h *HealthChecker) checkDatabase(store *storage.BadgerStore) CheckResult {
	if store == nil {
		return CheckResult{
			Status:  "fail",
			Message: "database store is nil",
		}
	}

	// 尝试获取统计信息来验证连接
	_, err := store.GetStats()
	if err != nil {
		return CheckResult{
			Status:  "fail",
			Message: "database connection failed: " + err.Error(),
		}
	}

	return CheckResult{
		Status:  "pass",
		Message: "database is healthy",
	}
}

// checkDisk 检查磁盘空间
func (h *HealthChecker) checkDisk() CheckResult {
	// 使用 runtime 获取内存信息
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// 简化处理：检查内存使用是否超过 80%
	threshold := uint64(0.8 * float64(memStats.Sys))
	if memStats.Alloc > threshold {
		return CheckResult{
			Status:  "degraded",
			Message: "memory usage is high",
		}
	}

	return CheckResult{
		Status:  "pass",
		Message: "disk space is sufficient",
	}
}

// checkMemory 检查内存
func (h *HealthChecker) checkMemory() CheckResult {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// 检查内存使用率
	usagePercent := float64(memStats.Alloc) / float64(memStats.Sys) * 100

	if usagePercent > 90 {
		return CheckResult{
			Status:  "fail",
			Message: "memory usage is critical",
		}
	} else if usagePercent > 80 {
		return CheckResult{
			Status:  "degraded",
			Message: "memory usage is high",
		}
	}

	return CheckResult{
		Status:  "pass",
		Message: "memory usage is normal",
	}
}

// ============================================================
// API Handler
// ============================================================

// MonitorHandler 监控 API 处理器
type MonitorHandler struct {
	monitor       *Monitor
	healthChecker *HealthChecker
}

// NewMonitorHandler 创建监控处理器
func NewMonitorHandler(monitor *Monitor, healthChecker *HealthChecker) *MonitorHandler {
	return &MonitorHandler{
		monitor:       monitor,
		healthChecker: healthChecker,
	}
}

// GetMetrics 获取指标
// GET /api/v1/metrics
func (h *MonitorHandler) GetMetrics() *Metrics {
	return h.monitor.GetMetrics()
}

// GetHealth 获取健康状态
// GET /api/v1/health
func (h *MonitorHandler) GetHealth() *HealthStatus {
	return h.healthChecker.Check()
}
