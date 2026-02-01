package sender

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ============================================================
// 模式 3: Reliable - WAL (Write-Ahead Log) 模式
// ============================================================
//
// 什么是 WAL？
// WAL = Write-Ahead Log = 预写日志
// 核心思想：先写磁盘，再做其他操作
//
// 工作流程：
//
//   1. 应用写入日志
//        │
//        ▼
//   2. 立即写入 WAL 文件（fsync 落盘）  ◄── 保证不丢失！
//        │
//        ▼
//   3. 返回成功给应用（写入已持久化）
//        │
//        ▼
//   4. 后台异步发送到服务器
//        │
//        ▼
//   5. 发送成功后，标记 WAL 记录为 SENT
//        │
//        ▼
//   6. 定期压缩，删除已发送的记录
//
// 崩溃恢复：
//   程序重启时，扫描 WAL 文件，重发所有未标记 SENT 的记录
//
// 文件结构：
//   wal/
//   ├── current.wal    # 当前 WAL 文件（JSON Lines 格式）
//   └── sent.log       # 已发送的序列号（简化实现）
//
// 每条 WAL 记录格式（JSON Lines）：
//   {"seq":1,"entry":{...日志内容...}}
//   {"seq":2,"entry":{...日志内容...}}
//

// WALRecord WAL 记录
type WALRecord struct {
	Seq   uint64   `json:"seq"`   // 序列号（递增）
	Entry LogEntry `json:"entry"` // 日志内容
}

// ============================================================
// WALWriter - WAL 写入器
// ============================================================

type WALWriter struct {
	walDir   string        // WAL 目录
	walFile  string        // 当前 WAL 文件路径
	sentFile string        // 已发送序列号文件路径
	seqFile  string        // 序列号元数据文件（优化：避免全盘扫描）
	file     *os.File      // 文件句柄
	writer   *bufio.Writer // 带缓冲的写入器
	seq      uint64        // 当前序列号
	mu       sync.Mutex    // 保护写入操作
}

// NewWALWriter 创建 WAL 写入器
func NewWALWriter(walDir string) (*WALWriter, error) {
	// 1. 创建 WAL 目录
	if err := os.MkdirAll(walDir, 0755); err != nil {
		return nil, fmt.Errorf("create wal dir: %w", err)
	}

	w := &WALWriter{
		walDir:   walDir,
		walFile:  filepath.Join(walDir, "current.wal"),
		sentFile: filepath.Join(walDir, "sent.log"),
		seqFile:  filepath.Join(walDir, "seq.meta"), // 元数据文件
	}

	// 2. 打开 WAL 文件（追加模式）
	file, err := os.OpenFile(w.walFile, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("open wal file: %w", err)
	}
	w.file = file
	w.writer = bufio.NewWriter(file)

	// 3. 加载序列号（O(1) 复杂度）
	w.seq = w.loadSeq()

	return w, nil
}

// loadSeq 从元数据文件加载序列号 - O(1) 复杂度
func (w *WALWriter) loadSeq() uint64 {
	data, err := os.ReadFile(w.seqFile)
	if err != nil {
		// 元数据文件不存在，可能是首次启动或旧版本
		// 降级到扫描模式（只执行一次）
		seq := w.scanMaxSeqFallback()
		w.saveSeq(seq) // 保存，下次就不用扫描了
		return seq
	}

	var seq uint64
	if _, err := fmt.Sscanf(string(data), "%d", &seq); err != nil {
		return 0
	}
	return seq
}

// saveSeq 保存序列号到元数据文件
func (w *WALWriter) saveSeq(seq uint64) {
	// 原子写入：先写临时文件，再重命名
	tmpFile := w.seqFile + ".tmp"
	if err := os.WriteFile(tmpFile, []byte(fmt.Sprintf("%d", seq)), 0644); err != nil {
		return
	}
	os.Rename(tmpFile, w.seqFile)
}

// scanMaxSeqFallback 全盘扫描获取最大序列号（仅作为降级方案）
func (w *WALWriter) scanMaxSeqFallback() uint64 {
	file, err := os.Open(w.walFile)
	if err != nil {
		return 0
	}
	defer file.Close()

	var maxSeq uint64
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var record WALRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err == nil {
			if record.Seq > maxSeq {
				maxSeq = record.Seq
			}
		}
	}
	return maxSeq
}

// Write 写入 WAL 记录
// 这是最关键的方法：保证日志先落盘再返回
func (w *WALWriter) Write(entry LogEntry) (uint64, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// 1. 生成序列号
	w.seq++
	seq := w.seq

	// 2. 构造记录
	record := WALRecord{
		Seq:   seq,
		Entry: entry,
	}

	// 3. 序列化
	data, err := json.Marshal(record)
	if err != nil {
		return 0, fmt.Errorf("marshal record: %w", err)
	}

	// 4. 写入文件（追加一行）
	if _, err := w.writer.Write(append(data, '\n')); err != nil {
		return 0, fmt.Errorf("write record: %w", err)
	}

	// 5. 刷新缓冲区到内核
	if err := w.writer.Flush(); err != nil {
		return 0, fmt.Errorf("flush buffer: %w", err)
	}

	// 6. fsync 强制写入磁盘！
	// 这是保证不丢失的关键！
	if err := w.file.Sync(); err != nil {
		return 0, fmt.Errorf("sync file: %w", err)
	}

	// 7. 定期保存序列号（每 100 条保存一次，平衡性能和安全）
	if seq%100 == 0 {
		w.saveSeq(seq)
	}

	return seq, nil
}

// MarkSent 标记序列号为已发送
// 简化实现：将已发送的序列号追加到 sent.log 文件
func (w *WALWriter) MarkSent(seqs []uint64) error {
	if len(seqs) == 0 {
		return nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// 打开 sent.log 文件（追加模式）
	file, err := os.OpenFile(w.sentFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open sent file: %w", err)
	}
	defer file.Close()

	// 写入每个序列号
	for _, seq := range seqs {
		if _, err := fmt.Fprintf(file, "%d\n", seq); err != nil {
			return fmt.Errorf("write seq: %w", err)
		}
	}

	return file.Sync()
}

// loadSentSeqs 加载已发送的序列号集合
func (w *WALWriter) loadSentSeqs() map[uint64]bool {
	sent := make(map[uint64]bool)

	file, err := os.Open(w.sentFile)
	if err != nil {
		return sent // 文件不存在，返回空集合
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var seq uint64
		if _, err := fmt.Sscanf(scanner.Text(), "%d", &seq); err == nil {
			sent[seq] = true
		}
	}

	return sent
}

// ReadPending 读取所有待发送的记录
func (w *WALWriter) ReadPending() ([]WALRecord, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// 1. 加载已发送的序列号
	sentSeqs := w.loadSentSeqs()

	// 2. 读取 WAL 文件
	file, err := os.Open(w.walFile)
	if err != nil {
		return nil, nil // 文件不存在
	}
	defer file.Close()

	// 3. 扫描并过滤
	var pending []WALRecord
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var record WALRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err == nil {
			// 跳过已发送的
			if !sentSeqs[record.Seq] {
				pending = append(pending, record)
			}
		}
	}

	return pending, nil
}

// Compact 压缩 WAL 文件
// 删除已发送的记录，减小文件大小
func (w *WALWriter) Compact() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// 1. 加载已发送的序列号
	sentSeqs := w.loadSentSeqs()
	if len(sentSeqs) == 0 {
		return nil // 没有可压缩的
	}

	// 2. 读取所有记录，过滤掉已发送的
	file, err := os.Open(w.walFile)
	if err != nil {
		return nil
	}

	var pending []WALRecord
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var record WALRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err == nil {
			if !sentSeqs[record.Seq] {
				pending = append(pending, record)
			}
		}
	}
	file.Close()

	// 3. 写入临时文件
	tmpFile := w.walFile + ".tmp"
	newFile, err := os.Create(tmpFile)
	if err != nil {
		return fmt.Errorf("create tmp file: %w", err)
	}

	writer := bufio.NewWriter(newFile)
	for _, record := range pending {
		data, _ := json.Marshal(record)
		writer.Write(append(data, '\n'))
	}
	writer.Flush()
	newFile.Sync()
	newFile.Close()

	// 4. 关闭当前文件，原子替换
	w.writer.Flush()
	w.file.Close()

	if err := os.Rename(tmpFile, w.walFile); err != nil {
		return fmt.Errorf("rename file: %w", err)
	}

	// 5. 重新打开文件
	w.file, err = os.OpenFile(w.walFile, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("reopen file: %w", err)
	}
	w.writer = bufio.NewWriter(w.file)

	// 6. 清空 sent.log（已经没用了）
	os.Remove(w.sentFile)

	return nil
}

// Close 关闭 WAL 写入器
func (w *WALWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// 保存最终序列号
	w.saveSeq(w.seq)

	if w.writer != nil {
		w.writer.Flush()
	}
	if w.file != nil {
		return w.file.Close()
	}
	return nil
}

// ============================================================
// ReliableSender - WAL 模式发送器
// ============================================================

type ReliableSender struct {
	BaseSender // 嵌入基础发送器

	cfg    Config
	client *HTTPClient
	wal    *WALWriter

	// 内存缓冲（用于批量发送）
	pending   []WALRecord
	pendingMu sync.Mutex

	// 统计
	stats   SenderStats
	statsMu sync.RWMutex

	// 控制
	stopCh    chan struct{}
	stoppedCh chan struct{}
}

// NewReliableSender 创建 WAL 模式发送器
func NewReliableSender(cfg Config) *ReliableSender {
	// 初始化 WAL
	wal, err := NewWALWriter(cfg.WALDir)
	if err != nil {
		fmt.Printf("[ReliableSender] WAL init failed: %v\n", err)
		return nil
	}

	s := &ReliableSender{
		cfg:       cfg,
		client:    NewHTTPClient(cfg.Endpoint, cfg.Timeout),
		wal:       wal,
		pending:   make([]WALRecord, 0, cfg.BatchSize),
		stopCh:    make(chan struct{}),
		stoppedCh: make(chan struct{}),
	}

	// 初始化基础发送器
	s.InitBase(cfg.Service, s)

	// 恢复未发送的记录
	s.recover()

	// 启动后台协程
	go s.backgroundLoop()
	go s.compactLoop()

	return s
}

// Send 发送日志
// 流程：WAL 写入 -> 内存缓冲 -> 触发发送
func (s *ReliableSender) Send(entry *LogEntry) error {
	// 深拷贝 entry，避免数据竞争
	// 虽然 WAL.Write 会立即序列化，但为了代码一致性和安全性，统一深拷贝
	entryCopy := s.copyEntry(entry)

	// 1. 先写入 WAL（持久化）
	// 这一步是关键：写入成功才算日志不会丢失
	seq, err := s.wal.Write(entryCopy)
	if err != nil {
		// WAL 写入失败，返回错误
		// 调用方可以选择重试或降级
		s.statsMu.Lock()
		s.stats.TotalFailed++
		s.stats.LastError = err.Error()
		s.stats.LastErrorTime = time.Now()
		s.statsMu.Unlock()
		return err
	}

	// 2. 添加到内存缓冲（用于批量发送）
	s.pendingMu.Lock()
	s.pending = append(s.pending, WALRecord{
		Seq:   seq,
		Entry: entryCopy, // 使用拷贝的 entry，避免数据竞争
	})
	shouldFlush := len(s.pending) >= s.cfg.BatchSize
	s.pendingMu.Unlock()

	// 3. 如果缓冲满了，触发发送
	if shouldFlush {
		go s.flush() // 异步发送，不阻塞写入
	}

	return nil
}

// recover 从 WAL 恢复未发送的记录
// 程序启动时调用
func (s *ReliableSender) recover() {
	records, err := s.wal.ReadPending()
	if err != nil {
		return
	}

	if len(records) > 0 {
		fmt.Printf("[ReliableSender] Recovering %d pending records from WAL\n", len(records))

		s.pendingMu.Lock()
		s.pending = append(s.pending, records...)
		s.pendingMu.Unlock()
	}
}

// backgroundLoop 后台发送循环
func (s *ReliableSender) backgroundLoop() {
	defer close(s.stoppedCh)

	ticker := time.NewTicker(s.cfg.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			// 关闭前最后一次刷新
			s.flush()
			return
		case <-ticker.C:
			// 定时刷新
			s.flush()
		}
	}
}

// compactLoop 定期压缩 WAL 文件
func (s *ReliableSender) compactLoop() {
	// 每小时压缩一次
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			if err := s.wal.Compact(); err != nil {
				fmt.Printf("[ReliableSender] Compact failed: %v\n", err)
			}
		}
	}
}

// flush 刷新发送
func (s *ReliableSender) flush() {
	// 1. 获取待发送的记录
	s.pendingMu.Lock()
	if len(s.pending) == 0 {
		s.pendingMu.Unlock()
		return
	}

	// 取出当前批次
	batch := s.pending
	s.pending = make([]WALRecord, 0, s.cfg.BatchSize)
	s.pendingMu.Unlock()

	// 2. 转换为 LogEntry 指针数组
	entries := make([]*LogEntry, len(batch))
	seqs := make([]uint64, len(batch))
	for i := range batch {
		entries[i] = &batch[i].Entry
		seqs[i] = batch[i].Seq
	}

	// 3. 发送（带重试）
	var sendErr error
	for retry := 0; retry < s.cfg.RetryCount; retry++ {
		sendErr = s.client.SendBatch(entries)
		if sendErr == nil {
			break // 发送成功
		}

		// 等待后重试
		if retry < s.cfg.RetryCount-1 {
			time.Sleep(s.cfg.RetryInterval)
		}
	}

	// 4. 处理结果
	if sendErr != nil {
		// 发送失败，放回队列等待下次重试
		s.pendingMu.Lock()
		s.pending = append(batch, s.pending...) // 放到前面优先发送
		s.pendingMu.Unlock()

		s.statsMu.Lock()
		s.stats.TotalFailed += int64(len(batch))
		s.stats.LastError = sendErr.Error()
		s.stats.LastErrorTime = time.Now()
		s.statsMu.Unlock()
	} else {
		// 发送成功，标记 WAL 记录为已发送
		if err := s.wal.MarkSent(seqs); err != nil {
			fmt.Printf("[ReliableSender] MarkSent failed: %v\n", err)
		}

		s.statsMu.Lock()
		s.stats.TotalSent += int64(len(batch))
		s.stats.LastSendTime = time.Now()
		s.statsMu.Unlock()
	}
}

// Flush 强制刷新
func (s *ReliableSender) Flush() error {
	s.flush()
	return nil
}

// copyEntry 深拷贝 LogEntry（避免与调用方的 sync.Pool 冲突）
func (s *ReliableSender) copyEntry(entry *LogEntry) LogEntry {
	copied := LogEntry{
		ID:         entry.ID,
		Timestamp:  entry.Timestamp,
		Message:    entry.Message,
		Level:      entry.Level,
		Service:    entry.Service,
		TraceID:    entry.TraceID,
		SpanID:     entry.SpanID,
		UserID:     entry.UserID,
		RequestID:  entry.RequestID,
		IP:         entry.IP,
		Caller:     entry.Caller,
		Function:   entry.Function,
		Package:    entry.Package,
		StackTrace: entry.StackTrace,
		StackHash:  entry.StackHash,
		ReceivedAt: entry.ReceivedAt,
		StoredAt:   entry.StoredAt,
		Metadata:   make(map[string]interface{}, len(entry.Metadata)),
	}

	// 深拷贝 Metadata
	for k, v := range entry.Metadata {
		copied.Metadata[k] = v
	}

	return copied
}

// Close 关闭发送器
func (s *ReliableSender) Close() error {
	close(s.stopCh)
	<-s.stoppedCh
	return s.wal.Close()
}

// Stats 获取统计
func (s *ReliableSender) Stats() *SenderStats {
	s.statsMu.RLock()
	defer s.statsMu.RUnlock()

	stats := s.stats
	return &stats
}
