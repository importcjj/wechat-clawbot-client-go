package store

import (
	"context"
	"fmt"
	"sync"
)

// MemoryStore is an in-process Store backed by maps. Data is lost on restart.
type MemoryStore struct {
	mu            sync.RWMutex
	credentials   map[string]Credentials
	syncBufs      map[string]string
	contextTokens map[string]string // key: "clientID:userID"
}

// NewMemoryStore creates a new in-memory Store.
func NewMemoryStore() Store {
	return &MemoryStore{
		credentials:   make(map[string]Credentials),
		syncBufs:      make(map[string]string),
		contextTokens: make(map[string]string),
	}
}

func (m *MemoryStore) SaveCredentials(_ context.Context, clientID string, creds Credentials) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.credentials[clientID] = creds
	return nil
}

func (m *MemoryStore) LoadCredentials(_ context.Context, clientID string) (Credentials, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	creds, ok := m.credentials[clientID]
	if !ok {
		return Credentials{}, fmt.Errorf("credentials not found for %q", clientID)
	}
	return creds, nil
}

func (m *MemoryStore) DeleteCredentials(_ context.Context, clientID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.credentials, clientID)
	return nil
}

func (m *MemoryStore) SaveSyncBuf(_ context.Context, clientID string, buf string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.syncBufs[clientID] = buf
	return nil
}

func (m *MemoryStore) LoadSyncBuf(_ context.Context, clientID string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.syncBufs[clientID], nil
}

func contextTokenKey(clientID, userID string) string {
	return clientID + ":" + userID
}

func (m *MemoryStore) SaveContextToken(_ context.Context, clientID, userID, token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.contextTokens[contextTokenKey(clientID, userID)] = token
	return nil
}

func (m *MemoryStore) LoadContextToken(_ context.Context, clientID, userID string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.contextTokens[contextTokenKey(clientID, userID)], nil
}
