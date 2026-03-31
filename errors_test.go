package clawbot

import (
	"strings"
	"testing"
)

func TestSessionExpiredError(t *testing.T) {
	err := &SessionExpiredError{AccountID: "bot-123"}
	s := err.Error()
	if !strings.Contains(s, "bot-123") || !strings.Contains(s, "-14") {
		t.Errorf("unexpected error message: %s", s)
	}
}

func TestAPIError(t *testing.T) {
	err := &APIError{Ret: -1, ErrCode: -14, ErrMsg: "session timeout"}
	s := err.Error()
	if !strings.Contains(s, "-14") || !strings.Contains(s, "session timeout") {
		t.Errorf("unexpected error message: %s", s)
	}
}

func TestCDNError(t *testing.T) {
	err := &CDNError{StatusCode: 500, Message: "server down", Retryable: true}
	s := err.Error()
	if !strings.Contains(s, "500") || !strings.Contains(s, "server down") || !strings.Contains(s, "true") {
		t.Errorf("unexpected error message: %s", s)
	}
}
