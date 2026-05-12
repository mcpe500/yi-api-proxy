package admin

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gorouter/gorouter/internal/db"
)

type AuditHandler struct {
	db db.DatabaseManager
}

func NewAuditHandler(db db.DatabaseManager) *AuditHandler {
	return &AuditHandler{db: db}
}

type AuditLogsResponse struct {
	Items      []AuditLogItem `json:"items"`
	Total      int64          `json:"total"`
	Limit      int            `json:"limit"`
	Offset     int            `json:"offset"`
	HasMore    bool           `json:"has_more"`
}

type AuditLogItem struct {
	ID         string    `json:"id"`
	ActorID    string    `json:"actor_id"`
	ActorEmail string    `json:"actor_email"`
	ActorRole  string    `json:"actor_role"`
	Action     string    `json:"action"`
	TargetType string    `json:"target_type"`
	TargetID   string    `json:"target_id"`
	Details    string    `json:"details,omitempty"`
	IPAddress  string    `json:"ip_address,omitempty"`
	UserAgent  string    `json:"user_agent,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

func (h *AuditHandler) Logs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}

	filter := &db.AuditFilter{
		Limit:  limit,
		Offset: offset,
	}

	if actorID := r.URL.Query().Get("actor_id"); actorID != "" {
		filter.ActorID = actorID
	}
	if action := r.URL.Query().Get("action"); action != "" {
		filter.Action = action
	}
	if from := r.URL.Query().Get("from"); from != "" {
		if t, err := time.Parse(time.RFC3339, from); err == nil {
			filter.From = t.Unix()
		}
	}
	if to := r.URL.Query().Get("to"); to != "" {
		if t, err := time.Parse(time.RFC3339, to); err == nil {
			filter.To = t.Unix()
		}
	}

	logs, total, err := h.db.AuditLogs().List(ctx, filter)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"failed to fetch audit logs"}`))
		return
	}

	items := make([]AuditLogItem, 0, len(logs))
	for _, l := range logs {
		items = append(items, AuditLogItem{
			ID:         l.ID,
			ActorID:    l.ActorID,
			ActorEmail: l.ActorEmail,
			ActorRole:  l.ActorRole,
			Action:     l.Action,
			TargetType: l.TargetType,
			TargetID:   l.TargetID,
			Details:    l.Details,
			IPAddress:  l.IPAddress,
			UserAgent:  l.UserAgent,
			CreatedAt:  l.CreatedAt,
		})
	}

	resp := AuditLogsResponse{
		Items:   items,
		Total:   total,
		Limit:   limit,
		Offset:  offset,
		HasMore: int64(offset+limit) < total,
	}

	data, _ := json.Marshal(resp)
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}