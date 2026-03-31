package monitor

import (
	"testing"
	"time"
)

func TestSessionGuard_PauseAndCheck(t *testing.T) {
	g := NewSessionGuard()

	if g.IsPaused("bot1") {
		t.Fatal("should not be paused initially")
	}

	g.Pause("bot1")

	if !g.IsPaused("bot1") {
		t.Fatal("should be paused after Pause()")
	}

	if g.IsPaused("bot2") {
		t.Fatal("different account should not be paused")
	}

	remaining := g.RemainingPause("bot1")
	if remaining <= 0 {
		t.Fatalf("remaining should be positive, got %v", remaining)
	}
	if remaining > SessionPauseDuration {
		t.Fatalf("remaining %v exceeds pause duration %v", remaining, SessionPauseDuration)
	}
}

func TestSessionGuard_Clear(t *testing.T) {
	g := NewSessionGuard()
	g.Pause("bot1")
	g.Clear("bot1")

	if g.IsPaused("bot1") {
		t.Fatal("should not be paused after Clear()")
	}
}

func TestSessionGuard_ExpiredPause(t *testing.T) {
	g := NewSessionGuard()

	// Manually set a past pause time
	g.mu.Lock()
	g.pauseUntil["bot1"] = time.Now().Add(-1 * time.Second)
	g.mu.Unlock()

	if g.IsPaused("bot1") {
		t.Fatal("should not be paused when time has passed")
	}

	if g.RemainingPause("bot1") != 0 {
		t.Fatal("remaining should be 0 for expired pause")
	}
}
