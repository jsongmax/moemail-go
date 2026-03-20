package moemail

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// newTestClient 创建一个连接到 httptest.Server 的测试客户端。
func newTestClient(ts *httptest.Server, domain string) *Client {
	opts := []Option{WithTimeout(5 * time.Second)}
	if domain != "" {
		opts = append(opts, WithDomain(domain))
	}
	return NewClient(ts.URL, "test-api-key", opts...)
}

// ---------- GetConfig ----------

func TestGetConfig(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/config" {
			http.NotFound(w, r)
			return
		}
		// 模拟 emailDomains 为逗号分隔字符串
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"defaultRole":  "CIVILIAN",
			"emailDomains": "moemail.app,example.com",
			"adminContact": "admin@example.com",
			"maxEmails":    10,
		})
	}))
	defer ts.Close()

	client := newTestClient(ts, "moemail.app")
	defer client.Close()

	cfg, err := client.GetConfig()
	if err != nil {
		t.Fatalf("GetConfig failed: %v", err)
	}
	if cfg.DefaultRole != "CIVILIAN" {
		t.Errorf("DefaultRole = %q, want %q", cfg.DefaultRole, "CIVILIAN")
	}
	if len(cfg.EmailDomains) != 2 || cfg.EmailDomains[0] != "moemail.app" || cfg.EmailDomains[1] != "example.com" {
		t.Errorf("EmailDomains = %v, want [moemail.app example.com]", cfg.EmailDomains)
	}
	if cfg.AdminContact != "admin@example.com" {
		t.Errorf("AdminContact = %q, want %q", cfg.AdminContact, "admin@example.com")
	}
	if cfg.MaxEmails != 10 {
		t.Errorf("MaxEmails = %d, want 10", cfg.MaxEmails)
	}
}

// ---------- GenerateEmail ----------

func TestGenerateEmail(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/emails/generate" || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		body, _ := io.ReadAll(r.Body)
		var req map[string]interface{}
		json.Unmarshal(body, &req)

		if req["name"] != "test" {
			t.Errorf("request name = %v, want test", req["name"])
		}
		if req["domain"] != "moemail.app" {
			t.Errorf("request domain = %v, want moemail.app", req["domain"])
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"id":    "email-123",
			"email": "test@moemail.app",
		})
	}))
	defer ts.Close()

	client := newTestClient(ts, "moemail.app")
	defer client.Close()

	email, err := client.GenerateEmail("test", 3600000)
	if err != nil {
		t.Fatalf("GenerateEmail failed: %v", err)
	}
	if email.ID != "email-123" {
		t.Errorf("ID = %q, want email-123", email.ID)
	}
	if email.Email != "test@moemail.app" {
		t.Errorf("Email = %q, want test@moemail.app", email.Email)
	}
}

func TestGenerateEmailNoDomain(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/config" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"defaultRole":  "CIVILIAN",
				"emailDomains": []string{"auto.com"},
				"adminContact": "",
				"maxEmails":    5,
			})
			return
		}
		if r.URL.Path == "/api/emails/generate" {
			callCount++
			body, _ := io.ReadAll(r.Body)
			var req map[string]interface{}
			json.Unmarshal(body, &req)
			if req["domain"] != "auto.com" {
				t.Errorf("domain = %v, want auto.com (from config)", req["domain"])
			}
			json.NewEncoder(w).Encode(map[string]string{
				"id":    "email-456",
				"email": "random@auto.com",
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	// 不指定 domain
	client := newTestClient(ts, "")
	defer client.Close()

	email, err := client.GenerateEmail("", 3600000)
	if err != nil {
		t.Fatalf("GenerateEmail failed: %v", err)
	}
	if email.Email != "random@auto.com" {
		t.Errorf("Email = %q, want random@auto.com", email.Email)
	}
}

// ---------- GetMessages ----------

func TestGetMessages(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/emails/email-123" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]string{
			{"id": "msg-1", "subject": "Hello", "from": "a@b.com"},
			{"id": "msg-2", "subject": "World", "from": "c@d.com"},
		})
	}))
	defer ts.Close()

	client := newTestClient(ts, "moemail.app")
	defer client.Close()

	msgs, err := client.GetMessages("email-123", "")
	if err != nil {
		t.Fatalf("GetMessages failed: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("len = %d, want 2", len(msgs))
	}
	if msgs[0].Subject != "Hello" || msgs[0].From != "a@b.com" {
		t.Errorf("msg[0] = %+v", msgs[0])
	}
}

// ---------- GetMessage ----------

func TestGetMessage(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/emails/email-123/msg-1" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"id":      "msg-1",
			"subject": "Test",
			"from":    "sender@test.com",
			"to":      "test@moemail.app",
			"text":    "验证码: 123456",
			"html":    "<p>验证码: 123456</p>",
		})
	}))
	defer ts.Close()

	client := newTestClient(ts, "moemail.app")
	defer client.Close()

	msg, err := client.GetMessage("email-123", "msg-1")
	if err != nil {
		t.Fatalf("GetMessage failed: %v", err)
	}
	if msg.ID != "msg-1" {
		t.Errorf("ID = %q, want msg-1", msg.ID)
	}
	if msg.Text != "验证码: 123456" {
		t.Errorf("Text = %q", msg.Text)
	}
}

// ---------- DeleteEmail ----------

func TestDeleteEmail(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/api/emails/email-123" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	client := newTestClient(ts, "moemail.app")
	defer client.Close()

	if err := client.DeleteEmail("email-123"); err != nil {
		t.Fatalf("DeleteEmail failed: %v", err)
	}
}

// ---------- WaitForMessage ----------

func TestWaitForMessage(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		if callCount <= 2 {
			// 前两次返回空
			json.NewEncoder(w).Encode([]map[string]string{})
		} else {
			// 第三次返回新消息
			json.NewEncoder(w).Encode([]map[string]string{
				{"id": "msg-new", "subject": "New!"},
			})
		}
	}))
	defer ts.Close()

	client := newTestClient(ts, "moemail.app")
	defer client.Close()

	msg, err := client.WaitForMessage("email-123", 3*time.Second, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("WaitForMessage failed: %v", err)
	}
	if msg.ID != "msg-new" {
		t.Errorf("ID = %q, want msg-new", msg.ID)
	}
	if msg.Subject != "New!" {
		t.Errorf("Subject = %q, want New!", msg.Subject)
	}
}

func TestWaitForMessageTimeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]string{})
	}))
	defer ts.Close()

	client := newTestClient(ts, "moemail.app")
	defer client.Close()

	_, err := client.WaitForMessage("email-123", 200*time.Millisecond, 50*time.Millisecond)
	if err == nil {
		t.Fatal("expected WaitTimeoutError, got nil")
	}
	var timeoutErr *WaitTimeoutError
	if !errors.As(err, &timeoutErr) {
		t.Errorf("error type = %T, want *WaitTimeoutError", err)
	}
}

// ---------- 异常处理 ----------

func TestAuthError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"Unauthorized"}`))
	}))
	defer ts.Close()

	client := newTestClient(ts, "moemail.app")
	defer client.Close()

	_, err := client.GetConfig()
	if err == nil {
		t.Fatal("expected AuthenticationError, got nil")
	}
	var authErr *AuthenticationError
	if !errors.As(err, &authErr) {
		t.Errorf("error type = %T, want *AuthenticationError", err)
	}
}

func TestNotFoundError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"Not Found"}`))
	}))
	defer ts.Close()

	client := newTestClient(ts, "moemail.app")
	defer client.Close()

	_, err := client.GetMessage("no-exist", "msg-1")
	if err == nil {
		t.Fatal("expected NotFoundError, got nil")
	}
	var notFound *NotFoundError
	if !errors.As(err, &notFound) {
		t.Errorf("error type = %T, want *NotFoundError", err)
	}
}

// ---------- 分享链接 ----------

func TestCreateEmailShare(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/emails/email-123/share" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"id":  "share-1",
			"url": "https://moemail.test/share/abc",
		})
	}))
	defer ts.Close()

	client := newTestClient(ts, "moemail.app")
	defer client.Close()

	share, err := client.CreateEmailShare("email-123", 86400000)
	if err != nil {
		t.Fatalf("CreateEmailShare failed: %v", err)
	}
	if share.ID != "share-1" {
		t.Errorf("ID = %q, want share-1", share.ID)
	}
	if share.URL != "https://moemail.test/share/abc" {
		t.Errorf("URL = %q", share.URL)
	}
}

// ---------- 验证 X-API-Key 请求头 ----------

func TestAPIKeyHeader(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("X-API-Key")
		if key != "test-api-key" {
			t.Errorf("X-API-Key = %q, want test-api-key", key)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"defaultRole":  "CIVILIAN",
			"emailDomains": []string{},
			"adminContact": "",
			"maxEmails":    0,
		})
	}))
	defer ts.Close()

	client := newTestClient(ts, "")
	defer client.Close()

	client.GetConfig()
}
