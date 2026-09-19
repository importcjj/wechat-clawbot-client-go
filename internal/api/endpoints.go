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

	respBody, err := tc.DoPOST(ctx, "ilink/bot/sendmessage", data)
	if err != nil {
		return fmt.Errorf("sendMessage: %w", err)
	}

	// The server answers HTTP 200 with a non-zero ret on rejection, so the
	// body has to be checked or failed sends look successful.
	var resp SendMessageResp
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return fmt.Errorf("unmarshaling sendMessage response: %w", err)
	}
	if resp.Ret != nil && *resp.Ret != 0 {
		return fmt.Errorf("sendMessage: ret=%d errmsg=%q", *resp.Ret, resp.ErrMsg)
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

// FetchQRCode retrieves a QR code for login. localTokens are the bot tokens
// already stored locally; the server uses them to recognise a bot that is
// already bound to this installation and answers binded_redirect instead of
// issuing a duplicate account.
func FetchQRCode(ctx context.Context, tc *TransportConfig, botType string, localTokens []string) (*QRCodeResp, error) {
	ctx, cancel := context.WithTimeout(ctx, DefaultQRCodeTimeout)
	defer cancel()

	if localTokens == nil {
		localTokens = []string{}
	}
	data, err := json.Marshal(QRCodeReq{LocalTokenList: localTokens})
	if err != nil {
		return nil, fmt.Errorf("marshaling get_bot_qrcode request: %w", err)
	}

	endpoint := "ilink/bot/get_bot_qrcode?bot_type=" + url.QueryEscape(botType)
	body, err := tc.DoPOST(ctx, endpoint, data)
	if err != nil {
		return nil, fmt.Errorf("fetchQRCode: %w", err)
	}

	var resp QRCodeResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("unmarshaling QR code response: %w", err)
	}
	return &resp, nil
}

// PollQRStatus long-polls for QR code login status. A non-empty verifyCode is
// echoed back to the server as the pairing digits the user read off WeChat.
//
// A client-side timeout means the long-poll simply held open with nothing to
// report, so it maps to "wait". Every other failure is returned to the caller:
// silently retrying an HTTP 4xx/5xx would leave the login spinning forever with
// no sign of what went wrong.
func PollQRStatus(ctx context.Context, tc *TransportConfig, baseURL, qrcode, verifyCode string) (*QRStatusResp, error) {
	ctx, cancel := context.WithTimeout(ctx, DefaultQRPollTimeout)
	defer cancel()

	endpoint := "ilink/bot/get_qrcode_status?qrcode=" + url.QueryEscape(qrcode)
	if verifyCode != "" {
		endpoint += "&verify_code=" + url.QueryEscape(verifyCode)
	}

	body, err := tc.DoGETBase(ctx, baseURL, endpoint)
	if err != nil {
		if ctx.Err() != nil {
			// Client-side timeout is normal for a long-poll.
			return &QRStatusResp{Status: QRStatusWait}, nil
		}
		return nil, fmt.Errorf("pollQRStatus: %w", err)
	}

	var resp QRStatusResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("unmarshaling QR status response: %w", err)
	}
	return &resp, nil
}

// NotifyStart tells the server this bot client is coming up.
func NotifyStart(ctx context.Context, tc *TransportConfig) error {
	return notify(ctx, tc, "ilink/bot/msg/notifystart")
}

// NotifyStop tells the server this bot client is shutting down.
func NotifyStop(ctx context.Context, tc *TransportConfig) error {
	return notify(ctx, tc, "ilink/bot/msg/notifystop")
}

func notify(ctx context.Context, tc *TransportConfig, endpoint string) error {
	ctx, cancel := context.WithTimeout(ctx, DefaultConfigTimeout)
	defer cancel()

	data, err := json.Marshal(struct {
		BaseInfo *BaseInfo `json:"base_info,omitempty"`
	}{BaseInfo: BuildBaseInfo(tc.Version)})
	if err != nil {
		return fmt.Errorf("marshaling %s request: %w", endpoint, err)
	}

	body, err := tc.DoPOST(ctx, endpoint, data)
	if err != nil {
		return fmt.Errorf("%s: %w", endpoint, err)
	}

	var resp NotifyResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("unmarshaling %s response: %w", endpoint, err)
	}
	if resp.Ret != nil && *resp.Ret != 0 {
		return fmt.Errorf("%s: ret=%d errmsg=%q", endpoint, *resp.Ret, resp.ErrMsg)
	}
	return nil
}
