package moemail

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client MoeMail 客户端。
type Client struct {
	baseURL    string
	apiKey     string
	domain     string
	httpClient *http.Client
}

// Option 客户端配置选项。
type Option func(*Client)

// WithDomain 指定邮箱域名，不设置则由服务端随机分配。
func WithDomain(domain string) Option {
	return func(c *Client) { c.domain = domain }
}

// WithTimeout 设置 HTTP 请求超时。
func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) { c.httpClient.Timeout = timeout }
}

// WithProxy 设置 HTTP 代理。
func WithProxy(proxyURL string) Option {
	return func(c *Client) {
		u, err := url.Parse(proxyURL)
		if err != nil {
			return
		}
		c.httpClient.Transport = &http.Transport{Proxy: http.ProxyURL(u)}
	}
}

// NewClient 创建 MoeMail 客户端。
func NewClient(baseURL, apiKey string, opts ...Option) *Client {
	c := &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Close 关闭客户端，释放连接。
func (c *Client) Close() {
	c.httpClient.CloseIdleConnections()
}

// ---------- 内部方法 ----------

func (c *Client) doRequest(method, path string, body any) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("moemail: marshal body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("moemail: create request: %w", err)
	}
	req.Header.Set("X-API-Key", c.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	return c.httpClient.Do(req)
}

func (c *Client) handleResponse(resp *http.Response, target any) error {
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return &AuthenticationError{MoeMailError{Message: "API Key 无效", StatusCode: 401}}
	}
	if resp.StatusCode == 404 {
		return &NotFoundError{MoeMailError{Message: "资源不存在", StatusCode: 404}}
	}
	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(resp.Body)
		return &APIError{MoeMailError{
			Message:    fmt.Sprintf("API 错误 %d: %s", resp.StatusCode, string(raw)),
			StatusCode: resp.StatusCode,
		}}
	}
	if resp.StatusCode == 204 || target == nil {
		return nil
	}

	return json.NewDecoder(resp.Body).Decode(target)
}

func (c *Client) get(path string, target any) error {
	resp, err := c.doRequest(http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	return c.handleResponse(resp, target)
}

func (c *Client) post(path string, body, target any) error {
	resp, err := c.doRequest(http.MethodPost, path, body)
	if err != nil {
		return err
	}
	return c.handleResponse(resp, target)
}

func (c *Client) delete(path string) error {
	resp, err := c.doRequest(http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	return c.handleResponse(resp, nil)
}

// ---------- 系统配置 ----------

// configRaw 用于处理 emailDomains 可能是字符串的情况。
type configRaw struct {
	DefaultRole  string      `json:"defaultRole"`
	EmailDomains interface{} `json:"emailDomains"`
	AdminContact string      `json:"adminContact"`
	MaxEmails    int         `json:"maxEmails"`
}

// GetConfig 获取系统配置。
func (c *Client) GetConfig() (*Config, error) {
	var raw configRaw
	if err := c.get("/api/config", &raw); err != nil {
		return nil, err
	}

	cfg := &Config{
		DefaultRole:  raw.DefaultRole,
		AdminContact: raw.AdminContact,
		MaxEmails:    raw.MaxEmails,
	}

	switch v := raw.EmailDomains.(type) {
	case string:
		for _, d := range strings.Split(v, ",") {
			d = strings.TrimSpace(d)
			if d != "" {
				cfg.EmailDomains = append(cfg.EmailDomains, d)
			}
		}
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok {
				cfg.EmailDomains = append(cfg.EmailDomains, s)
			}
		}
	}

	return cfg, nil
}

// ---------- 邮箱管理 ----------

// GenerateEmail 创建临时邮箱。
//
// name 为邮箱前缀（可选，传空字符串忽略）。
// expiryTime 为有效期（毫秒）：3600000=1h, 86400000=24h, 0=永久。
func (c *Client) GenerateEmail(name string, expiryTime int) (*Email, error) {
	domain := c.domain
	if domain == "" {
		cfg, err := c.GetConfig()
		if err != nil {
			return nil, err
		}
		if len(cfg.EmailDomains) > 0 {
			domain = cfg.EmailDomains[rand.Intn(len(cfg.EmailDomains))]
		}
	}

	body := map[string]interface{}{"expiryTime": expiryTime}
	if name != "" {
		body["name"] = name
	}
	if domain != "" {
		body["domain"] = domain
	}

	var email Email
	if err := c.post("/api/emails/generate", body, &email); err != nil {
		return nil, err
	}
	return &email, nil
}

// GetEmails 获取邮箱列表。
func (c *Client) GetEmails(cursor string) ([]Email, error) {
	path := "/api/emails"
	if cursor != "" {
		path += "?cursor=" + url.QueryEscape(cursor)
	}

	var raw json.RawMessage
	if err := c.get(path, &raw); err != nil {
		return nil, err
	}

	// 尝试解析为 {"emails": [...]} 或直接 [...]
	var wrapper struct {
		Emails []Email `json:"emails"`
	}
	if err := json.Unmarshal(raw, &wrapper); err == nil && wrapper.Emails != nil {
		return wrapper.Emails, nil
	}

	var emails []Email
	if err := json.Unmarshal(raw, &emails); err != nil {
		return nil, err
	}
	return emails, nil
}

// GetMessages 获取某邮箱的消息列表。
func (c *Client) GetMessages(emailID, cursor string) ([]Message, error) {
	path := "/api/emails/" + emailID
	if cursor != "" {
		path += "?cursor=" + url.QueryEscape(cursor)
	}

	var raw json.RawMessage
	if err := c.get(path, &raw); err != nil {
		return nil, err
	}

	// 尝试解析为 {"messages": [...]} 或直接 [...]
	var wrapper struct {
		Messages []Message `json:"messages"`
	}
	if err := json.Unmarshal(raw, &wrapper); err == nil && wrapper.Messages != nil {
		return wrapper.Messages, nil
	}

	var messages []Message
	if err := json.Unmarshal(raw, &messages); err != nil {
		return nil, err
	}
	return messages, nil
}

// GetMessage 获取单条消息。
func (c *Client) GetMessage(emailID, messageID string) (*Message, error) {
	var msg Message
	if err := c.get("/api/emails/"+emailID+"/"+messageID, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// DeleteEmail 删除邮箱。
func (c *Client) DeleteEmail(emailID string) error {
	return c.delete("/api/emails/" + emailID)
}

// WaitForMessage 轮询等待新邮件。
//
// 超时返回 *WaitTimeoutError。
func (c *Client) WaitForMessage(emailID string, timeout, interval time.Duration) (*Message, error) {
	existingMsgs, err := c.GetMessages(emailID, "")
	if err != nil {
		return nil, err
	}
	existing := make(map[string]struct{}, len(existingMsgs))
	for _, m := range existingMsgs {
		existing[m.ID] = struct{}{}
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		time.Sleep(interval)
		messages, err := c.GetMessages(emailID, "")
		if err != nil {
			return nil, err
		}
		for i := range messages {
			if _, seen := existing[messages[i].ID]; !seen {
				return &messages[i], nil
			}
		}
	}

	return nil, &WaitTimeoutError{MoeMailError{
		Message: fmt.Sprintf("等待新邮件超时 (%s)", timeout),
	}}
}

// ---------- 邮箱分享 ----------

// CreateEmailShare 创建邮箱分享链接。
func (c *Client) CreateEmailShare(emailID string, expiresIn int) (*ShareLink, error) {
	var link ShareLink
	body := map[string]interface{}{"expiresIn": expiresIn}
	if err := c.post("/api/emails/"+emailID+"/share", body, &link); err != nil {
		return nil, err
	}
	return &link, nil
}

// GetEmailShares 获取邮箱分享链接列表。
func (c *Client) GetEmailShares(emailID string) ([]ShareLink, error) {
	var links []ShareLink
	if err := c.get("/api/emails/"+emailID+"/share", &links); err != nil {
		return nil, err
	}
	return links, nil
}

// DeleteEmailShare 删除邮箱分享链接。
func (c *Client) DeleteEmailShare(emailID, shareID string) error {
	return c.delete("/api/emails/" + emailID + "/share/" + shareID)
}

// ---------- 消息分享 ----------

// CreateMessageShare 创建消息分享链接。
func (c *Client) CreateMessageShare(emailID, messageID string, expiresIn int) (*ShareLink, error) {
	var link ShareLink
	body := map[string]interface{}{"expiresIn": expiresIn}
	path := fmt.Sprintf("/api/emails/%s/messages/%s/share", emailID, messageID)
	if err := c.post(path, body, &link); err != nil {
		return nil, err
	}
	return &link, nil
}

// GetMessageShares 获取消息分享链接列表。
func (c *Client) GetMessageShares(emailID, messageID string) ([]ShareLink, error) {
	var links []ShareLink
	path := fmt.Sprintf("/api/emails/%s/messages/%s/share", emailID, messageID)
	if err := c.get(path, &links); err != nil {
		return nil, err
	}
	return links, nil
}

// DeleteMessageShare 删除消息分享链接。
func (c *Client) DeleteMessageShare(emailID, messageID, shareID string) error {
	path := fmt.Sprintf("/api/emails/%s/messages/%s/share/%s", emailID, messageID, shareID)
	return c.delete(path)
}
