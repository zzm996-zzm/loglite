package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/google/uuid"
)

// LogEntry 日志条目
type LogEntry struct {
	ID         string                 `json:"id"`
	Timestamp  time.Time              `json:"timestamp"`
	Message    string                 `json:"message"`
	Level      string                 `json:"level"`
	Service    string                 `json:"service"`
	TraceID    string                 `json:"trace_id,omitempty"`
	SpanID     string                 `json:"span_id,omitempty"`
	UserID     string                 `json:"user_id,omitempty"`
	RequestID  string                 `json:"request_id,omitempty"`
	IP         string                 `json:"ip,omitempty"`
	Caller     string                 `json:"caller,omitempty"`
	Function   string                 `json:"function,omitempty"`
	Package    string                 `json:"package,omitempty"`
	StackTrace string                 `json:"stack_trace,omitempty"`
	StackHash  string                 `json:"stack_hash,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// Client LogLite 客户端
type Client struct {
	endpoint      string
	service       string
	httpClient    *http.Client
	buffer        chan *LogEntry
	batchSize     int
	flushInterval time.Duration
	callerDepth   int
	enableCaller  bool
	enableStack   bool
	wg            sync.WaitGroup
	closed        bool
	mu            sync.RWMutex

	// 可靠性相关
	fallbackFile string
	fallbackMu   sync.Mutex
}

// Option 客户端选项
type Option func(*Client)

// WithEndpoint 设置服务端地址
func WithEndpoint(endpoint string) Option {
	return func(c *Client) {
		c.endpoint = endpoint
	}
}

// WithBatchSize 设置批量大小
func WithBatchSize(size int) Option {
	return func(c *Client) {
		c.batchSize = size
	}
}

// WithFlushInterval 设置刷新间隔
func WithFlushInterval(d time.Duration) Option {
	return func(c *Client) {
		c.flushInterval = d
	}
}

// WithCaller 启用调用位置采集
func WithCaller(enable bool) Option {
	return func(c *Client) {
		c.enableCaller = enable
	}
}

// WithStackTrace 启用堆栈采集（仅 error 级别）
func WithStackTrace(enable bool) Option {
	return func(c *Client) {
		c.enableStack = enable
	}
}

// WithFallbackFile 设置降级文件
func WithFallbackFile(path string) Option {
	return func(c *Client) {
		c.fallbackFile = path
	}
}

// WithHTTPTimeout 设置 HTTP 超时
func WithHTTPTimeout(d time.Duration) Option {
	return func(c *Client) {
		c.httpClient.Timeout = d
	}
}

// Init 初始化客户端
func Init(service string, opts ...Option) *Client {
	c := &Client{
		endpoint:      "http://localhost:8080",
		service:       service,
		httpClient:    &http.Client{Timeout: 5 * time.Second},
		buffer:        make(chan *LogEntry, 10000),
		batchSize:     100,
		flushInterval: 100 * time.Millisecond,
		callerDepth:   3,
		enableCaller:  true,
		enableStack:   true,
	}

	for _, opt := range opts {
		opt(c)
	}

	// 启动后台发送协程
	c.wg.Add(1)
	go c.backgroundFlush()

	return c
}

// Debug 记录 debug 日志
func (c *Client) Debug(message string, keyvals ...interface{}) {
	c.log("debug", message, keyvals...)
}

// Info 记录 info 日志
func (c *Client) Info(message string, keyvals ...interface{}) {
	c.log("info", message, keyvals...)
}

// Warn 记录 warn 日志
func (c *Client) Warn(message string, keyvals ...interface{}) {
	c.log("warn", message, keyvals...)
}

// Error 记录 error 日志
func (c *Client) Error(message string, keyvals ...interface{}) {
	c.log("error", message, keyvals...)
}

// With 创建带有固定字段的 Logger
func (c *Client) With(fields map[string]interface{}) *Logger {
	return &Logger{
		client: c,
		fields: fields,
	}
}

// WithContext 从 Context 创建 Logger
func (c *Client) WithContext(ctx context.Context) *Logger {
	l := &Logger{
		client: c,
		fields: make(map[string]interface{}),
	}

	// 从 context 提取常用字段
	if traceID, ok := ctx.Value(TraceIDKey).(string); ok {
		l.fields["trace_id"] = traceID
	}
	if requestID, ok := ctx.Value(RequestIDKey).(string); ok {
		l.fields["request_id"] = requestID
	}
	if userID, ok := ctx.Value(UserIDKey).(string); ok {
		l.fields["user_id"] = userID
	}

	return l
}

// log 内部日志方法
func (c *Client) log(level, message string, keyvals ...interface{}) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return
	}
	c.mu.RUnlock()

	entry := &LogEntry{
		ID:        uuid.New().String(),
		Timestamp: time.Now(),
		Message:   message,
		Level:     level,
		Service:   c.service,
		Metadata:  make(map[string]interface{}),
	}

	// 解析 keyvals
	for i := 0; i < len(keyvals)-1; i += 2 {
		key, ok := keyvals[i].(string)
		if !ok {
			continue
		}
		value := keyvals[i+1]

		// 特殊字段
		switch key {
		case "trace_id":
			entry.TraceID = fmt.Sprintf("%v", value)
		case "request_id":
			entry.RequestID = fmt.Sprintf("%v", value)
		case "user_id":
			entry.UserID = fmt.Sprintf("%v", value)
		case "ip":
			entry.IP = fmt.Sprintf("%v", value)
		default:
			entry.Metadata[key] = value
		}
	}

	// 采集调用位置
	if c.enableCaller {
		if pc, file, line, ok := runtime.Caller(c.callerDepth); ok {
			entry.Caller = fmt.Sprintf("%s:%d", filepath.Base(file), line)
			if fn := runtime.FuncForPC(pc); fn != nil {
				entry.Function = filepath.Base(fn.Name())
			}
		}
	}

	// error 级别采集堆栈
	if level == "error" && c.enableStack {
		buf := make([]byte, 4096)
		n := runtime.Stack(buf, false)
		entry.StackTrace = string(buf[:n])
	}

	// 写入缓冲区
	select {
	case c.buffer <- entry:
		// 成功
	default:
		// 缓冲区满，写降级文件
		c.writeFallback(entry)
	}
}

// backgroundFlush 后台刷新协程
func (c *Client) backgroundFlush() {
	defer c.wg.Done()

	ticker := time.NewTicker(c.flushInterval)
	defer ticker.Stop()

	batch := make([]*LogEntry, 0, c.batchSize)

	for {
		select {
		case entry, ok := <-c.buffer:
			if !ok {
				// channel 关闭，发送剩余日志
				if len(batch) > 0 {
					c.sendBatch(batch)
				}
				return
			}

			batch = append(batch, entry)
			if len(batch) >= c.batchSize {
				c.sendBatch(batch)
				batch = make([]*LogEntry, 0, c.batchSize)
			}

		case <-ticker.C:
			if len(batch) > 0 {
				c.sendBatch(batch)
				batch = make([]*LogEntry, 0, c.batchSize)
			}
		}
	}
}

// sendBatch 批量发送日志
func (c *Client) sendBatch(entries []*LogEntry) {
	if len(entries) == 0 {
		return
	}

	body, err := json.Marshal(map[string]interface{}{
		"logs": entries,
	})
	if err != nil {
		c.writeFallbackBatch(entries)
		return
	}

	req, err := http.NewRequest("POST", c.endpoint+"/api/v1/logs/batch", bytes.NewReader(body))
	if err != nil {
		c.writeFallbackBatch(entries)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.writeFallbackBatch(entries)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.writeFallbackBatch(entries)
	}
}

// writeFallback 写入降级文件
func (c *Client) writeFallback(entry *LogEntry) {
	if c.fallbackFile == "" {
		return
	}

	c.fallbackMu.Lock()
	defer c.fallbackMu.Unlock()

	f, err := os.OpenFile(c.fallbackFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	data, _ := json.Marshal(entry)
	f.Write(data)
	f.WriteString("\n")
}

// writeFallbackBatch 批量写入降级文件
func (c *Client) writeFallbackBatch(entries []*LogEntry) {
	for _, entry := range entries {
		c.writeFallback(entry)
	}
}

// Close 关闭客户端
func (c *Client) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.mu.Unlock()

	// 关闭 buffer，触发 backgroundFlush 退出
	close(c.buffer)

	// 等待所有日志发送完成（最多 5 秒）
	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("close timeout")
	}
}

// Flush 强制刷新缓冲区
func (c *Client) Flush() {
	// 发送一个同步信号
	// 简单实现：等待一个刷新周期
	time.Sleep(c.flushInterval + 10*time.Millisecond)
}
