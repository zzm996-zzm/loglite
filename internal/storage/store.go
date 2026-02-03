package storage

import (
	"context"
	"time"

	"github.com/loglite/loglite/internal/model"
)

// 使用你的 model.LogEntry，不重新定义

// Query 查询参数
type Query struct {
	// 时间范围
	From time.Time `json:"from"`
	To   time.Time `json:"to"`

	// 过滤条件
	Services   []string `json:"services,omitempty"`
	Levels     []string `json:"levels,omitempty"`
	TraceIDs   []string `json:"trace_ids,omitempty"`
	UserIDs    []string `json:"user_ids,omitempty"`
	RequestIDs []string `json:"request_ids,omitempty"`

	// 代码位置过滤
	Functions []string `json:"functions,omitempty"`
	Packages  []string `json:"packages,omitempty"`
	Callers   []string `json:"callers,omitempty"`

	// 关键词搜索
	Keywords []string `json:"keywords,omitempty"`

	// 分页
	Limit  int `json:"limit,omitempty"`
	Offset int `json:"offset,omitempty"`

	// 排序
	SortBy string `json:"sort_by,omitempty"` // "time_desc", "time_asc", "level"

	// 聚合选项
	GroupBy []string `json:"group_by,omitempty"` // ["service", "level", "hour"]
}

// Result 查询结果
type Result struct {
	Entries []*model.LogEntry `json:"entries"`
	Total   int               `json:"total"`
	HasMore bool              `json:"has_more"`
	Took    time.Duration     `json:"took"`

	// 聚合结果（如果GroupBy不为空）
	Aggregates map[string]interface{} `json:"aggregates,omitempty"`
}

// Stats 存储统计
type Stats struct {
	Type       string        `json:"type"`        // 存储类型
	TotalLogs  int64         `json:"total_logs"`  // 总日志数
	TotalSize  int64         `json:"total_size"`  // 存储大小(bytes)
	Uptime     time.Duration `json:"uptime"`      // 运行时间
	MemoryUsed int64         `json:"memory_used"` // 内存使用(bytes)
	Healthy    bool          `json:"healthy"`     // 健康状态

	// 分级存储统计
	HotStats  *TierStats `json:"hot_stats,omitempty"`
	WarmStats *TierStats `json:"warm_stats,omitempty"`
	ColdStats *TierStats `json:"cold_stats,omitempty"`
}

// TierStats 分级存储统计
type TierStats struct {
	Count      int64         `json:"count"`
	Size       int64         `json:"size"`
	TTL        time.Duration `json:"ttl"`
	LastAccess time.Time     `json:"last_access"`
}

// Store 存储接口
type Store interface {
	// 写操作
	Write(ctx context.Context, entry *model.LogEntry) error
	WriteMany(ctx context.Context, entries []*model.LogEntry) error

	// 读操作
	Query(ctx context.Context, q *Query) (*Result, error)
	Get(ctx context.Context, id string) (*model.LogEntry, error)

	// 管理操作
	Stats(ctx context.Context) (*Stats, error)
	Close() error

	// 清理操作（按时间）
	Cleanup(ctx context.Context, before time.Time) (int64, error)

	// 清理数据
	Delete(ctx context.Context, id string) error
	DeleteMany(ctx context.Context, ids []string) error

	// 健康检查
	Health(ctx context.Context) error
}

// Creator 创建器接口（用于扩展）
type Creator interface {
	// 创建存储实例
	Create(ctx context.Context, cfg Config) (Store, error)

	// 验证配置
	Validate(cfg Config) error

	// 默认配置
	DefaultConfig() Config
}
