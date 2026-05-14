package combo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gorouter/gorouter/internal/crypto"
	"github.com/gorouter/gorouter/internal/db"
	"github.com/gorouter/gorouter/internal/translator"
)

type ComboManager struct {
	db         db.DatabaseManager
	httpClient *http.Client
}

func NewComboManager(database db.DatabaseManager) *ComboManager {
	return &ComboManager{
		db: database,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
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
	Model    string              `json:"model"`
	Messages []map[string]string `json:"messages"`
	Stream   bool                `json:"stream,omitempty"`
}

type ExecuteResponse struct {
	Success       bool           `json:"success"`
	Model         string         `json:"model,omitempty"`
	Response      map[string]any `json:"response,omitempty"`
	Error         *ExecuteError  `json:"error,omitempty"`
	FallbackCount int            `json:"fallback_count"`
	LatencyMs     int            `json:"latency_ms"`
}

type ExecuteError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
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

	adapter, err := translator.Get(provider.Provider)
	if err != nil {
		return &itemResult{
			Success:          false,
			FallbackEligible: true,
			Error: &ExecuteError{
				Code:    "adapter_not_found",
				Message: fmt.Sprintf("no adapter for provider %s", provider.Provider),
			},
		}, nil
	}

	apiKey, err := crypto.Decrypt(string(provider.EncryptedSecret))
	if err != nil {
		return &itemResult{
			Success:          false,
			FallbackEligible: true,
			Error: &ExecuteError{
				Code:    "decrypt_error",
				Message: "failed to decrypt provider secret",
			},
		}, nil
	}

	normReq := &translator.NormalizedChatRequest{
		Model:    item.ModelID,
		Messages: convertMessages(req.Messages),
		Stream:   false,
	}

	httpReq, err := adapter.TranslateRequest(normReq, provider.BaseURL, apiKey)
	if err != nil {
		return &itemResult{
			Success:          false,
			FallbackEligible: true,
			Error: &ExecuteError{
				Code:    "translate_error",
				Message: fmt.Sprintf("failed to translate request: %v", err),
			},
		}, nil
	}

	resp, err := m.httpClient.Do(httpReq.WithContext(ctx))
	if err != nil {
		m.markCooldown(provider, err.Error())
		return &itemResult{
			Success:          false,
			FallbackEligible: true,
			Error: &ExecuteError{
				Code:    "request_failed",
				Message: fmt.Sprintf("HTTP request failed: %v", err),
			},
		}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		eligible := resp.StatusCode == 429 || resp.StatusCode >= 500
		if eligible {
			m.markCooldown(provider, fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(bodyBytes)))
		}
		return &itemResult{
			Success:          false,
			FallbackEligible: eligible,
			Error: &ExecuteError{
				Code:    fmt.Sprintf("http_%d", resp.StatusCode),
				Message: fmt.Sprintf("upstream returned %d: %s", resp.StatusCode, string(bodyBytes)),
			},
		}, nil
	}

	chatResp, err := adapter.ParseResponse(resp)
	if err != nil {
		return &itemResult{
			Success:          false,
			FallbackEligible: true,
			Error: &ExecuteError{
				Code:    "parse_error",
				Message: fmt.Sprintf("failed to parse response: %v", err),
			},
		}, nil
	}

	m.db.Providers().ClearCooldown(ctx, provider.ID)

	respData := serializeResponse(chatResp)
	return &itemResult{
		Success: true,
		Response: respData,
	}, nil
}

func (m *ComboManager) markCooldown(provider *db.ProviderConnection, errMsg string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	backoff := 30 * time.Second
	if provider.BackoffLevel > 0 {
		backoff = time.Duration(provider.BackoffLevel) * 30 * time.Second
		if backoff > 10*time.Minute {
			backoff = 10 * time.Minute
		}
	}
	until := time.Now().Add(backoff).Unix()

	m.db.Providers().MarkCooldown(ctx, provider.ID, until, errMsg)
}

func convertMessages(msgs []map[string]string) []translator.NormalizedMessage {
	result := make([]translator.NormalizedMessage, len(msgs))
	for i, msg := range msgs {
		result[i] = translator.NormalizedMessage{
			Role:    msg["role"],
			Content: msg["content"],
		}
	}
	return result
}

func serializeResponse(resp *translator.NormalizedChatResponse) map[string]any {
	data, err := json.Marshal(resp)
	if err != nil {
		return map[string]any{"error": "serialization failed"}
	}
	var result map[string]any
	json.Unmarshal(data, &result)
	return result
}