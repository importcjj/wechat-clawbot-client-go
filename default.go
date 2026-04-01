package clawbot

import "github.com/importcjj/wechat-clawbot-client-go/store"

// DefaultClient is a Client without custom state, for the common case.
type DefaultClient = Client[struct{}]

// DefaultOption is an Option without custom state.
type DefaultOption = Option[struct{}]

// DefaultEventHooks provides event callbacks without the state parameter,
// for use with DefaultClient.
type DefaultEventHooks struct {
	OnMessage        func(clientID string, msg *Message)
	OnQRCode         func(clientID string, qrCodeURL string)
	OnQRScanned      func(clientID string)
	OnQRExpired      func(clientID string, refreshCount int)
	OnConnected      func(clientID string)
	OnSessionExpired func(clientID string)
	OnDisconnected   func(clientID string, err error)
	OnError          func(clientID string, err error)
}

// NewDefault creates a Client without custom state.
//
//	client := wechat.NewDefault("my-bot", nil)
//	client := wechat.NewDefault("my-bot", store.NewFileStore("./data"),
//	    wechat.WithDefaultEventHooks(wechat.DefaultEventHooks{
//	        OnMessage:   func(id string, msg *wechat.Message) { ... },
//	        OnConnected: func(id string) { ... },
//	    }),
//	)
func NewDefault(clientID string, s store.Store, opts ...DefaultOption) *DefaultClient {
	return New[struct{}](clientID, struct{}{}, s, opts...)
}

// WithDefaultEventHooks sets event hooks for a DefaultClient.
// It wraps each callback to match the EventHooks[struct{}] signature.
func WithDefaultEventHooks(h DefaultEventHooks) DefaultOption {
	generic := EventHooks[struct{}]{}
	if h.OnMessage != nil {
		generic.OnMessage = func(c *DefaultClient, msg *Message) { h.OnMessage(c.clientID, msg) }
	}
	if h.OnQRCode != nil {
		generic.OnQRCode = func(c *DefaultClient, url string) { h.OnQRCode(c.clientID, url) }
	}
	if h.OnQRScanned != nil {
		generic.OnQRScanned = func(c *DefaultClient) { h.OnQRScanned(c.clientID) }
	}
	if h.OnQRExpired != nil {
		generic.OnQRExpired = func(c *DefaultClient, count int) { h.OnQRExpired(c.clientID, count) }
	}
	if h.OnConnected != nil {
		generic.OnConnected = func(c *DefaultClient) { h.OnConnected(c.clientID) }
	}
	if h.OnSessionExpired != nil {
		generic.OnSessionExpired = func(c *DefaultClient) { h.OnSessionExpired(c.clientID) }
	}
	if h.OnDisconnected != nil {
		generic.OnDisconnected = func(c *DefaultClient, err error) { h.OnDisconnected(c.clientID, err) }
	}
	if h.OnError != nil {
		generic.OnError = func(c *DefaultClient, err error) { h.OnError(c.clientID, err) }
	}
	return WithEventHooks[struct{}](generic)
}
