package clawbot

import "github.com/importcjj/wechat-clawbot-client-go/store"

// DefaultClient is a Client without custom state, for the common case.
type DefaultClient = Client[struct{}]

// DefaultOption is an Option without custom state.
type DefaultOption = Option[struct{}]

// DefaultEventHooks is EventHooks without custom state.
type DefaultEventHooks = EventHooks[struct{}]

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
// Shorthand for WithEventHooks[struct{}](h).
func WithDefaultEventHooks(h DefaultEventHooks) DefaultOption {
	return WithEventHooks[struct{}](h)
}
