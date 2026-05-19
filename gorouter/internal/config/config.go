package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type AppConfig struct {
	ListenAddr              string
	LogLevel                string
	DatabaseDriver          string
	DatabaseDSN             string
	SessionSecret           string
	BootstrapAdminEmail     string
	BootstrapAdminPassword  string
	SecretEncryptionKey     string
	RequireAPIKey           bool
	RedisURL                string
	EnableRequestBodyLog    bool
	UsageRetentionDays      int
	RequestLogRetentionDays int
	AuditLogRetentionDays   int
	IsProduction            bool
	RoutingStrategy         string
	AllowedOrigins          []string
	RTKEnabled              bool
	CavemanLevel            string
	OnlineSyncURL           string
	OnlineSyncToken         string
}

func Load() *AppConfig {
	cfg := &AppConfig{
		ListenAddr:              getEnv("GOROUTER_LISTEN_ADDR", ":20128"),
		LogLevel:                getEnv("GOROUTER_LOG_LEVEL", "info"),
		DatabaseDriver:          getEnv("GOROUTER_DATABASE_DRIVER", "sqlite"),
		DatabaseDSN:             getEnv("GOROUTER_DATABASE_DSN", "gorouter.db"),
		SessionSecret:           getEnv("GOROUTER_SESSION_SECRET", ""),
		BootstrapAdminEmail:     getEnv("GOROUTER_BOOTSTRAP_ADMIN_EMAIL", ""),
		BootstrapAdminPassword:  getEnv("GOROUTER_BOOTSTRAP_ADMIN_PASSWORD", ""),
		SecretEncryptionKey:     getEnv("GOROUTER_SECRET_ENCRYPTION_KEY", ""),
		RedisURL:                getEnv("GOROUTER_REDIS_URL", ""),
		UsageRetentionDays:      getEnvInt("GOROUTER_USAGE_RETENTION_DAYS", 180),
		RequestLogRetentionDays: getEnvInt("GOROUTER_REQUEST_LOG_RETENTION_DAYS", 30),
		AuditLogRetentionDays:   getEnvInt("GOROUTER_AUDIT_LOG_RETENTION_DAYS", 365),
		OnlineSyncURL:           getEnv("GOROUTER_ONLINE_SYNC_URL", ""),
		OnlineSyncToken:         getEnv("GOROUTER_ONLINE_SYNC_TOKEN", ""),
	}
	cfg.RequireAPIKey = getEnvBool("GOROUTER_REQUIRE_API_KEY", false)
	cfg.EnableRequestBodyLog = getEnvBool("GOROUTER_ENABLE_REQUEST_BODY_LOG", false)
	cfg.RoutingStrategy = getEnv("GOROUTER_ROUTING_STRATEGY", "priority")
	cfg.IsProduction = getEnv("APP_ENV", "development") == "production"
	cfg.RTKEnabled = getEnvBool("GOROUTER_RTK_ENABLED", false)
	cfg.CavemanLevel = getEnv("GOROUTER_CAVEMAN_LEVEL", "")

	origins := getEnv("GOROUTER_ALLOWED_ORIGINS", "*")
	if origins == "*" {
		cfg.AllowedOrigins = []string{"*"}
	} else {
		cfg.AllowedOrigins = strings.Split(origins, ",")
	}

	if !isValidStrategy(cfg.RoutingStrategy) {
		cfg.RoutingStrategy = "priority"
	}

	if cfg.SessionSecret == "" && cfg.IsProduction {
		fmt.Println("CRITICAL: GOROUTER_SESSION_SECRET is not set in production. Forcing shutdown for security.")
		os.Exit(1)
	}
	if cfg.SessionSecret == "" {
		cfg.SessionSecret = "dev-secret-change-me"
	}
	if cfg.SecretEncryptionKey == "" {
		cfg.SecretEncryptionKey = getEnv("SECRET_ENCRYPTION_KEY", "")
	}

	return cfg
}

func (c *AppConfig) Validate() error {
	if c.DatabaseDriver != "json" && c.DatabaseDriver != "sqlite" && c.DatabaseDriver != "postgres" && c.DatabaseDriver != "postgresql" {
		return fmt.Errorf("unsupported database driver: %s (use json, sqlite, or postgres)", c.DatabaseDriver)
	}
	if c.SessionSecret == "" && c.IsProduction {
		return fmt.Errorf("GOROUTER_SESSION_SECRET is required in production")
	}
	if c.BootstrapAdminEmail != "" && c.BootstrapAdminPassword == "" {
		return fmt.Errorf("GOROUTER_BOOTSTRAP_ADMIN_PASSWORD is required when GOROUTER_BOOTSTRAP_ADMIN_EMAIL is set")
	}
	if c.SecretEncryptionKey == "" && c.IsProduction {
		return fmt.Errorf("GOROUTER_SECRET_ENCRYPTION_KEY is required in production for secure provider secret storage")
	}
	if c.UsageRetentionDays < 1 {
		return fmt.Errorf("GOROUTER_USAGE_RETENTION_DAYS must be at least 1, got %d", c.UsageRetentionDays)
	}
	if c.RequestLogRetentionDays < 1 {
		return fmt.Errorf("GOROUTER_REQUEST_LOG_RETENTION_DAYS must be at least 1, got %d", c.RequestLogRetentionDays)
	}
	if c.AuditLogRetentionDays < 1 {
		return fmt.Errorf("GOROUTER_AUDIT_LOG_RETENTION_DAYS must be at least 1, got %d", c.AuditLogRetentionDays)
	}
	return nil
}

func (c *AppConfig) NormalizedDriver() string {
	if c.DatabaseDriver == "postgresql" {
		return "postgres"
	}
	return c.DatabaseDriver
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		lower := strings.ToLower(v)
		return lower == "true" || lower == "1" || lower == "yes"
	}
	return fallback
}

func mustGenerateOrPanic(key string) string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("failed to generate %s: %v", key, err))
	}
	return hex.EncodeToString(b)
}

func isValidStrategy(s string) bool {
	switch s {
	case "priority", "weighted", "latency", "cost", "fallback", "tier":
		return true
	}
	return false
}
