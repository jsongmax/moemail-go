// MoeMail Go SDK 演示脚本
//
// 使用前请修改以下常量：
//
//	baseURL: 你的 MoeMail 服务地址
//	apiKey:  你的 API Key
//	domain:  指定域名（留空则随机分配）
package main

import (
	"fmt"
	"log"

	moemail "github.com/jsongmax/moemail-go"
)

const (
	baseURL = "https://your-moemail-server.com/" // 改成你的地址
	apiKey  = "your-api-key"                     // 改成你的 API Key
	domain  = ""                                 // 指定域名，留空则随机
)

func main() {
	// 构建客户端
	opts := []moemail.Option{moemail.WithTimeout(30e9)} // 30s
	if domain != "" {
		opts = append(opts, moemail.WithDomain(domain))
	}
	client := moemail.NewClient(baseURL, apiKey, opts...)
	defer client.Close()

	// 1. 获取系统配置
	fmt.Println("========================================")
	fmt.Println("1. 获取系统配置")
	config, err := client.GetConfig()
	if err != nil {
		log.Fatalf("获取配置失败: %v", err)
	}
	fmt.Printf("   可用域名: %v\n", config.EmailDomains)
	fmt.Printf("   最大邮箱数: %d\n", config.MaxEmails)

	// 2. 创建临时邮箱
	fmt.Println("\n2. 创建临时邮箱")
	email, err := client.GenerateEmail("demo", 3600000)
	if err != nil {
		log.Fatalf("创建邮箱失败: %v", err)
	}
	fmt.Printf("   邮箱地址: %s\n", email.Email)
	fmt.Printf("   邮箱 ID:  %s\n", email.ID)

	// 3. 等待新邮件
	fmt.Printf("\n3. 等待新邮件（请向 %s 发送一封测试邮件）\n", email.Email)
	fmt.Println("   轮询中，超时时间 120 秒...")
	msg, err := client.WaitForMessage(email.ID, 120e9, 5e9) // 120s timeout, 5s interval
	if err != nil {
		fmt.Printf("   ⏰ %v\n", err)
	} else {
		fmt.Println("   ✅ 收到邮件!")
		fmt.Printf("   主题: %s\n", msg.Subject)
		fmt.Printf("   发件人: %s\n", msg.From)
		text := msg.Text
		if len(text) > 200 {
			text = text[:200]
		}
		if text == "" {
			text = "(无)"
		}
		fmt.Printf("   纯文本: %s\n", text)
	}

	// 4. 获取所有消息
	fmt.Println("\n4. 获取所有消息")
	messages, err := client.GetMessages(email.ID, "")
	if err != nil {
		log.Fatalf("获取消息失败: %v", err)
	}
	fmt.Printf("   共 %d 条消息\n", len(messages))
	for _, m := range messages {
		fmt.Printf("   - [%s] %s\n", m.ID, m.Subject)
	}

	fmt.Println("\n完成！")
}
