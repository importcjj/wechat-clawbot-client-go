package clawbot

import (
	"encoding/json"
	"time"

	"github.com/importcjj/wechat-clawbot-client-go/internal/monitor"
)

// Message represents a decoded inbound message from a WeChat user.
type Message = monitor.Message

// MediaAttachment holds decoded media data.
type MediaAttachment = monitor.MediaAttachment

// VoiceAttachment holds decoded voice data.
type VoiceAttachment = monitor.VoiceAttachment

// RefMessage holds quoted message info.
type RefMessage = monitor.RefMessage

// These aliases ensure the types are accessible from the wechat package
// without users importing internal packages.
var (
	_ *Message         = (*monitor.Message)(nil)
	_ *MediaAttachment = (*monitor.MediaAttachment)(nil)
)

// NewTextMessage is a helper for tests and internal use.
func newTextMessage(clientID, from, text string) *Message {
	return &Message{
		ClientID:  clientID,
		From:      from,
		Text:      text,
		CreatedAt: time.Now(),
		Raw:       json.RawMessage("{}"),
	}
}
