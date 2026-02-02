package sender

import (
	"sync"
	"sync/atomic"
	"time"
)

// ============================================================
// 模式 1: BestEffort - 最基本的 Buffer -> Send
// ============================================================
//
// 架构图:
//
//   应用层
//     │
//     ▼
//   ┌─────────────────────────────────────────────┐
//   │              Ring Buffer (无锁)              │
//   │  ┌───┬───┬───┬───┬───┬───┬───┬───┬───┬───┐ │
//   │  │ 0 │ 1 │ 2 │ 3 │ 4 │ 5 │ 6 │ 7 │ 8 │ 9 │ │
//   │  └───┴───┴───┴───┴───┴───┴───┴───┴───┴───┘ │
//   │       ▲                       ▲             │
//   │     write                   read            │
//   └─────────────────────────────────────────────┘
//                       │
//                       ▼ (定时/满批量触发)
//              ┌────────────────┐
//              │  HTTP Client   │
//              │   SendBatch    │
//              └────────────────┘
//                       │
//                       ▼
//              ┌────────────────┐
//              │  LogLite 服务端 │
//              └────────────────┘
//
// 特点：
// - 缓冲区满直接丢弃（不阻塞应用）
// - 发送失败不重试
// - 最高性能，适合容忍丢失的场景

// BestEffortSender 最高性能模式发送器
type BestEffortSender struct {
	BaseSender // 嵌入基础发送器

	cfg    Config
	client *HTTPClient

	// Ring Buffer - 三指针设计
	buffer    []*LogEntry
	writePos  int64 // 写指针：下一个可写位置（写入前 CAS 抢占）
	commitPos int64 // 提交指针：已完成写入的位置（数据可读）
	readPos   int64 // 读指针：下一个可读位置
	mask      int64 // bufferSize - 1, 用于位运算取模

	// 统计
	stats SenderStats

	// 控制
	stopCh    chan struct{}
	stoppedCh chan struct{}
	mu        sync.RWMutex
}

// NewBestEffortSender 创建 BestEffort 发送器
func NewBestEffortSender(cfg Config) *BestEffortSender {
	// 确保 bufferSize 是 2 的幂（方便位运算取模）
	size := nextPowerOfTwo(cfg.BufferSize)

	s := &BestEffortSender{
		cfg:       cfg,
		client:    NewHTTPClient(cfg.Endpoint, cfg.Timeout),
		buffer:    make([]*LogEntry, size),
		mask:      int64(size - 1),
		stopCh:    make(chan struct{}),
		stoppedCh: make(chan struct{}),
	}

	// 初始化基础发送器
	s.InitBase(cfg.Service, s)

	// 启动后台发送协程
	go s.backgroundLoop()

	return s
}

// Send 发送日志（非阻塞）
//
// 并发安全设计：
// 1. 使用 CAS 抢占写位置（避免两个 goroutine 写同一位置）
// 2. 写入完成后，按顺序更新 commitPos（保证读取时数据已写入）
//
// 三指针含义：
// - writePos:  下一个可写位置（CAS 抢占）
// - commitPos: 数据已写入的位置（顺序提交）
// - readPos:   下一个可读位置
//
// 示例状态：
// writePos=8, commitPos=5, readPos=3
// 表示：位置 3,4 可读，位置 5,6,7 正在写入，位置 8 可抢占
func (s *BestEffortSender) Send(entry *LogEntry) error {
	for {
		// 1. 读取当前指针
		writePos := atomic.LoadInt64(&s.writePos)
		readPos := atomic.LoadInt64(&s.readPos)

		// 2. 检查缓冲区是否满
		if writePos-readPos >= int64(len(s.buffer)) {
			atomic.AddInt64(&s.stats.TotalDropped, 1)
			return ErrBufferFull
		}

		// 3. CAS 抢占写位置
		// 如果 writePos 还是原来的值，就 +1 抢占这个位置
		// 如果失败说明被其他 goroutine 抢走了，重试
		if atomic.CompareAndSwapInt64(&s.writePos, writePos, writePos+1) {
			// 4. 抢占成功！写入数据
			idx := writePos & s.mask
			s.buffer[idx] = entry

			// 5. 等待前面的写入都完成，然后提交
			// commitPos 必须顺序递增，保证读取时数据连续
			for {
				if atomic.CompareAndSwapInt64(&s.commitPos, writePos, writePos+1) {
					// 提交成功
					break
				}
				// 前面还有未提交的，等一下
				// （实际中很少 spin，因为写入很快）
			}
			return nil
		}
		// CAS 失败，重试
	}
}

// backgroundLoop 后台发送循环
// 1. 使用 ticker 定时触发
// 2. 从 buffer 读取一批数据
// 3. 调用 client.SendBatch
// 4. 更新 readPos 和统计
func (s *BestEffortSender) backgroundLoop() {
	defer close(s.stoppedCh)

	ticker := time.NewTicker(s.cfg.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			// 退出前尝试发送剩余数据
			s.flush()
			return
		case <-ticker.C:
			s.flush()
		}
	}
}

// flush 刷新缓冲区
//
// 关键：读取 commitPos 而不是 writePos！
// commitPos 表示数据已经写入完成，可以安全读取
func (s *BestEffortSender) flush() {
	for {
		readPos := atomic.LoadInt64(&s.readPos)
		commitPos := atomic.LoadInt64(&s.commitPos) // 注意：用 commitPos！

		// 只读取已提交的数据
		if readPos >= commitPos {
			return // 没有可读数据
		}

		// 计算本次要处理的数量
		available := commitPos - readPos
		batchSize := available
		if batchSize > int64(s.cfg.BatchSize) {
			batchSize = int64(s.cfg.BatchSize)
		}

		newReadPos := readPos + batchSize

		// CAS 抢占读取权
		if atomic.CompareAndSwapInt64(&s.readPos, readPos, newReadPos) {
			// 成功！处理这个批次
			s.processBatch(readPos, newReadPos)
			return
		}
		// 被其他 goroutine 抢走了，重试
	}
}

// 如果处理数据赶不上writePos，会导致数据丢失
func (s *BestEffortSender) processBatch(start, end int64) {
	// 处理从start到end-1的数据
	// 其他goroutine不会处理这些数据，因为readPos已经>=start
	batchSize := end - start

	// 收集数据
	batch := make([]*LogEntry, 0, batchSize)
	for i := int64(0); i < batchSize; i++ {
		idx := (start + i) & s.mask
		if entry := s.buffer[idx]; entry != nil {
			batch = append(batch, entry)
			s.buffer[idx] = nil
		}
	}

	// 发送（如果失败，数据已丢失，因为readPos已更新）
	if err := s.client.SendBatch(batch); err != nil {
		atomic.AddInt64(&s.stats.TotalFailed, int64(len(batch)))
	} else {
		atomic.AddInt64(&s.stats.TotalSent, int64(len(batch)))
	}
}

// Flush 强制刷新
func (s *BestEffortSender) Flush() error {
	s.flush()
	return nil
}

// Close 关闭发送器
func (s *BestEffortSender) Close() error {
	close(s.stopCh)
	<-s.stoppedCh
	return nil
}

// Stats 获取统计
func (s *BestEffortSender) Stats() *SenderStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := s.stats
	stats.BufferSize = len(s.buffer)
	stats.PendingCount = int(atomic.LoadInt64(&s.writePos) - atomic.LoadInt64(&s.readPos))
	return &stats
}

// ============================================================
// 工具函数
// ============================================================

// nextPowerOfTwo 返回大于等于 n 的最小 2 的幂
func nextPowerOfTwo(n int) int {
	if n <= 0 {
		return 1
	}
	n--
	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16
	return n + 1
}
