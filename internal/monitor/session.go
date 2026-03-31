package monitor

import (
	"sync"
	"time"
)

const (
	// SessionExpiredErrCode is returned by the server when the bot session has expired.
	SessionExpiredErrCode = -14

	// SessionPauseDuration is how long to pause after session expiry.
	SessionPauseDuration = 60 * time.Minute
)

// SessionGuard tracks per-account session pause state.
type SessionGuard struct {
	mu         sync.RWMutex
	pauseUntil map[string]time.Time
}

// NewSessionGuard creates a new SessionGuard.
func NewSessionGuard() *SessionGuard {
	return &SessionGuard{
		pauseUntil: make(map[string]time.Time),
	}
}

// Pause marks an account as paused for SessionPauseDuration.
func (g *SessionGuard) Pause(accountID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.pauseUntil[accountID] = time.Now().Add(SessionPauseDuration)
}

// IsPaused returns true if the account is within its pause window.
func (g *SessionGuard) IsPaused(accountID string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	until, ok := g.pauseUntil[accountID]
	if !ok {
		return false
	}
	if time.Now().After(until) {
		return false
	}
	return true
}

// RemainingPause returns how long until the pause expires.
func (g *SessionGuard) RemainingPause(accountID string) time.Duration {
	g.mu.RLock()
	defer g.mu.RUnlock()
	until, ok := g.pauseUntil[accountID]
	if !ok {
		return 0
	}
	remaining := time.Until(until)
	if remaining <= 0 {
		return 0
	}
	return remaining
}

// Clear removes the pause for an account (e.g. after re-login).
func (g *SessionGuard) Clear(accountID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.pauseUntil, accountID)
}
