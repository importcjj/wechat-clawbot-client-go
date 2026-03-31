package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"
)

const (
	DefaultLongPollTimeout = 35 * time.Second
	DefaultAPITimeout      = 15 * time.Second
	DefaultConfigTimeout   = 10 * time.Second
	DefaultQRCodeTimeout   = 5 * time.Second
	DefaultQRPollTimeout   = 35 * time.Second
)

// GetUpdates performs a long-poll to receive inbound messages.
// On client-side timeout, returns an empty response (normal for long-poll).
func GetUpdates(ctx context.Context, tc *TransportConfig, buf string, timeout time.Duration) (*GetUpdatesResp, error) {
	if timeout == 0 {
		timeout = DefaultLongPollTimeout
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	reqBody := GetUpdatesReq{
		GetUpdatesBuf: buf,
		BaseInfo:      BuildBaseInfo(tc.Version),
	}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling getUpdates request: %w", err)
	}

	respBody, err := tc.DoPOST(ctx, "ilink/bot/getupdates", data)
	if err != nil {
		if ctx.Err() != nil {
			// Client-side timeout is normal for long-poll
			tc.logger().Debug("getUpdates: client-side timeout, returning empty response")
			return &GetUpdatesResp{Msgs: nil, GetUpdatesBuf: buf}, nil
		}
		return nil, fmt.Errorf("getUpdates: %w", err)
	}

	var resp GetUpdatesResp
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshaling getUpdates response: %w", err)
	}
	return &resp, nil
}

// SendMessage sends a single message downstream.
func SendMessage(ctx context.Context, tc *TransportConfig, msg *WeixinMessage) error {
	ctx, cancel := context.WithTimeout(ctx, DefaultAPITimeout)
	defer cancel()

	reqBody := SendMessageReq{
		Msg:      msg,
		BaseInfo: BuildBaseInfo(tc.Version),
	}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshaling sendMessage request: %w", err)
	}

	_, err = tc.DoPOST(ctx, "ilink/bot/sendmessage", data)
	if err != nil {
		return fmt.Errorf("sendMessage: %w", err)
	}
	return nil
}

// GetUploadURL requests a pre-signed CDN upload URL.
func GetUploadURL(ctx context.Context, tc *TransportConfig, req *GetUploadURLReq) (*GetUploadURLResp, error) {
	ctx, cancel := context.WithTimeout(ctx, DefaultAPITimeout)
	defer cancel()

	req.BaseInfo = BuildBaseInfo(tc.Version)
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshaling getUploadUrl request: %w", err)
	}

	respBody, err := tc.DoPOST(ctx, "ilink/bot/getuploadurl", data)
	if err != nil {
		return nil, fmt.Errorf("getUploadUrl: %w", err)
	}

	var resp GetUploadURLResp
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshaling getUploadUrl response: %w", err)
	}
	return &resp, nil
}

// GetConfig fetches bot config (including typing_ticket) for a user.
func GetConfig(ctx context.Context, tc *TransportConfig, userID, contextToken string) (*GetConfigResp, error) {
	ctx, cancel := context.WithTimeout(ctx, DefaultConfigTimeout)
	defer cancel()

	reqBody := GetConfigReq{
		ILinkUserID:  userID,
		ContextToken: contextToken,
		BaseInfo:     BuildBaseInfo(tc.Version),
	}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling getConfig request: %w", err)
	}

	respBody, err := tc.DoPOST(ctx, "ilink/bot/getconfig", data)
	if err != nil {
		return nil, fmt.Errorf("getConfig: %w", err)
	}

	var resp GetConfigResp
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshaling getConfig response: %w", err)
	}
	return &resp, nil
}

// SendTyping sends a typing indicator to a user.
func SendTyping(ctx context.Context, tc *TransportConfig, userID, ticket string, status int) error {
	ctx, cancel := context.WithTimeout(ctx, DefaultConfigTimeout)
	defer cancel()

	reqBody := SendTypingReq{
		ILinkUserID:  userID,
		TypingTicket: ticket,
		Status:       status,
		BaseInfo:     BuildBaseInfo(tc.Version),
	}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshaling sendTyping request: %w", err)
	}

	_, err = tc.DoPOST(ctx, "ilink/bot/sendtyping", data)
	if err != nil {
		return fmt.Errorf("sendTyping: %w", err)
	}
	return nil
}

// FetchQRCode retrieves a QR code for login.
func FetchQRCode(ctx context.Context, tc *TransportConfig, botType string) (*QRCodeResp, error) {
	ctx, cancel := context.WithTimeout(ctx, DefaultQRCodeTimeout)
	defer cancel()

	endpoint := "ilink/bot/get_bot_qrcode?bot_type=" + url.QueryEscape(botType)
	body, err := tc.DoGET(ctx, endpoint)
	if err != nil {
		return nil, fmt.Errorf("fetchQRCode: %w", err)
	}

	var resp QRCodeResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("unmarshaling QR code response: %w", err)
	}
	return &resp, nil
}

// PollQRStatus long-polls for QR code login status.
// On timeout, returns a "wait" status (normal behavior).
func PollQRStatus(ctx context.Context, tc *TransportConfig, baseURL, qrcode string) (*QRStatusResp, error) {
	ctx, cancel := context.WithTimeout(ctx, DefaultQRPollTimeout)
	defer cancel()

	endpoint := "ilink/bot/get_qrcode_status?qrcode=" + url.QueryEscape(qrcode)

	// Use custom base URL (may differ during IDC redirect)
	origBase := tc.BaseURL
	tc.BaseURL = baseURL
	defer func() { tc.BaseURL = origBase }()

	body, err := tc.DoGET(ctx, endpoint)
	if err != nil {
		if ctx.Err() != nil {
			// Timeout is normal for long-poll
			return &QRStatusResp{Status: "wait"}, nil
		}
		// Network/gateway errors: treat as wait, continue polling
		tc.logger().Warn("pollQRStatus: network error, will retry", "error", err)
		return &QRStatusResp{Status: "wait"}, nil
	}

	var resp QRStatusResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("unmarshaling QR status response: %w", err)
	}
	return &resp, nil
}
