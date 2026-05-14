package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/gorouter/gorouter/internal/crypto"
	"github.com/gorouter/gorouter/internal/db"
)

type TokenRefresher struct {
	db         db.DatabaseManager
	httpClient *http.Client
}

func NewTokenRefresher(db db.DatabaseManager) *TokenRefresher {
	return &TokenRefresher{
		db:         db,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (tr *TokenRefresher) GetValidToken(ctx context.Context, provider *db.ProviderConnection) (string, error) {
	if provider.OAuthTokenURL == "" || provider.OAuthRefreshToken == "" {
		return crypto.Decrypt(string(provider.EncryptedSecret))
	}

	if provider.OAuthAccessToken != "" && provider.OAuthExpiresAt > time.Now().Unix() {
		return provider.OAuthAccessToken, nil
	}

	return tr.RefreshToken(ctx, provider)
}

func (tr *TokenRefresher) RefreshToken(ctx context.Context, provider *db.ProviderConnection) (string, error) {
	data := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {provider.OAuthRefreshToken},
		"client_id":     {provider.OAuthClientID},
		"scope":         {provider.OAuthScopes},
	}

	if provider.OAuthClientSecret != "" {
		decrypted, err := crypto.Decrypt(provider.OAuthClientSecret)
		if err == nil {
			data.Set("client_secret", decrypted)
		}
	}

	resp, err := tr.httpClient.PostForm(provider.OAuthTokenURL, data)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token refresh failed: %d %s", resp.StatusCode, string(body))
	}

	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	provider.OAuthAccessToken = result.AccessToken
	if result.RefreshToken != "" {
		provider.OAuthRefreshToken = result.RefreshToken
	}
	if result.ExpiresIn > 0 {
		provider.OAuthExpiresAt = time.Now().Unix() + int64(result.ExpiresIn) - 60
	}

	if err := tr.db.Providers().Update(ctx, provider); err != nil {
		return "", err
	}

	return result.AccessToken, nil
}
