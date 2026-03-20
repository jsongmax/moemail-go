# MoeMail Go SDK

MoeMail 临时邮箱 API 的 Go 客户端，零外部依赖（仅使用 Go 标准库）。

## 安装

```bash
go get github.com/jsongmax/moemail-go
```

## 快速开始

```go
package main

import (
    "fmt"
    "log"

    moemail "github.com/jsongmax/moemail-go"
)

func main() {
    client := moemail.NewClient(
        "https://mail.apigo.site/",
        "your-api-key",
        moemail.WithDomain("example.com"),
        moemail.WithTimeout(30e9),
    )
    defer client.Close()

    // 获取系统配置
    config, _ := client.GetConfig()
    fmt.Println("可用域名:", config.EmailDomains)

    // 创建临时邮箱
    email, _ := client.GenerateEmail("demo", 3600000)
    fmt.Println("邮箱地址:", email.Email)

    // 等待新邮件（轮询）
    msg, err := client.WaitForMessage(email.ID, 120e9, 5e9)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println("收到邮件:", msg.Subject)
}
```

## API

| 方法 | 说明 |
|---|---|
| `NewClient(baseURL, apiKey, ...Option)` | 创建客户端 |
| `WithDomain(domain)` | 指定邮箱域名 |
| `WithTimeout(duration)` | 设置超时 |
| `WithProxy(proxyURL)` | 设置代理 |
| `GetConfig()` | 获取系统配置 |
| `GenerateEmail(name, expiryTime)` | 创建临时邮箱 |
| `GetEmails(cursor)` | 获取邮箱列表 |
| `GetMessages(emailID, cursor)` | 获取消息列表 |
| `GetMessage(emailID, messageID)` | 获取单条消息 |
| `DeleteEmail(emailID)` | 删除邮箱 |
| `WaitForMessage(emailID, timeout, interval)` | 轮询等待新邮件 |
| `CreateEmailShare(emailID, expiresIn)` | 创建邮箱分享链接 |
| `CreateMessageShare(emailID, messageID, expiresIn)` | 创建消息分享链接 |

## License

MIT
