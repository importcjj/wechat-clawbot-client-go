package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	qrcode "github.com/skip2/go-qrcode"

	clawbot "github.com/importcjj/wechat-clawbot-client-go"
	"github.com/importcjj/wechat-clawbot-client-go/store"
)

const dataDir = "./data"
const downloadDir = "./downloads"


func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := os.MkdirAll(downloadDir, 0o755); err != nil {
		log.Fatalf("创建下载目录失败: %v", err)
	}

	client := clawbot.NewDefault("terminal-echo", store.NewFileStore(dataDir),
		clawbot.WithEventHooks(clawbot.EventHooks[struct{}]{
			OnMessage: func(c *clawbot.DefaultClient, msg *clawbot.Message) {
				handleMessage(ctx, c, msg)
			},
			OnConnected: func(_ *clawbot.DefaultClient) {
				fmt.Println("\n✅ 已连接! 现在可以在微信上发消息了。")
				fmt.Println("   - 发送文本 → 自动 echo 回复")
				fmt.Println("   - 发送图片 → 下载到本地并回复确认")
				fmt.Println("   - Ctrl+C 退出")
			},
			OnSessionExpired: func(_ *clawbot.DefaultClient) {
				fmt.Println("\n⚠️  会话过期，需要重新扫码登录。")
			},
			OnDisconnected: func(_ *clawbot.DefaultClient, err error) {
				if err != nil && !errors.Is(err, context.Canceled) {
					fmt.Printf("\n❌ 连接断开: %v\n", err)
				}
			},
			OnError: func(_ *clawbot.DefaultClient, err error) {
				fmt.Printf("⚠️  错误: %v\n", err)
			},
		}),
	)

	// 尝试 Start（从 Store 恢复凭证）
	fmt.Println("🔄 尝试恢复上次登录状态...")
	err := client.Start(ctx)

	if errors.Is(err, clawbot.ErrNotLoggedIn) {
		// 首次运行或凭证丢失，走 QR 扫码
		fmt.Println("📱 需要扫码登录")

		session, loginErr := client.Login(ctx)
		if loginErr != nil {
			log.Fatalf("获取二维码失败: %v", loginErr)
		}

		// 终端渲染二维码
		printQRCode(session.QRCodeURL())

		fmt.Println("\n⏳ 等待扫码...")
		if waitErr := session.Wait(ctx); waitErr != nil {
			log.Fatalf("登录失败: %v", waitErr)
		}

		// 登录成功，启动监听
		err = client.Start(ctx)
	}

	if err != nil && !errors.Is(err, context.Canceled) {
		log.Fatalf("运行失败: %v", err)
	}
}

func handleMessage(ctx context.Context, client *clawbot.DefaultClient, msg *clawbot.Message) {
	from := msg.From
	timestamp := msg.CreatedAt.Format("15:04:05")

	// 处理图片
	if len(msg.Images) > 0 {
		for i, img := range msg.Images {
			filename := fmt.Sprintf("img_%s_%d.jpg", time.Now().Format("20060102_150405"), i)
			savePath := filepath.Join(downloadDir, filename)

			if err := os.WriteFile(savePath, img.Data, 0o644); err != nil {
				fmt.Printf("[%s] ❌ 保存图片失败: %v\n", timestamp, err)
				continue
			}

			fmt.Printf("[%s] 📷 收到图片 from %s → 已保存到 %s (%d bytes)\n",
				timestamp, from, savePath, len(img.Data))

			// 回复确认
			reply := fmt.Sprintf("✅ 图片已下载保存\n📁 文件: %s\n📐 大小: %d bytes", filename, len(img.Data))
			if err := client.SendText(ctx, from, reply); err != nil {
				fmt.Printf("[%s] ❌ 回复失败: %v\n", timestamp, err)
			}
		}
		return
	}

	// 处理视频
	if len(msg.Videos) > 0 {
		for i, vid := range msg.Videos {
			filename := fmt.Sprintf("vid_%s_%d.mp4", time.Now().Format("20060102_150405"), i)
			savePath := filepath.Join(downloadDir, filename)

			if err := os.WriteFile(savePath, vid.Data, 0o644); err != nil {
				fmt.Printf("[%s] ❌ 保存视频失败: %v\n", timestamp, err)
				continue
			}

			fmt.Printf("[%s] 🎬 收到视频 from %s → 已保存到 %s (%d bytes)\n",
				timestamp, from, savePath, len(vid.Data))

			reply := fmt.Sprintf("✅ 视频已下载保存\n📁 文件: %s\n📐 大小: %d bytes", filename, len(vid.Data))
			if err := client.SendText(ctx, from, reply); err != nil {
				fmt.Printf("[%s] ❌ 回复失败: %v\n", timestamp, err)
			}
		}
		return
	}

	// 处理文件
	if len(msg.Files) > 0 {
		for _, f := range msg.Files {
			filename := f.Filename
			if filename == "" {
				filename = fmt.Sprintf("file_%s.bin", time.Now().Format("20060102_150405"))
			}
			savePath := filepath.Join(downloadDir, filename)

			if err := os.WriteFile(savePath, f.Data, 0o644); err != nil {
				fmt.Printf("[%s] ❌ 保存文件失败: %v\n", timestamp, err)
				continue
			}

			fmt.Printf("[%s] 📎 收到文件 from %s → 已保存到 %s (%d bytes)\n",
				timestamp, from, savePath, len(f.Data))

			reply := fmt.Sprintf("✅ 文件已下载保存\n📁 文件: %s\n📐 大小: %d bytes", filename, len(f.Data))
			if err := client.SendText(ctx, from, reply); err != nil {
				fmt.Printf("[%s] ❌ 回复失败: %v\n", timestamp, err)
			}
		}
		return
	}

	// 处理语音
	if msg.Voice != nil {
		filename := fmt.Sprintf("voice_%s.silk", time.Now().Format("20060102_150405"))
		savePath := filepath.Join(downloadDir, filename)

		if err := os.WriteFile(savePath, msg.Voice.Data, 0o644); err != nil {
			fmt.Printf("[%s] ❌ 保存语音失败: %v\n", timestamp, err)
		} else {
			fmt.Printf("[%s] 🎤 收到语音 from %s → 已保存到 %s (%d bytes)\n",
				timestamp, from, savePath, len(msg.Voice.Data))
		}

		// 如果有转文字，echo 文字内容
		if msg.Voice.Transcript != "" {
			fmt.Printf("[%s] 📝 语音转文字: %s\n", timestamp, msg.Voice.Transcript)
			reply := fmt.Sprintf("🎤 语音转文字: %s", msg.Voice.Transcript)
			if err := client.SendText(ctx, from, reply); err != nil {
				fmt.Printf("[%s] ❌ 回复失败: %v\n", timestamp, err)
			}
		}
		return
	}

	// 处理纯文本 → echo
	if msg.Text != "" {
		fmt.Printf("[%s] 💬 %s: %s\n", timestamp, from, msg.Text)

		reply := "Echo: " + msg.Text
		if err := client.SendText(ctx, from, reply); err != nil {
			fmt.Printf("[%s] ❌ 回复失败: %v\n", timestamp, err)
		}
		return
	}

	fmt.Printf("[%s] ❓ 收到未知类型消息 from %s\n", timestamp, from)
}

func printQRCode(url string) {
	qr, err := qrcode.New(url, qrcode.Medium)
	if err != nil {
		fmt.Printf("二维码生成失败，请手动打开链接扫码:\n%s\n", url)
		return
	}

	fmt.Println(qr.ToSmallString(false))
	fmt.Printf("如果二维码显示异常，请用浏览器打开:\n%s\n", url)
}
