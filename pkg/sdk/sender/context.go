package sender

import (
	"crypto/md5"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
)

// ============================================================
// 自动上下文采集
// 解决「日志上下文不足」痛点
// ============================================================

// CallerInfo 调用者信息
type CallerInfo struct {
	File     string // 文件路径
	Line     int    // 行号
	Function string // 函数名
	Package  string // 包名
}

// CaptureContext 自动捕获日志上下文
// skip: 跳过的调用栈层数
//   - 0: CaptureContext 自己
//   - 1: 调用 CaptureContext 的函数
//   - 2: 调用上层函数的地方（通常是这个）
func CaptureContext(skip int) *CallerInfo {
	// 获取调用者信息
	pc, file, line, ok := runtime.Caller(skip)
	if !ok {
		return nil
	}

	// 获取函数信息
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return &CallerInfo{
			File: filepath.Base(file),
			Line: line,
		}
	}

	// 解析函数名和包名
	fullName := fn.Name()
	parts := strings.Split(fullName, "/")
	lastPart := parts[len(parts)-1]

	// 分离包名和函数名
	// 例如: "github.com/user/project/pkg.(*Struct).Method"
	//   -> Package: "github.com/user/project/pkg"
	//   -> Function: "(*Struct).Method"
	var pkgName, funcName string
	if idx := strings.LastIndex(lastPart, "."); idx != -1 {
		pkgName = strings.Join(parts[:len(parts)-1], "/") + "/" + lastPart[:idx]
		funcName = lastPart[idx+1:]
	} else {
		funcName = lastPart
	}

	return &CallerInfo{
		File:     filepath.Base(file),
		Line:     line,
		Function: funcName,
		Package:  pkgName,
	}
}

// EnrichLogEntry 自动填充日志上下文
// skip: 跳过的调用栈层数（相对于调用者）
func EnrichLogEntry(entry *LogEntry, skip int) {
	if entry == nil {
		return
	}

	// 捕获调用位置（skip+1 因为要跳过 EnrichLogEntry 自己）
	caller := CaptureContext(skip + 1)
	if caller != nil {
		entry.Caller = fmt.Sprintf("%s:%d", caller.File, caller.Line)
		entry.Function = caller.Function
		entry.Package = caller.Package
	}

	// 对于 error 级别，自动捕获堆栈
	if entry.Level == "error" && entry.StackTrace == "" {
		entry.StackTrace = CaptureStack(skip + 1)
		entry.StackHash = HashStack(entry.StackTrace)
	}
}

// CaptureStack 捕获当前调用栈
// skip: 跳过的调用栈层数
func CaptureStack(skip int) string {
	const maxDepth = 32 // 最多捕获 32 层
	pcs := make([]uintptr, maxDepth)
	n := runtime.Callers(skip+2, pcs) // +2 跳过 Callers 和 CaptureStack

	if n == 0 {
		return ""
	}

	var sb strings.Builder
	frames := runtime.CallersFrames(pcs[:n])

	for {
		frame, more := frames.Next()
		
		// 跳过 runtime 内部函数
		if strings.HasPrefix(frame.Function, "runtime.") {
			if !more {
				break
			}
			continue
		}

		// 格式化堆栈行
		sb.WriteString(fmt.Sprintf("%s\n\t%s:%d\n",
			frame.Function,
			frame.File,
			frame.Line,
		))

		if !more {
			break
		}
	}

	return sb.String()
}

// CaptureStackSimple 捕获简化的堆栈（只有函数名和行号）
func CaptureStackSimple(skip int) string {
	const maxDepth = 32
	pcs := make([]uintptr, maxDepth)
	n := runtime.Callers(skip+2, pcs)

	if n == 0 {
		return ""
	}

	var sb strings.Builder
	frames := runtime.CallersFrames(pcs[:n])

	for {
		frame, more := frames.Next()
		
		if strings.HasPrefix(frame.Function, "runtime.") {
			if !more {
				break
			}
			continue
		}

		// 只保留文件名和行号
		file := filepath.Base(frame.File)
		funcParts := strings.Split(frame.Function, "/")
		funcName := funcParts[len(funcParts)-1]
		
		sb.WriteString(fmt.Sprintf("%s (%s:%d)\n", funcName, file, frame.Line))

		if !more {
			break
		}
	}

	return sb.String()
}

// HashStack 生成堆栈指纹（用于错误聚合）
// 原理：提取堆栈中的关键信息（函数名+文件名），生成 MD5
func HashStack(stack string) string {
	if stack == "" {
		return ""
	}

	// 提取关键信息：去除行号，只保留函数名和文件名
	// 这样即使错误发生在不同行，也能聚合到同一类
	var normalized strings.Builder
	
	lines := strings.Split(stack, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 跳过文件路径行（包含 :数字）
		if strings.Contains(line, ":") {
			// 提取文件名（去除行号）
			if idx := strings.LastIndex(line, ":"); idx != -1 {
				file := line[:idx]
				// 只保留文件名，不要完整路径
				file = filepath.Base(file)
				normalized.WriteString(file)
				normalized.WriteString("|")
			}
		} else {
			// 函数名行
			// 去除包路径，只保留最后的函数名
			parts := strings.Split(line, "/")
			funcName := parts[len(parts)-1]
			normalized.WriteString(funcName)
			normalized.WriteString("|")
		}
	}

	// 生成 MD5 哈希
	hash := md5.Sum([]byte(normalized.String()))
	return fmt.Sprintf("%x", hash[:8]) // 取前 8 字节，16 个字符
}

// ============================================================
// 便捷函数
// ============================================================

// NewLogEntryWithContext 创建带自动上下文的日志条目
func NewLogEntryWithContext(level, message string) *LogEntry {
	entry := &LogEntry{
		Level:   level,
		Message: message,
	}
	EnrichLogEntry(entry, 1) // skip=1 跳过 NewLogEntryWithContext
	return entry
}

// MustCaptureCaller 必须捕获调用者（测试用）
// 如果失败会 panic
func MustCaptureCaller(skip int) *CallerInfo {
	caller := CaptureContext(skip + 1)
	if caller == nil {
		panic("failed to capture caller")
	}
	return caller
}
