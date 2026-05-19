package sync

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorouter/gorouter/internal/config"
	"github.com/gorouter/gorouter/internal/db"
)

type SyncPayload struct {
	Providers []*db.ProviderConnection `json:"providers"`
	Models    []*db.Model              `json:"models"`
	Combos    []*db.Combo              `json:"combos"`
	Timestamp time.Time                `json:"timestamp"`
}

type Worker struct {
	cfg    *config.AppConfig
	db     db.DatabaseManager
	client *http.Client
	logger *slog.Logger
	stop   chan struct{}
	done   chan struct{}
}

func NewWorker(cfg *config.AppConfig, dbMgr db.DatabaseManager, logger *slog.Logger) *Worker {
	return &Worker{
		cfg:    cfg,
		db:     dbMgr,
		client: &http.Client{Timeout: 30 * time.Second},
		logger: logger,
		stop:   make(chan struct{}),
		done:   make(chan struct{}),
	}
}

func (w *Worker) Start() {
	if w.cfg.OnlineSyncURL == "" {
		w.logger.Info("Online sync disabled: no ONLINE_SYNC_URL configured")
		return
	}
	w.logger.Info("Online sync worker starting", "url", w.cfg.OnlineSyncURL)

	go func() {
		defer close(w.done)
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		w.sync()

		for {
			select {
			case <-w.stop:
				w.logger.Info("Online sync worker stopped")
				return
			case <-ticker.C:
				w.sync()
			}
		}
	}()
}

func (w *Worker) Stop() {
	close(w.stop)
	<-w.done
}

func (w *Worker) sync() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	providers, err := w.db.Providers().List(ctx)
	if err != nil {
		w.logger.Error("Online sync failed to load providers", "error", err)
		return
	}

	var models []*db.Model
	for _, p := range providers {
		m, _ := w.db.Models().ListByProvider(ctx, p.ID)
		models = append(models, m...)
	}

	combos, _ := w.db.Combos().List(ctx)

	payload := SyncPayload{
		Providers: providers,
		Models:    models,
		Combos:    combos,
		Timestamp: time.Now(),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		w.logger.Error("Online sync failed to marshal payload", "error", err)
		return
	}

	req, err := http.NewRequestWithContext(ctx, "POST", w.cfg.OnlineSyncURL, bytes.NewReader(data))
	if err != nil {
		w.logger.Error("Online sync failed to create request", "error", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	if w.cfg.OnlineSyncToken != "" {
		req.Header.Set("Authorization", "Bearer "+w.cfg.OnlineSyncToken)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		w.logger.Error("Online sync failed to send", "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		w.logger.Info("Online sync successful", "providers", len(providers), "models", len(models), "combos", len(combos))
	} else {
		w.logger.Warn("Online sync returned non-success status", "status", resp.StatusCode)
	}
}