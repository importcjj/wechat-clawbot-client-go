package clawbot

import (
	"log/slog"
	"net/http"
)

// Option configures a Client. Use With* functions to create options.
type Option[T any] func(*clientConfig[T])

type clientConfig[T any] struct {
	hooks      EventHooks[T]
	logger     *slog.Logger
	httpClient *http.Client
	baseURL    string
	cdnBaseURL string
	version    string
	routeTag   string
	msgBufSize int
}

// EventHooks receives lifecycle events. All callbacks receive *Client[T] as
// the first parameter, giving access to the client, its ID, and user state.
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

// WithEventHooks sets lifecycle event callbacks.
func WithEventHooks[T any](h EventHooks[T]) Option[T] {
	return func(c *clientConfig[T]) {
		c.hooks = h
	}
}

// WithLogger sets a structured logger.
func WithLogger[T any](l *slog.Logger) Option[T] {
	return func(c *clientConfig[T]) {
		c.logger = l
	}
}

// WithHTTPClient sets a custom http.Client for all requests.
func WithHTTPClient[T any](hc *http.Client) Option[T] {
	return func(c *clientConfig[T]) {
		c.httpClient = hc
	}
}

// WithBaseURL overrides the default API base URL.
func WithBaseURL[T any](url string) Option[T] {
	return func(c *clientConfig[T]) {
		c.baseURL = url
	}
}

// WithCDNBaseURL overrides the default CDN base URL.
func WithCDNBaseURL[T any](url string) Option[T] {
	return func(c *clientConfig[T]) {
		c.cdnBaseURL = url
	}
}

// WithVersion sets the channel version string.
func WithVersion[T any](version string) Option[T] {
	return func(c *clientConfig[T]) {
		c.version = version
	}
}

// WithRouteTag sets the SKRouteTag header value.
func WithRouteTag[T any](tag string) Option[T] {
	return func(c *clientConfig[T]) {
		c.routeTag = tag
	}
}

// WithMessageBufferSize sets the OnMessage internal buffer size.
func WithMessageBufferSize[T any](n int) Option[T] {
	return func(c *clientConfig[T]) {
		c.msgBufSize = n
	}
}
