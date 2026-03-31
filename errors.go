package clawbot

import (
	"errors"
	"fmt"
)

// ErrNotLoggedIn is returned by Start when no credentials are available.
var ErrNotLoggedIn = errors.New("wechat: not logged in, call Login() first")

// SessionExpiredError indicates the server returned errcode -14.
type SessionExpiredError struct {
	AccountID string
}

func (e *SessionExpiredError) Error() string {
	return fmt.Sprintf("wechat: session expired for account %s (errcode -14)", e.AccountID)
}

// APIError represents a non-zero ret/errcode from the server.
type APIError struct {
	Ret     int
	ErrCode int
	ErrMsg  string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("wechat: API error ret=%d errcode=%d errmsg=%q", e.Ret, e.ErrCode, e.ErrMsg)
}

// CDNError represents a CDN upload or download failure.
type CDNError struct {
	StatusCode int
	Message    string
	Retryable  bool
}

func (e *CDNError) Error() string {
	return fmt.Sprintf("wechat: CDN error status=%d retryable=%v: %s", e.StatusCode, e.Retryable, e.Message)
}
