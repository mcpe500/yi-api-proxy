package admin

import (
	"encoding/json"
	"net/http"
	"runtime"
	"time"

	"github.com/gorouter/gorouter/internal/config"
)

type SystemHandler struct {
	cfg       *config.AppConfig
	startTime time.Time
	version   string
}

func NewSystemHandler(cfg *config.AppConfig, version string) *SystemHandler {
	return &SystemHandler{
		cfg:       cfg,
		startTime: time.Now(),
		version:   version,
	}
}

type SystemInfoResponse struct {
	Version   string    `json:"version"`
	GoVersion string    `json:"go_version"`
	Uptime    string    `json:"uptime"`
	StartTime time.Time `json:"start_time"`
}

func (h *SystemHandler) Info(w http.ResponseWriter, r *http.Request) {
	info := SystemInfoResponse{
		Version:   h.version,
		GoVersion: runtime.Version(),
		Uptime:    time.Since(h.startTime).String(),
		StartTime: h.startTime,
	}

	data, _ := json.Marshal(info)
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

type SystemConfigResponse struct {
	ListenAddr         string `json:"listen_addr"`
	LogLevel           string `json:"log_level"`
	DatabaseDriver     string `json:"database_driver"`
	RequireAPIKey      bool   `json:"require_api_key"`
	EnableRequestBodyLog bool `json:"enable_request_body_log"`
	UsageRetentionDays int    `json:"usage_retention_days"`
	AuditLogRetentionDays int `json:"audit_log_retention_days"`
	IsProduction       bool   `json:"is_production"`
	RedisURL           string `json:"redis_url,omitempty"`
}

func (h *SystemHandler) Config(w http.ResponseWriter, r *http.Request) {
	cfg := SystemConfigResponse{
		ListenAddr:          h.cfg.ListenAddr,
		LogLevel:           h.cfg.LogLevel,
		DatabaseDriver:     h.cfg.DatabaseDriver,
		RequireAPIKey:     h.cfg.RequireAPIKey,
		EnableRequestBodyLog: h.cfg.EnableRequestBodyLog,
		UsageRetentionDays:   h.cfg.UsageRetentionDays,
		AuditLogRetentionDays: h.cfg.AuditLogRetentionDays,
		IsProduction:       h.cfg.IsProduction,
		RedisURL:          h.cfg.RedisURL,
	}

	data, _ := json.Marshal(cfg)
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}