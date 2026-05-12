package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/gorouter/gorouter/internal/audit"
)

type contextKeyAuditActor string

const AuditActorKey contextKeyAuditActor = "audit_actor"

func AuditContextMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actor := &audit.AuditActor{
				IPAddress: GetClientIP(r),
				UserAgent: r.UserAgent(),
			}

			if user := GetUserFromContext(r.Context()); user != nil {
				actor.UserID = user.UserID
				actor.Email = user.Email
				actor.Role = user.Role
			} else if claims := GetUserClaims(r.Context()); claims != nil {
				actor.UserID = claims.UserID
				actor.Email = claims.Email
				actor.Role = claims.Role
			}

			ctx := context.WithValue(r.Context(), AuditActorKey, actor)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetAuditActor(ctx context.Context) *audit.AuditActor {
	if v := ctx.Value(AuditActorKey); v != nil {
		if a, ok := v.(*audit.AuditActor); ok {
			return a
		}
	}
	return nil
}

func GetClientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		parts := strings.SplitN(ip, ",", 2)
		return strings.TrimSpace(parts[0])
	}
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	return r.RemoteAddr
}
