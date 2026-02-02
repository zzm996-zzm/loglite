package sender

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ============================================================
// 模式 2: Balanced - 双缓冲 + 定期快照
// ============================================================
//
// 架构图:
//
//   应用层
//     │
//     ▼
//   ┌─────────────────────────────────────────────────────────┐
//   │                    Double Buffer                         │
//   │  ┌──────────────────┐    ┌──────────────────┐           │
//   │  │   Active Buffer  │    │  Standby Buffer  │           │
//   │  │   (写入目标)      │    │  (发送中/空闲)    │           │
//   │  │                  │    │                  │           │
//   │  │  [log][log][log] │    │  [log][log]      │ ──► HTTP  │
//   │  │        ▲         │    │                  │           │
//   │  │      write       │    │                  │           │
//   │  └──────────────────┘    └──────────────────┘           │
//   │           │                       ▲                     │
//   │           └───── swap (原子切换) ──┘                     │
//   │                                                         │
//   │  ┌──────────────────────────────────────────┐          │
//   │  │           Snapshot (定期快照)             │          │
//   │  │  每 5s 将 Active Buffer 写入磁盘          │          │
//   │  │  崩溃后可从快照恢复                       │          │
//   │  └──────────────────────────────────────────┘          │
//   └─────────────────────────────────────────────────────────┘
//                       │
//                       ▼ (发送失败)
//              ┌────────────────┐
//              │  Fallback File │
//              │  (降级文件)     │
//              └────────────────┘
//
// 特点：
// - 双缓冲切换，写入和发送互不阻塞
// - 定期快照，崩溃最多丢失一个快照周期
// - 发送失败降级到本地文件

// DoubleBuffer 双缓冲结构
//
// 设计思路：
// - front: 前台缓冲区，接收新写入
// - back:  后台缓冲区，用于发送
// - Swap() 时交换两个缓冲区的角色
//
// 示意图：
//
//	Write() ──► [front] ◄──swap──► [back] ──► Send()
type DoubleBuffer struct {
	front []*LogEntry // 前台：正在接收写入
	back  []*LogEntry // 后台：等待/正在发送
	cap   int         // 缓冲区容量
	mu    sync.Mutex
}

// NewDoubleBuffer 创建双缓冲
func NewDoubleBuffer(size int) *DoubleBuffer {
	return &DoubleBuffer{
		front: make([]*LogEntry, 0, size),
		back:  make([]*LogEntry, 0, size),
		cap:   size,
	}
}

// Write 写入前台缓冲区
// 返回 true 表示 buffer 满了，需要触发 swap
func (db *DoubleBuffer) Write(entry *LogEntry) bool {
	db.mu.Lock()
	defer db.mu.Unlock()

	db.front = append(db.front, entry)
	return len(db.front) >= db.cap
}

// Swap 交换前后台，返回后台数据用于发送
//
// 操作：front 和 back 互换
// 返回：原 front 的数据（现在是 back）
func (db *DoubleBuffer) Swap() []*LogEntry {
	db.mu.Lock()
	defer db.mu.Unlock()

	db.front, db.back = db.back, db.front

	// 返回 back 的数据（就是刚才的 front）
	// 复制一份，因为 back 会被清空重用
	data := make([]*LogEntry, len(db.back))
	copy(data, db.back)

	// 清空 back，为下次 swap 做准备
	db.back = db.back[:0]

	return data
}

// GetActive 获取前台缓冲区的数据（用于快照，不清空）
func (db *DoubleBuffer) GetActive() []*LogEntry {
	db.mu.Lock()
	defer db.mu.Unlock()

	data := make([]*LogEntry, len(db.front))
	copy(data, db.front)
	return data
}

// BalancedSender 平衡模式发送器
type BalancedSender struct {
	BaseSender // 嵌入基础发送器

	cfg    Config
	client *HTTPClient

	// 双缓冲
	doubleBuffer *DoubleBuffer

	// 统计
	stats   SenderStats
	statsMu sync.RWMutex

	// 快照
	snapshotFile string

	// 降级文件
	fallbackFile *os.File

	// 控制
	stopCh    chan struct{}
	stoppedCh chan struct{}
	swapCh    chan struct{} // 触发 swap 的信号
}

// NewBalancedSender 创建平衡模式发送器
func NewBalancedSender(cfg Config) *BalancedSender {
	// 确保目录存在
	os.MkdirAll(cfg.SnapshotDir, 0755)

	s := &BalancedSender{
		cfg:          cfg,
		client:       NewHTTPClient(cfg.Endpoint, cfg.Timeout),
		doubleBuffer: NewDoubleBuffer(cfg.BatchSize * 2), // 双缓冲大小
		snapshotFile: filepath.Join(cfg.SnapshotDir, "buffer.snapshot"),
		stopCh:       make(chan struct{}),
		stoppedCh:    make(chan struct{}),
		swapCh:       make(chan struct{}, 1),
	}

	// 初始化基础发送器
	s.InitBase(cfg.Service, s)

	// 恢复快照
	s.recoverFromSnapshot()

	// 恢复降级文件（启动时尝试重发）
	s.recoverFromFallback()

	// 启动后台协程
	go s.backgroundLoop()
	go s.snapshotLoop()
	go s.fallbackRetryLoop() // 定期重试降级文件

	return s
}

// Send 发送日志
func (s *BalancedSender) Send(entry *LogEntry) error {
	// 写入双缓冲
	isFull := s.doubleBuffer.Write(entry)

	// 如果满了，触发 swap
	if isFull {
		select {
		case s.swapCh <- struct{}{}:
		default:
			// swapCh 已有信号，忽略
		}
	}

	return nil
}

// backgroundLoop 后台发送循环
// 提示：
// 1. 监听 swapCh 或定时器
// 2. 调用 doubleBuffer.Swap() 获取数据
// 3. 发送数据
// 4. 失败时写入降级文件
func (s *BalancedSender) backgroundLoop() {
	defer close(s.stoppedCh)

	ticker := time.NewTicker(s.cfg.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			s.finalFlush()
			return
		case <-s.swapCh:
			s.swapAndSend()
		case <-ticker.C:
			s.swapAndSend()
		}
	}
}

// swapAndSend 切换并发送
func (s *BalancedSender) swapAndSend() {

	entries := s.doubleBuffer.Swap()
	if len(entries) == 0 {
		return
	}

	// 发送
	if err := s.client.SendBatch(entries); err != nil {
		// 发送失败，写入降级文件
		s.writeToFallback(entries)
		s.statsMu.Lock()
		s.stats.TotalFailed += int64(len(entries))
		s.stats.LastError = err.Error()
		s.stats.LastErrorTime = time.Now()
		s.statsMu.Unlock()
	} else {
		s.statsMu.Lock()
		s.stats.TotalSent += int64(len(entries))
		s.stats.LastSendTime = time.Now()
		s.statsMu.Unlock()
	}
}

// snapshotLoop 快照循环

// 1. 定时将当前 buffer 写入快照文件
// 2. 使用原子写入（先写临时文件，再重命名）
func (s *BalancedSender) snapshotLoop() {
	ticker := time.NewTicker(s.cfg.SnapshotInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.saveSnapshot()
		}
	}
}

// saveSnapshot 保存快照
func (s *BalancedSender) saveSnapshot() {

	//
	data := s.doubleBuffer.GetActive()
	if len(data) == 0 {
		return
	}

	// 序列化
	content, err := json.Marshal(data)
	if err != nil {
		return
	}

	// 原子写入（写临时文件后重命名）
	tmpFile := s.snapshotFile + ".tmp"
	if err := os.WriteFile(tmpFile, content, 0644); err != nil {
		return
	}
	os.Rename(tmpFile, s.snapshotFile)
}

// recoverFromSnapshot 从快照恢复
func (s *BalancedSender) recoverFromSnapshot() {

	content, err := os.ReadFile(s.snapshotFile)
	if err != nil {
		return // 没有快照文件
	}

	var entries []*LogEntry
	if err := json.Unmarshal(content, &entries); err != nil {
		return
	}

	// 重新写入缓冲区
	for _, entry := range entries {
		s.doubleBuffer.Write(entry)
	}

	// 删除快照文件
	os.Remove(s.snapshotFile)
}

// writeToFallback 写入降级文件
func (s *BalancedSender) writeToFallback(entries []*LogEntry) {
	if s.cfg.FallbackFile == "" {
		return
	}

	file, err := os.OpenFile(s.cfg.FallbackFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer file.Close()

	for _, entry := range entries {
		line, _ := json.Marshal(entry)
		file.Write(append(line, '\n'))
	}
}

// recoverFromFallback 从降级文件恢复并重发
// 启动时和定期调用
func (s *BalancedSender) recoverFromFallback() {
	if s.cfg.FallbackFile == "" {
		return
	}

	// 检查文件是否存在
	content, err := os.ReadFile(s.cfg.FallbackFile)
	if err != nil {
		return // 文件不存在或读取失败
	}

	if len(content) == 0 {
		return
	}

	// 解析每一行
	var entries []*LogEntry
	lines := splitLines(content)
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		var entry LogEntry
		if err := json.Unmarshal(line, &entry); err == nil {
			entries = append(entries, &entry)
		}
	}

	if len(entries) == 0 {
		return
	}

	// 尝试重新发送
	if err := s.client.SendBatch(entries); err != nil {
		// 发送失败，保留文件等待下次重试
		return
	}

	// 发送成功，删除降级文件
	os.Remove(s.cfg.FallbackFile)

	// 更新统计
	s.statsMu.Lock()
	s.stats.TotalSent += int64(len(entries))
	s.stats.LastSendTime = time.Now()
	s.statsMu.Unlock()
}

// splitLines 按行分割字节数组
func splitLines(data []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i, b := range data {
		if b == '\n' {
			if i > start {
				lines = append(lines, data[start:i])
			}
			start = i + 1
		}
	}
	// 处理最后一行（没有换行符的情况）
	if start < len(data) {
		lines = append(lines, data[start:])
	}
	return lines
}

// fallbackRetryLoop 降级文件重试循环
func (s *BalancedSender) fallbackRetryLoop() {
	// 每 30 秒尝试重发降级文件
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.recoverFromFallback()
		}
	}
}

// finalFlush 最终刷新
func (s *BalancedSender) finalFlush() {
	// 保存最后的快照
	s.saveSnapshot()
	// 发送剩余数据
	s.swapAndSend()
}

// Flush 强制刷新
func (s *BalancedSender) Flush() error {
	s.swapAndSend()
	return nil
}

// Close 关闭发送器
func (s *BalancedSender) Close() error {
	close(s.stopCh)
	<-s.stoppedCh
	return nil
}

// Stats 获取统计
func (s *BalancedSender) Stats() *SenderStats {
	s.statsMu.RLock()
	defer s.statsMu.RUnlock()

	stats := s.stats
	return &stats
}

// 确保实现接口
var _ = json.Marshal // 避免 import 未使用警告
