package store

import (
	"context"
	"time"
)

// Credentials holds the authentication data for a bot account.
// Serialized as opaque bytes by the SDK; the Store doesn't need to parse it.
type Credentials struct {
	Token   string    `json:"token"`
	BaseURL string    `json:"base_url"`
	UserID  string    `json:"user_id"`
	SavedAt time.Time `json:"saved_at"`
}

// CredentialStore persists account credentials across restarts.
type CredentialStore interface {
	SaveCredentials(ctx context.Context, clientID string, creds Credentials) error
	LoadCredentials(ctx context.Context, clientID string) (Credentials, error)
	DeleteCredentials(ctx context.Context, clientID string) error
}

// SyncStore persists the long-poll sync cursor.
type SyncStore interface {
	SaveSyncBuf(ctx context.Context, clientID string, buf string) error
	LoadSyncBuf(ctx context.Context, clientID string) (string, error)
}

// ContextTokenStore persists per-user context tokens for message replies.
type ContextTokenStore interface {
	SaveContextToken(ctx context.Context, clientID, userID, token string) error
	LoadContextToken(ctx context.Context, clientID, userID string) (string, error)
}

// Store combines all storage concerns.
type Store interface {
	CredentialStore
	SyncStore
	ContextTokenStore
}

