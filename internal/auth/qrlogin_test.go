package auth

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/importcjj/wechat-clawbot-client-go/internal/api"
)

var errNoTerminal = errors.New("no terminal to prompt on")

// qrServer is a fake ilink endpoint that hands out QR codes and replays a
// scripted sequence of status responses.
type qrServer struct {
	t *testing.T

	mu         sync.Mutex
	statuses   []map[string]any // consumed one per poll; last entry repeats
	issued     []string         // qrcodes handed out, in order
	polled     []string         // qrcode seen on each poll
	verifySeen []string         // verify_code seen on each poll ("" when absent)
	qrBodies   []string         // raw bodies of get_bot_qrcode requests
}

func (s *qrServer) handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/ilink/bot/get_bot_qrcode", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)

		s.mu.Lock()
		s.qrBodies = append(s.qrBodies, string(body))
		code := "qr-" + string(rune('a'+len(s.issued)))
		s.issued = append(s.issued, code)
		s.mu.Unlock()

		if r.Method != http.MethodPost {
			s.t.Errorf("get_bot_qrcode method = %s, want POST", r.Method)
		}
		writeJSON(w, map[string]any{
			"qrcode":             code,
			"qrcode_img_content": "https://example.test/q/" + code,
			"ret":                0,
		})
	})

	mux.HandleFunc("/ilink/bot/get_qrcode_status", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		s.mu.Lock()
		s.polled = append(s.polled, q.Get("qrcode"))
		s.verifySeen = append(s.verifySeen, q.Get("verify_code"))
		next := s.statuses[0]
		if len(s.statuses) > 1 {
			s.statuses = s.statuses[1:]
		}
		s.mu.Unlock()

		writeJSON(w, next)
	})

	return mux
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func (s *qrServer) snapshot() (issued, polled, verify []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.issued...),
		append([]string(nil), s.polled...),
		append([]string(nil), s.verifySeen...)
}

func newTestTransport(t *testing.T, baseURL string) *api.TransportConfig {
	t.Helper()
	return &api.TransportConfig{
		BaseURL: baseURL,
		AppID:   api.DefaultAppID,
		Version: api.DefaultVersion,
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		Client:  &http.Client{Timeout: 5 * time.Second},
	}
}

func TestStartQRLoginSendsLocalTokens(t *testing.T) {
	t.Parallel()

	srv := &qrServer{t: t, statuses: []map[string]any{{"status": "wait"}}}
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	session, err := StartQRLogin(context.Background(), newTestTransport(t, ts.URL), []string{"tok-1", "tok-2"}, LoginCallbacks{})
	if err != nil {
		t.Fatalf("StartQRLogin: %v", err)
	}
	if got, want := session.QRCodeURL(), "https://example.test/q/qr-a"; got != want {
		t.Errorf("QRCodeURL() = %q, want %q", got, want)
	}

	srv.mu.Lock()
	body := srv.qrBodies[0]
	srv.mu.Unlock()

	var req api.QRCodeReq
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatalf("unmarshaling get_bot_qrcode body %q: %v", body, err)
	}
	if want := []string{"tok-1", "tok-2"}; strings.Join(req.LocalTokenList, ",") != strings.Join(want, ",") {
		t.Errorf("local_token_list = %v, want %v", req.LocalTokenList, want)
	}
}

func TestWaitStatusHandling(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		statuses  []map[string]any
		verifyOut string
		verifyErr error

		wantErr      error
		wantErrPart  string
		wantBotID    string
		wantQRIssued int      // total QR codes handed out (1 = no refresh)
		wantVerify   []string // verify_code seen per poll
	}{
		{
			name: "confirmed returns credentials",
			statuses: []map[string]any{
				{"status": "wait"},
				{"status": "confirmed", "bot_token": "tok", "ilink_bot_id": "bot@im.bot", "ilink_user_id": "u1", "baseurl": "https://idc.test"},
			},
			wantBotID:    "bot@im.bot",
			wantQRIssued: 1,
		},
		{
			name: "expired refreshes the code and keeps polling",
			statuses: []map[string]any{
				{"status": "expired"},
				{"status": "confirmed", "ilink_bot_id": "bot@im.bot"},
			},
			wantBotID:    "bot@im.bot",
			wantQRIssued: 2,
		},
		{
			name: "unknown status does not abort the login",
			statuses: []map[string]any{
				{"status": "some_future_status"},
				{"status": "confirmed", "ilink_bot_id": "bot@im.bot"},
			},
			wantBotID:    "bot@im.bot",
			wantQRIssued: 1,
		},
		{
			name:         "binded_redirect reports already bound",
			statuses:     []map[string]any{{"status": "binded_redirect"}},
			wantErr:      ErrAlreadyBound,
			wantQRIssued: 1,
		},
		{
			name:         "need_verifycode without a callback fails loudly",
			statuses:     []map[string]any{{"status": "need_verifycode"}},
			wantErr:      ErrVerifyCodeRequired,
			wantQRIssued: 1,
		},
		{
			name: "need_verifycode submits the pairing code",
			statuses: []map[string]any{
				{"status": "need_verifycode"},
				{"status": "confirmed", "ilink_bot_id": "bot@im.bot"},
			},
			verifyOut:    "482913",
			wantBotID:    "bot@im.bot",
			wantQRIssued: 1,
			wantVerify:   []string{"", "482913"},
		},
		{
			name: "verify_code_blocked refreshes the code and retries",
			statuses: []map[string]any{
				{"status": "verify_code_blocked"},
				{"status": "confirmed", "ilink_bot_id": "bot@im.bot"},
			},
			wantBotID:    "bot@im.bot",
			wantQRIssued: 2,
		},
		{
			name:         "verify_code_blocked past the refresh limit gives up",
			statuses:     []map[string]any{{"status": "verify_code_blocked"}},
			wantErr:      ErrVerifyCodeBlocked,
			wantQRIssued: 1 + maxQRRefreshCount,
		},
		{
			name:         "repeated expiry gives up after the refresh limit",
			statuses:     []map[string]any{{"status": "expired"}},
			wantErrPart:  "expired 3 times",
			wantQRIssued: 1 + maxQRRefreshCount,
		},
		{
			name:         "a failing verify code callback aborts the login",
			statuses:     []map[string]any{{"status": "need_verifycode"}},
			verifyErr:    errNoTerminal,
			wantErrPart:  "obtaining pairing code",
			wantQRIssued: 1,
		},
		{
			name:         "confirmed without a bot id is rejected",
			statuses:     []map[string]any{{"status": "confirmed", "bot_token": "tok"}},
			wantErrPart:  "ilink_bot_id missing",
			wantQRIssued: 1,
		},
	}

	for _, tt := range tests {
		tt := tt // go.mod targets 1.21, where the loop variable is shared
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv := &qrServer{t: t, statuses: tt.statuses}
			ts := httptest.NewServer(srv.handler())
			defer ts.Close()

			cb := LoginCallbacks{}
			if tt.verifyOut != "" || tt.verifyErr != nil {
				cb.OnVerifyCode = func(bool) (string, error) { return tt.verifyOut, tt.verifyErr }
			}

			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()

			session, err := StartQRLogin(ctx, newTestTransport(t, ts.URL), nil, cb)
			if err != nil {
				t.Fatalf("StartQRLogin: %v", err)
			}

			result, err := session.Wait(ctx)

			switch {
			case tt.wantErr != nil:
				if err != tt.wantErr {
					t.Fatalf("Wait error = %v, want %v", err, tt.wantErr)
				}
			case tt.wantErrPart != "":
				if err == nil || !strings.Contains(err.Error(), tt.wantErrPart) {
					t.Fatalf("Wait error = %v, want one containing %q", err, tt.wantErrPart)
				}
			default:
				if err != nil {
					t.Fatalf("Wait: %v", err)
				}
				if result.BotID != tt.wantBotID {
					t.Errorf("BotID = %q, want %q", result.BotID, tt.wantBotID)
				}
			}

			issued, _, verify := srv.snapshot()
			if len(issued) != tt.wantQRIssued {
				t.Errorf("QR codes issued = %d (%v), want %d", len(issued), issued, tt.wantQRIssued)
			}
			if tt.wantVerify != nil && strings.Join(verify, "|") != strings.Join(tt.wantVerify, "|") {
				t.Errorf("verify_code per poll = %v, want %v", verify, tt.wantVerify)
			}
		})
	}
}

// An expired code is replaced, and the replacement is what both QRCodeURL and
// OnQRCode report. A UI that keeps rendering the first value shows a code the
// server no longer polls, so the scan never registers.
func TestWaitRefreshesQRCodeURLOnExpiry(t *testing.T) {
	t.Parallel()

	srv := &qrServer{t: t, statuses: []map[string]any{
		{"status": "expired"},
		{"status": "confirmed", "ilink_bot_id": "bot@im.bot"},
	}}
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	var (
		mu       sync.Mutex
		notified []string
	)
	cb := LoginCallbacks{
		OnQRCode: func(u string) {
			mu.Lock()
			notified = append(notified, u)
			mu.Unlock()
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	session, err := StartQRLogin(ctx, newTestTransport(t, ts.URL), nil, cb)
	if err != nil {
		t.Fatalf("StartQRLogin: %v", err)
	}

	first := session.QRCodeURL()
	if _, err := session.Wait(ctx); err != nil {
		t.Fatalf("Wait: %v", err)
	}

	second := session.QRCodeURL()
	if second == first {
		t.Fatalf("QRCodeURL() still %q after the code expired; it must report the refreshed code", first)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(notified) != 2 {
		t.Fatalf("OnQRCode fired %d times (%v), want 2", len(notified), notified)
	}
	if notified[0] != first || notified[1] != second {
		t.Errorf("OnQRCode reported %v, want [%q %q]", notified, first, second)
	}

	_, polled, _ := srv.snapshot()
	if len(polled) != 2 || polled[0] == polled[1] {
		t.Errorf("polled qrcodes = %v, want two distinct codes", polled)
	}
}

// Transient poll failures are tolerated, but not forever: a login that can
// never reach the server has to surface an error rather than spin silently.
func TestWaitGivesUpAfterRepeatedPollFailures(t *testing.T) {
	t.Parallel()

	var polls int
	mux := http.NewServeMux()
	mux.HandleFunc("/ilink/bot/get_bot_qrcode", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{"qrcode": "qr-a", "qrcode_img_content": "https://example.test/q/qr-a"})
	})
	mux.HandleFunc("/ilink/bot/get_qrcode_status", func(w http.ResponseWriter, _ *http.Request) {
		polls++
		http.Error(w, "bad gateway", http.StatusBadGateway)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	session, err := StartQRLogin(ctx, newTestTransport(t, ts.URL), nil, LoginCallbacks{})
	if err != nil {
		t.Fatalf("StartQRLogin: %v", err)
	}
	if _, err := session.Wait(ctx); err == nil {
		t.Fatal("Wait returned nil error despite every poll failing")
	}
	if polls != maxPollFailures {
		t.Errorf("polls = %d, want %d", polls, maxPollFailures)
	}
}

// A scaned_but_redirect moves polling to the host the server names, without
// disturbing the transport shared with the rest of the client.
func TestWaitFollowsIDCRedirect(t *testing.T) {
	t.Parallel()

	var (
		mu           sync.Mutex
		redirectHits int
	)
	// The server names a bare host and the client prefixes https://, so the
	// redirect target has to speak TLS.
	redirect := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		redirectHits++
		mu.Unlock()
		writeJSON(w, map[string]any{"status": "confirmed", "ilink_bot_id": "bot@im.bot"})
	}))
	defer redirect.Close()

	host := strings.TrimPrefix(redirect.URL, "https://")
	srv := &qrServer{t: t, statuses: []map[string]any{
		{"status": "scaned_but_redirect", "redirect_host": host},
		// Never reached: polling has moved to the redirect host by now.
		{"status": "expired"},
	}}
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	tc := newTestTransport(t, ts.URL)
	tc.Client = &http.Client{
		Timeout:   5 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}, //nolint:gosec // test server
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	session, err := StartQRLogin(ctx, tc, nil, LoginCallbacks{})
	if err != nil {
		t.Fatalf("StartQRLogin: %v", err)
	}

	result, err := session.Wait(ctx)
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if result.BotID != "bot@im.bot" {
		t.Errorf("BotID = %q, want %q", result.BotID, "bot@im.bot")
	}

	mu.Lock()
	hits := redirectHits
	mu.Unlock()
	if hits == 0 {
		t.Error("polling never moved to the redirect host")
	}
	if tc.BaseURL != ts.URL {
		t.Errorf("tc.BaseURL = %q, want it left at %q", tc.BaseURL, ts.URL)
	}
}
