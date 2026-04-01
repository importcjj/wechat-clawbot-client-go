package clawbot

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/importcjj/wechat-clawbot-client-go/internal/api"
	"github.com/importcjj/wechat-clawbot-client-go/internal/auth"
	"github.com/importcjj/wechat-clawbot-client-go/internal/media"
	"github.com/importcjj/wechat-clawbot-client-go/internal/monitor"
	"github.com/importcjj/wechat-clawbot-client-go/store"
)

// Client[T] is the main entry point for the WeChat iLink Bot SDK.
// T is a user-defined state type for carrying business context.
type Client[T any] struct {
	clientID  string
	userState T
	store     store.Store
	cfg       clientConfig[T]
	tc        *api.TransportConfig

	state  atomic.Int32
	cancel context.CancelFunc
	mu     sync.RWMutex
	creds  *store.Credentials // set after login
}

// New creates a new Client.
//   - clientID: business identifier (e.g. "user-123"), used as Store key and in EventHook callbacks
//   - state: user-defined state accessible via UserState()
//   - s: storage backend, nil uses in-memory store
func New[T any](clientID string, state T, s store.Store, opts ...Option[T]) *Client[T] {
	if s == nil {
		s = store.NewMemoryStore()
	}

	cfg := clientConfig[T]{
		version:    api.DefaultVersion,
		baseURL:    api.DefaultBaseURL,
		cdnBaseURL: api.DefaultCDNBaseURL,
		msgBufSize: 256,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	logger := cfg.logger
	if logger == nil {
		logger = slog.Default()
	}

	tc := &api.TransportConfig{
		BaseURL:    cfg.baseURL,
		CDNBaseURL: cfg.cdnBaseURL,
		AppID:      api.DefaultAppID,
		Version:    cfg.version,
		RouteTag:   cfg.routeTag,
		Logger:     logger,
		Client:     cfg.httpClient,
	}

	c := &Client[T]{
		clientID:  clientID,
		userState: state,
		store:     s,
		cfg:       cfg,
		tc:        tc,
	}
	c.state.Store(int32(StateNew))
	return c
}

// ClientID returns the business identifier passed at creation.
func (c *Client[T]) ClientID() string { return c.clientID }

// UserState returns the user-defined state.
func (c *Client[T]) UserState() T { return c.userState }

// State returns the current lifecycle state.
func (c *Client[T]) State() ClientState { return ClientState(c.state.Load()) }

func (c *Client[T]) setState(s ClientState) { c.state.Store(int32(s)) }

// HasCredentials checks if the Store has credentials for this clientID.
func (c *Client[T]) HasCredentials() bool {
	c.mu.RLock()
	if c.creds != nil {
		c.mu.RUnlock()
		return true
	}
	c.mu.RUnlock()

	_, err := c.store.LoadCredentials(context.Background(), c.clientID)
	return err == nil
}

// LoginSession represents an in-progress QR login.
type LoginSession struct {
	qrCodeURL string
	session   *auth.QRLoginSession
	client    interface{ onLoginComplete(*auth.LoginResult) }
}

// QRCodeURL returns the URL for displaying the QR code.
func (s *LoginSession) QRCodeURL() string { return s.qrCodeURL }

// Wait blocks until the QR login completes, times out, or ctx is cancelled.
func (s *LoginSession) Wait(ctx context.Context) error {
	result, err := s.session.Wait(ctx)
	if err != nil {
		return err
	}
	s.client.onLoginComplete(result)
	return nil
}

// Login initiates a QR code login. Returns a LoginSession whose QRCodeURL()
// can be displayed immediately. Call session.Wait() to block until completion.
func (c *Client[T]) Login(ctx context.Context) (*LoginSession, error) {
	c.setState(StateLoggingIn)

	callbacks := auth.LoginCallbacks{
		OnQRCode: func(url string) {
			if c.cfg.hooks.OnQRCode != nil {
				c.cfg.hooks.OnQRCode(c, url)
			}
		},
		OnQRScanned: func() {
			if c.cfg.hooks.OnQRScanned != nil {
				c.cfg.hooks.OnQRScanned(c)
			}
		},
		OnQRExpired: func(count int) {
			if c.cfg.hooks.OnQRExpired != nil {
				c.cfg.hooks.OnQRExpired(c, count)
			}
		},
	}

	session, err := auth.StartQRLogin(ctx, c.tc, callbacks)
	if err != nil {
		c.setState(StateNew)
		return nil, fmt.Errorf("starting QR login: %w", err)
	}

	return &LoginSession{
		qrCodeURL: session.QRCodeURL(),
		session:   session,
		client:    c,
	}, nil
}

func (c *Client[T]) onLoginComplete(result *auth.LoginResult) {
	creds := store.Credentials{
		Token:   result.BotToken,
		BaseURL: result.BaseURL,
		UserID:  result.UserID,
		SavedAt: time.Now(),
	}

	c.mu.Lock()
	c.creds = &creds
	c.mu.Unlock()

	// Update transport with new token and baseURL
	if result.BotToken != "" {
		c.tc.Token = result.BotToken
	}
	if result.BaseURL != "" {
		c.tc.BaseURL = result.BaseURL
	}

	// Persist to store
	if err := c.store.SaveCredentials(context.Background(), c.clientID, creds); err != nil {
		c.tc.Logger.Error("failed to save credentials", "error", err)
	}

	c.setState(StateReady)

	if c.cfg.hooks.OnConnected != nil {
		c.cfg.hooks.OnConnected(c)
	}
}

// Start begins the long-poll monitor loop. Blocks until ctx is cancelled.
// If Login() was not called, Start tries to load credentials from Store.
// Returns ErrNotLoggedIn if no credentials are available.
func (c *Client[T]) Start(ctx context.Context) error {
	// Load credentials if not already set
	c.mu.RLock()
	hasCreds := c.creds != nil
	c.mu.RUnlock()

	if !hasCreds {
		creds, err := c.store.LoadCredentials(ctx, c.clientID)
		if err != nil {
			return ErrNotLoggedIn
		}
		c.mu.Lock()
		c.creds = &creds
		c.mu.Unlock()
		c.tc.Token = creds.Token
		if creds.BaseURL != "" {
			c.tc.BaseURL = creds.BaseURL
		}
	}

	ctx, cancel := context.WithCancel(ctx)
	c.mu.Lock()
	c.cancel = cancel
	c.mu.Unlock()

	c.setState(StateRunning)
	if c.cfg.hooks.OnConnected != nil {
		c.cfg.hooks.OnConnected(c)
	}

	// Bridge generic EventHooks[T] to internal monitor.Callbacks.
	var onMessage func(clientID string, msg *Message)
	if c.cfg.hooks.OnMessage != nil {
		onMessage = func(_ string, msg *Message) {
			c.cfg.hooks.OnMessage(c, msg)
		}
	}

	loopCfg := &monitor.LoopConfig{
		ClientID:   c.clientID,
		TC:         c.tc,
		CDNBaseURL: c.cfg.cdnBaseURL,
		Store:      c.store,
		Logger:     c.tc.Logger,
		Callbacks: monitor.Callbacks{
			OnMessage: onMessage,
			OnSessionExpired: func(_ string) {
				c.setState(StateSessionExpired)
				if c.cfg.hooks.OnSessionExpired != nil {
					c.cfg.hooks.OnSessionExpired(c)
				}
			},
			OnError: func(_ string, err error) {
				if c.cfg.hooks.OnError != nil {
					c.cfg.hooks.OnError(c, err)
				}
			},
		},
	}

	err := monitor.Run(ctx, loopCfg)
	c.setState(StateStopped)
	if c.cfg.hooks.OnDisconnected != nil {
		c.cfg.hooks.OnDisconnected(c, err)
	}
	return err
}

// Stop stops the long-poll monitor loop.
func (c *Client[T]) Stop() {
	c.mu.RLock()
	cancel := c.cancel
	c.mu.RUnlock()
	if cancel != nil {
		cancel()
	}

	// If Stop is called before Start (e.g. during login), there's no
	// monitor loop to cancel. Reset to StateNew so the client can be
	// reused for a fresh Login() cycle.
	if s := c.State(); s != StateRunning && s != StateStopped {
		c.setState(StateNew)
	}
}

// SendText sends a plain text message to a user.
func (c *Client[T]) SendText(ctx context.Context, to, text string) error {
	contextToken, _ := c.store.LoadContextToken(ctx, c.clientID, to)

	msg := &api.WeixinMessage{
		FromUserID:   "",
		ToUserID:     to,
		ClientID:     generateClientID(),
		MessageType:  api.MessageTypeBot,
		MessageState: api.MessageStateFinish,
		ItemList: []*api.MessageItem{
			{Type: api.MessageItemTypeText, TextItem: &api.TextItem{Text: text}},
		},
		ContextToken: contextToken,
	}

	return api.SendMessage(ctx, c.tc, msg)
}

// SendImage uploads and sends an image.
func (c *Client[T]) SendImage(ctx context.Context, to string, data []byte, caption string) error {
	return c.sendMedia(ctx, to, data, "image.png", caption)
}

// SendVideo uploads and sends a video.
func (c *Client[T]) SendVideo(ctx context.Context, to string, data []byte, caption string) error {
	return c.sendMedia(ctx, to, data, "video.mp4", caption)
}

// SendFile uploads and sends a file attachment.
func (c *Client[T]) SendFile(ctx context.Context, to string, data []byte, filename, caption string) error {
	return c.sendMedia(ctx, to, data, filename, caption)
}

func (c *Client[T]) sendMedia(ctx context.Context, to string, data []byte, filename, caption string) error {
	contextToken, _ := c.store.LoadContextToken(ctx, c.clientID, to)

	item, err := media.UploadAndBuildItem(ctx, c.tc, c.cfg.cdnBaseURL, data, filename, to)
	if err != nil {
		return fmt.Errorf("uploading media: %w", err)
	}

	// Send caption as separate text message first (if provided)
	if caption != "" {
		textMsg := &api.WeixinMessage{
			ToUserID:     to,
			ClientID:     generateClientID(),
			MessageType:  api.MessageTypeBot,
			MessageState: api.MessageStateFinish,
			ItemList:     []*api.MessageItem{{Type: api.MessageItemTypeText, TextItem: &api.TextItem{Text: caption}}},
			ContextToken: contextToken,
		}
		if err := api.SendMessage(ctx, c.tc, textMsg); err != nil {
			return fmt.Errorf("sending caption: %w", err)
		}
	}

	// Send media item
	mediaMsg := &api.WeixinMessage{
		ToUserID:     to,
		ClientID:     generateClientID(),
		MessageType:  api.MessageTypeBot,
		MessageState: api.MessageStateFinish,
		ItemList:     []*api.MessageItem{item},
		ContextToken: contextToken,
	}
	return api.SendMessage(ctx, c.tc, mediaMsg)
}

// SendTyping sends a typing indicator to a user.
func (c *Client[T]) SendTyping(ctx context.Context, to string) error {
	contextToken, _ := c.store.LoadContextToken(ctx, c.clientID, to)
	resp, err := api.GetConfig(ctx, c.tc, to, contextToken)
	if err != nil {
		return fmt.Errorf("getting typing ticket: %w", err)
	}
	return api.SendTyping(ctx, c.tc, to, resp.TypingTicket, api.TypingStatusTyping)
}

// CancelTyping cancels the typing indicator for a user.
func (c *Client[T]) CancelTyping(ctx context.Context, to string) error {
	contextToken, _ := c.store.LoadContextToken(ctx, c.clientID, to)
	resp, err := api.GetConfig(ctx, c.tc, to, contextToken)
	if err != nil {
		return fmt.Errorf("getting typing ticket: %w", err)
	}
	return api.SendTyping(ctx, c.tc, to, resp.TypingTicket, api.TypingStatusCancel)
}

func generateClientID() string {
	buf := make([]byte, 8)
	_, _ = rand.Read(buf)
	return "wx-" + hex.EncodeToString(buf)
}
