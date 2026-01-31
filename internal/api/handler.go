package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/loglite/loglite/internal/model"
	"github.com/loglite/loglite/internal/storage"
)

// Handler API 处理器
type Handler struct {
	store *storage.BadgerStore
}

// NewHandler 创建处理器
func NewHandler(store *storage.BadgerStore) *Handler {
	return &Handler{store: store}
}

// ReceiveLog 接收单条日志
// POST /api/v1/logs
func (h *Handler) ReceiveLog(c *gin.Context) {
	var entry model.LogEntry
	if err := c.ShouldBindJSON(&entry); err != nil {
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
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	// 设置接收时间
	entry.ReceivedAt = time.Now()

	// 保存
	if err := h.store.Save(&entry); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "failed to save log",
			"error":   err.Error(),
		})
		return
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
	var req struct {
		Logs []*model.LogEntry `json:"logs"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "invalid request body",
			"error":   err.Error(),
		})
		return
	}

	if len(req.Logs) == 0 {
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

	// 批量保存
	if err := h.store.SaveBatch(req.Logs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "failed to save logs",
			"error":   err.Error(),
		})
		return
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
	params := storage.QueryParams{
		Service: c.Query("service"),
		Level:   c.Query("level"),
		Keyword: c.Query("q"),
	}

	// 解析时间范围
	if start := c.Query("start"); start != "" {
		if t, err := time.Parse(time.RFC3339, start); err == nil {
			params.StartTime = t
		}
	}
	if end := c.Query("end"); end != "" {
		if t, err := time.Parse(time.RFC3339, end); err == nil {
			params.EndTime = t
		}
	}

	// 解析分页
	if limit := c.Query("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil && l > 0 {
			params.Limit = l
		}
	}
	if params.Limit == 0 {
		params.Limit = 100
	}
	if params.Limit > 1000 {
		params.Limit = 1000
	}

	if offset := c.Query("offset"); offset != "" {
		if o, err := strconv.Atoi(offset); err == nil && o >= 0 {
			params.Offset = o
		}
	}

	// 查询
	logs, total, err := h.store.Query(params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "query failed",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data": gin.H{
			"logs":   logs,
			"total":  total,
			"limit":  params.Limit,
			"offset": params.Offset,
		},
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

	entry, err := h.store.GetByID(id)
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
	stats, err := h.store.GetStats()
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
		"time":   time.Now().Format(time.RFC3339),
	})
}
