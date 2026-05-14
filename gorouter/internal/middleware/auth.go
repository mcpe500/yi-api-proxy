package middleware

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
)

type APIKeyConfig struct {
	RequireAPIKey bool
	ValidKeys     map[string]string
}

type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code,omitempty"`
}

type contextKeyUser string

const UserContextKey contextKeyUser = "user"

type UserContext struct {
	UserID  string `json:"user_id"`
	Email   string `json:"email"`
	Role    string `json:"role"`
}

func GetUserFromContext(ctx context.Context) *UserContext {
	if v := ctx.Value(UserContextKey); v != nil {
		if u, ok := v.(*UserContext); ok {
			return u
		}
	}
	return nil
}

func RequireAuth(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("auth_token")
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(ErrorResponse{
					Error: ErrorDetail{Message: "Missing auth token", Type: "authentication_error"},
				})
				return
			}

			token, err := jwt.Parse(cookie.Value, func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return []byte(jwtSecret), nil
			})

			if err != nil || !token.Valid {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(ErrorResponse{
					Error: ErrorDetail{Message: "Invalid auth token", Type: "authentication_error"},
				})
				return
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(ErrorResponse{
					Error: ErrorDetail{Message: "Invalid token claims", Type: "authentication_error"},
				})
				return
			}

			userID, _ := claims["user_id"].(string)
			email, _ := claims["email"].(string)
			role, _ := claims["role"].(string)

			ctx := context.WithValue(r.Context(), UserContextKey, &UserContext{
				UserID: userID,
				Email:  email,
				Role:   role,
			})
			ctx = context.WithValue(ctx, UserClaimsKey, &UserClaims{
				UserID: userID,
				Email:  email,
				Role:   role,
			})

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequireAdmin() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := GetUserFromContext(r.Context())
			if user == nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(ErrorResponse{
					Error: ErrorDetail{Message: "Unauthorized", Type: "authentication_error"},
				})
				return
			}
			if user.Role != "admin" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(ErrorResponse{
					Error: ErrorDetail{Message: "Admin access required", Type: "authorization_error"},
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
