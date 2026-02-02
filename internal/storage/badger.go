package storage

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/dgraph-io/badger/v4"
	"github.com/loglite/loglite/internal/model"
)

// BadgerStore BadgerDB 存储实现
type BadgerStore struct {
	db        *badger.DB
	dataDir   string
	retention time.Duration
	mu        sync.RWMutex
}

// NewBadgerStore 创建 BadgerDB 存储
func NewBadgerStore(dataDir string, retention time.Duration) (*BadgerStore, error) {
	opts := badger.DefaultOptions(dataDir)
	opts.Logger = nil // 禁用 badger 内部日志

	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open badger db: %w", err)
	}

	store := &BadgerStore{
		db:        db,
		dataDir:   dataDir,
		retention: retention,
	}

	// 启动 GC 协程
	go store.runGC()

	return store, nil
}

// Save 保存日志条目
func (s *BadgerStore) Save(entry *model.LogEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 生成 key: service:timestamp:id
	key := s.buildKey(entry)

	data, err := sonic.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal entry: %w", err)
	}

	err = s.db.Update(func(txn *badger.Txn) error {
		e := badger.NewEntry(key, data)
		if s.retention > 0 {
			e = e.WithTTL(s.retention)
		}
		return txn.SetEntry(e)
	})

	if err != nil {
		return fmt.Errorf("failed to save entry: %w", err)
	}

	return nil
}

// SaveBatch 批量保存日志条目
func (s *BadgerStore) SaveBatch(entries []*model.LogEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	wb := s.db.NewWriteBatch()
	defer wb.Cancel()

	for _, entry := range entries {
		key := s.buildKey(entry)
		data, err := sonic.Marshal(entry)
		if err != nil {
			continue
		}

		e := badger.NewEntry(key, data)
		if s.retention > 0 {
			e = e.WithTTL(s.retention)
		}
		if err := wb.SetEntry(e); err != nil {
			return fmt.Errorf("failed to set entry in batch: %w", err)
		}
	}

	if err := wb.Flush(); err != nil {
		return fmt.Errorf("failed to flush batch: %w", err)
	}

	return nil
}

// QueryParams 查询参数
type QueryParams struct {
	Service      string
	ServiceFuzzy bool // 服务名模糊匹配
	Level        string
	StartTime    time.Time
	EndTime      time.Time
	Keyword      string
	Limit        int
	Offset       int
}

// Query 查询日志
func (s *BadgerStore) Query(params QueryParams) ([]*model.LogEntry, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*model.LogEntry
	total := 0

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Reverse = true // 从最新的开始
		it := txn.NewIterator(opts)
		defer it.Close()

		// 构建前缀（仅精确匹配时使用）
		var prefix []byte
		if params.Service != "" && !params.ServiceFuzzy {
			prefix = []byte(params.Service + ":")
		}

		// 如果有服务前缀，使用 Seek
		if len(prefix) > 0 {
			// 寻找该服务的最新日志
			seekKey := s.buildSeekKey(params.Service, params.EndTime)
			it.Seek(seekKey)
		} else {
			it.Rewind()
		}

		for it.Valid() {
			item := it.Item()
			key := item.Key()

			// 检查前缀（仅精确匹配）
			if len(prefix) > 0 && !hasPrefix(key, prefix) {
				break
			}

			// 解析时间
			entryTime := s.parseTimeFromKey(key)
			if !params.StartTime.IsZero() && entryTime.Before(params.StartTime) {
				// 模糊匹配时不能 break，因为不是按服务分组的
				if len(prefix) > 0 {
					break
				}
				it.Next()
				continue
			}
			if !params.EndTime.IsZero() && entryTime.After(params.EndTime) {
				it.Next()
				continue
			}

			// 读取数据
			var entry model.LogEntry
			err := item.Value(func(val []byte) error {
				return sonic.Unmarshal(val, &entry)
			})
			if err != nil {
				it.Next()
				continue
			}

			// 服务名过滤（模糊匹配）
			if params.Service != "" && params.ServiceFuzzy {
				if !strings.Contains(strings.ToLower(entry.Service), strings.ToLower(params.Service)) {
					it.Next()
					continue
				}
			}

			// 过滤条件
			if params.Level != "" && entry.Level != params.Level {
				it.Next()
				continue
			}
			// 关键词搜索（同时匹配消息和服务名）
			if params.Keyword != "" {
				keyword := strings.ToLower(params.Keyword)
				messageMatch := strings.Contains(strings.ToLower(entry.Message), keyword)
				serviceMatch := strings.Contains(strings.ToLower(entry.Service), keyword)
				functionMatch := strings.Contains(strings.ToLower(entry.Function), keyword)
				if !messageMatch && !serviceMatch && !functionMatch {
					it.Next()
					continue
				}
			}

			total++

			// 分页
			if total <= params.Offset {
				it.Next()
				continue
			}
			if params.Limit > 0 && len(results) >= params.Limit {
				it.Next()
				continue
			}

			results = append(results, &entry)
			it.Next()
		}

		return nil
	})

	if err != nil {
		return nil, 0, fmt.Errorf("query failed: %w", err)
	}

	return results, total, nil
}

// GetByID 根据 ID 获取日志
func (s *BadgerStore) GetByID(id string) (*model.LogEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var entry *model.LogEntry

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()

			err := item.Value(func(val []byte) error {
				var e model.LogEntry
				if err := sonic.Unmarshal(val, &e); err != nil {
					return nil
				}
				if e.ID == id {
					entry = &e
				}
				return nil
			})
			if err != nil {
				continue
			}
			if entry != nil {
				break
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, fmt.Errorf("entry not found: %s", id)
	}

	return entry, nil
}

// Stats 存储统计
type Stats struct {
	TotalLogs   int64  `json:"total_logs"`
	StorageSize int64  `json:"storage_size"`
	DataDir     string `json:"data_dir"`
}

// GetStats 获取统计信息
func (s *BadgerStore) GetStats() (*Stats, error) {
	lsm, vlog := s.db.Size()

	var count int64
	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			count++
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &Stats{
		TotalLogs:   count,
		StorageSize: lsm + vlog,
		DataDir:     s.dataDir,
	}, nil
}

// Close 关闭存储
func (s *BadgerStore) Close() error {
	return s.db.Close()
}

// buildKey 构建存储 key
func (s *BadgerStore) buildKey(entry *model.LogEntry) []byte {
	// 格式: service:timestamp_nano:id
	ts := entry.Timestamp.UnixNano()
	return []byte(fmt.Sprintf("%s:%020d:%s", entry.Service, ts, entry.ID))
}

// buildSeekKey 构建查询 seek key
func (s *BadgerStore) buildSeekKey(service string, t time.Time) []byte {
	if t.IsZero() {
		t = time.Now()
	}
	ts := t.UnixNano()
	return []byte(fmt.Sprintf("%s:%020d:", service, ts))
}

// parseTimeFromKey 从 key 解析时间
func (s *BadgerStore) parseTimeFromKey(key []byte) time.Time {
	parts := strings.Split(string(key), ":")
	if len(parts) < 2 {
		return time.Time{}
	}
	var ts int64
	fmt.Sscanf(parts[1], "%d", &ts)
	return time.Unix(0, ts)
}

// runGC 运行垃圾回收
func (s *BadgerStore) runGC() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.db.RunValueLogGC(0.5)
	}
}

// hasPrefix 检查前缀
func hasPrefix(s, prefix []byte) bool {
	if len(s) < len(prefix) {
		return false
	}
	for i := range prefix {
		if s[i] != prefix[i] {
			return false
		}
	}
	return true
}
