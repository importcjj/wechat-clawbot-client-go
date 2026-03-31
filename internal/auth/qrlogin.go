package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/importcjj/wechat-clawbot-client-go/internal/api"
)

const (
	DefaultBotType       = "3"
	maxQRRefreshCount    = 3
	defaultLoginTimeout  = 480 * time.Second
	pollInterval         = 1 * time.Second
)

// LoginResult contains the data returned on successful QR login.
type LoginResult struct {
	BotToken   string
	BotID      string // ilink_bot_id
	BaseURL    string
	UserID     string // the user who scanned the QR
}

// LoginCallbacks are invoked during the QR login process.
type LoginCallbacks struct {
	OnQRCode    func(qrCodeURL string)
	OnQRScanned func()
	OnQRExpired func(refreshCount int)
}

// QRLoginSession holds state for an in-progress QR login.
type QRLoginSession struct {
	tc         *api.TransportConfig
	qrcode     string
	qrcodeURL  string
	currentBase string
	callbacks  LoginCallbacks
	done       chan LoginResult
	err        chan error
}

// StartQRLogin initiates a QR code login and returns a session.
// The session's QRCodeURL() can be displayed immediately.
// Call Wait() to block until login completes.
func StartQRLogin(ctx context.Context, tc *api.TransportConfig, callbacks LoginCallbacks) (*QRLoginSession, error) {
	resp, err := api.FetchQRCode(ctx, tc, DefaultBotType)
	if err != nil {
		return nil, fmt.Errorf("fetching QR code: %w", err)
	}

	if resp.QRCodeImgContent == "" {
		return nil, fmt.Errorf("server returned empty QR code URL")
	}

	session := &QRLoginSession{
		tc:          tc,
		qrcode:      resp.QRCode,
		qrcodeURL:   resp.QRCodeImgContent,
		currentBase: api.DefaultBaseURL,
		callbacks:   callbacks,
		done:        make(chan LoginResult, 1),
		err:         make(chan error, 1),
	}

	if callbacks.OnQRCode != nil {
		callbacks.OnQRCode(resp.QRCodeImgContent)
	}

	return session, nil
}

// QRCodeURL returns the URL for displaying the QR code.
func (s *QRLoginSession) QRCodeURL() string {
	return s.qrcodeURL
}

// Wait blocks until the QR login completes or times out.
func (s *QRLoginSession) Wait(ctx context.Context) (*LoginResult, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultLoginTimeout)
	defer cancel()

	var scannedNotified bool
	refreshCount := 0

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("login timed out: %w", ctx.Err())
		default:
		}

		resp, err := api.PollQRStatus(ctx, s.tc, s.currentBase, s.qrcode)
		if err != nil {
			return nil, fmt.Errorf("polling QR status: %w", err)
		}

		switch resp.Status {
		case "wait":
			// Normal, continue polling

		case "scaned":
			if !scannedNotified && s.callbacks.OnQRScanned != nil {
				s.callbacks.OnQRScanned()
				scannedNotified = true
			}

		case "scaned_but_redirect":
			if resp.RedirectHost != "" {
				s.currentBase = "https://" + resp.RedirectHost
			}

		case "expired":
			refreshCount++
			if refreshCount >= maxQRRefreshCount {
				return nil, fmt.Errorf("QR code expired %d times, giving up", maxQRRefreshCount)
			}

			if s.callbacks.OnQRExpired != nil {
				s.callbacks.OnQRExpired(refreshCount)
			}

			// Refresh QR code
			newResp, err := api.FetchQRCode(ctx, s.tc, DefaultBotType)
			if err != nil {
				return nil, fmt.Errorf("refreshing QR code: %w", err)
			}
			s.qrcode = newResp.QRCode
			s.qrcodeURL = newResp.QRCodeImgContent
			scannedNotified = false

			if s.callbacks.OnQRCode != nil {
				s.callbacks.OnQRCode(newResp.QRCodeImgContent)
			}

		case "confirmed":
			if resp.ILinkBotID == "" {
				return nil, fmt.Errorf("login confirmed but ilink_bot_id missing")
			}
			result := &LoginResult{
				BotToken: resp.BotToken,
				BotID:    resp.ILinkBotID,
				BaseURL:  resp.BaseURL,
				UserID:   resp.ILinkUserID,
			}
			return result, nil

		default:
			return nil, fmt.Errorf("unexpected QR status: %s", resp.Status)
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("login timed out: %w", ctx.Err())
		case <-time.After(pollInterval):
		}
	}
}
