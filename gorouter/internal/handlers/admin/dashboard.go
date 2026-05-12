package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorouter/gorouter/internal/db"
)

type DashboardHandler struct {
	db db.DatabaseManager
}

func NewDashboardHandler(db db.DatabaseManager) *DashboardHandler {
	return &DashboardHandler{db: db}
}

type StatsResponse struct {
	TotalUsers     int64       `json:"total_users"`
	TotalApiKeys   int64       `json:"total_api_keys"`
	TotalProviders int64       `json:"total_providers"`
	TotalModels    int64       `json:"total_models"`
	UsageToday      *UsageStats `json:"usage_today,omitempty"`
}

type UsageStats struct {
	TotalRequests  int64   `json:"total_requests"`
	TotalTokens     int64   `json:"total_tokens"`
	EstimatedCost   float64 `json:"estimated_cost"`
	AvgLatencyMs    float64 `json:"avg_latency_ms"`
}

func (h *DashboardHandler) Stats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	resp := StatsResponse{}

	if users, err := h.db.Users().List(ctx, db.UserFilter{}); err == nil {
		resp.TotalUsers = int64(len(users))
	}

	if keys, err := h.db.ApiKeys().List(ctx); err == nil {
		resp.TotalApiKeys = int64(len(keys))
	}

	if providers, err := h.db.Providers().List(ctx); err == nil {
		resp.TotalProviders = int64(len(providers))
	}

	if models, err := h.db.Models().List(ctx); err == nil {
		resp.TotalModels = int64(len(models))
	}

	today := time.Now()
	startOfDay := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())
	usage, err := h.getUsageStats(ctx, startOfDay.Unix(), today.Unix())
	if err == nil {
		resp.UsageToday = usage
	}

	data, _ := json.Marshal(resp)
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func (h *DashboardHandler) getUsageStats(ctx context.Context, from, to int64) (*UsageStats, error) {
	stats := &UsageStats{}

	var totalLatency int64
	events, _ := h.db.UsageEvents().FindByDateRange(ctx, "", from, to)
	for _, e := range events {
		stats.TotalRequests++
		stats.TotalTokens += int64(e.TotalTokens)
		stats.EstimatedCost += e.EstimatedCost
		totalLatency += int64(e.LatencyMs)
	}

	if stats.TotalRequests > 0 {
		stats.AvgLatencyMs = float64(totalLatency) / float64(stats.TotalRequests)
	}

	return stats, nil
}

type ActivityResponse struct {
	Items []ActivityItem `json:"items"`
}

type ActivityItem struct {
	ID            string    `json:"id"`
	UserID        string    `json:"user_id"`
	Model         string    `json:"model"`
	Provider      string    `json:"provider"`
	StatusCode    int       `json:"status_code"`
	LatencyMs     int       `json:"latency_ms"`
	Tokens        int       `json:"tokens"`
	EstimatedCost float64   `json:"estimated_cost"`
	CreatedAt     time.Time `json:"created_at"`
}

func (h *DashboardHandler) RecentActivity(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	limit := 50

	items := []ActivityItem{}
	events, _ := h.db.UsageEvents().FindByUserID(ctx, "", limit)
	for _, e := range events {
		items = append(items, ActivityItem{
			ID:            e.ID,
			UserID:        e.UserID,
			Model:         e.FinalModel,
			Provider:      e.Provider,
			StatusCode:    e.StatusCode,
			LatencyMs:     e.LatencyMs,
			Tokens:        e.TotalTokens,
			EstimatedCost: e.EstimatedCost,
			CreatedAt:     e.CreatedAt,
		})
	}

	data, _ := json.Marshal(ActivityResponse{Items: items})
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

type HealthResponse struct {
	Status    string            `json:"status"`
	Checks    map[string]Check  `json:"checks"`
	Timestamp time.Time         `json:"timestamp"`
}

type Check struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

func (h *DashboardHandler) Health(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	checks := make(map[string]Check)

	checks["database"] = Check{Status: "ok"}

	providers, _ := h.db.Providers().List(ctx)
	providerHealth := "ok"
	for _, p := range providers {
		if p.Status == "error" || p.Status == "cooldown" {
			providerHealth = "degraded"
			break
		}
	}
	checks["providers"] = Check{Status: providerHealth}

	models, _ := h.db.Models().List(ctx)
	modelHealth := "ok"
	if len(models) == 0 {
		modelHealth = "warning"
	}
	checks["models"] = Check{Status: modelHealth}

	overallStatus := "ok"
	for _, c := range checks {
		if c.Status != "ok" {
			overallStatus = "degraded"
			break
		}
	}

	data, _ := json.Marshal(HealthResponse{
		Status:    overallStatus,
		Checks:    checks,
		Timestamp: time.Now(),
	})
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}