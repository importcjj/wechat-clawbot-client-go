package store

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// FileStore persists data as JSON files in a directory.
type FileStore struct {
	dir string
	mu  sync.RWMutex
}

// NewFileStore creates a file-based Store. The directory is created if needed.
func NewFileStore(dir string) Store {
	return &FileStore{dir: dir}
}

func (f *FileStore) ensureDir() error {
	return os.MkdirAll(f.dir, 0o755)
}

func (f *FileStore) credPath(clientID string) string {
	return filepath.Join(f.dir, sanitize(clientID)+".creds.json")
}

func (f *FileStore) syncPath(clientID string) string {
	return filepath.Join(f.dir, sanitize(clientID)+".sync.json")
}

func (f *FileStore) tokenPath(clientID string) string {
	return filepath.Join(f.dir, sanitize(clientID)+".tokens.json")
}

func (f *FileStore) SaveCredentials(_ context.Context, clientID string, creds Credentials) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.ensureDir(); err != nil {
		return fmt.Errorf("creating store dir: %w", err)
	}
	return writeJSON(f.credPath(clientID), creds)
}

func (f *FileStore) LoadCredentials(_ context.Context, clientID string) (Credentials, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	var creds Credentials
	if err := readJSON(f.credPath(clientID), &creds); err != nil {
		return Credentials{}, fmt.Errorf("loading credentials for %q: %w", clientID, err)
	}
	return creds, nil
}

func (f *FileStore) DeleteCredentials(_ context.Context, clientID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	os.Remove(f.credPath(clientID))
	return nil
}

func (f *FileStore) SaveSyncBuf(_ context.Context, clientID string, buf string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.ensureDir(); err != nil {
		return fmt.Errorf("creating store dir: %w", err)
	}
	return writeJSON(f.syncPath(clientID), map[string]string{"get_updates_buf": buf})
}

func (f *FileStore) LoadSyncBuf(_ context.Context, clientID string) (string, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	var data map[string]string
	if err := readJSON(f.syncPath(clientID), &data); err != nil {
		return "", nil // not found is fine
	}
	return data["get_updates_buf"], nil
}

func (f *FileStore) SaveContextToken(_ context.Context, clientID, userID, token string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.ensureDir(); err != nil {
		return fmt.Errorf("creating store dir: %w", err)
	}

	tokens := f.loadTokenMap(clientID)
	tokens[userID] = token
	return writeJSON(f.tokenPath(clientID), tokens)
}

func (f *FileStore) LoadContextToken(_ context.Context, clientID, userID string) (string, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	tokens := f.loadTokenMap(clientID)
	return tokens[userID], nil
}

func (f *FileStore) loadTokenMap(clientID string) map[string]string {
	var tokens map[string]string
	if err := readJSON(f.tokenPath(clientID), &tokens); err != nil {
		return make(map[string]string)
	}
	return tokens
}

// sanitize makes a string safe for use as a filename component.
func sanitize(s string) string {
	result := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			result = append(result, c)
		} else {
			result = append(result, '-')
		}
	}
	return string(result)
}

func writeJSON(path string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	// Atomic write: write to temp, then rename
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func readJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}
