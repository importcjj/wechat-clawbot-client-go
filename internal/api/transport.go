package api

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

// TransportConfig holds shared configuration for all API calls.
type TransportConfig struct {
	BaseURL    string
	CDNBaseURL string
	Token      string
	AppID      string
	Version    string
	RouteTag   string
	Logger     *slog.Logger
	Client     *http.Client
}

func (tc *TransportConfig) logger() *slog.Logger {
	if tc.Logger != nil {
		return tc.Logger
	}
	return slog.Default()
}

func (tc *TransportConfig) httpClient() *http.Client {
	if tc.Client != nil {
		return tc.Client
	}
	return http.DefaultClient
}

func (tc *TransportConfig) baseURL() string {
	if tc.BaseURL != "" {
		return tc.BaseURL
	}
	return DefaultBaseURL
}

func (tc *TransportConfig) ensureTrailingSlash(u string) string {
	if strings.HasSuffix(u, "/") {
		return u
	}
	return u + "/"
}

// DoGET performs a GET request to the given endpoint with common headers.
func (tc *TransportConfig) DoGET(ctx context.Context, endpoint string) ([]byte, error) {
	return tc.DoGETBase(ctx, "", endpoint)
}

// DoGETBase is DoGET against an explicit base URL, leaving tc.BaseURL alone.
// An empty baseURL falls back to the configured one. Used by QR polling, which
// may be redirected to another IDC host mid-login.
func (tc *TransportConfig) DoGETBase(ctx context.Context, baseURL, endpoint string) ([]byte, error) {
	if baseURL == "" {
		baseURL = tc.baseURL()
	}
	base := tc.ensureTrailingSlash(baseURL)
	url := base + endpoint

	tc.logger().Debug("GET", "url", url)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating GET request: %w", err)
	}
	SetCommonHeaders(req.Header, tc.AppID, tc.Version, tc.RouteTag)

	resp, err := tc.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", endpoint, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading GET response: %w", err)
	}

	tc.logger().Debug("GET response", "endpoint", endpoint, "status", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: status %d: %s", endpoint, resp.StatusCode, string(body))
	}

	return body, nil
}

// DoPOST performs a POST request with JSON body, auth headers, and common headers.
func (tc *TransportConfig) DoPOST(ctx context.Context, endpoint string, jsonBody []byte) ([]byte, error) {
	return tc.DoPOSTBase(ctx, "", endpoint, jsonBody)
}

// DoPOSTBase is DoPOST against an explicit base URL, leaving tc.BaseURL alone.
func (tc *TransportConfig) DoPOSTBase(ctx context.Context, baseURL, endpoint string, jsonBody []byte) ([]byte, error) {
	if baseURL == "" {
		baseURL = tc.baseURL()
	}
	base := tc.ensureTrailingSlash(baseURL)
	url := base + endpoint

	tc.logger().Debug("POST", "url", url)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(jsonBody)))
	if err != nil {
		return nil, fmt.Errorf("creating POST request: %w", err)
	}
	SetCommonHeaders(req.Header, tc.AppID, tc.Version, tc.RouteTag)
	SetPOSTHeaders(req.Header, tc.Token, len(jsonBody))

	resp, err := tc.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("POST %s: %w", endpoint, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading POST response: %w", err)
	}

	tc.logger().Debug("POST response", "endpoint", endpoint, "status", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("POST %s: status %d: %s", endpoint, resp.StatusCode, string(body))
	}

	return body, nil
}
