package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testTransport(baseURL string) *TransportConfig {
	return &TransportConfig{
		BaseURL: baseURL,
		AppID:   DefaultAppID,
		Version: DefaultVersion,
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		Client:  &http.Client{Timeout: 5 * time.Second},
	}
}

// message_id is uint64 on the wire. Decoded into an int64 the whole response
// fails to unmarshal once an id passes 2^63, and the long-poll then retries
// the same batch forever instead of delivering it.
func TestGetUpdatesDecodesLargeMessageID(t *testing.T) {
	t.Parallel()

	const bigID = uint64(18446744073709551000)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"ret":0,"msgs":[{"message_id":18446744073709551000,"from_user_id":"u1"}],"get_updates_buf":"buf"}`)
	}))
	defer ts.Close()

	resp, err := GetUpdates(context.Background(), testTransport(ts.URL), "", time.Second)
	if err != nil {
		t.Fatalf("GetUpdates: %v", err)
	}
	if len(resp.Msgs) != 1 {
		t.Fatalf("msgs = %d, want 1", len(resp.Msgs))
	}
	if got := resp.Msgs[0].MessageID; got != bigID {
		t.Errorf("MessageID = %d, want %d", got, bigID)
	}
}

// The server answers HTTP 200 with a non-zero ret when it rejects a send.
// Ignoring the body makes a dropped message look delivered.
func TestSendMessageChecksRet(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		{name: "ret zero succeeds", body: `{"ret":0,"message_id":"42"}`},
		{name: "absent ret succeeds", body: `{}`},
		{name: "non-zero ret fails", body: `{"ret":-14,"errmsg":"token expired"}`, wantErr: "ret=-14"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(w, tt.body)
			}))
			defer ts.Close()

			err := SendMessage(context.Background(), testTransport(ts.URL), &WeixinMessage{ToUserID: "u1"})
			switch {
			case tt.wantErr == "":
				if err != nil {
					t.Fatalf("SendMessage: %v", err)
				}
			default:
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("SendMessage error = %v, want one containing %q", err, tt.wantErr)
				}
			}
		})
	}
}

func TestNotifyStartStop(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		call    func(context.Context, *TransportConfig) error
		wantHit string
	}{
		{name: "start", call: NotifyStart, wantHit: "/ilink/bot/msg/notifystart"},
		{name: "stop", call: NotifyStop, wantHit: "/ilink/bot/msg/notifystop"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var (
				gotPath string
				gotBody []byte
			)
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				gotBody, _ = io.ReadAll(r.Body)
				_, _ = io.WriteString(w, `{"ret":0}`)
			}))
			defer ts.Close()

			if err := tt.call(context.Background(), testTransport(ts.URL)); err != nil {
				t.Fatalf("notify: %v", err)
			}
			if gotPath != tt.wantHit {
				t.Errorf("path = %q, want %q", gotPath, tt.wantHit)
			}

			var body struct {
				BaseInfo *BaseInfo `json:"base_info"`
			}
			if err := json.Unmarshal(gotBody, &body); err != nil {
				t.Fatalf("unmarshaling body %q: %v", gotBody, err)
			}
			if body.BaseInfo == nil || body.BaseInfo.BotAgent != DefaultBotAgent {
				t.Errorf("base_info = %+v, want bot_agent %q", body.BaseInfo, DefaultBotAgent)
			}
		})
	}
}

func TestNotifyReportsServerError(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"ret":-1,"errmsg":"nope"}`)
	}))
	defer ts.Close()

	err := NotifyStart(context.Background(), testTransport(ts.URL))
	if err == nil || !strings.Contains(err.Error(), "ret=-1") {
		t.Fatalf("NotifyStart error = %v, want one containing %q", err, "ret=-1")
	}
}

// get_bot_qrcode became a POST carrying local_token_list in plugin 2.4.9.
func TestFetchQRCodePostsLocalTokenList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		tokens []string
		want   string
	}{
		{name: "with tokens", tokens: []string{"a", "b"}, want: `{"local_token_list":["a","b"]}`},
		{name: "nil becomes an empty list", tokens: nil, want: `{"local_token_list":[]}`},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var (
				gotMethod string
				gotQuery  string
				gotBody   []byte
			)
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod = r.Method
				gotQuery = r.URL.Query().Get("bot_type")
				gotBody, _ = io.ReadAll(r.Body)
				_, _ = io.WriteString(w, `{"qrcode":"q","qrcode_img_content":"https://example.test/q"}`)
			}))
			defer ts.Close()

			if _, err := FetchQRCode(context.Background(), testTransport(ts.URL), "3", tt.tokens); err != nil {
				t.Fatalf("FetchQRCode: %v", err)
			}
			if gotMethod != http.MethodPost {
				t.Errorf("method = %s, want POST", gotMethod)
			}
			if gotQuery != "3" {
				t.Errorf("bot_type = %q, want %q", gotQuery, "3")
			}
			if got := strings.TrimSpace(string(gotBody)); got != tt.want {
				t.Errorf("body = %s, want %s", got, tt.want)
			}
		})
	}
}

// An IDC redirect must not rewrite the base URL the rest of the client shares.
func TestPollQRStatusLeavesTransportBaseURLAlone(t *testing.T) {
	t.Parallel()

	var gotVerify string
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotVerify = r.URL.Query().Get("verify_code")
		_, _ = io.WriteString(w, `{"ret":0,"status":"confirmed","ilink_bot_id":"bot@im.bot"}`)
	}))
	defer other.Close()

	tc := testTransport("https://configured.test")

	resp, err := PollQRStatus(context.Background(), tc, other.URL, "qr-a", "482913")
	if err != nil {
		t.Fatalf("PollQRStatus: %v", err)
	}
	if resp.Status != QRStatusConfirmed {
		t.Errorf("status = %q, want %q", resp.Status, QRStatusConfirmed)
	}
	if gotVerify != "482913" {
		t.Errorf("verify_code = %q, want %q", gotVerify, "482913")
	}
	if tc.BaseURL != "https://configured.test" {
		t.Errorf("tc.BaseURL = %q, want it untouched", tc.BaseURL)
	}
}
