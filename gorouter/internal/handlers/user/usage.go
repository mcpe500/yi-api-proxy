package user

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gorouter/gorouter/internal/db"
	"github.com/gorouter/gorouter/internal/middleware"
)

type UsageHandler struct {
	db db.DatabaseManager
}

func NewUsageHandler(dbManager db.DatabaseManager) *UsageHandler {
	return &UsageHandler{db: dbManager}
}

type UsageHistoryResponse struct {
	Object string              `json:"object"`
	Data   []*db.UsageEvent    `json:"data"`
	Total  int                 `json:"total"`
}

func (h *UsageHandler) History(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}

	events, err := h.db.UsageEvents().FindByUserID(ctx, claims.UserID, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if events == nil {
		events = []*db.UsageEvent{}
	}

	resp := UsageHistoryResponse{
		Object: "list",
		Data:   events,
		Total:  len(events),
	}

	writeJSON(w, http.StatusOK, resp)
}

type UsageSummaryResponse struct {
	Object     string `json:"object"`
	TotalDays  int    `json:"total_days"`
	Summary    struct {
		TotalRequests       int64   `json:"total_requests"`
		TotalPromptTokens   int64   `json:"total_prompt_tokens"`
		TotalCompletionTokens int64 `json:"total_completion_tokens"`
		TotalTokens         int64   `json:"total_tokens"`
		TotalCost           float64 `json:"total_cost"`
		AvgLatencyMs        float64 `json:"avg_latency_ms"`
		ErrorCount          int64   `json:"error_count"`
	} `json:"summary"`
}

func (h *UsageHandler) Summary(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	now := time.Now()
	thirtyDaysAgo := now.AddDate(0, 0, -30).Unix()

	summary, err := h.db.UsageEvents().GetSummary(ctx, claims.UserID, thirtyDaysAgo, now.Unix())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	resp := UsageSummaryResponse{
		Object:    "usage.summary",
		TotalDays: 30,
	}
	resp.Summary.TotalRequests = summary.TotalRequests
	resp.Summary.TotalPromptTokens = summary.TotalPromptTokens
	resp.Summary.TotalCompletionTokens = summary.TotalCompletionTokens
	resp.Summary.TotalTokens = summary.TotalTokens
	resp.Summary.TotalCost = summary.TotalCost
	resp.Summary.AvgLatencyMs = summary.AvgLatencyMs
	resp.Summary.ErrorCount = summary.ErrorCount

	writeJSON(w, http.StatusOK, resp)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
