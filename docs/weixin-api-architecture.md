# WeChat iLink Bot API Architecture

> 微信 iLink Bot HTTP API 交互逻辑，用于 Go 版本实现参考。
> 原始参考实现: `openclaw-weixin-src/`

---

## 1. 概览

该系统通过微信 iLink Bot HTTP API 实现 bot 与微信用户的双向消息通信。核心流程：

```
QR码登录 → 获取 bot_token → 长轮询收消息 → 处理 → 回复消息
                                ↕
                          CDN 媒体上传/下载（AES-128-ECB 加密）
```

## 2. 基础配置

### 2.1 服务端点

| 用途 | URL |
|------|-----|
| API 基础地址 | `https://ilinkai.weixin.qq.com` |
| CDN 基础地址 | `https://novac2c.cdn.weixin.qq.com/c2c` |

> 登录确认后服务端可能返回不同的 `baseurl`，后续 API 调用应使用该地址。

### 2.2 公共请求头

所有请求（GET/POST）共享：

```
iLink-App-Id: bot                          // package.json 中的 ilink_appid
iLink-App-ClientVersion: 131329            // 版本号编码为 uint32: major<<16 | minor<<8 | patch
SKRouteTag: {可选路由标签}                   // 从配置文件读取
```

POST 请求额外添加：

```
Content-Type: application/json
Authorization: Bearer {bot_token}
AuthorizationType: ilink_bot_token
Content-Length: {body 字节长度}
X-WECHAT-UIN: {random_uint32 的十进制字符串再 base64 编码}
```

### 2.3 版本号编码

```
version = "2.1.1"
clientVersion = (2 << 16) | (1 << 8) | 1 = 0x00020101 = 131329
```

### 2.4 BaseInfo

每个 POST 请求 body 中都包含：

```json
{
  "base_info": {
    "channel_version": "2.1.1"
  }
}
```

## 3. 认证流程（QR 码登录）

### 3.1 获取二维码

```
GET https://ilinkai.weixin.qq.com/ilink/bot/get_bot_qrcode?bot_type=3
Timeout: 5s
Headers: 公共 GET 头

Response:
{
  "qrcode": "二维码内容字符串",
  "qrcode_img_content": "二维码图片URL"
}
```

### 3.2 轮询登录状态

```
GET {currentBaseUrl}/ilink/bot/get_qrcode_status?qrcode={qrcode}
Timeout: 35s（长轮询）
Headers: 公共 GET 头

Response:
{
  "status": "wait" | "scaned" | "confirmed" | "expired" | "scaned_but_redirect",
  "bot_token": "登录成功后的令牌",
  "ilink_bot_id": "bot账号ID",
  "baseurl": "后续API使用的基础URL",
  "ilink_user_id": "扫码用户ID",
  "redirect_host": "IDC重定向主机名"
}
```

### 3.3 登录状态机

```
                  ┌──────────────────────────────────────┐
                  │                                      │
                  ▼                                      │
  ┌─────────┐  轮询  ┌──────┐  扫码  ┌─────────┐  确认  ┌───────────┐
  │ 获取QR码 │──────→│ wait │──────→│ scaned  │──────→│ confirmed │
  └─────────┘       └──────┘       └─────────┘       └───────────┘
       ▲               │                                    │
       │               ▼                                    │ 返回:
       │          ┌─────────┐                               │ bot_token
       └──────────│ expired │                               │ ilink_bot_id
     最多3次刷新   └─────────┘                               │ baseurl
                                                            │ ilink_user_id
                  ┌─────────────────────┐                   │
                  │ scaned_but_redirect │                   │
                  └─────────────────────┘                   │
                       │                                    │
                       │ 切换 polling host                   │
                       │ 到 redirect_host                    │
                       └────────────────────────────────────┘
```

- 超时后回退为 `wait`，继续轮询
- `expired` 时自动刷新 QR 码，最多 3 次
- `scaned_but_redirect` 时切换轮询地址到 `https://{redirect_host}`
- 整体登录超时：480s（8 分钟）
- QR 码有效期：5 分钟

### 3.4 凭证存储

登录成功后需持久化以下数据，实现者可自行选择存储方式（文件、数据库、配置中心等）：

**账号列表**: 所有已注册的 accountId

**账号凭证** (per account):

```json
{
  "token": "bot_token",
  "savedAt": "2026-03-27T10:00:00.000Z",
  "baseUrl": "https://...",
  "userId": "扫码用户ID"
}
```

> 凭证包含敏感令牌，存储时应注意权限控制。

### 3.5 账号 ID 归一化

原始 ID 如 `b0f5860fdecb@im.bot` 归一化为安全格式 `b0f5860fdecb-im-bot`（`@` → `-`，`.` → `-`），用于存储 key 或标识符。

## 4. 消息收发

### 4.1 长轮询收消息（GetUpdates）

```
POST {baseUrl}/ilink/bot/getupdates
Timeout: 35s（可由服务端 longpolling_timeout_ms 调整）

Request:
{
  "get_updates_buf": "上次返回的同步状态，首次为空字符串",
  "base_info": { "channel_version": "2.1.1" }
}

Response:
{
  "ret": 0,                           // 0=成功
  "errcode": 0,                       // 错误码，-14=会话过期
  "errmsg": "",
  "msgs": [WeixinMessage, ...],       // 消息数组
  "get_updates_buf": "新的同步状态",    // 必须缓存，下次请求带上
  "longpolling_timeout_ms": 35000     // 服务端建议的下次超时
}
```

客户端超时（AbortError）视为正常，返回空响应继续重试。

### 4.2 WeixinMessage 结构

```json
{
  "seq": 1,
  "message_id": 12345,
  "from_user_id": "hex@im.wechat",
  "to_user_id": "hex@im.bot",
  "client_id": "唯一消息ID",
  "create_time_ms": 1711540800000,
  "update_time_ms": 1711540800000,
  "session_id": "会话ID",
  "group_id": "群组ID",
  "message_type": 1,          // 1=USER, 2=BOT
  "message_state": 0,         // 0=NEW, 1=GENERATING, 2=FINISH
  "item_list": [MessageItem],
  "context_token": "必须在回复时原样回传"
}
```

### 4.3 MessageItem 类型

| type | 名称 | 数据字段 | 关键属性 |
|------|------|----------|----------|
| 0 | NONE | - | - |
| 1 | TEXT | `text_item` | `text: string` |
| 2 | IMAGE | `image_item` | `media: CDNMedia`, `aeskey: hex_string`(优先), `thumb_media` |
| 3 | VOICE | `voice_item` | `media: CDNMedia`, `encode_type`, `sample_rate`, `playtime`, `text`(语音转文字) |
| 4 | FILE | `file_item` | `media: CDNMedia`, `file_name`, `len: string`(字节数) |
| 5 | VIDEO | `video_item` | `media: CDNMedia`, `video_size`, `play_length`, `thumb_media` |

每个 MessageItem 还可包含 `ref_msg`（引用消息）：

```json
{
  "ref_msg": {
    "message_item": { ... },   // 被引用的消息内容
    "title": "摘要文字"
  }
}
```

### 4.4 CDNMedia 结构

```json
{
  "encrypt_query_param": "下载URL参数",
  "aes_key": "base64编码的AES密钥",
  "encrypt_type": 1,           // 0=只加密fileid, 1=打包信息
  "full_url": "完整下载URL（优先使用）"
}
```

### 4.5 发送消息

```
POST {baseUrl}/ilink/bot/sendmessage
Timeout: 15s

Request:
{
  "msg": {
    "from_user_id": "",                // 发送方留空
    "to_user_id": "目标用户ID",
    "client_id": "{前缀}-{随机ID}",           // 原始实现前缀为 "openclaw-weixin"
    "message_type": 2,                 // BOT
    "message_state": 2,                // FINISH
    "item_list": [单个MessageItem],     // 每次只发一个item
    "context_token": "从入站消息中获取"
  },
  "base_info": { "channel_version": "2.1.1" }
}
```

**重要**: 每个 `item_list` 只包含一个 item。如果要发文字+图片，分两次请求发送（先文字后图片）。

### 4.6 Context Token 管理

- 每条入站消息携带 `context_token`
- 必须在回复同一用户时原样回传
- 内存缓存 + 持久化存储（确保重启后可恢复）
- 数据结构: `map[userId]contextToken`，按 accountId 隔离

### 4.7 Typing 指示器

#### 获取 Typing Ticket

```
POST {baseUrl}/ilink/bot/getconfig
Timeout: 10s

Request:
{
  "ilink_user_id": "目标用户ID",
  "context_token": "当前context_token",
  "base_info": { ... }
}

Response:
{
  "ret": 0,
  "typing_ticket": "base64编码的ticket"
}
```

#### 发送 Typing 状态

```
POST {baseUrl}/ilink/bot/sendtyping
Timeout: 10s

Request:
{
  "ilink_user_id": "目标用户ID",
  "typing_ticket": "从getconfig获取",
  "status": 1,    // 1=正在输入, 2=取消
  "base_info": { ... }
}
```

#### Typing Ticket 缓存策略

- 24h TTL，随机刷新时间（`Math.random() * 24h`）
- 失败时指数退避重试：2s → 4s → 8s → ... → 最大 1h
- 每个用户独立缓存

## 5. CDN 媒体系统

### 5.1 加密方案

- 算法: **AES-128-ECB**，PKCS7 填充
- 密钥: 随机生成 16 字节
- 密文大小: `ceil((plaintext_size + 1) / 16) * 16`

### 5.2 上传流程

```
1. 读取文件 → 计算 MD5 → 生成随机 16 字节 AES key 和 filekey

2. 获取上传 URL:
   POST {baseUrl}/ilink/bot/getuploadurl
   {
     "filekey": "32字符hex",
     "media_type": 1,              // 1=IMAGE, 2=VIDEO, 3=FILE, 4=VOICE
     "to_user_id": "目标用户",
     "rawsize": 原文件字节数,
     "rawfilemd5": "原文件MD5 hex",
     "filesize": AES加密后字节数,
     "no_need_thumb": true,
     "aeskey": "AES key hex编码",
     "base_info": { ... }
   }

   Response:
   {
     "upload_param": "加密的上传参数",
     "upload_full_url": "完整上传URL（优先使用）"
   }

3. 上传加密文件到 CDN:
   POST {upload_full_url 或 拼接URL}
   Content-Type: application/octet-stream
   Body: AES-128-ECB 加密后的字节流

   拼接URL格式:
   {cdnBaseUrl}/upload?encrypted_query_param={uploadParam}&filekey={filekey}

   Response Header:
   x-encrypted-param: "下载用的加密参数"

4. 重试策略:
   - 最多 3 次
   - 4xx 立即失败
   - 5xx 重试
```

### 5.3 下载解密流程

```
1. 确定下载 URL:
   - 优先使用 CDNMedia.full_url
   - 回退拼接: {cdnBaseUrl}/download?encrypted_query_param={encrypt_query_param}

2. 解析 AES key（两种编码格式）:
   - base64 解码得 16 字节 → 直接作为 key
   - base64 解码得 32 字节 ASCII → 当作 hex 字符串再解码为 16 字节 key

3. GET 下载 → AES-128-ECB 解密 → 明文

4. 特殊处理:
   - IMAGE: aeskey 优先从 image_item.aeskey（hex string）取，
            转换方式: hex → raw bytes → base64
   - VOICE: 解密后是 SILK 格式，需转码为 WAV
   - 无 aes_key 时直接下载（不加密的情况）
```

### 5.4 发送媒体消息

上传完成后构造对应的 MessageItem 发送：

#### 图片

```json
{
  "type": 2,
  "image_item": {
    "media": {
      "encrypt_query_param": "CDN下载参数",
      "aes_key": "aeskey hex字符串的 base64 编码",
      "encrypt_type": 1
    },
    "mid_size": "密文大小（字节）"
  }
}
```

#### 视频

```json
{
  "type": 5,
  "video_item": {
    "media": {
      "encrypt_query_param": "...",
      "aes_key": "...",
      "encrypt_type": 1
    },
    "video_size": "密文大小"
  }
}
```

#### 文件

```json
{
  "type": 4,
  "file_item": {
    "media": {
      "encrypt_query_param": "...",
      "aes_key": "...",
      "encrypt_type": 1
    },
    "file_name": "原始文件名",
    "len": "明文大小（字符串）"
  }
}
```

### 5.5 媒体发送路由

根据 MIME 类型自动选择：

```
video/*  → uploadVideoToWeixin  + sendVideoMessageWeixin
image/*  → uploadFileToWeixin   + sendImageMessageWeixin
其他     → uploadFileAttachment + sendFileMessageWeixin
```

最大文件: 100 MB。

## 6. 监控循环（Monitor Loop）

### 6.1 主循环逻辑

```
初始化:
  - 从存储加载 get_updates_buf（或空字符串）
  - 初始化 ConfigManager（typing ticket 缓存）
  - consecutiveFailures = 0

循环 (直到 abort):
  1. 调用 getUpdates(get_updates_buf, timeout)

  2. 检查错误:
     - errcode=-14 (会话过期):
       → 暂停该账号所有请求 1 小时
       → sleep 60s 后继续
     - 其他 API 错误:
       → consecutiveFailures++
       → ≥3 次连续失败 → 退避 30s
       → 否则 → 重试延迟 2s

  3. 成功:
     → 重置 consecutiveFailures
     → 保存新的 get_updates_buf 到存储
     → 更新服务端建议的超时时间
     → 逐条处理消息

  4. 处理每条消息:
     → 获取 typing ticket（从缓存或 getConfig API）
     → 提取文本和媒体
     → 下载解密媒体文件
     → 分发处理
     → 存储 context_token
```

### 6.2 同步状态持久化

每个账号需持久化 `get_updates_buf`（字符串），确保重启后从上次位置继续拉取消息，避免消息重复或丢失。

### 6.3 超时配置

| 场景 | 超时(ms) |
|------|----------|
| 长轮询 getUpdates | 35,000 |
| 常规 API (sendMessage, getUploadUrl) | 15,000 |
| 轻量 API (getConfig, sendTyping) | 10,000 |
| QR 码获取 | 5,000 |
| QR 状态轮询 | 35,000 |
| 登录整体等待 | 480,000（8 分钟） |

## 7. 错误处理

### 7.1 API 响应错误格式

所有 POST API 共用统一错误格式：

```json
{
  "ret": -14,          // 返回码，0=成功，非 0=失败
  "errcode": -14,      // 错误码（可能与 ret 相同或独立）
  "errmsg": "session timeout"
}
```

判断 API 错误: `ret !== 0 || errcode !== 0`（任一非零即为错误）。

### 7.2 已知错误码

| 错误码 | 含义 | 出现场景 | 处理方式 |
|--------|------|----------|----------|
| `-14` | 会话过期 (SESSION_EXPIRED) | getUpdates 响应的 `ret` 或 `errcode` | 暂停该账号所有 API 调用 1 小时，**无自动重登录**，需用户手动重新扫码 |

> 目前源码中仅显式处理了 `-14` 这一个错误码，其他非零错误码统一走通用重试逻辑。

### 7.3 会话过期保护机制

```
检测: getUpdates 响应中 ret === -14 或 errcode === -14
   ↓
暂停: pauseUntilMap[accountId] = now + 1h
   ↓
阻断: 后续所有 API 调用前检查 pauseUntilMap，暂停期内直接拒绝
   ↓
恢复: 1 小时后自动解除暂停，重新发起 getUpdates
   ↓
仍失败: 如果仍然返回 -14，再次暂停 1 小时（循环）
```

**重要**: 原始实现**没有自动重新登录**逻辑。会话过期后需要用户主动触发 QR 码登录流程。Go 实现可考虑加入回调通知机制，让上层应用决定是否发起重登录。

### 7.4 HTTP 状态码处理

#### API 请求（iLink 接口）

| HTTP 状态 | 处理 |
|-----------|------|
| 200 | 解析 JSON 响应，进一步检查 `ret`/`errcode` |
| 4xx/5xx | 抛出异常: `"{label} {status}: {responseBody}"`，由调用方重试逻辑处理 |

#### CDN 上传

| HTTP 状态 | 处理 | 是否重试 |
|-----------|------|----------|
| 200 | 成功，读取 `x-encrypted-param` 响应头 | - |
| 200 但缺少 `x-encrypted-param` 头 | 视为失败 | 是（最多 3 次） |
| 400-499 | 客户端错误，读取 `x-error-message` 响应头或 body | **不重试，立即失败** |
| 5xx / 其他非 200 | 服务端错误，读取 `x-error-message` 响应头 | 是（最多 3 次） |

#### CDN 下载

| HTTP 状态 | 处理 |
|-----------|------|
| 200 | 返回 body 字节流 |
| 非 200 | 抛出异常: `"CDN download {status} {statusText} body={body}"` |
| 网络错误 | 记录 error.cause/code，直接抛出 |

### 7.5 网络错误处理

| 错误类型 | 出现场景 | 处理 |
|----------|----------|------|
| 客户端超时 (AbortError) | getUpdates 长轮询 | **正常行为**，返回空响应 `{ret:0, msgs:[]}` 继续下轮轮询 |
| 客户端超时 (AbortError) | QR 状态轮询 | 返回 `{status:"wait"}`，继续轮询 |
| 网关超时 (如 Cloudflare 524) | QR 状态轮询 | 视为 `wait` 状态，继续轮询 |
| 网络不可达 / 连接拒绝 | CDN 下载 | 记录 error.cause/code，抛出异常 |
| 其他网络错误 | 监控循环中任何请求 | 触发连续失败计数，走退避逻辑 |

### 7.6 QR 码登录错误

| 错误场景 | 处理 |
|----------|------|
| 获取 QR 码请求失败 | 返回错误消息，不抛异常 |
| 轮询超时 (AbortError) | 视为 `wait`，继续轮询 |
| 网关/网络错误 | 视为 `wait`，继续轮询 |
| QR 码过期 (`expired`) | 自动刷新，最多 3 次 |
| QR 码刷新失败 | 返回 `connected:false`，终止流程 |
| 刷新 3 次仍过期 | 返回 `connected:false`，提示"登录超时" |
| `confirmed` 但缺少 `ilink_bot_id` | 返回 `connected:false`，提示"登录失败" |
| 整体超时 (默认 8 分钟) | 返回 `connected:false`，提示"登录超时" |

### 7.7 媒体下载/解密错误

| 错误场景 | 处理 |
|----------|------|
| CDN 下载失败 | 记录日志，跳过该媒体，消息正文仍正常处理 |
| AES key 格式错误 (非 16/32 字节) | 抛出异常: `"aes_key must decode to 16 raw bytes or 32-char hex string"` |
| SILK 转码失败 | 降级保存原始 SILK 格式 |
| 远程图片下载失败 | 抛出: `"remote media download failed: {status} {statusText}"` |

### 7.8 消息发送错误

| 错误场景 | 处理 |
|----------|------|
| sendMessage API 失败 | 记录日志，抛出异常由上层处理 |
| 多 item 发送中间失败 | 停止发送剩余 item，抛出异常 |
| 上层收到发送异常 | 根据错误消息分类，向用户发送友好提示 |

错误消息分类与用户提示：

| 错误关键词匹配 | 用户提示 |
|----------------|----------|
| `"remote media download failed"` 或 `"fetch"` | "媒体文件下载失败，请检查链接是否可访问" |
| `"getUploadUrl"` 或 `"CDN upload"` 或 `"upload_param"` | "媒体文件上传失败，请稍后重试" |
| 其他 | "消息发送失败：{错误信息}" |

> 向用户发送错误提示本身如果也失败，静默忽略（fire-and-forget），避免错误循环。

### 7.9 退避策略汇总

| 场景 | 策略 | 常量 |
|------|------|------|
| getUpdates 返回非 -14 错误 | 2s 后重试 | `RETRY_DELAY_MS = 2000` |
| getUpdates 连续 3 次失败 | 退避 30s，重置计数器 | `BACKOFF_DELAY_MS = 30000` |
| getUpdates 返回 -14 | 暂停 1 小时 | `SESSION_PAUSE_DURATION_MS = 3600000` |
| getConfig (typing ticket) 失败 | 指数退避 2s → 4s → 8s → ... | 最大 1h，成功后 24h TTL 随机刷新 |
| CDN 上传失败 (5xx) | 立即重试，最多 3 次 | `UPLOAD_MAX_RETRIES = 3` |
| CDN 上传失败 (4xx) | **不重试**，立即失败 | - |
| 监控循环中 uncaught exception | 同 getUpdates 非 -14 错误逻辑 | 2s / 30s |

## 8. 持久化数据一览

以下数据需要持久化存储（存储方式由实现者决定: 文件、SQLite、Redis 等）：

| 数据 | 粒度 | 内容 | 用途 |
|------|------|------|------|
| 账号列表 | 全局 | `[]accountId` | 管理所有已注册的 bot 账号 |
| 账号凭证 | per account | token, baseUrl, userId, savedAt | 认证和 API 调用 |
| 同步状态 | per account | `get_updates_buf` (string) | 长轮询断点续传 |
| Context Token | per account | `map[userId]token` | 回复消息时回传 |

## 9. Go Port 实现要点

### 9.1 加密

```go
// AES-128-ECB（Go 标准库没有 ECB 模式，需手动实现）
func encryptAESECB(plaintext, key []byte) []byte {
    block, _ := aes.NewCipher(key)
    // PKCS7 填充
    padLen := aes.BlockSize - len(plaintext)%aes.BlockSize
    padded := append(plaintext, bytes.Repeat([]byte{byte(padLen)}, padLen)...)
    ciphertext := make([]byte, len(padded))
    for i := 0; i < len(padded); i += aes.BlockSize {
        block.Encrypt(ciphertext[i:i+aes.BlockSize], padded[i:i+aes.BlockSize])
    }
    return ciphertext
}
```

### 9.2 AES Key 解析（下载时）

```go
func parseAESKey(aesKeyBase64 string) ([]byte, error) {
    decoded, _ := base64.StdEncoding.DecodeString(aesKeyBase64)
    if len(decoded) == 16 {
        return decoded, nil  // 直接是 raw key
    }
    if len(decoded) == 32 {
        // hex 编码的 key
        return hex.DecodeString(string(decoded))
    }
    return nil, fmt.Errorf("invalid aes_key length: %d", len(decoded))
}
```

### 9.3 X-WECHAT-UIN 头

```go
func randomWechatUin() string {
    buf := make([]byte, 4)
    rand.Read(buf)
    uint32Val := binary.BigEndian.Uint32(buf)
    return base64.StdEncoding.EncodeToString([]byte(strconv.FormatUint(uint64(uint32Val), 10)))
}
```

### 9.4 关键设计模式

1. **长轮询**: `http.Client` + `context.WithTimeout`，超时返回空响应继续重试
2. **并发**: 监控循环跑在 goroutine，通过 `context.Context` 控制生命周期
3. **持久化**: 通过 `Store` 接口抽象，实现者可选文件/数据库/KV 等
4. **重试**: CDN 上传最多 3 次，4xx 立即放弃，5xx 重试
5. **Client ID 生成**: `{自定义前缀}-{随机字符串}`，需保证唯一性

## 10. API 端点汇总

| 方法 | 路径 | 用途 | 超时 |
|------|------|------|------|
| GET | `ilink/bot/get_bot_qrcode` | 获取登录二维码 | 5s |
| GET | `ilink/bot/get_qrcode_status` | 轮询登录状态 | 35s |
| POST | `ilink/bot/getupdates` | 长轮询收消息 | 35s |
| POST | `ilink/bot/sendmessage` | 发送消息 | 15s |
| POST | `ilink/bot/getuploadurl` | 获取CDN上传URL | 15s |
| POST | `ilink/bot/getconfig` | 获取typing ticket | 10s |
| POST | `ilink/bot/sendtyping` | 发送输入状态 | 10s |
| POST | `{cdn}/upload` | CDN 上传文件 | - |
| GET | `{cdn}/download` | CDN 下载文件 | - |
