package storage

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bytedance/sonic"

	"github.com/dgraph-io/badger/v4"
	opt "github.com/dgraph-io/badger/v4/options"

	"github.com/loglite/loglite/internal/model"
)

// BadgerStore 全新的实现
type BadgerStore struct {
	db    *badger.DB
	cache sync.Map // 简单的内存缓存
	path  string
	ttl   time.Duration // 数据过期时间

	mu      sync.RWMutex
	stats   Stats
	startAt time.Time
	closed  bool // 关闭状态标记
}

// NewBadgerStore 创建函数
func NewBadgerStore(cfg Config) (Store, error) {
	path := cfg.GetString("path", "./data/badger")
	ttl := cfg.GetDuration("ttl", 168*time.Hour) // 7天

	// 创建目录
	if err := os.MkdirAll(path, 0755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	// Badger配置
	opts := badger.DefaultOptions(path).
		WithMemTableSize(64 << 20).      // 64MB
		WithValueLogFileSize(256 << 20). // 256MB
		WithCompression(opt.ZSTD).
		WithLogger(nil)

	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("open badger: %w", err)
	}

	store := &BadgerStore{
		db:      db,
		path:    path,
		ttl:     ttl,
		startAt: time.Now(),
	}

	// 启动后台任务
	go store.startBackgroundTasks()

	return store, nil
}

// 同时更新buildKey，确保格式一致
func (s *BadgerStore) buildKey(entry *model.LogEntry) []byte {
	// 确保格式：t|timestamp|s|service|l|level|id|id
	// 所有字段都不允许包含"|"字符

	// 清理字段中的"|"字符（安全起见）
	safeService := strings.ReplaceAll(entry.Service, "|", "_")
	safeLevel := strings.ReplaceAll(entry.Level, "|", "_")
	safeID := strings.ReplaceAll(entry.ID, "|", "_")

	ts := entry.Timestamp.UnixNano()
	return []byte(fmt.Sprintf("t|%020d|s|%s|l|%s|id|%s",
		ts,
		safeService,
		safeLevel,
		safeID))
}

// parseKey 解析键（完整实现）
func (s *BadgerStore) parseKey(key []byte) (time.Time, string, string, string) {
	// 格式: t|timestamp|s|service|l|level|id|id
	// 示例: t|00000001637145678000|s|auth-service|l|error|id|abc123

	if len(key) == 0 {
		return time.Time{}, "", "", ""
	}

	// 快速路径：先检查格式前缀
	if !bytes.HasPrefix(key, []byte("t|")) {
		return time.Time{}, "", "", ""
	}

	parts := bytes.Split(key, []byte("|"))
	if len(parts) < 8 {
		return time.Time{}, "", "", ""
	}

	// 解析时间戳（第2部分）
	var ts time.Time
	if timestamp, err := strconv.ParseInt(string(parts[1]), 10, 64); err == nil {
		ts = time.Unix(0, timestamp)
	}

	// 解析服务名（第4部分）
	service := string(parts[3])

	// 解析级别（第6部分）
	level := string(parts[5])

	// 解析ID（第8部分）
	id := string(parts[7])

	return ts, service, level, id
}

// 为了提高性能，可以添加一个快速解析时间的函数
func (s *BadgerStore) parseTimeFromKey(key []byte) time.Time {
	// 快速解析：直接定位时间戳部分，避免分割整个字符串
	// 格式: t|timestamp|s|service|l|level|id|id
	// 时间戳在第一个"|"之后，第二个"|"之前

	// 找到第一个"|"
	firstPipe := bytes.IndexByte(key, '|')
	if firstPipe == -1 {
		return time.Time{}
	}

	// 找到第二个"|"
	secondPipe := bytes.IndexByte(key[firstPipe+1:], '|')
	if secondPipe == -1 {
		return time.Time{}
	}

	// 提取时间戳部分
	timestampStr := string(key[firstPipe+1 : firstPipe+1+secondPipe])

	if timestamp, err := strconv.ParseInt(timestampStr, 10, 64); err == nil {
		return time.Unix(0, timestamp)
	}

	return time.Time{}
}

// 添加一个快速解析服务的函数（用于前缀扫描）
func (s *BadgerStore) parseServiceFromKey(key []byte) string {
	// 格式: t|timestamp|s|service|l|level|id|id
	// 服务名在第三个"|"之后，第四个"|"之前

	// 统计"|"的位置
	pipeCount := 0
	for i, b := range key {
		if b == '|' {
			pipeCount++
			if pipeCount == 3 {
				// 找到第三个"|"，继续找第四个"|"
				for j := i + 1; j < len(key); j++ {
					if key[j] == '|' {
						// 找到第四个"|"，提取服务名
						return string(key[i+1 : j])
					}
				}
				break
			}
		}
	}

	return ""
}

// Write 写入日志
func (s *BadgerStore) Write(ctx context.Context, entry *model.LogEntry) error {
	// 确保时间戳
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	key := s.buildKey(entry)
	val, err := sonic.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal entry: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	err = s.db.Update(func(txn *badger.Txn) error {
		// 写入主数据
		e := badger.NewEntry(key, val)
		if s.ttl > 0 {
			e = e.WithTTL(s.ttl)
		}
		if err := txn.SetEntry(e); err != nil {
			return err
		}

		// 写入ID索引：id|{uuid} -> 实际的key
		// 空间换时间：额外存储索引，但查询时从 O(n) 全表扫描优化到 O(1) 索引查找
		idKey := s.buildIDKey(entry.ID)
		idIndexEntry := badger.NewEntry(idKey, key)
		if s.ttl > 0 {
			idIndexEntry = idIndexEntry.WithTTL(s.ttl)
		}
		return txn.SetEntry(idIndexEntry)
	})

	if err != nil {
		return fmt.Errorf("write to badger: %w", err)
	}

	// 更新缓存
	s.cache.Store(entry.ID, entry)

	// 更新统计
	s.stats.TotalLogs++

	return nil
}

// WriteMany 批量写入
func (s *BadgerStore) WriteMany(ctx context.Context, entries []*model.LogEntry) error {
	if len(entries) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 使用WriteBatch提高性能
	wb := s.db.NewWriteBatch()
	defer wb.Cancel()

	for _, entry := range entries {
		key := s.buildKey(entry)
		val, err := sonic.Marshal(entry)
		if err != nil {
			continue
		}

		// 写入主数据
		e := badger.NewEntry(key, val)
		if s.ttl > 0 {
			e = e.WithTTL(s.ttl)
		}

		if err := wb.SetEntry(e); err != nil {
			return fmt.Errorf("batch write: %w", err)
		}

		// 写入ID索引（空间换时间策略）
		idKey := s.buildIDKey(entry.ID)
		idIndexEntry := badger.NewEntry(idKey, key)
		if s.ttl > 0 {
			idIndexEntry = idIndexEntry.WithTTL(s.ttl)
		}
		if err := wb.SetEntry(idIndexEntry); err != nil {
			return fmt.Errorf("batch write index: %w", err)
		}

		// 更新缓存
		s.cache.Store(entry.ID, entry)
	}

	if err := wb.Flush(); err != nil {
		return fmt.Errorf("flush batch: %w", err)
	}

	// 更新统计
	s.stats.TotalLogs += int64(len(entries))

	return nil
}

// Query 查询（优化的实现）
func (s *BadgerStore) Query(ctx context.Context, q *Query) (*Result, error) {
	start := time.Now()

	// 检查上下文
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// 构建查询计划
	plan := s.buildQueryPlan(q)

	// 执行查询
	entries, err := s.executeQuery(ctx, q, plan)
	if err != nil {
		return nil, err
	}

	// 应用分页
	total := len(entries)
	if q.Offset > 0 && q.Offset < total {
		entries = entries[q.Offset:]
		total -= q.Offset
	}

	if q.Limit > 0 && q.Limit < len(entries) {
		entries = entries[:q.Limit]
	}

	return &Result{
		Entries: entries,
		Total:   total,
		HasMore: q.Limit > 0 && len(entries) >= q.Limit,
		Took:    time.Since(start),
	}, nil
}

// buildQueryPlan，构建更精确的前缀
func (s *BadgerStore) buildQueryPlan(q *Query) queryPlan {
	plan := queryPlan{}

	// 如果有服务过滤，使用前缀
	if len(q.Services) == 1 {
		plan.usePrefix = true
		// 构建前缀：t|timestamp|s|service
		// 为了精确扫描，我们可以使用时间范围的上限
		ts := q.To
		if ts.IsZero() {
			ts = time.Now()
		}
		plan.prefix = []byte(fmt.Sprintf("t|%020d|s|%s",
			ts.UnixNano(),
			q.Services[0]))
	}

	// 确定扫描方向
	if q.SortBy == "time_asc" {
		plan.reverse = false
	} else {
		plan.reverse = true // 默认从新到旧
	}

	return plan
}

type queryPlan struct {
	usePrefix bool
	prefix    []byte
	reverse   bool
}

// 更新executeQuery中的时间过滤部分，使用快速解析
func (s *BadgerStore) executeQuery(ctx context.Context, q *Query, plan queryPlan) ([]*model.LogEntry, error) {
	var entries []*model.LogEntry

	s.mu.RLock()
	defer s.mu.RUnlock()

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Reverse = plan.reverse
		opts.PrefetchSize = 100

		it := txn.NewIterator(opts)
		defer it.Close()

		// 定位起始位置
		if plan.usePrefix {
			it.Seek(plan.prefix)
		} else {
			if plan.reverse {
				// 反向：从最新开始
				seekKey := []byte(fmt.Sprintf("t|%020d", q.To.UnixNano()))
				it.Seek(seekKey)
			} else {
				// 正向：从最旧开始
				seekKey := []byte(fmt.Sprintf("t|%020d", q.From.UnixNano()))
				it.Seek(seekKey)
			}
		}

		count := 0
		for it.Valid() {
			// 检查上下文
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			item := it.Item()
			key := item.Key()

			// 如果使用前缀，检查前缀是否匹配
			if plan.usePrefix && !hasPrefix(key, plan.prefix) {
				break
			}

			// 快速解析时间（避免完整解析）
			ts := s.parseTimeFromKey(key)
			if ts.IsZero() {
				it.Next()
				continue
			}

			// 时间过滤（使用快速解析的时间）
			if !q.From.IsZero() && ts.Before(q.From) {
				if plan.reverse {
					// 反向遍历，更早的数据不符合
					it.Next()
					continue
				} else {
					// 正向遍历，更早的数据已经过了
					break
				}
			}
			if !q.To.IsZero() && ts.After(q.To) {
				if plan.reverse {
					// 反向遍历，更新的数据可能符合
					break
				} else {
					// 正向遍历，更新的数据不符合
					it.Next()
					continue
				}
			}

			// 如果需要服务过滤，快速解析服务名
			if len(q.Services) > 0 {
				service := s.parseServiceFromKey(key)
				if service == "" || !contains(q.Services, service) {
					it.Next()
					continue
				}
			}

			// 读取完整数据（用于其他过滤）
			var entry model.LogEntry
			err := item.Value(func(val []byte) error {
				return sonic.Unmarshal(val, &entry)
			})
			if err != nil {
				it.Next()
				continue
			}

			// 添加用户ID过滤
			if len(q.UserIDs) > 0 && entry.UserID != "" {
				if !contains(q.UserIDs, entry.UserID) {
					it.Next()
					continue
				}
			}

			// 添加函数/包过滤
			if len(q.Functions) > 0 && entry.Function != "" {
				if !contains(q.Functions, entry.Function) {
					it.Next()
					continue
				}
			}

			// 级别过滤（需要完整数据）
			if len(q.Levels) > 0 && !contains(q.Levels, entry.Level) {
				it.Next()
				continue
			}

			// TraceID过滤
			if len(q.TraceIDs) > 0 && entry.TraceID != "" {
				if !contains(q.TraceIDs, entry.TraceID) {
					it.Next()
					continue
				}
			}

			// 关键词搜索
			if len(q.Keywords) > 0 {
				if !s.matchKeywords(&entry, q.Keywords) {
					it.Next()
					continue
				}
			}

			entries = append(entries, &entry)
			count++

			// 简单限制，避免内存爆炸
			if count >= 10000 {
				break
			}

			it.Next()
		}

		return nil
	})

	return entries, err
}

// matchKeywords 关键词匹配
func (s *BadgerStore) matchKeywords(entry *model.LogEntry, keywords []string) bool {
	for _, keyword := range keywords {
		kw := keyword
		if containsString(entry.Message, kw) ||
			containsString(entry.Service, kw) ||
			containsString(entry.Function, kw) {
			return true
		}
	}
	return false
}

// buildIDKey 构建ID索引的key
// 索引结构：id|{uuid} -> t|timestamp|s|service|l|level|id|{uuid}
// 这是空间换时间的策略：额外存储索引数据，但查询性能从 O(n) 提升到 O(1)
func (s *BadgerStore) buildIDKey(id string) []byte {
	return []byte(fmt.Sprintf("id|%s", id))
}

// Get 通过ID获取
func (s *BadgerStore) Get(ctx context.Context, id string) (*model.LogEntry, error) {
	// 先查缓存
	if val, ok := s.cache.Load(id); ok {
		return val.(*model.LogEntry), nil
	}

	var entry *model.LogEntry

	s.mu.RLock()
	defer s.mu.RUnlock()

	err := s.db.View(func(txn *badger.Txn) error {
		// 方案1：通过ID索引查找（O(1) 时间复杂度，空间换时间）
		// 索引：id|{uuid} -> t|timestamp|s|service|l|level|id|{uuid}
		idKey := s.buildIDKey(id)
		item, err := txn.Get(idKey)
		if err == nil {
			// 找到索引，获取实际的key
			var actualKey []byte
			err := item.Value(func(val []byte) error {
				actualKey = make([]byte, len(val))
				copy(actualKey, val)
				return nil
			})
			if err == nil {
				// 使用实际的key获取日志条目
				item, err := txn.Get(actualKey)
				if err == nil {
					return item.Value(func(val []byte) error {
						return sonic.Unmarshal(val, &entry)
					})
				}
			}
		}

		// 方案2：如果索引不存在（向后兼容），通过扫描key查找
		// 优化：只解析key，不读取value，性能比原来的全表扫描好
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false // 不预取value，只扫描key
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			key := it.Item().Key()

			// 跳过索引key
			// 注意：BadgerDB 是单数据库存储，索引和数据都在同一个数据库中
			// 不像 MySQL 那样有多个表文件，而是通过 key 前缀来区分：
			// - "id|{uuid}" 开头的 key 是索引数据
			// - "t|timestamp|..." 开头的 key 是实际日志数据
			// 在扫描时需要跳过索引 key，只处理实际数据 key
			if bytes.HasPrefix(key, []byte("id|")) {
				it.Next()
				continue
			}

			// 解析key中的ID（只处理数据key，格式：t|timestamp|s|service|l|level|id|{uuid}）
			_, _, _, keyID := s.parseKey(key)
			if keyID == id {
				// 找到匹配的key，读取value
				item := it.Item()
				return item.Value(func(val []byte) error {
					return sonic.Unmarshal(val, &entry)
				})
			}
		}

		return fmt.Errorf("entry not found: %s", id)
	})

	if err != nil {
		return nil, err
	}

	if entry == nil {
		return nil, fmt.Errorf("entry not found: %s", id)
	}

	// 更新缓存
	s.cache.Store(id, entry)

	return entry, nil
}

// Stats 获取统计信息
func (s *BadgerStore) Stats(ctx context.Context) (*Stats, error) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return nil, fmt.Errorf("store is closed")
	}
	s.mu.RUnlock()

	lsm, vlog := s.db.Size()

	return &Stats{
		Type:      "badger",
		TotalLogs: s.stats.TotalLogs,
		TotalSize: lsm + vlog,
		Uptime:    time.Since(s.startAt),
		Healthy:   true,
	}, nil
}

// Close 关闭
func (s *BadgerStore) Close() error {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()

	return s.db.Close()
}

// Cleanup 清理旧数据
func (s *BadgerStore) Cleanup(ctx context.Context, before time.Time) (int64, error) {
	// Badger会自动根据TTL清理，这里暂时不实现手动清理
	return 0, nil
}

// Health 健康检查
func (s *BadgerStore) Health(ctx context.Context) error {
	return s.db.View(func(txn *badger.Txn) error {
		// 执行一个空操作来检查连接
		return nil
	})
}

// startBackgroundTasks 启动后台任务
func (s *BadgerStore) startBackgroundTasks() {
	// 定期GC
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			s.db.RunValueLogGC(0.5)
		}
	}()

	// 定期清理缓存
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			s.cleanupCache()
		}
	}()
}

// cleanupCache 清理缓存
func (s *BadgerStore) cleanupCache() {
	// 简单的缓存清理：随机删除一些旧条目
	// 实际可以使用LRU等策略
	s.cache.Range(func(key, value interface{}) bool {
		// 这里可以添加清理逻辑
		return true
	})
}
