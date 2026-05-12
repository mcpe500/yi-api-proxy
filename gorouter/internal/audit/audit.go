package audit

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/gorouter/gorouter/internal/db"
)

type AuditLogger struct {
	db    db.DatabaseManager
	quiet bool
}

type AuditActor struct {
	UserID    string
	Email     string
	Role      string
	IPAddress string
	UserAgent string
}

func NewAuditLogger(dbManager db.DatabaseManager) *AuditLogger {
	return &AuditLogger{db: dbManager, quiet: false}
}

func (l *AuditLogger) Log(ctx context.Context, actor *AuditActor, action, targetType, targetID, details string) {
	if l.quiet || l.db == nil || l.db.AuditLogs() == nil {
		return
	}

	event := &db.AuditLog{
		ID:         "audit_" + uuid.New().String(),
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		Details:    details,
		CreatedAt:  time.Now(),
	}

	if actor != nil {
		event.ActorID = actor.UserID
		event.ActorEmail = actor.Email
		event.ActorRole = actor.Role
		event.IPAddress = actor.IPAddress
		event.UserAgent = actor.UserAgent
	}

	go func() {
		if err := l.db.AuditLogs().Create(context.Background(), event); err != nil {
			slog.Error("audit log write failed", "action", action, "error", err)
		}
	}()
}
