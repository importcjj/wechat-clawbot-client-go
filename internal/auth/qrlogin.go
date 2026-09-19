package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/importcjj/wechat-clawbot-client-go/internal/api"
)

const (
	DefaultBotType      = "3"
	maxQRRefreshCount   = 3
	defaultLoginTimeout = 480 * time.Second
	pollInterval        = 1 * time.Second
	// maxPollFailures bounds how many consecutive poll errors are tolerated.
	// Gateway hiccups (Cloudflare 524 and friends) are expected on a 35s
	// long-poll, but swallowing them indefinitely turns a broken login into a
	// silent eight-minute wait.
	maxPollFailures = 5
)

var (
	// ErrAlreadyBound is returned when the scanned bot is already bound to
	// this installation (server status binded_redirect). No new credentials
	// are issued; the ones already stored locally remain valid.
	ErrAlreadyBound = errors.New("bot is already bound to this installation")

	// ErrVerifyCodeRequired is returned when the server asks for the pairing
	// digits shown in WeChat but no OnVerifyCode callback was supplied.
	ErrVerifyCodeRequired = errors.New("server requires a pairing code but no OnVerifyCode callback was set")

	// ErrVerifyCodeBlocked is returned after too many wrong pairing codes.
	ErrVerifyCodeBlocked = errors.New("pairing code entered incorrectly too many times")
)

// LoginResult contains the data returned on successful QR login.
type LoginResult struct {
	BotToken string
	BotID    string // ilink_bot_id
	BaseURL  string
	UserID   string // the user who scanned the QR
}

// LoginCallbacks are invoked during the QR login process.
type LoginCallbacks struct {
	// OnQRCode fires with the initial QR URL and again after every refresh.
	// The displayed code MUST be replaced each time it fires: a QR expires
	// after roughly 90 seconds and the old one stops being polled.
	OnQRCode    func(qrCodeURL string)
	OnQRScanned func()
	OnQRExpired func(refreshCount int)

	// OnVerifyCode is called when the server asks for the pairing digits
	// WeChat shows on the phone. retry is true when the previous code was
	// rejected. Returning an error aborts the login. When nil, a login that
	// reaches this state fails with ErrVerifyCodeRequired.
	OnVerifyCode func(retry bool) (string, error)
}

// QRLoginSession holds state for an in-progress QR login.
type QRLoginSession struct {
	tc          *api.TransportConfig
	botType     string
	localTokens []string
	callbacks   LoginCallbacks

	mu          sync.RWMutex
	qrcode      string
	qrcodeURL   string
	currentBase string
}

// StartQRLogin initiates a QR code login and returns a session.
// The session's QRCodeURL() can be displayed immediately.
// Call Wait() to block until login completes.
//
// localTokens are bot tokens already stored locally. The server uses them to
// recognise a bot that is already bound to this installation, in which case
// Wait returns ErrAlreadyBound rather than issuing duplicate credentials.
func StartQRLogin(ctx context.Context, tc *api.TransportConfig, localTokens []string, callbacks LoginCallbacks) (*QRLoginSession, error) {
	resp, err := api.FetchQRCode(ctx, tc, DefaultBotType, localTokens)
	if err != nil {
		return nil, fmt.Errorf("fetching QR code: %w", err)
	}

	if resp.QRCodeImgContent == "" {
		return nil, fmt.Errorf("server returned empty QR code URL")
	}

	session := &QRLoginSession{
		tc:          tc,
		botType:     DefaultBotType,
		localTokens: localTokens,
		callbacks:   callbacks,
		qrcode:      resp.QRCode,
		qrcodeURL:   resp.QRCodeImgContent,
		currentBase: pollBase(tc),
	}

	if callbacks.OnQRCode != nil {
		callbacks.OnQRCode(resp.QRCodeImgContent)
	}

	return session, nil
}

// pollBase is the host QR polling starts on. It honours a configured base URL
// (tests, staging) and otherwise uses the production host, matching the
// reference implementation's fixed QR endpoint.
func pollBase(tc *api.TransportConfig) string {
	if tc != nil && tc.BaseURL != "" {
		return tc.BaseURL
	}
	return api.DefaultBaseURL
}

// QRCodeURL returns the URL of the QR code currently being polled. Wait
// refreshes the code when the server expires it, so re-read this (or watch the
// OnQRCode callback) instead of caching the first value.
func (s *QRLoginSession) QRCodeURL() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.qrcodeURL
}

func (s *QRLoginSession) logger() *slog.Logger {
	if s.tc != nil && s.tc.Logger != nil {
		return s.tc.Logger
	}
	return slog.Default()
}

func (s *QRLoginSession) snapshot() (qrcode, base string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.qrcode, s.currentBase
}

// refresh fetches a new QR code and publishes it to the caller.
func (s *QRLoginSession) refresh(ctx context.Context, refreshCount int) error {
	if s.callbacks.OnQRExpired != nil {
		s.callbacks.OnQRExpired(refreshCount)
	}

	resp, err := api.FetchQRCode(ctx, s.tc, s.botType, s.localTokens)
	if err != nil {
		return fmt.Errorf("refreshing QR code: %w", err)
	}
	if resp.QRCodeImgContent == "" {
		return fmt.Errorf("server returned empty QR code URL on refresh")
	}

	s.mu.Lock()
	s.qrcode = resp.QRCode
	s.qrcodeURL = resp.QRCodeImgContent
	s.mu.Unlock()

	if s.callbacks.OnQRCode != nil {
		s.callbacks.OnQRCode(resp.QRCodeImgContent)
	}
	return nil
}

// Wait blocks until the QR login completes or times out.
func (s *QRLoginSession) Wait(ctx context.Context) (*LoginResult, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultLoginTimeout)
	defer cancel()

	var (
		scannedNotified bool
		pendingVerify   string
		refreshCount    int
		pollFailures    int
	)

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("login timed out: %w", ctx.Err())
		default:
		}

		qrcode, base := s.snapshot()
		resp, err := api.PollQRStatus(ctx, s.tc, base, qrcode, pendingVerify)
		if err != nil {
			pollFailures++
			if pollFailures >= maxPollFailures {
				return nil, fmt.Errorf("polling QR status: %d consecutive failures: %w", pollFailures, err)
			}
			s.logger().Warn("QR status poll failed, retrying",
				"error", err, "failures", pollFailures, "max", maxPollFailures)
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("login timed out: %w", ctx.Err())
			case <-time.After(pollInterval):
			}
			continue
		}
		pollFailures = 0

		// An expired code is answered immediately rather than long-polled, so
		// skip the inter-poll sleep on paths that already did network work.
		skipSleep := false

		switch resp.Status {
		case api.QRStatusWait:
			// Normal, continue polling

		case api.QRStatusScaned:
			// Reaching "scaned" while a code was in flight means it was accepted.
			pendingVerify = ""
			if !scannedNotified && s.callbacks.OnQRScanned != nil {
				s.callbacks.OnQRScanned()
				scannedNotified = true
			}

		case api.QRStatusScanedButRedirect:
			if resp.RedirectHost != "" {
				s.mu.Lock()
				s.currentBase = "https://" + resp.RedirectHost
				s.mu.Unlock()
			} else {
				s.logger().Warn("scaned_but_redirect without redirect_host, keeping current host")
			}

		case api.QRStatusNeedVerifyCode:
			if s.callbacks.OnVerifyCode == nil {
				return nil, ErrVerifyCodeRequired
			}
			code, err := s.callbacks.OnVerifyCode(pendingVerify != "")
			if err != nil {
				return nil, fmt.Errorf("obtaining pairing code: %w", err)
			}
			pendingVerify = code
			// Submit the code immediately instead of waiting out the interval.
			continue

		case api.QRStatusVerifyCodeBlocked:
			pendingVerify = ""
			refreshCount++
			if refreshCount > maxQRRefreshCount {
				return nil, ErrVerifyCodeBlocked
			}
			if err := s.refresh(ctx, refreshCount); err != nil {
				return nil, err
			}
			skipSleep = true

		case api.QRStatusBindedRedirect:
			return nil, ErrAlreadyBound

		case api.QRStatusExpired:
			refreshCount++
			if refreshCount > maxQRRefreshCount {
				return nil, fmt.Errorf("QR code expired %d times, giving up", maxQRRefreshCount)
			}
			if err := s.refresh(ctx, refreshCount); err != nil {
				return nil, err
			}
			scannedNotified = false
			pendingVerify = ""
			skipSleep = true

		case api.QRStatusConfirmed:
			if resp.ILinkBotID == "" {
				return nil, fmt.Errorf("login confirmed but ilink_bot_id missing")
			}
			return &LoginResult{
				BotToken: resp.BotToken,
				BotID:    resp.ILinkBotID,
				BaseURL:  resp.BaseURL,
				UserID:   resp.ILinkUserID,
			}, nil

		default:
			// Keep polling through statuses added server-side after this
			// release rather than failing the login outright.
			s.logger().Warn("unexpected QR status, continuing to poll", "status", resp.Status)
		}

		if skipSleep {
			continue
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("login timed out: %w", ctx.Err())
		case <-time.After(pollInterval):
		}
	}
}
