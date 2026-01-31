package sender

import "errors"

// 错误定义
var (
	ErrBufferFull    = errors.New("buffer is full, log dropped")
	ErrSendFailed    = errors.New("failed to send logs")
	ErrWALWriteFailed = errors.New("failed to write WAL")
	ErrClosed        = errors.New("sender is closed")
)
