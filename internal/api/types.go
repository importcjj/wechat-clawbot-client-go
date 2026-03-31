package api

// Weixin iLink Bot protocol types (JSON over HTTP).
// Mirrors the wire format from the reference TypeScript implementation.

// BaseInfo is attached to every outgoing API request body.
type BaseInfo struct {
	ChannelVersion string `json:"channel_version,omitempty"`
}

// UploadMediaType enumerates media types for CDN upload.
const (
	UploadMediaTypeImage = 1
	UploadMediaTypeVideo = 2
	UploadMediaTypeFile  = 3
	UploadMediaTypeVoice = 4
)

// MessageType identifies the sender role.
const (
	MessageTypeNone = 0
	MessageTypeUser = 1
	MessageTypeBot  = 2
)

// MessageItemType identifies content type within a message.
const (
	MessageItemTypeNone  = 0
	MessageItemTypeText  = 1
	MessageItemTypeImage = 2
	MessageItemTypeVoice = 3
	MessageItemTypeFile  = 4
	MessageItemTypeVideo = 5
)

// MessageState tracks message lifecycle.
const (
	MessageStateNew        = 0
	MessageStateGenerating = 1
	MessageStateFinish     = 2
)

// TypingStatus for sendTyping requests.
const (
	TypingStatusTyping = 1
	TypingStatusCancel = 2
)

// CDNMedia is a CDN media reference embedded in message items.
type CDNMedia struct {
	EncryptQueryParam string `json:"encrypt_query_param,omitempty"`
	AESKey            string `json:"aes_key,omitempty"`
	EncryptType       int    `json:"encrypt_type,omitempty"`
	FullURL           string `json:"full_url,omitempty"`
}

type TextItem struct {
	Text string `json:"text,omitempty"`
}

type ImageItem struct {
	Media       *CDNMedia `json:"media,omitempty"`
	ThumbMedia  *CDNMedia `json:"thumb_media,omitempty"`
	AESKey      string    `json:"aeskey,omitempty"` // hex string, preferred over media.aes_key for inbound
	URL         string    `json:"url,omitempty"`
	MidSize     int       `json:"mid_size,omitempty"`
	ThumbSize   int       `json:"thumb_size,omitempty"`
	ThumbHeight int       `json:"thumb_height,omitempty"`
	ThumbWidth  int       `json:"thumb_width,omitempty"`
	HDSize      int       `json:"hd_size,omitempty"`
}

type VoiceItem struct {
	Media         *CDNMedia `json:"media,omitempty"`
	EncodeType    int       `json:"encode_type,omitempty"`
	BitsPerSample int      `json:"bits_per_sample,omitempty"`
	SampleRate    int       `json:"sample_rate,omitempty"`
	Playtime      int       `json:"playtime,omitempty"` // milliseconds
	Text          string    `json:"text,omitempty"`     // speech-to-text
}

type FileItem struct {
	Media    *CDNMedia `json:"media,omitempty"`
	FileName string    `json:"file_name,omitempty"`
	MD5      string    `json:"md5,omitempty"`
	Len      string    `json:"len,omitempty"` // plaintext bytes as string
}

type VideoItem struct {
	Media       *CDNMedia `json:"media,omitempty"`
	VideoSize   int       `json:"video_size,omitempty"`
	PlayLength  int       `json:"play_length,omitempty"`
	VideoMD5    string    `json:"video_md5,omitempty"`
	ThumbMedia  *CDNMedia `json:"thumb_media,omitempty"`
	ThumbSize   int       `json:"thumb_size,omitempty"`
	ThumbHeight int       `json:"thumb_height,omitempty"`
	ThumbWidth  int       `json:"thumb_width,omitempty"`
}

type RefMessage struct {
	MessageItem *MessageItem `json:"message_item,omitempty"`
	Title       string       `json:"title,omitempty"`
}

type MessageItem struct {
	Type        int          `json:"type,omitempty"`
	CreateTimeMs int64       `json:"create_time_ms,omitempty"`
	UpdateTimeMs int64       `json:"update_time_ms,omitempty"`
	IsCompleted bool         `json:"is_completed,omitempty"`
	MsgID       string       `json:"msg_id,omitempty"`
	RefMsg      *RefMessage  `json:"ref_msg,omitempty"`
	TextItem    *TextItem    `json:"text_item,omitempty"`
	ImageItem   *ImageItem   `json:"image_item,omitempty"`
	VoiceItem   *VoiceItem   `json:"voice_item,omitempty"`
	FileItem    *FileItem    `json:"file_item,omitempty"`
	VideoItem   *VideoItem   `json:"video_item,omitempty"`
}

// WeixinMessage is the unified message structure from getUpdates / sendMessage.
type WeixinMessage struct {
	Seq          int            `json:"seq,omitempty"`
	MessageID    int64          `json:"message_id,omitempty"`
	FromUserID   string         `json:"from_user_id,omitempty"`
	ToUserID     string         `json:"to_user_id,omitempty"`
	ClientID     string         `json:"client_id,omitempty"`
	CreateTimeMs int64          `json:"create_time_ms,omitempty"`
	UpdateTimeMs int64          `json:"update_time_ms,omitempty"`
	DeleteTimeMs int64          `json:"delete_time_ms,omitempty"`
	SessionID    string         `json:"session_id,omitempty"`
	GroupID      string         `json:"group_id,omitempty"`
	MessageType  int            `json:"message_type,omitempty"`
	MessageState int            `json:"message_state,omitempty"`
	ItemList     []*MessageItem `json:"item_list,omitempty"`
	ContextToken string         `json:"context_token,omitempty"`
}

// GetUpdatesReq is the request body for ilink/bot/getupdates.
type GetUpdatesReq struct {
	GetUpdatesBuf string    `json:"get_updates_buf"`
	BaseInfo      *BaseInfo `json:"base_info,omitempty"`
}

// GetUpdatesResp is the response from ilink/bot/getupdates.
type GetUpdatesResp struct {
	Ret                 *int             `json:"ret,omitempty"`
	ErrCode             *int             `json:"errcode,omitempty"`
	ErrMsg              string           `json:"errmsg,omitempty"`
	Msgs                []*WeixinMessage `json:"msgs,omitempty"`
	GetUpdatesBuf       string           `json:"get_updates_buf,omitempty"`
	LongPollingTimeoutMs *int            `json:"longpolling_timeout_ms,omitempty"`
}

// SendMessageReq wraps a single WeixinMessage for ilink/bot/sendmessage.
type SendMessageReq struct {
	Msg      *WeixinMessage `json:"msg,omitempty"`
	BaseInfo *BaseInfo      `json:"base_info,omitempty"`
}

// GetUploadURLReq is the request body for ilink/bot/getuploadurl.
type GetUploadURLReq struct {
	FileKey       string    `json:"filekey,omitempty"`
	MediaType     int       `json:"media_type,omitempty"`
	ToUserID      string    `json:"to_user_id,omitempty"`
	RawSize       int       `json:"rawsize,omitempty"`
	RawFileMD5    string    `json:"rawfilemd5,omitempty"`
	FileSize      int       `json:"filesize,omitempty"`
	ThumbRawSize  int       `json:"thumb_rawsize,omitempty"`
	ThumbRawMD5   string    `json:"thumb_rawfilemd5,omitempty"`
	ThumbFileSize int       `json:"thumb_filesize,omitempty"`
	NoNeedThumb   bool      `json:"no_need_thumb,omitempty"`
	AESKey        string    `json:"aeskey,omitempty"`
	BaseInfo      *BaseInfo `json:"base_info,omitempty"`
}

// GetUploadURLResp is the response from ilink/bot/getuploadurl.
type GetUploadURLResp struct {
	UploadParam      string `json:"upload_param,omitempty"`
	ThumbUploadParam string `json:"thumb_upload_param,omitempty"`
	UploadFullURL    string `json:"upload_full_url,omitempty"`
}

// GetConfigReq is the request body for ilink/bot/getconfig.
type GetConfigReq struct {
	ILinkUserID  string    `json:"ilink_user_id,omitempty"`
	ContextToken string    `json:"context_token,omitempty"`
	BaseInfo     *BaseInfo `json:"base_info,omitempty"`
}

// GetConfigResp is the response from ilink/bot/getconfig.
type GetConfigResp struct {
	Ret          *int   `json:"ret,omitempty"`
	ErrMsg       string `json:"errmsg,omitempty"`
	TypingTicket string `json:"typing_ticket,omitempty"`
}

// SendTypingReq is the request body for ilink/bot/sendtyping.
type SendTypingReq struct {
	ILinkUserID  string    `json:"ilink_user_id,omitempty"`
	TypingTicket string    `json:"typing_ticket,omitempty"`
	Status       int       `json:"status,omitempty"`
	BaseInfo     *BaseInfo `json:"base_info,omitempty"`
}

// QRCodeResp is the response from ilink/bot/get_bot_qrcode.
type QRCodeResp struct {
	QRCode        string `json:"qrcode"`
	QRCodeImgContent string `json:"qrcode_img_content"`
}

// QRStatusResp is the response from ilink/bot/get_qrcode_status.
type QRStatusResp struct {
	Status       string `json:"status"` // wait, scaned, confirmed, expired, scaned_but_redirect
	BotToken     string `json:"bot_token,omitempty"`
	ILinkBotID   string `json:"ilink_bot_id,omitempty"`
	BaseURL      string `json:"baseurl,omitempty"`
	ILinkUserID  string `json:"ilink_user_id,omitempty"`
	RedirectHost string `json:"redirect_host,omitempty"`
}
