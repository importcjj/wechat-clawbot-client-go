# wechat-clawbot-client-go

[![Go Reference](https://pkg.go.dev/badge/github.com/importcjj/wechat-clawbot-client-go.svg)](https://pkg.go.dev/github.com/importcjj/wechat-clawbot-client-go)

[中文文档](README.md)

Go SDK for the WeChat iLink Bot API. Integrate WeChat bot capabilities into any Go project — backend services, CLI tools, or customer-service systems.

## Features

- **Generic `Client[T]`** — carry custom business context (DB connections, user info, etc.)
- **QR Code Login** — `Login()` returns a `LoginSession` with QR URL; `Wait()` blocks until scan completes
- **Auto Credential Management** — credentials are saved to Store after login; `Start()` restores them automatically
- **Pluggable Storage** — implement the `Store` interface for Redis, MySQL, etc. Built-in: `MemoryStore`, `FileStore`, `RedisStore`, `MySQLStore`, `SQLiteStore`
- **Event Callbacks** — all callbacks receive `*Client[T]`, providing direct access to client methods and custom state
- **State Machine** — query `client.State()` anytime: `new → logging_in → ready → running ⇄ session_expired → stopped`
- **Multimedia** — send/receive images, videos, files, and voice messages with automatic CDN encryption
- **Typing Indicators** — `SendTyping()` / `CancelTyping()` with automatic ticket caching

## Installation

```bash
go get github.com/importcjj/wechat-clawbot-client-go
```

## Quick Start

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

    // Start loads credentials from Store automatically.
    // First run: no credentials → ErrNotLoggedIn → QR login
    // Subsequent runs: credentials found → long-poll directly
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
// Create (generic, with custom state)
func New[T any](clientID string, state T, s store.Store, opts ...Option[T]) *Client[T]

// Create (shorthand without custom state)
func NewDefault(clientID string, s store.Store, opts ...DefaultOption) *DefaultClient

// Login
func (c *Client[T]) Login(ctx context.Context) (*LoginSession, error)

// Lifecycle
func (c *Client[T]) Start(ctx context.Context) error  // blocks until ctx is cancelled
func (c *Client[T]) Stop()
func (c *Client[T]) State() ClientState
func (c *Client[T]) ClientID() string
func (c *Client[T]) UserState() T
func (c *Client[T]) HasCredentials() bool

// Send
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

| Implementation | Constructor | Use Case |
|----------------|-------------|----------|
| MemoryStore | `store.NewMemoryStore()` | Development / Testing |
| FileStore | `store.NewFileStore(dir)` | CLI / Single machine |
| RedisStore | `store.NewRedisStore(client)` | Distributed deployment |
| SQLiteStore | `store.NewSQLiteStore(db)` | Embedded persistence |
| MySQLStore | `store.NewMySQLStore(db)` | Production server |

### EventHooks

The first parameter of every callback is `*Client[T]`, giving direct access to client methods and custom state:

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

For the simple case without custom state, use `DefaultEventHooks` (no `*Client` in signature):

```go
type DefaultEventHooks struct {
    OnMessage        func(clientID string, msg *Message)
    OnConnected      func(clientID string)
    // ...
}
```

## Examples

### [Terminal Echo Chat](examples/terminal-echo-chat/)

Terminal-based echo bot for quick testing and debugging.

```bash
cd examples/terminal-echo-chat && go run .
```

- QR code rendered in terminal, scan to start
- Text echo, image/video/file download, voice-to-text
- Credentials auto-persisted, no re-login on restart

### [Stateful Bot](examples/stateful-bot/)

Demonstrates `Client[T]` generic usage with custom state in callbacks.

```bash
cd examples/stateful-bot && go run .
```

- Custom `BotState` with per-user message counter
- `EventHooks[*BotState]` callbacks receive `*Client[*BotState]` directly
- Access state via `c.UserState()`, reply via `c.SendText()`

### [Multi-Account Server](examples/multi-account-server/)

Multi-account management dashboard (Go + React + Ant Design) for server-side integration reference.

```bash
# Backend
cd examples/multi-account-server/server && go run .
# Frontend
cd examples/multi-account-server/ui && pnpm install && pnpm dev
```

- Bot list management (add / activate / deactivate / status)
- QR code login popup, auto-restore all bots on restart
- Chat UI: message history, send text/image/file, typing indicators
- Message persistence across restarts

## License

MIT
