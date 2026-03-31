package monitor

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/importcjj/wechat-clawbot-client-go/internal/api"
	"github.com/importcjj/wechat-clawbot-client-go/internal/cdn"
	"github.com/importcjj/wechat-clawbot-client-go/store"
)

const (
	maxConsecutiveFailures = 3
	backoffDelay           = 30 * time.Second
	retryDelay             = 2 * time.Second
)

// Message is the SDK's public message type, built from WeixinMessage.
type Message struct {
	ClientID  string
	MessageID int64
	From      string
	To        string
	MsgClientID string
	SessionID string
	CreatedAt time.Time

	Text   string
	Images []MediaAttachment
	Voice  *VoiceAttachment
	Files  []MediaAttachment
	Videos []MediaAttachment
	Ref    *RefMessage

	Raw json.RawMessage
}

// MediaAttachment holds decoded media data.
type MediaAttachment struct {
	Data        []byte
	Filename    string
	ContentType string
}

// VoiceAttachment holds decoded voice data.
type VoiceAttachment struct {
	MediaAttachment
	Duration   int
	Transcript string
	Format     string
}

// RefMessage holds quoted message info.
type RefMessage struct {
	Text  string
	Title string
}

// Callbacks for the monitor loop to notify the client layer.
type Callbacks struct {
	OnMessage        func(clientID string, msg *Message)
	OnSessionExpired func(clientID string)
	OnDisconnected   func(clientID string, err error)
	OnError          func(clientID string, err error)
}

// LoopConfig holds everything the monitor loop needs.
type LoopConfig struct {
	ClientID   string
	TC         *api.TransportConfig
	CDNBaseURL string
	Store      store.Store
	Logger     *slog.Logger
	Callbacks  Callbacks
}

// Run executes the long-poll monitor loop. Blocks until ctx is cancelled.
func Run(ctx context.Context, cfg *LoopConfig) error {
	log := cfg.Logger
	if log == nil {
		log = slog.Default()
	}

	syncBuf, _ := cfg.Store.LoadSyncBuf(ctx, cfg.ClientID)
	guard := NewSessionGuard()
	typingCache := NewTypingCache(cfg.TC)

	var nextTimeout time.Duration
	consecutiveFailures := 0

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if guard.IsPaused(cfg.ClientID) {
			remaining := guard.RemainingPause(cfg.ClientID)
			log.Info("session paused, waiting", "remaining", remaining)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(remaining):
			}
			continue
		}

		resp, err := api.GetUpdates(ctx, cfg.TC, syncBuf, nextTimeout)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			consecutiveFailures++
			log.Error("getUpdates error", "error", err, "failures", consecutiveFailures)
			if consecutiveFailures >= maxConsecutiveFailures {
				consecutiveFailures = 0
				sleepCtx(ctx, backoffDelay)
			} else {
				sleepCtx(ctx, retryDelay)
			}
			continue
		}

		// Check for API-level errors
		isAPIError := (resp.Ret != nil && *resp.Ret != 0) || (resp.ErrCode != nil && *resp.ErrCode != 0)
		if isAPIError {
			retVal := 0
			if resp.Ret != nil {
				retVal = *resp.Ret
			}
			errCode := 0
			if resp.ErrCode != nil {
				errCode = *resp.ErrCode
			}

			isSessionExpired := retVal == SessionExpiredErrCode || errCode == SessionExpiredErrCode
			if isSessionExpired {
				guard.Pause(cfg.ClientID)
				log.Error("session expired", "errcode", SessionExpiredErrCode)
				if cfg.Callbacks.OnSessionExpired != nil {
					cfg.Callbacks.OnSessionExpired(cfg.ClientID)
				}
				consecutiveFailures = 0
				continue
			}

			consecutiveFailures++
			log.Error("getUpdates API error", "ret", retVal, "errcode", errCode, "errmsg", resp.ErrMsg, "failures", consecutiveFailures)
			if consecutiveFailures >= maxConsecutiveFailures {
				consecutiveFailures = 0
				sleepCtx(ctx, backoffDelay)
			} else {
				sleepCtx(ctx, retryDelay)
			}
			continue
		}

		// Success
		consecutiveFailures = 0

		if resp.LongPollingTimeoutMs != nil && *resp.LongPollingTimeoutMs > 0 {
			nextTimeout = time.Duration(*resp.LongPollingTimeoutMs) * time.Millisecond
		}

		if resp.GetUpdatesBuf != "" {
			syncBuf = resp.GetUpdatesBuf
			if err := cfg.Store.SaveSyncBuf(ctx, cfg.ClientID, syncBuf); err != nil {
				log.Error("failed to save sync buf", "error", err)
			}
		}

		for _, wxMsg := range resp.Msgs {
			msg := convertMessage(ctx, cfg, wxMsg, typingCache, log)
			if msg == nil {
				continue
			}

			// Save context token
			if wxMsg.ContextToken != "" && wxMsg.FromUserID != "" {
				if err := cfg.Store.SaveContextToken(ctx, cfg.ClientID, wxMsg.FromUserID, wxMsg.ContextToken); err != nil {
					log.Error("failed to save context token", "error", err)
				}
			}

			if cfg.Callbacks.OnMessage != nil {
				cfg.Callbacks.OnMessage(cfg.ClientID, msg)
			}
		}
	}
}

func convertMessage(ctx context.Context, cfg *LoopConfig, wxMsg *api.WeixinMessage, _ *TypingCache, log *slog.Logger) *Message {
	msg := &Message{
		ClientID:    cfg.ClientID,
		MessageID:   wxMsg.MessageID,
		From:        wxMsg.FromUserID,
		To:          wxMsg.ToUserID,
		MsgClientID: wxMsg.ClientID,
		SessionID:   wxMsg.SessionID,
		CreatedAt:   time.UnixMilli(wxMsg.CreateTimeMs),
	}

	// Serialize raw for advanced usage
	if raw, err := json.Marshal(wxMsg); err == nil {
		msg.Raw = raw
	}

	for _, item := range wxMsg.ItemList {
		switch item.Type {
		case api.MessageItemTypeText:
			if item.TextItem != nil && item.TextItem.Text != "" {
				if msg.Text != "" {
					msg.Text += "\n"
				}
				msg.Text += item.TextItem.Text
			}
			// Handle ref_msg for quoted text
			if item.RefMsg != nil {
				msg.Ref = &RefMessage{
					Title: item.RefMsg.Title,
				}
				if item.RefMsg.MessageItem != nil && item.RefMsg.MessageItem.TextItem != nil {
					msg.Ref.Text = item.RefMsg.MessageItem.TextItem.Text
				}
			}

		case api.MessageItemTypeImage:
			att := downloadImage(ctx, cfg.CDNBaseURL, item.ImageItem, log)
			if att != nil {
				msg.Images = append(msg.Images, *att)
			}

		case api.MessageItemTypeVoice:
			att := downloadVoice(ctx, cfg.CDNBaseURL, item.VoiceItem, log)
			if att != nil {
				msg.Voice = att
			}

		case api.MessageItemTypeFile:
			att := downloadFile(ctx, cfg.CDNBaseURL, item.FileItem, log)
			if att != nil {
				msg.Files = append(msg.Files, *att)
			}

		case api.MessageItemTypeVideo:
			att := downloadVideo(ctx, cfg.CDNBaseURL, item.VideoItem, log)
			if att != nil {
				msg.Videos = append(msg.Videos, *att)
			}
		}
	}

	// Voice speech-to-text as fallback text
	if msg.Text == "" && msg.Voice != nil && msg.Voice.Transcript != "" {
		msg.Text = msg.Voice.Transcript
	}

	return msg
}

func downloadImage(ctx context.Context, cdnBaseURL string, img *api.ImageItem, log *slog.Logger) *MediaAttachment {
	if img == nil || img.Media == nil {
		return nil
	}
	if img.Media.EncryptQueryParam == "" && img.Media.FullURL == "" {
		return nil
	}

	var data []byte
	var err error

	if img.AESKey != "" {
		// Prefer image_item.aeskey (hex string)
		data, err = cdn.DownloadAndDecryptWithHexKey(ctx, img.Media.EncryptQueryParam, img.AESKey, cdnBaseURL, img.Media.FullURL)
	} else if img.Media.AESKey != "" {
		data, err = cdn.DownloadAndDecrypt(ctx, img.Media.EncryptQueryParam, img.Media.AESKey, cdnBaseURL, img.Media.FullURL)
	} else {
		data, err = cdn.DownloadPlain(ctx, img.Media.EncryptQueryParam, cdnBaseURL, img.Media.FullURL)
	}

	if err != nil {
		log.Error("image download failed", "error", err)
		return nil
	}
	return &MediaAttachment{Data: data, ContentType: "image/*"}
}

func downloadVoice(ctx context.Context, cdnBaseURL string, voice *api.VoiceItem, log *slog.Logger) *VoiceAttachment {
	if voice == nil || voice.Media == nil || voice.Media.AESKey == "" {
		return nil
	}
	if voice.Media.EncryptQueryParam == "" && voice.Media.FullURL == "" {
		return nil
	}

	data, err := cdn.DownloadAndDecrypt(ctx, voice.Media.EncryptQueryParam, voice.Media.AESKey, cdnBaseURL, voice.Media.FullURL)
	if err != nil {
		log.Error("voice download failed", "error", err)
		return nil
	}

	return &VoiceAttachment{
		MediaAttachment: MediaAttachment{Data: data, ContentType: "audio/silk"},
		Duration:        voice.Playtime,
		Transcript:      voice.Text,
		Format:          "silk",
	}
}

func downloadFile(ctx context.Context, cdnBaseURL string, file *api.FileItem, log *slog.Logger) *MediaAttachment {
	if file == nil || file.Media == nil || file.Media.AESKey == "" {
		return nil
	}
	if file.Media.EncryptQueryParam == "" && file.Media.FullURL == "" {
		return nil
	}

	data, err := cdn.DownloadAndDecrypt(ctx, file.Media.EncryptQueryParam, file.Media.AESKey, cdnBaseURL, file.Media.FullURL)
	if err != nil {
		log.Error("file download failed", "error", err)
		return nil
	}

	return &MediaAttachment{Data: data, Filename: file.FileName, ContentType: "application/octet-stream"}
}

func downloadVideo(ctx context.Context, cdnBaseURL string, video *api.VideoItem, log *slog.Logger) *MediaAttachment {
	if video == nil || video.Media == nil || video.Media.AESKey == "" {
		return nil
	}
	if video.Media.EncryptQueryParam == "" && video.Media.FullURL == "" {
		return nil
	}

	data, err := cdn.DownloadAndDecrypt(ctx, video.Media.EncryptQueryParam, video.Media.AESKey, cdnBaseURL, video.Media.FullURL)
	if err != nil {
		log.Error("video download failed", "error", err)
		return nil
	}

	return &MediaAttachment{Data: data, ContentType: "video/mp4"}
}

func sleepCtx(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}
