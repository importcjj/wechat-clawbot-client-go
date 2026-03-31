package store

import (
	"context"
	"encoding/json"
	"fmt"
)

// RedisClient is the minimal interface that a Redis client must implement.
// Compatible with github.com/redis/go-redis/v9 and github.com/go-redis/redis/v8.
type RedisClient interface {
	Get(ctx context.Context, key string) StringResult
	Set(ctx context.Context, key string, value any, expiration int64) StatusResult
	Del(ctx context.Context, keys ...string) IntResult
	HGet(ctx context.Context, key, field string) StringResult
	HSet(ctx context.Context, key string, values ...any) IntResult
}

// StringResult is the result of a Redis GET/HGET command.
type StringResult interface {
	Result() (string, error)
}

// StatusResult is the result of a Redis SET command.
type StatusResult interface {
	Err() error
}

// IntResult is the result of a Redis DEL/HSET command.
type IntResult interface {
	Err() error
}

// RedisStore implements Store using Redis.
//
// Key layout:
//
//	wechat:creds:{clientID}          → JSON(Credentials)
//	wechat:sync:{clientID}           → string (get_updates_buf)
//	wechat:ctx_tokens:{clientID}     → Hash { userID: token }
type RedisStore struct {
	client    RedisClient
	keyPrefix string
}

// RedisOption configures a RedisStore.
type RedisOption func(*RedisStore)

// WithRedisKeyPrefix sets a custom key prefix (default: "wechat").
func WithRedisKeyPrefix(prefix string) RedisOption {
	return func(s *RedisStore) {
		s.keyPrefix = prefix
	}
}

// NewRedisStore creates a Store backed by Redis.
// The client parameter must implement RedisClient.
//
// With go-redis/v9:
//
//	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
//	s := store.NewRedisStore(store.NewGoRedisAdapter(rdb))
func NewRedisStore(client RedisClient, opts ...RedisOption) Store {
	s := &RedisStore{
		client:    client,
		keyPrefix: "wechat",
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *RedisStore) credKey(clientID string) string {
	return s.keyPrefix + ":creds:" + clientID
}

func (s *RedisStore) syncKey(clientID string) string {
	return s.keyPrefix + ":sync:" + clientID
}

func (s *RedisStore) tokenKey(clientID string) string {
	return s.keyPrefix + ":ctx_tokens:" + clientID
}

func (s *RedisStore) SaveCredentials(ctx context.Context, clientID string, creds Credentials) error {
	data, err := json.Marshal(creds)
	if err != nil {
		return fmt.Errorf("marshaling credentials: %w", err)
	}
	if err := s.client.Set(ctx, s.credKey(clientID), string(data), 0).Err(); err != nil {
		return fmt.Errorf("redis SET credentials: %w", err)
	}
	return nil
}

func (s *RedisStore) LoadCredentials(ctx context.Context, clientID string) (Credentials, error) {
	val, err := s.client.Get(ctx, s.credKey(clientID)).Result()
	if err != nil {
		return Credentials{}, fmt.Errorf("redis GET credentials for %q: %w", clientID, err)
	}
	var creds Credentials
	if err := json.Unmarshal([]byte(val), &creds); err != nil {
		return Credentials{}, fmt.Errorf("unmarshaling credentials: %w", err)
	}
	return creds, nil
}

func (s *RedisStore) DeleteCredentials(ctx context.Context, clientID string) error {
	if err := s.client.Del(ctx, s.credKey(clientID)).Err(); err != nil {
		return fmt.Errorf("redis DEL credentials: %w", err)
	}
	return nil
}

func (s *RedisStore) SaveSyncBuf(ctx context.Context, clientID string, buf string) error {
	if err := s.client.Set(ctx, s.syncKey(clientID), buf, 0).Err(); err != nil {
		return fmt.Errorf("redis SET sync buf: %w", err)
	}
	return nil
}

func (s *RedisStore) LoadSyncBuf(ctx context.Context, clientID string) (string, error) {
	val, err := s.client.Get(ctx, s.syncKey(clientID)).Result()
	if err != nil {
		return "", nil // not found is fine
	}
	return val, nil
}

func (s *RedisStore) SaveContextToken(ctx context.Context, clientID, userID, token string) error {
	if err := s.client.HSet(ctx, s.tokenKey(clientID), userID, token).Err(); err != nil {
		return fmt.Errorf("redis HSET context token: %w", err)
	}
	return nil
}

func (s *RedisStore) LoadContextToken(ctx context.Context, clientID, userID string) (string, error) {
	val, err := s.client.HGet(ctx, s.tokenKey(clientID), userID).Result()
	if err != nil {
		return "", nil // not found is fine
	}
	return val, nil
}
