package routing

import (
	"context"
	"math/rand"
	"sort"

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
	for _, p := range providers {
		if p.Status == "active" {
			active = append(active, p)
		}
	}
	if len(active) == 0 {
		active = providers
	}

	switch r.strat {
	case StrategyPriority:
		return r.byPriority(active), nil
	case StrategyWeighted:
		return r.byWeighted(active), nil
	case StrategyLatency:
		return r.byLatency(active), nil
	case StrategyCost:
		return r.byCost(ctx, active, modelID), nil
	case StrategyFallback:
		return r.byFallback(active), nil
	default:
		return r.byPriority(active), nil
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

func (r *Router) byCost(ctx context.Context, providers []*db.ProviderConnection, modelID string) *db.ProviderConnection {
	models, err := r.db.Models().ListEnabled(ctx)
	if err != nil {
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
	if len(providers) > 0 {
		return providers[0]
	}
	return nil
}