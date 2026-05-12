package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gorouter/gorouter/internal/apikeys"
	"github.com/gorouter/gorouter/internal/db"
)

type contextKey string

const (
	UserClaimsKey contextKey = "user_claims"
)

type UserClaims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
}

type contextKeyApiKey string

const ApiKeyValidationKey contextKeyApiKey = "apikey_validation"

func GetUserClaims(ctx context.Context) *UserClaims {
	if v := ctx.Value(UserClaimsKey); v != nil {
		if c, ok := v.(*UserClaims); ok {
			return c
		}
	}
	if v := ctx.Value(ApiKeyValidationKey); v != nil {
		if c, ok := v.(*db.ApiKeyValidation); ok {
			return &UserClaims{UserID: c.UserID, Role: c.UserRole}
		}
	}
	return nil
}

func ExtractAPIKey(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	if auth != "" {
		return auth
	}
	return r.Header.Get("X-API-Key")
}

func RequireAPIKey(svc *apikeys.ApiKeyService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			keyString := ExtractAPIKey(r)
			if keyString == "" {
				writeAuthError(w, "Missing API key. Provide Authorization: Bearer <key>")
				return
			}

			if !strings.HasPrefix(keyString, "sk-gorouter-") {
				writeAuthError(w, "Invalid API key format")
				return
			}

			validation, err := svc.ValidateKey(r.Context(), keyString)
			if err != nil {
				writeAuthError(w, "Invalid API key")
				return
			}

			ctx := context.WithValue(r.Context(), ApiKeyValidationKey, validation)
			ctx = context.WithValue(ctx, UserClaimsKey, &UserClaims{
				UserID: validation.UserID,
				Role:   validation.UserRole,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequireScope(scope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			validation, ok := r.Context().Value(ApiKeyValidationKey).(*db.ApiKeyValidation)
			if !ok {
				writeAuthError(w, "API key context missing")
				return
			}

			hasScope := false
			for _, s := range validation.Scopes {
				if s == scope {
					hasScope = true
					break
				}
			}
			if !hasScope {
				writeError(w, http.StatusForbidden, "Insufficient scope: requires "+scope)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func writeAuthError(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{
			"message": message,
			"type":    "authentication_error",
			"code":    "invalid_api_key",
		},
	})
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{
			"message": message,
		},
	})
}