# Terminal Echo Chat

终端 echo bot 示例，支持终端二维码渲染、文本 echo 回复、图片/视频/文件下载保存。

## 运行

```bash
cd examples/terminal-echo-chat
go run .
```

首次运行会在终端显示二维码，用微信扫码登录。凭证自动保存到 `./data/`，后续重启自动恢复。

## 功能

| 消息类型 | 行为 |
|---------|------|
| 文本 | 回复 `Echo: {原文}` |
| 图片 | 保存到 `./downloads/`，回复文件名和大小 |
| 视频 | 保存到 `./downloads/`，回复确认 |
| 文件 | 保存到 `./downloads/`，回复确认 |
| 语音 | 保存到 `./downloads/`，如有转文字则 echo 文字内容 |

## 文件

```
main.go         # 示例代码
data/           # 凭证存储（自动创建，gitignore）
downloads/      # 媒体下载目录（自动创建，gitignore）
go.mod          # 独立模块，依赖 go-qrcode
```
