package sdk

import (
	"crypto/md5"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
)

// ============================================================
// 自动上下文采集（SDK 内部使用）
// ============================================================

// callerInfo 调用者信息
type callerInfo struct {
	File     string
	Line     int
	Function string
	Package  string
}

// captureContext 捕获调用上下文
func captureContext(skip int) *callerInfo {
	pc, file, line, ok := runtime.Caller(skip)
	if !ok {
		return nil
	}

	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return &callerInfo{
			File: filepath.Base(file),
			Line: line,
		}
	}

	// 解析函数名和包名
	// 例如: "github.com/user/project/pkg.(*Struct).Method"
	fullName := fn.Name()
	parts := strings.Split(fullName, "/")
	lastPart := parts[len(parts)-1]

	var pkgName, funcName string
	if idx := strings.LastIndex(lastPart, "."); idx != -1 {
		// 包名：去掉最后的函数名部分
		pkgName = strings.Join(parts[:len(parts)-1], "/") + "/" + lastPart[:idx]
		// 函数名：最后一个点之后的部分
		funcName = lastPart[idx+1:]
	} else {
		funcName = lastPart
	}

	return &callerInfo{
		File:     filepath.Base(file),
		Line:     line,
		Function: funcName,
		Package:  pkgName,
	}
}

// captureStack 捕获堆栈信息
func captureStack(skip int) string {
	const maxDepth = 32
	pcs := make([]uintptr, maxDepth)
	n := runtime.Callers(skip+2, pcs) // +2 跳过 Callers 和 captureStack

	if n == 0 {
		return ""
	}

	var sb strings.Builder
	frames := runtime.CallersFrames(pcs[:n])

	for {
		frame, more := frames.Next()

		// 跳过 runtime 和 sdk 内部函数
		if strings.HasPrefix(frame.Function, "runtime.") ||
			strings.HasPrefix(frame.Function, "github.com/loglite/loglite/pkg/sdk.") {
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

// hashStack 生成堆栈指纹（用于错误聚合）
func hashStack(stack string) string {
	if stack == "" {
		return ""
	}

	// 标准化堆栈：去除行号，只保留函数名和文件名
	var normalized strings.Builder

	lines := strings.Split(stack, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 文件路径行（包含 :数字）
		if strings.Contains(line, ":") {
			if idx := strings.LastIndex(line, ":"); idx != -1 {
				file := line[:idx]
				file = filepath.Base(file)
				normalized.WriteString(file)
				normalized.WriteString("|")
			}
		} else {
			// 函数名行
			parts := strings.Split(line, "/")
			funcName := parts[len(parts)-1]
			normalized.WriteString(funcName)
			normalized.WriteString("|")
		}
	}

	// 生成 MD5 哈希（取前 8 字节）
	hash := md5.Sum([]byte(normalized.String()))
	return fmt.Sprintf("%x", hash[:8])
}
