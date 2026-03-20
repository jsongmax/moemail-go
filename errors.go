package moemail

import "fmt"

// MoeMailError 基础错误类型。
type MoeMailError struct {
	Message    string
	StatusCode int
}

func (e *MoeMailError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("moemail: %s (status %d)", e.Message, e.StatusCode)
	}
	return fmt.Sprintf("moemail: %s", e.Message)
}

// AuthenticationError API Key 无效 (401)。
type AuthenticationError struct{ MoeMailError }

// NotFoundError 资源不存在 (404)。
type NotFoundError struct{ MoeMailError }

// APIError 其他 API 错误。
type APIError struct{ MoeMailError }

// WaitTimeoutError 等待新邮件超时。
type WaitTimeoutError struct{ MoeMailError }
