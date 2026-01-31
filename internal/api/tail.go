package api

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/loglite/loglite/internal/model"
)

// TailHub 实时日志推送中心
type TailHub struct {
	clients    map[string]*TailClient
	register   chan *TailClient
	unregister chan *TailClient
	broadcast  chan *model.LogEntry
	mu         sync.RWMutex
}

// TailClient 订阅客户端
type TailClient struct {
	ID      string
	Service string
	Level   string
	Send    chan *model.LogEntry
}

// NewTailHub 创建推送中心
func NewTailHub() *TailHub {
	hub := &TailHub{
		clients:    make(map[string]*TailClient),
		register:   make(chan *TailClient),
		unregister: make(chan *TailClient),
		broadcast:  make(chan *model.LogEntry, 1000),
	}
	go hub.run()
	return hub
}

// run 运行推送中心
func (h *TailHub) run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client.ID] = client
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client.ID]; ok {
				delete(h.clients, client.ID)
				close(client.Send)
			}
			h.mu.Unlock()

		case entry := <-h.broadcast:
			h.mu.RLock()
			for _, client := range h.clients {
				// 过滤
				if client.Service != "" && client.Service != entry.Service {
					continue
				}
				if client.Level != "" && client.Level != entry.Level {
					continue
				}

				select {
				case client.Send <- entry:
				default:
					// 客户端处理不过来，跳过
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Broadcast 广播日志
func (h *TailHub) Broadcast(entry *model.LogEntry) {
	select {
	case h.broadcast <- entry:
	default:
		// 缓冲区满，丢弃
	}
}

// ClientCount 获取客户端数量
func (h *TailHub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// TailHandler 实时日志处理器
type TailHandler struct {
	hub *TailHub
}

// NewTailHandler 创建处理器
func NewTailHandler(hub *TailHub) *TailHandler {
	return &TailHandler{hub: hub}
}

// HandleTail 处理 SSE 连接
// GET /api/v1/tail
func (th *TailHandler) HandleTail(c *gin.Context) {
	// 设置 SSE 头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	// 创建客户端
	client := &TailClient{
		ID:      fmt.Sprintf("%d", time.Now().UnixNano()),
		Service: c.Query("service"),
		Level:   c.Query("level"),
		Send:    make(chan *model.LogEntry, 100),
	}

	// 注册
	th.hub.register <- client

	// 发送连接成功消息
	c.SSEvent("connected", gin.H{
		"message":   "connected to tail stream",
		"client_id": client.ID,
		"filter": gin.H{
			"service": client.Service,
			"level":   client.Level,
		},
	})
	c.Writer.Flush()

	// 心跳
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// 监听
	clientGone := c.Request.Context().Done()
	for {
		select {
		case <-clientGone:
			th.hub.unregister <- client
			return

		case entry, ok := <-client.Send:
			if !ok {
				return
			}
			data, _ := json.Marshal(entry)
			c.SSEvent("log", string(data))
			c.Writer.Flush()

		case <-ticker.C:
			c.SSEvent("ping", gin.H{"time": time.Now().Format("2006-01-02 15:04:05")})
			c.Writer.Flush()
		}
	}
}
