package provider

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorouter/gorouter/internal/db"
)

type HealthMonitor struct {
	db         db.DatabaseManager
	httpClient *http.Client
	interval   time.Duration
	stopCh     chan struct{}
	wg         sync.WaitGroup
}

func NewHealthMonitor(db db.DatabaseManager, interval time.Duration) *HealthMonitor {
	return &HealthMonitor{
		db:         db,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		interval:   interval,
		stopCh:     make(chan struct{}),
	}
}

func (h *HealthMonitor) Start() {
	h.wg.Add(1)
	go h.run()
}

func (h *HealthMonitor) Stop() {
	close(h.stopCh)
	h.wg.Wait()
}

func (h *HealthMonitor) run() {
	defer h.wg.Done()
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	for {
		select {
		case <-h.stopCh:
			return
		case <-ticker.C:
			h.checkAllProviders()
		}
	}
}

func (h *HealthMonitor) checkAllProviders() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	providers, err := h.db.Providers().List(ctx)
	if err != nil {
		return
	}

	for _, p := range providers {
		if p.Status != "active" {
			continue
		}
		go h.checkProvider(p)
	}
}

func (h *HealthMonitor) checkProvider(p *db.ProviderConnection) {
	baseURL := p.BaseURL
	if baseURL == "" {
		baseURL = getDefaultURL(p.Provider)
	}

	if baseURL == "" {
		return
	}

	start := time.Now()
	req, _ := http.NewRequest(http.MethodGet, baseURL+"/v1/models", nil)
	req.Header.Set("Authorization", "Bearer test")

	resp, err := h.httpClient.Do(req)
	latency := int(time.Since(start).Milliseconds())

	if err != nil || resp == nil {
		h.recordFailure(p.ID, fmt.Sprintf("connection failed: %v", err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 500 {
		h.recordLatency(p.ID, latency)
	} else {
		h.recordFailure(p.ID, fmt.Sprintf("HTTP %d", resp.StatusCode))
	}
}

func (h *HealthMonitor) recordLatency(providerID string, latencyMs int) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	p, err := h.db.Providers().FindByID(ctx, providerID)
	if err != nil || p == nil {
		return
	}

	p.LastLatencyMs = latencyMs
	h.db.Providers().Update(ctx, p)
}

func (h *HealthMonitor) recordFailure(providerID string, errMsg string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	p, err := h.db.Providers().FindByID(ctx, providerID)
	if err != nil || p == nil {
		return
	}

	now := time.Now()
	p.LastError = errMsg
	p.LastErrorAt = &now
	p.BackoffLevel++

	if p.BackoffLevel >= 3 {
		p.Status = "cooldown"
		t := now.Add(time.Duration(p.BackoffLevel) * time.Minute)
		p.CooldownUntil = &t
	}

	h.db.Providers().Update(ctx, p)
}

func getDefaultURL(provider string) string {
	defaults := map[string]string{
		"openai":     "https://api.openai.com",
		"anthropic":  "https://api.anthropic.com",
		"google":     "https://generativelanguage.googleapis.com",
		"deepseek":   "https://api.deepseek.com",
		"mistral":    "https://api.mistral.ai",
		"groq":       "https://api.groq.com",
		"together":   "https://api.together.xyz",
		"perplexity": "https://api.perplexity.ai",
		"ollama":     "http://localhost:11434",
		"lmstudio":   "http://localhost:1234",
	}
	if url, ok := defaults[provider]; ok {
		return url
	}
	return ""
}