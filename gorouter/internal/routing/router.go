package routing

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"time"

	"github.com/gorouter/gorouter/internal/db"
)

type Strategy string

const (
	StrategyPriority Strategy = "priority"
	StrategyWeighted Strategy = "weighted"
	StrategyLatency  Strategy = "latency"
	StrategyCost     Strategy = "cost"
	StrategyFallback Strategy = "fallback"
)

type Router struct {
	db     db.DatabaseManager
	strat  Strategy
}

func NewRouter(db db.DatabaseManager, strat Strategy) *Router {
	return &Router{db: db, strat: strat}
}

func (r *Router) SelectProvider(ctx context.Context, modelID string) (*db.ProviderConnection, error) {
	providers, err := r.db.Providers().List(ctx)
	if err != nil || len(providers) == 0 {
		return nil, err
	}

	var active []*db.ProviderConnection
	now := time.Now()
	for _, p := range providers {
		if p.Status == "active" && (p.CooldownUntil == nil || now.After(*p.CooldownUntil)) {
			active = append(active, p)
		}
	}
	if len(active) == 0 {
		return nil, fmt.Errorf("no active providers available (all cooldown or inactive)")
	}

	return r.selectByStrategy(active, modelID), nil
}

func (r *Router) SelectProviderForModel(ctx context.Context, modelID string) (*db.ProviderConnection, error) {
	allModels, err := r.db.Models().ListEnabled(ctx)
	if err != nil {
		return nil, err
	}

	var targetProviderIDs []string
	for _, m := range allModels {
		if m.ModelID == modelID || m.ModelName == modelID || m.ID == modelID {
			targetProviderIDs = append(targetProviderIDs, m.ProviderID)
		}
	}

	if len(targetProviderIDs) == 0 {
		return nil, fmt.Errorf("no providers found for model: %s", modelID)
	}

	allProviders, err := r.db.Providers().List(ctx)
	if err != nil || len(allProviders) == 0 {
		return nil, err
	}

	var candidates []*db.ProviderConnection
	for _, p := range allProviders {
		for _, id := range targetProviderIDs {
			if p.ID == id {
				candidates = append(candidates, p)
				break
			}
		}
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no active providers for model: %s", modelID)
	}

	var active []*db.ProviderConnection
	now := time.Now()
	for _, p := range candidates {
		if p.Status == "active" && (p.CooldownUntil == nil || now.After(*p.CooldownUntil)) {
			active = append(active, p)
		}
	}
	if len(active) == 0 {
		return nil, fmt.Errorf("no active providers for model: %s (all cooldown or inactive)", modelID)
	}

	return r.selectByStrategy(active, modelID), nil
}

func (r *Router) selectByStrategy(providers []*db.ProviderConnection, modelID string) *db.ProviderConnection {
	if len(providers) == 0 {
		return nil
	}

	switch r.strat {
	case StrategyPriority:
		return r.byPriority(providers)
	case StrategyWeighted:
		return r.byWeighted(providers)
	case StrategyLatency:
		return r.byLatency(providers)
	case StrategyCost:
		return r.byCost(providers, modelID)
	case StrategyFallback:
		return r.byFallback(providers)
	default:
		return r.byPriority(providers)
	}
}

func (r *Router) byPriority(providers []*db.ProviderConnection) *db.ProviderConnection {
	sort.Slice(providers, func(i, j int) bool {
		return providers[i].Priority > providers[j].Priority
	})
	return providers[0]
}

func (r *Router) byWeighted(providers []*db.ProviderConnection) *db.ProviderConnection {
	total := 0
	for _, p := range providers {
		total += p.Weight
	}
	if total == 0 {
		return providers[0]
	}
	rng := rand.Intn(total)
	for _, p := range providers {
		rng -= p.Weight
		if rng < 0 {
			return p
		}
	}
	return providers[0]
}

func (r *Router) byLatency(providers []*db.ProviderConnection) *db.ProviderConnection {
	sort.Slice(providers, func(i, j int) bool {
		return providers[i].LastLatencyMs < providers[j].LastLatencyMs
	})
	for _, p := range providers {
		if p.LastLatencyMs > 0 {
			return p
		}
	}
	return providers[0]
}

func (r *Router) byCost(providers []*db.ProviderConnection, modelID string) *db.ProviderConnection {
	models, _ := r.db.Models().ListEnabled(context.Background())
	if models == nil {
		return providers[0]
	}
	sort.Slice(providers, func(i, j int) bool {
		costI := r.getProviderCost(providers[i], models, modelID)
		costJ := r.getProviderCost(providers[j], models, modelID)
		return costI < costJ
	})
	return providers[0]
}

func (r *Router) getProviderCost(p *db.ProviderConnection, models []*db.Model, modelID string) float64 {
	for _, m := range models {
		if m.ProviderID == p.ID && (m.ModelID == modelID || m.ModelName == modelID) {
			return m.InputCostPer1k + m.OutputCostPer1k
		}
	}
	return 0.1
}

func (r *Router) byFallback(providers []*db.ProviderConnection) *db.ProviderConnection {
	sort.Slice(providers, func(i, j int) bool {
		return providers[i].Priority > providers[j].Priority
	})
	return providers[0]
}

func (r *Router) SelectProvidersForModel(ctx context.Context, modelID string) ([]*db.ProviderConnection, error) {
	allModels, err := r.db.Models().ListEnabled(ctx)
	if err != nil {
		return nil, err
	}

	var targetProviderIDs []string
	for _, m := range allModels {
		if m.ModelID == modelID || m.ModelName == modelID || m.ID == modelID {
			targetProviderIDs = append(targetProviderIDs, m.ProviderID)
		}
	}

	if len(targetProviderIDs) == 0 {
		return nil, fmt.Errorf("no providers found for model: %s", modelID)
	}

	allProviders, err := r.db.Providers().List(ctx)
	if err != nil || len(allProviders) == 0 {
		return nil, err
	}

	var candidates []*db.ProviderConnection
	for _, p := range allProviders {
		for _, id := range targetProviderIDs {
			if p.ID == id {
				candidates = append(candidates, p)
				break
			}
		}
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no providers for model: %s", modelID)
	}

	var active []*db.ProviderConnection
	now := time.Now()
	for _, p := range candidates {
		if p.Status == "active" && (p.CooldownUntil == nil || now.After(*p.CooldownUntil)) {
			active = append(active, p)
		}
	}
	if len(active) == 0 {
		return nil, fmt.Errorf("no active providers for model: %s", modelID)
	}

	switch r.strat {
	case StrategyPriority, StrategyFallback:
		sort.Slice(active, func(i, j int) bool {
			return active[i].Priority > active[j].Priority
		})
	case StrategyWeighted:
		// Weighted doesn't produce ordered list for fallback; use priority as fallback order
		sort.Slice(active, func(i, j int) bool {
			return active[i].Priority > active[j].Priority
		})
	case StrategyLatency:
		sort.Slice(active, func(i, j int) bool {
			return active[i].LastLatencyMs < active[j].LastLatencyMs
		})
	case StrategyCost:
		models, _ := r.db.Models().ListEnabled(context.Background())
		sort.Slice(active, func(i, j int) bool {
			costI := r.getProviderCost(active[i], models, modelID)
			costJ := r.getProviderCost(active[j], models, modelID)
			return costI < costJ
		})
	default:
		sort.Slice(active, func(i, j int) bool {
			return active[i].Priority > active[j].Priority
		})
	}

	return active, nil
}