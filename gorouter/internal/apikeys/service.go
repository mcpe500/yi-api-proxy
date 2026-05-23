package apikeys

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/gorouter/gorouter/internal/db"
)

type ApiKeyService struct {
	db db.DatabaseManager
}

func NewApiKeyService(dbm db.DatabaseManager) *ApiKeyService {
	return &ApiKeyService{db: dbm}
}

func (s *ApiKeyService) CreateKey(ctx context.Context, userID, name string, scopes []string, expiresAt *time.Time) (*db.ApiKeyCreated, error) {
	if scopes == nil {
		scopes = db.DefaultScopes
	}

	count, err := s.db.ApiKeys().CountByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("count keys: %w", err)
	}
	if count >= 10 {
		return nil, fmt.Errorf("user has reached maximum of 10 API keys")
	}

	fullKey, keyHash, keyPrefix, err := GenerateKey("")
	if err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}

	apiKey := &db.ApiKey{
		ID:        uuid.New().String(),
		UserID:    userID,
		Name:      name,
		KeyPrefix: keyPrefix,
		KeyHash:   keyHash,
		Scopes:    scopes,
		Status:    "active",
		ExpiresAt: expiresAt,
		CreatedAt: time.Now(),
	}

	if err := s.db.ApiKeys().Create(ctx, apiKey); err != nil {
		return nil, fmt.Errorf("create key: %w", err)
	}

	return &db.ApiKeyCreated{
		ID:        apiKey.ID,
		Key:       fullKey,
		Name:      name,
		KeyPrefix: keyPrefix,
		Scopes:    scopes,
		ExpiresAt: expiresAt,
		CreatedAt: apiKey.CreatedAt,
	}, nil
}

func (s *ApiKeyService) ValidateKey(ctx context.Context, keyString string) (*db.ApiKeyValidation, error) {
	// Extract prefix from the API key (format: sk-gorouter-<prefix>_<secret>)
	prefix := keyString[:18] // "sk-gorouter-" + 6 char prefix

	candidates, err := s.db.ApiKeys().FindActiveByPrefix(ctx, prefix)
	if err != nil {
		return nil, fmt.Errorf("find key: %w", err)
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("invalid API key")
	}

	// Verify the key against stored hashes
	for _, apiKey := range candidates {
		if !VerifyKey(keyString, apiKey.KeyHash) {
			continue
		}

		if apiKey.Status != "active" {
			return nil, fmt.Errorf("API key revoked")
		}
		if apiKey.ExpiresAt != nil && apiKey.ExpiresAt.Before(time.Now()) {
			return nil, fmt.Errorf("API key expired")
		}

		user, err := s.db.Users().FindByID(ctx, apiKey.UserID)
		if err != nil {
			return nil, fmt.Errorf("find user: %w", err)
		}
		if user == nil || user.Status != "active" {
			return nil, fmt.Errorf("user suspended or not found")
		}

		_ = s.db.ApiKeys().UpdateLastUsed(ctx, apiKey.ID)

		return &db.ApiKeyValidation{
			UserID:    user.ID,
			UserRole:  user.Role,
			KeyID:     apiKey.ID,
			KeyPrefix: apiKey.KeyPrefix,
			Scopes:    apiKey.Scopes,
		}, nil
	}

	return nil, fmt.Errorf("invalid API key")
}

func (s *ApiKeyService) RevokeKey(ctx context.Context, keyID string) error {
	key, err := s.db.ApiKeys().FindByID(ctx, keyID)
	if err != nil {
		return fmt.Errorf("find key: %w", err)
	}
	if key == nil {
		return fmt.Errorf("API key not found")
	}
	return s.db.ApiKeys().Revoke(ctx, keyID)
}

func (s *ApiKeyService) RotateKey(ctx context.Context, keyID string) (*db.ApiKeyCreated, error) {
	oldKey, err := s.db.ApiKeys().FindByID(ctx, keyID)
	if err != nil {
		return nil, fmt.Errorf("find key: %w", err)
	}
	if oldKey == nil {
		return nil, fmt.Errorf("API key not found")
	}
	if oldKey.Status != "active" {
		return nil, fmt.Errorf("cannot rotate revoked key")
	}

	created, err := s.CreateKey(ctx, oldKey.UserID, oldKey.Name, oldKey.Scopes, oldKey.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("create replacement: %w", err)
	}

	if err := s.db.ApiKeys().Revoke(ctx, keyID); err != nil {
		return nil, fmt.Errorf("revoke old key: %w", err)
	}

	return created, nil
}

func (s *ApiKeyService) ListKeys(ctx context.Context, userID string) ([]*db.ApiKey, error) {
	return s.db.ApiKeys().FindByUserID(ctx, userID)
}

func (s *ApiKeyService) ListAllKeys(ctx context.Context) ([]*db.ApiKey, error) {
	return s.db.ApiKeys().List(ctx)
}

func (s *ApiKeyService) GetKey(ctx context.Context, keyID string) (*db.ApiKey, error) {
	return s.db.ApiKeys().FindByID(ctx, keyID)
}

func (s *ApiKeyService) RevokeAllForUser(ctx context.Context, userID string) error {
	return s.db.ApiKeys().RevokeAllForUser(ctx, userID)
}
