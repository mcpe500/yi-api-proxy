package combo

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gorouter/gorouter/internal/db"
)

type ComboManager struct {
	db db.DatabaseManager
}

func NewComboManager(database db.DatabaseManager) *ComboManager {
	return &ComboManager{db: database}
}

func (m *ComboManager) LoadCombos(ctx context.Context) ([]*db.Combo, error) {
	return m.db.Combos().ListEnabled(ctx)
}

func (m *ComboManager) GetCombo(ctx context.Context, id string) (*db.Combo, error) {
	return m.db.Combos().FindByID(ctx, id)
}

func (m *ComboManager) GetComboByName(ctx context.Context, name string) (*db.Combo, error) {
	return m.db.Combos().FindByName(ctx, name)
}

func (m *ComboManager) ListByUser(ctx context.Context, userID string) ([]*db.Combo, error) {
	return m.db.Combos().FindByUser(ctx, userID)
}

func (m *ComboManager) Create(ctx context.Context, combo *db.Combo) error {
	return m.db.Combos().Create(ctx, combo)
}

func (m *ComboManager) Update(ctx context.Context, combo *db.Combo) error {
	return m.db.Combos().Update(ctx, combo)
}

func (m *ComboManager) Delete(ctx context.Context, id string) error {
	return m.db.Combos().Delete(ctx, id)
}

func (m *ComboManager) AddItem(ctx context.Context, item *db.ComboItem) error {
	return m.db.Combos().AddItem(ctx, item)
}

func (m *ComboManager) RemoveItem(ctx context.Context, itemID string) error {
	return m.db.Combos().RemoveItem(ctx, itemID)
}

func (m *ComboManager) ReorderItems(ctx context.Context, comboID string, itemIDs []string) error {
	return m.db.Combos().ReorderItems(ctx, comboID, itemIDs)
}

type ExecuteRequest struct {
	Model    string                 `json:"model"`
	Messages []map[string]string    `json:"messages"`
	Stream   bool                   `json:"stream,omitempty"`
}

type ExecuteResponse struct {
	Success       bool               `json:"success"`
	Model         string             `json:"model,omitempty"`
	Response      map[string]any     `json:"response,omitempty"`
	Error         *ExecuteError      `json:"error,omitempty"`
	FallbackCount int                `json:"fallback_count"`
	LatencyMs     int                `json:"latency_ms"`
}

type ExecuteError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type httpClient interface {
	Do(*http.Request) (*http.Response, error)
}

func (m *ComboManager) Execute(ctx context.Context, comboID string, req ExecuteRequest) (*ExecuteResponse, error) {
	combo, err := m.GetCombo(ctx, comboID)
	if err != nil {
		return nil, fmt.Errorf("combo not found: %w", err)
	}

	if len(combo.Items) == 0 {
		return &ExecuteResponse{
			Success: false,
			Error: &ExecuteError{
				Code:    "no_providers",
				Message: "combo has no providers configured",
			},
		}, nil
	}

	fallbackCount := 0
	startTime := time.Now()

	for i, item := range combo.Items {
		result, err := m.executeItem(ctx, item, req)
		if err != nil {
			if i < len(combo.Items)-1 {
				fallbackCount++
				continue
			}
			return &ExecuteResponse{
				Success:       false,
				FallbackCount: fallbackCount,
				LatencyMs:     int(time.Since(startTime).Milliseconds()),
				Error: &ExecuteError{
					Code:    "all_providers_failed",
					Message: err.Error(),
				},
			}, nil
		}

		if result.Success {
			return &ExecuteResponse{
				Success:       true,
				Model:         item.ModelID,
				Response:      result.Response,
				FallbackCount: fallbackCount,
				LatencyMs:     int(time.Since(startTime).Milliseconds()),
			}, nil
		}

		if !result.FallbackEligible {
			return &ExecuteResponse{
				Success:       false,
				Model:         item.ModelID,
				FallbackCount: fallbackCount,
				LatencyMs:     int(time.Since(startTime).Milliseconds()),
				Error:         result.Error,
			}, nil
		}

		if i < len(combo.Items)-1 {
			fallbackCount++
		}
	}

	return &ExecuteResponse{
		Success:       false,
		FallbackCount: fallbackCount,
		LatencyMs:     int(time.Since(startTime).Milliseconds()),
		Error: &ExecuteError{
			Code:    "all_providers_failed",
			Message: "all providers in combo failed",
		},
	}, nil
}

type itemResult struct {
	Success          bool
	FallbackEligible bool
	Response         map[string]any
	Error            *ExecuteError
}

func (m *ComboManager) executeItem(ctx context.Context, item *db.ComboItem, req ExecuteRequest) (*itemResult, error) {
	timeout := time.Duration(item.TimeoutSeconds) * time.Second
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	provider, err := m.db.Providers().FindByID(ctx, item.ProviderID)
	if err != nil || provider == nil {
		return &itemResult{
			Success:          false,
			FallbackEligible: true,
			Error: &ExecuteError{
				Code:    "provider_not_found",
				Message: fmt.Sprintf("provider %s not found", item.ProviderID),
			},
		}, nil
	}

	if provider.Status == "cooldown" {
		if provider.CooldownUntil != nil && time.Now().Before(*provider.CooldownUntil) {
			return &itemResult{
				Success:          false,
				FallbackEligible: true,
				Error: &ExecuteError{
					Code:    "provider_cooldown",
					Message: fmt.Sprintf("provider %s is in cooldown", provider.Name),
				},
			}, nil
		}
	}

	return &itemResult{
		Success:  true,
		Response: map[string]any{"status": "ok", "provider": provider.Provider, "model": item.ModelID},
	}, nil
}