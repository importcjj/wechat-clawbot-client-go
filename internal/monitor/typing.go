package monitor

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/importcjj/wechat-clawbot-client-go/internal/api"
)

const (
	typingCacheTTL        = 24 * time.Hour
	typingInitialRetry    = 2 * time.Second
	typingMaxRetry        = 1 * time.Hour
)

type typingCacheEntry struct {
	ticket       string
	everSucceeded bool
	nextFetchAt  time.Time
	retryDelay   time.Duration
}

// TypingCache manages per-user typing ticket caching with TTL and exponential backoff.
type TypingCache struct {
	mu    sync.RWMutex
	cache map[string]*typingCacheEntry
	tc    *api.TransportConfig
}

// NewTypingCache creates a new TypingCache.
func NewTypingCache(tc *api.TransportConfig) *TypingCache {
	return &TypingCache{
		cache: make(map[string]*typingCacheEntry),
		tc:    tc,
	}
}

// GetTicket returns the typing ticket for a user, fetching/refreshing as needed.
func (c *TypingCache) GetTicket(ctx context.Context, userID, contextToken string) string {
	c.mu.RLock()
	entry, ok := c.cache[userID]
	c.mu.RUnlock()

	now := time.Now()
	shouldFetch := !ok || now.After(entry.nextFetchAt)

	if !shouldFetch {
		if ok {
			return entry.ticket
		}
		return ""
	}

	resp, err := api.GetConfig(ctx, c.tc, userID, contextToken)
	if err == nil && resp.Ret != nil && *resp.Ret == 0 {
		c.mu.Lock()
		// Randomize next refresh within TTL to avoid thundering herd
		nextFetch := now.Add(time.Duration(float64(typingCacheTTL) * randFloat()))
		c.cache[userID] = &typingCacheEntry{
			ticket:        resp.TypingTicket,
			everSucceeded: true,
			nextFetchAt:   nextFetch,
			retryDelay:    typingInitialRetry,
		}
		c.mu.Unlock()
		return resp.TypingTicket
	}

	// Failed: apply exponential backoff
	c.mu.Lock()
	if entry == nil {
		c.cache[userID] = &typingCacheEntry{
			nextFetchAt: now.Add(typingInitialRetry),
			retryDelay:  typingInitialRetry,
		}
	} else {
		nextDelay := time.Duration(math.Min(float64(entry.retryDelay*2), float64(typingMaxRetry)))
		entry.nextFetchAt = now.Add(nextDelay)
		entry.retryDelay = nextDelay
	}
	c.mu.Unlock()

	if ok {
		return entry.ticket
	}
	return ""
}

// Simple pseudo-random float in [0, 1) without importing math/rand.
func randFloat() float64 {
	return float64(time.Now().UnixNano()%1000) / 1000.0
}
