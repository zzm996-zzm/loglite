package api

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/loglite/loglite/internal/storage"
)

// StatsHandler 统计处理器
type StatsHandler struct {
	store storage.Store
}

// NewStatsHandler 创建统计处理器
func NewStatsHandler(store storage.Store) *StatsHandler {
	return &StatsHandler{store: store}
}

// GetErrorStats 获取错误统计
// GET /api/v1/stats/errors
func (h *StatsHandler) GetErrorStats(c *gin.Context) {
	// 解析参数
	groupBy := c.DefaultQuery("group_by", "function")
	timeRange := c.DefaultQuery("time_range", "1h")
	top, _ := strconv.Atoi(c.DefaultQuery("top", "10"))
	service := c.Query("service")

	if top <= 0 || top > 100 {
		top = 10
	}

	// 计算时间范围
	now := time.Now()
	var startTime time.Time
	switch timeRange {
	case "5m":
		startTime = now.Add(-5 * time.Minute)
	case "15m":
		startTime = now.Add(-15 * time.Minute)
	case "30m":
		startTime = now.Add(-30 * time.Minute)
	case "1h":
		startTime = now.Add(-1 * time.Hour)
	case "6h":
		startTime = now.Add(-6 * time.Hour)
	case "24h":
		startTime = now.Add(-24 * time.Hour)
	case "7d":
		startTime = now.AddDate(0, 0, -7)
	default:
		startTime = now.Add(-1 * time.Hour)
	}

	// 查询错误日志
	params := &storage.Query{
		Levels: []string{"error"},
		From:   startTime,
		To:     now,
		Limit:  10000, // 获取足够多的日志用于统计
	}
	if service != "" {
		params.Services = []string{service}
	}

	result, err := h.store.Query(context.Background(), params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "query failed",
			"error":   err.Error(),
		})
		return
	}
	logs := result.Entries
	total := result.Total

	// 聚合统计
	groups := make(map[string]*ErrorGroup)
	for _, log := range logs {
		var key string
		switch groupBy {
		case "function":
			key = log.Function
			if key == "" {
				key = "unknown"
			}
		case "caller":
			key = log.Caller
			if key == "" {
				key = "unknown"
			}
		case "service":
			key = log.Service
		case "message":
			// 取消息前50个字符
			key = log.Message
			if len(key) > 50 {
				key = key[:50] + "..."
			}
		default:
			key = log.Function
			if key == "" {
				key = "unknown"
			}
		}

		if _, ok := groups[key]; !ok {
			groups[key] = &ErrorGroup{
				Key:         key,
				Count:       0,
				LastSeen:    log.Timestamp.Format("2006-01-02 15:04:05"),
				SampleError: log.Message,
				Package:     log.Package,
			}
		}

		g := groups[key]
		g.Count++
		logTime := log.Timestamp.Format("2006-01-02 15:04:05")
		if logTime > g.LastSeen {
			g.LastSeen = logTime
			g.SampleError = log.Message
		}
	}

	// 排序并取 Top N
	errorGroups := sortAndTopN(groups, top, total)

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data": gin.H{
			"time_range":   startTime.Format("2006-01-02 15:04:05") + " ~ " + now.Format("2006-01-02 15:04:05"),
			"total_errors": total,
			"group_by":     groupBy,
			"groups":       errorGroups,
		},
	})
}

// ErrorGroup 错误分组
type ErrorGroup struct {
	Key         string `json:"key"`
	Count       int    `json:"count"`
	Percentage  string `json:"percentage"`
	LastSeen    string `json:"last_seen"`
	SampleError string `json:"sample_error"`
	Package     string `json:"package,omitempty"`
}

// sortAndTopN 排序并取前 N
func sortAndTopN(groups map[string]*ErrorGroup, top int, total int) []*ErrorGroup {
	// 转为切片
	list := make([]*ErrorGroup, 0, len(groups))
	for _, g := range groups {
		if total > 0 {
			g.Percentage = strconv.FormatFloat(float64(g.Count)/float64(total)*100, 'f', 1, 64) + "%"
		}
		list = append(list, g)
	}

	// 冒泡排序（简单实现）
	for i := 0; i < len(list)-1; i++ {
		for j := 0; j < len(list)-i-1; j++ {
			if list[j].Count < list[j+1].Count {
				list[j], list[j+1] = list[j+1], list[j]
			}
		}
	}

	// 取 Top N
	if len(list) > top {
		list = list[:top]
	}

	return list
}

// GetErrorTrend 获取错误趋势
// GET /api/v1/stats/errors/trend
func (h *StatsHandler) GetErrorTrend(c *gin.Context) {
	timeRange := c.DefaultQuery("time_range", "24h")
	interval := c.DefaultQuery("interval", "1h")
	service := c.Query("service")

	now := time.Now()
	var startTime time.Time
	var intervalDuration time.Duration

	// 解析时间范围
	switch timeRange {
	case "1h":
		startTime = now.Add(-1 * time.Hour)
		intervalDuration = 5 * time.Minute
	case "6h":
		startTime = now.Add(-6 * time.Hour)
		intervalDuration = 30 * time.Minute
	case "24h":
		startTime = now.Add(-24 * time.Hour)
		intervalDuration = 1 * time.Hour
	case "7d":
		startTime = now.AddDate(0, 0, -7)
		intervalDuration = 24 * time.Hour
	default:
		startTime = now.Add(-24 * time.Hour)
		intervalDuration = 1 * time.Hour
	}

	// 解析间隔
	switch interval {
	case "5m":
		intervalDuration = 5 * time.Minute
	case "15m":
		intervalDuration = 15 * time.Minute
	case "30m":
		intervalDuration = 30 * time.Minute
	case "1h":
		intervalDuration = 1 * time.Hour
	case "6h":
		intervalDuration = 6 * time.Hour
	case "24h":
		intervalDuration = 24 * time.Hour
	}

	// 查询错误日志
	params := &storage.Query{
		Levels: []string{"error"},
		From:   startTime,
		To:     now,
		Limit:  50000,
	}
	if service != "" {
		params.Services = []string{service}
	}

	result, err := h.store.Query(context.Background(), params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "query failed",
			"error":   err.Error(),
		})
		return
	}
	logs := result.Entries

	// 按时间段聚合
	buckets := make(map[string]int)
	for _, log := range logs {
		bucketTime := log.Timestamp.Truncate(intervalDuration)
		key := bucketTime.Format("2006-01-02 15:04")
		buckets[key]++
	}

	// 生成完整的时间序列
	var trend []gin.H
	for t := startTime.Truncate(intervalDuration); t.Before(now); t = t.Add(intervalDuration) {
		key := t.Format("2006-01-02 15:04")
		count := buckets[key]
		trend = append(trend, gin.H{
			"time":  key,
			"count": count,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data": gin.H{
			"time_range": timeRange,
			"interval":   interval,
			"trend":      trend,
		},
	})
}
