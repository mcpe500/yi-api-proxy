package db

import (
	"context"
	"log/slog"
	"time"
)

// RetentionPolicy defines cleanup intervals and retention periods
type RetentionPolicy struct {
	CleanupInterval time.Duration // How often to run cleanup
	UsageRetention  time.Duration // How long to keep usage events
	RequestRetention time.Duration // How long to keep request logs
	AuditRetention  time.Duration // How long to keep audit logs
}

// DefaultRetentionPolicy creates a policy based on config values (in days)
func DefaultRetentionPolicy(usageDays, requestDays, auditDays int) RetentionPolicy {
	return RetentionPolicy{
		CleanupInterval: 24 * time.Hour, // Run daily
		UsageRetention:  time.Duration(usageDays) * 24 * time.Hour,
		RequestRetention: time.Duration(requestDays) * 24 * time.Hour,
		AuditRetention:  time.Duration(auditDays) * 24 * time.Hour,
	}
}

// StartRetentionCleanup starts a background goroutine that periodically cleans up old records
func StartRetentionCleanup(ctx context.Context, db DatabaseManager, policy RetentionPolicy, logger *slog.Logger) {
	if policy.UsageRetention <= 0 && policy.AuditRetention <= 0 {
		logger.Info("retention cleanup disabled (retention periods set to 0)")
		return
	}

	logger.Info("starting retention cleanup worker",
		"usage_retention_days", int(policy.UsageRetention.Hours()/24),
		"audit_retention_days", int(policy.AuditRetention.Hours()/24),
		"cleanup_interval_hours", int(policy.CleanupInterval.Hours()))

	ticker := time.NewTicker(policy.CleanupInterval)
	go func() {
		// Run once on startup after a brief delay
		time.Sleep(30 * time.Second)
		runCleanup(ctx, db, policy, logger)
		
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				logger.Info("retention cleanup worker stopped")
				return
			case <-ticker.C:
				runCleanup(ctx, db, policy, logger)
			}
		}
	}()
}

func runCleanup(ctx context.Context, db DatabaseManager, policy RetentionPolicy, logger *slog.Logger) {
	now := time.Now()
	
	// Cleanup usage events
	if policy.UsageRetention > 0 {
		cutoff := now.Add(-policy.UsageRetention).Unix()
		deleted, err := db.UsageEvents().DeleteOlderThan(ctx, cutoff)
		if err != nil {
			logger.Error("failed to cleanup usage events", "error", err)
		} else if deleted > 0 {
			logger.Info("cleaned up old usage events", "deleted", deleted, "older_than_days", int(policy.UsageRetention.Hours()/24))
		}
	}
	
	// Cleanup audit logs
	if policy.AuditRetention > 0 {
		cutoff := now.Add(-policy.AuditRetention).Unix()
		deleted, err := db.AuditLogs().DeleteOlderThan(ctx, cutoff)
		if err != nil {
			logger.Error("failed to cleanup audit logs", "error", err)
		} else if deleted > 0 {
			logger.Info("cleaned up old audit logs", "deleted", deleted, "older_than_days", int(policy.AuditRetention.Hours()/24))
		}
	}
}