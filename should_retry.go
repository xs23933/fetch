package fetch

import (
	"errors"
	"io"
	"net"
	"os"
	"strings"
	"syscall"
)

func shouldRetry(err error) bool {
	if err == nil {
		return false
	}

	// 检查错误链中是否包含特定的可重试错误
	var (
		netErr     net.Error
		syscallErr *os.SyscallError
	)

	switch {
	// 1. 检查特定的错误类型 (使用 errors.Is 检查错误链)
	case errors.Is(err, io.EOF), // 基本的EOF
		errors.Is(err, io.ErrUnexpectedEOF),  // 意外的EOF (更常见于HTTP请求)
		errors.Is(err, syscall.ECONNRESET),   // 连接被对端重置
		errors.Is(err, syscall.ECONNABORTED), // 连接中止
		errors.Is(err, syscall.ECONNREFUSED): // 连接被拒绝
		return true

	// 2. 检查网络错误及其超时属性
	case errors.As(err, &netErr):
		if netErr.Timeout() {
			return true // 所有超时错误都应该重试
		}

	// 3. 检查HTTP响应错误 (如5xx状态码)
	// 注意: 这需要特殊处理，因为err可能是从http.Response.Body.Read返回的
	// 通常5xx错误应该在处理响应时单独判断，而不是在这里

	// 4. 检查系统调用错误
	case errors.As(err, &syscallErr):
		if syscallErr.Err == syscall.ECONNRESET ||
			syscallErr.Err == syscall.ECONNABORTED ||
			syscallErr.Err == syscall.ECONNREFUSED {
			return true
		}
	}
	// 5. 检查错误消息中的特定模式 (作为后备方案)
	msg := strings.ToLower(err.Error())

	for _, pattern := range retryPatterns {
		if strings.Contains(msg, pattern) {
			return true
		}
	}

	return false
}

var (
	retryPatterns = []string{
		"connection refused",
		"bad gateway",
		"stream timeout",
		"connection reset by peer",
		"broken pipe",
		"unexpected eof",
		"upstream connect error or discon",
		"i/o timeout",
		"no such host",
		"tls: handshake failure",           // 某些TLS握手错误可以重试
		"use of closed network connection", // 连接已关闭
		"server misbehaving",               // DNS错误
	}
)
