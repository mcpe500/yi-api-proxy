package v1

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorouter/gorouter/internal/crypto"
	"github.com/gorouter/gorouter/internal/db"
)

type SearchHandler struct {
	DB     db.DatabaseManager
	Logger *slog.Logger
}

func NewSearchHandler(db db.DatabaseManager, log *slog.Logger) *SearchHandler {
	return &SearchHandler{DB: db, Logger: log}
}

func (h *SearchHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, "method not allowed", "invalid_request_error", "method_not_allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, "failed to read body", "invalid_request_error", "read_error", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// For search, we need a model or provider.
	// We'll look for a 'model' field in JSON, or default to a search-capable provider.
	// Simplified: just try to find a provider that might support search.
	
	ctx := r.Context()
	providers, err := h.DB.Providers().List(ctx)
	if err != nil || len(providers) == 0 {
		writeError(w, "no providers available", "upstream_error", "no_providers", http.StatusServiceUnavailable)
		return
	}

	// Placeholder logic: use the first provider that isn't on cooldown
	var providerConn *db.ProviderConnection
	for _, p := range providers {
		if p.CooldownUntil == nil || p.CooldownUntil.Before(time.Now()) {
			providerConn = p
			break
		}
	}

	if providerConn == nil {
		writeError(w, "all providers on cooldown", "upstream_error", "all_cooldown", http.StatusServiceUnavailable)
		return
	}

	decryptedKey, err := crypto.Decrypt(string(providerConn.EncryptedSecret))
	if err != nil {
		writeError(w, "auth error", "upstream_error", "auth_error", http.StatusServiceUnavailable)
		return
	}

	// Try common search paths
	searchPath := "/v1/search"
	if providerConn.Provider == "minimax" {
		searchPath = "/v1/web/search"
	}

	proxyURL := providerConn.BaseURL + searchPath
	proxyReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, proxyURL, bytes.NewReader(body))
	proxyReq.Header.Set("Content-Type", "application/json")
	proxyReq.Header.Set("Authorization", "Bearer "+decryptedKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(proxyReq)
	if err != nil {
		writeError(w, "upstream request failed", "upstream_error", "request_failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusNotImplemented {
		writeError(w, "web search not supported by provider", "invalid_request_error", "not_supported", http.StatusNotImplemented)
		return
	}

	for k, v := range resp.Header {
		w.Header()[k] = v
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
