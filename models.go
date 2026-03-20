// Package moemail 提供 MoeMail 临时邮箱 API 的 Go 客户端。
package moemail

// Config 系统配置。
type Config struct {
	DefaultRole  string   `json:"defaultRole"`
	EmailDomains []string `json:"emailDomains"`
	AdminContact string   `json:"adminContact"`
	MaxEmails    int      `json:"maxEmails"`
}

// Email 临时邮箱。
type Email struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

// Message 邮件消息。
type Message struct {
	ID      string `json:"id"`
	Subject string `json:"subject,omitempty"`
	From    string `json:"from,omitempty"`
	To      string `json:"to,omitempty"`
	Text    string `json:"text,omitempty"`
	HTML    string `json:"html,omitempty"`
	Date    string `json:"date,omitempty"`
}

// ShareLink 分享链接。
type ShareLink struct {
	ID        string `json:"id"`
	URL       string `json:"url,omitempty"`
	ExpiresAt string `json:"expiresAt,omitempty"`
}
