# wechat-clawbot-client-go

[![Go Reference](https://pkg.go.dev/badge/github.com/importcjj/wechat-clawbot-client-go.svg)](https://pkg.go.dev/github.com/importcjj/wechat-clawbot-client-go)

微信 iLink Bot API 的 Go SDK。将微信 bot 能力集成到任意 Go 项目中 —— 后端服务、CLI 工具、聊天客服系统。

## 特性

- **泛型 `Client[T]`** — 携带自定义业务上下文（数据库连接、用户信息等）
- **QR 码登录** — `Login()` 返回含 QR URL 的 `LoginSession`；`Wait()` 阻塞直到扫码完成
- **凭证自动管理** — 登录后自动保存到 Store；`Start()` 自动从 Store 恢复
- **可插拔存储** — 实现 `Store` 接口即可对接 Redis、MySQL 等。内置 `MemoryStore`、`FileStore`、`RedisStore`、`MySQLStore`、`SQLiteStore`
- **事件回调** — 所有回调携带 `*Client[T]`，直接访问客户端方法和自定义状态
- **状态机** — 随时查询 `client.State()`：`new → logging_in → ready → running ⇄ session_expired → stopped`
- **多媒体支持** — 收发图片、视频、文件、语音，CDN 加密自动处理
- **输入状态** — `SendTyping()` / `CancelTyping()`，自动缓存 typing ticket

## 安装

```bash
go get github.com/importcjj/wechat-clawbot-client-go
```

## 快速开始

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "os/signal"

    clawbot "github.com/importcjj/wechat-clawbot-client-go"
    "github.com/importcjj/wechat-clawbot-client-go/store"
)

func main() {
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
    defer stop()

    client := clawbot.NewDefault("my-bot", store.NewFileStore("./data"),
        clawbot.WithEventHooks(clawbot.EventHooks[struct{}]{
            OnMessage: func(c *clawbot.DefaultClient, msg *clawbot.Message) {
                fmt.Printf("%s: %s\n", msg.From, msg.Text)
                c.SendText(ctx, msg.From, "Echo: "+msg.Text)
            },
            OnConnected: func(c *clawbot.DefaultClient) { fmt.Println("Connected!") },
        }),
    )

    // Start 自动从 Store 加载凭证
    // 首次运行: 无凭证 → ErrNotLoggedIn → 扫码登录
    // 后续运行: 有凭证 → 直接长轮询
    err := client.Start(ctx)
    if err == clawbot.ErrNotLoggedIn {
        session, _ := client.Login(ctx)
        fmt.Println("Scan QR:", session.QRCodeURL())
        session.Wait(ctx)
        client.Start(ctx)
    }
}
```

## API

### Client

```go
// 创建（泛型，携带自定义状态）
func New[T any](clientID string, state T, s store.Store, opts ...Option[T]) *Client[T]

// 创建（无自定义状态的简写）
func NewDefault(clientID string, s store.Store, opts ...DefaultOption) *DefaultClient

// 登录
func (c *Client[T]) Login(ctx context.Context) (*LoginSession, error)

// 生命周期
func (c *Client[T]) Start(ctx context.Context) error  // 阻塞直到 ctx 取消
func (c *Client[T]) Stop()
func (c *Client[T]) State() ClientState
func (c *Client[T]) ClientID() string
func (c *Client[T]) UserState() T
func (c *Client[T]) HasCredentials() bool

// 发送
func (c *Client[T]) SendText(ctx context.Context, to, text string) error
func (c *Client[T]) SendImage(ctx context.Context, to string, data []byte, caption string) error
func (c *Client[T]) SendVideo(ctx context.Context, to string, data []byte, caption string) error
func (c *Client[T]) SendFile(ctx context.Context, to string, data []byte, filename, caption string) error
func (c *Client[T]) SendTyping(ctx context.Context, to string) error
func (c *Client[T]) CancelTyping(ctx context.Context, to string) error
```

### Store

```go
type Store interface {
    CredentialStore   // SaveCredentials / LoadCredentials / DeleteCredentials
    SyncStore         // SaveSyncBuf / LoadSyncBuf
    ContextTokenStore // SaveContextToken / LoadContextToken
}
```

| 实现 | 构造函数 | 适用场景 |
|------|---------|---------|
| MemoryStore | `store.NewMemoryStore()` | 开发测试 |
| FileStore | `store.NewFileStore(dir)` | CLI / 单机 |
| RedisStore | `store.NewRedisStore(client)` | 分布式部署 |
| SQLiteStore | `store.NewSQLiteStore(db)` | 嵌入式持久化 |
| MySQLStore | `store.NewMySQLStore(db)` | 生产服务端 |

### EventHooks

回调的第一个参数是 `*Client[T]`，可以直接调用客户端方法和访问自定义状态：

```go
type EventHooks[T any] struct {
    OnMessage        func(client *Client[T], msg *Message)
    OnQRCode         func(client *Client[T], qrCodeURL string)
    OnQRScanned      func(client *Client[T])
    OnQRExpired      func(client *Client[T], refreshCount int)
    OnConnected      func(client *Client[T])
    OnSessionExpired func(client *Client[T])
    OnDisconnected   func(client *Client[T], err error)
    OnError          func(client *Client[T], err error)
}
```

不需要自定义状态时，可用 `DefaultEventHooks`（签名不含 `*Client`，保持简洁）：

```go
type DefaultEventHooks struct {
    OnMessage        func(clientID string, msg *Message)
    OnConnected      func(clientID string)
    // ...
}
```

## 示例

### [Terminal Echo Chat](examples/terminal-echo-chat/)

终端 echo bot，适合快速体验和调试。

```bash
cd examples/terminal-echo-chat && go run .
```

- 终端渲染二维码，扫码即用
- 文本 echo、图片/视频/文件下载保存、语音转文字
- 凭证自动持久化，重启免登录

### [Stateful Bot](examples/stateful-bot/)

展示 `Client[T]` 泛型用法，在回调中直接访问自定义状态。

```bash
cd examples/stateful-bot && go run .
```

- 自定义 `BotState` 携带 per-user 消息计数器
- `EventHooks[*BotState]` 回调直接拿到 `*Client[*BotState]`
- 通过 `c.UserState()` 访问状态，`c.SendText()` 回复

### [Multi-Account Server](examples/multi-account-server/)

多账号管理后台（Go + React + Ant Design），适合服务端集成参考。

```bash
# 后端
cd examples/multi-account-server/server && go run .
# 前端
cd examples/multi-account-server/ui && pnpm install && pnpm dev
```

- Bot 列表管理（添加/激活/停用/状态查询）
- QR 码扫码登录弹窗，重启自动恢复所有 bot
- 聊天界面：消息历史、发送文本/图片/文件、typing 状态
- 消息持久化，服务重启不丢失

## 许可证

MIT
