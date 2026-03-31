# Multi-Account Server

多账号微信 bot 管理后台，Go 后端 + React 前端（Vite + Ant Design）。

## 运行

```bash
# Terminal 1: 启动后端
cd server
go run .

# Terminal 2: 启动前端
cd ui
pnpm install
pnpm dev
```

后端监听 `localhost:9099`，前端 Vite dev server 自动代理 `/api` 到后端。

## 功能

### Bot 管理
- 添加 bot（输入 ID，点 Add）
- 激活 bot（无凭证时弹出 QR 码扫码登录，有凭证自动恢复）
- 停用 bot（Deactivate）
- 查看状态（running / session_expired / stopped / logging_in）
- 重启自动恢复所有已激活的 bot

### 聊天
- 查看与绑定用户的消息历史（持久化，重启不丢失）
- 发送文本消息
- 发送图片（自动识别图片类型走 CDN 上传）
- 发送文件（非图片文件走文件附件通道）
- 输入状态指示器（typing indicator）
- 接收消息中的图片预览、文件信息展示

## API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/bots` | 列出所有 bot 及状态 |
| POST | `/api/bots/:id/activate` | 激活 bot（恢复凭证或 QR 登录） |
| POST | `/api/bots/:id/deactivate` | 停用 bot |
| GET | `/api/bots/:id/state` | 查询 bot 状态 |
| GET | `/api/bots/:id/messages` | 获取消息历史 |
| POST | `/api/bots/:id/send` | 发送文本消息 |
| POST | `/api/bots/:id/send-image` | 发送图片（base64） |
| POST | `/api/bots/:id/send-file` | 发送文件（base64） |
| POST | `/api/bots/:id/typing` | 发送/取消输入状态 |

## 文件结构

```
server/
  main.go       # Go 后端（BotManager + MessageStore + HTTP API）
  go.mod        # 独立模块
ui/
  src/
    api.ts      # API client
    App.tsx     # 布局：左侧 bot 列表 | 右侧聊天
    BotList.tsx # Bot 列表 + 添加 + QR 码弹窗
    ChatPanel.tsx # 消息历史 + 发送文本/图片/文件 + typing
  vite.config.ts # /api 代理到后端
```

## 数据存储

```
server/data/
  bots.json                    # 已激活 bot ID 列表（server 管理）
  {clientID}.creds.json        # 凭证（SDK FileStore 管理）
  {clientID}.sync.json         # 长轮询断点
  {clientID}.tokens.json       # context token
  {clientID}.messages.json     # 消息历史
```
