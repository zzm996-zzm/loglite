package api

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/loglite/loglite/internal/model"
	"github.com/loglite/loglite/internal/query"
	"github.com/loglite/loglite/internal/storage"
)

// Handler API 处理器
type Handler struct {
	store storage.Store
}

// NewHandler 创建处理器
func NewHandler(store storage.Store) *Handler {
	return &Handler{store: store}
}

// ReceiveLog 接收单条日志
// POST /api/v1/logs
func (h *Handler) ReceiveLog(c *gin.Context) {
	startTime := time.Now()

	var entry model.LogEntry
	if err := c.ShouldBindJSON(&entry); err != nil {
		if m := GetMonitor(); m != nil {
			m.IncrRequestError()
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "invalid request body",
			"error":   err.Error(),
		})
		return
	}

	// 生成 ID
	if entry.ID == "" {
		entry.ID = uuid.New().String()
	}

	// 验证
	if err := entry.Validate(); err != nil {
		if m := GetMonitor(); m != nil {
			m.IncrRequestError()
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	// 设置接收时间
	entry.ReceivedAt = time.Now()

	// 记录接收延迟
	ingestLatency := time.Since(startTime)
	if m := GetMonitor(); m != nil {
		m.IncrLogReceived(1)
		m.RecordIngestLatency(ingestLatency)
	}

	// 保存
	saveStart := time.Now()
	if err := h.store.Write(context.Background(), &entry); err != nil {
		if m := GetMonitor(); m != nil {
			m.IncrRequestError()
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "failed to save log",
			"error":   err.Error(),
		})
		return
	}

	// 记录存储延迟
	if m := GetMonitor(); m != nil {
		m.IncrLogStored(1)
		m.RecordStorageLatency(time.Since(saveStart))
	}

	// 广播到实时流
	if hub := GetTailHub(); hub != nil {
		hub.Broadcast(&entry)
	}

	if m := GetMonitor(); m != nil {
		m.IncrRequestTotal()
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data": gin.H{
			"id": entry.ID,
		},
	})
}

// ReceiveBatch 批量接收日志
// POST /api/v1/logs/batch
func (h *Handler) ReceiveBatch(c *gin.Context) {
	startTime := time.Now()

	var req struct {
		Logs []*model.LogEntry `json:"logs"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		if m := GetMonitor(); m != nil {
			m.IncrRequestError()
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "invalid request body",
			"error":   err.Error(),
		})
		return
	}

	if len(req.Logs) == 0 {
		if m := GetMonitor(); m != nil {
			m.IncrRequestError()
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "logs array is empty",
		})
		return
	}

	// 处理每条日志
	now := time.Now()
	for _, entry := range req.Logs {
		if entry.ID == "" {
			entry.ID = uuid.New().String()
		}
		entry.Validate()
		entry.ReceivedAt = now
	}

	// 记录接收延迟
	ingestLatency := time.Since(startTime)
	if m := GetMonitor(); m != nil {
		m.IncrLogReceived(int64(len(req.Logs)))
		m.RecordIngestLatency(ingestLatency)
	}

	// 批量保存
	saveStart := time.Now()
	if err := h.store.WriteMany(context.Background(), req.Logs); err != nil {
		if m := GetMonitor(); m != nil {
			m.IncrRequestError()
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "failed to save logs",
			"error":   err.Error(),
		})
		return
	}

	// 记录存储延迟
	if m := GetMonitor(); m != nil {
		m.IncrLogStored(int64(len(req.Logs)))
		m.RecordStorageLatency(time.Since(saveStart))
	}

	// 广播到实时流
	if hub := GetTailHub(); hub != nil {
		for _, entry := range req.Logs {
			hub.Broadcast(entry)
		}
	}

	if m := GetMonitor(); m != nil {
		m.IncrRequestTotal()
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data": gin.H{
			"count": len(req.Logs),
		},
	})
}

// QueryLogs 查询日志
// GET /api/v1/query
func (h *Handler) QueryLogs(c *gin.Context) {
	queryStart := time.Now()

	var qry storage.Query

	// 检查是否是自然语言查询
	q := c.Query("q")
	if q != "" && c.Query("service") == "" && c.Query("level") == "" && c.Query("start") == "" {
		// 自然语言查询
		parser := query.NewNaturalQueryParser()
		parsedQuery := parser.Parse(q)
		// 如果解析后没有关键词，保留原始查询
		if len(parsedQuery.Keywords) == 0 && len(parsedQuery.Levels) == 0 && len(parsedQuery.Services) == 0 {
			parsedQuery.Keywords = []string{q}
		}
		qry = *parsedQuery
	} else {
		// 结构化查询
		if service := c.Query("service"); service != "" {
			qry.Services = []string{service}
		}
		if level := c.Query("level"); level != "" {
			qry.Levels = []string{level}
		}
		if q != "" {
			qry.Keywords = []string{q}
		}

		// 解析时间范围（支持多种格式）
		if start := c.Query("start"); start != "" {
			qry.From = parseTime(start)
		}
		if end := c.Query("end"); end != "" {
			qry.To = parseTime(end)
		}
	}

	// 解析分页
	if limit := c.Query("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil && l > 0 {
			qry.Limit = l
		}
	}
	if qry.Limit == 0 {
		qry.Limit = 100
	}
	if qry.Limit > 1000 {
		qry.Limit = 1000
	}

	if offset := c.Query("offset"); offset != "" {
		if o, err := strconv.Atoi(offset); err == nil && o >= 0 {
			qry.Offset = o
		}
	}

	// 查询
	result, err := h.store.Query(context.Background(), &qry)
	if err != nil {
		if m := GetMonitor(); m != nil {
			m.IncrRequestError()
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "query failed",
			"error":   err.Error(),
		})
		return
	}

	// 记录查询延迟
	if m := GetMonitor(); m != nil {
		m.RecordQueryLatency(time.Since(queryStart))
		m.IncrRequestTotal()
	}

	// 构建响应（包含解析后的查询参数，方便调试）
	response := gin.H{
		"logs":   result.Entries,
		"total":  result.Total,
		"limit":  qry.Limit,
		"offset": qry.Offset,
	}

	// 如果是自然语言查询，返回解析结果
	if c.Query("debug") == "1" || c.Query("debug") == "true" {
		level := ""
		if len(qry.Levels) > 0 {
			level = qry.Levels[0]
		}
		service := ""
		if len(qry.Services) > 0 {
			service = qry.Services[0]
		}
		keyword := ""
		if len(qry.Keywords) > 0 {
			keyword = qry.Keywords[0]
		}
		response["parsed"] = gin.H{
			"level":      level,
			"service":    service,
			"keyword":    keyword,
			"start_time": qry.From.Format("2006-01-02 15:04:05"),
			"end_time":   qry.To.Format("2006-01-02 15:04:05"),
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    response,
	})
}

// GetLog 获取单条日志
// GET /api/v1/logs/:id
func (h *Handler) GetLog(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "id is required",
		})
		return
	}

	entry, err := h.store.Get(context.Background(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "log not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    entry,
	})
}

// GetStats 获取统计信息
// GET /api/v1/stats
func (h *Handler) GetStats(c *gin.Context) {
	stats, err := h.store.Stats(context.Background())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "failed to get stats",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    stats,
	})
}

// Health 健康检查
// GET /health
func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"time":   time.Now().Format("2006-01-02 15:04:05"),
	})
}

// parseTime 解析多种时间格式
func parseTime(s string) time.Time {
	// 支持的时间格式
	formats := []string{
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
		"2006-1-2 15:04:05",
		"2006-1-2 15:04",
		"2006-1-2",
		"2006/01/02 15:04:05",
		"2006/01/02",
		"2006/1/2",
		time.RFC3339,
	}

	for _, format := range formats {
		if t, err := time.ParseInLocation(format, s, time.Local); err == nil {
			return t
		}
	}

	return time.Time{}
}
