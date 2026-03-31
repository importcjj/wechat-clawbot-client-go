package clawbot

import "testing"

func TestClientState_String(t *testing.T) {
	tests := []struct {
		state ClientState
		want  string
	}{
		{StateNew, "new"},
		{StateLoggingIn, "logging_in"},
		{StateReady, "ready"},
		{StateRunning, "running"},
		{StateSessionExpired, "session_expired"},
		{StateStopped, "stopped"},
		{ClientState(99), "unknown"},
	}

	for _, tt := range tests {
		got := tt.state.String()
		if got != tt.want {
			t.Errorf("ClientState(%d).String() = %q, want %q", tt.state, got, tt.want)
		}
	}
}
