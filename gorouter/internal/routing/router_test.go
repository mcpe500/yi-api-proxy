package routing

import (
	"context"
	"testing"
	"time"

	"github.com/gorouter/gorouter/internal/db"
)

type mockDB struct {
	db.DatabaseManager
	providers *mockProviderRepo
	models    *mockModelRepo
}

func (m *mockDB) Providers() db.ProviderRepository { return m.providers }
func (m *mockDB) Models() db.ModelRepository       { return m.models }

type mockProviderRepo struct {
	db.ProviderRepository
	list []*db.ProviderConnection
}

func (m *mockProviderRepo) List(ctx context.Context) ([]*db.ProviderConnection, error) {
	return m.list, nil
}

type mockModelRepo struct {
	db.ModelRepository
	list []*db.Model
}

func (m *mockModelRepo) ListEnabled(ctx context.Context) ([]*db.Model, error) {
	return m.list, nil
}

func TestRouter_Strategies(t *testing.T) {
	p1 := &db.ProviderConnection{ID: "p1", Priority: 10, Weight: 10, LastLatencyMs: 100, Status: "active"}
	p2 := &db.ProviderConnection{ID: "p2", Priority: 20, Weight: 90, LastLatencyMs: 50, Status: "active"}
	p3 := &db.ProviderConnection{ID: "p3", Priority: 5, Weight: 0, LastLatencyMs: 200, Status: "active"}

	providers := []*db.ProviderConnection{p1, p2, p3}

	t.Run("byPriority", func(t *testing.T) {
		r := &Router{strat: StrategyPriority}
		selected := r.byPriority(providers)
		if selected.ID != "p2" {
			t.Errorf("expected p2 (priority 20), got %s", selected.ID)
		}
	})

	t.Run("byWeighted", func(t *testing.T) {
		r := &Router{strat: StrategyWeighted}
		hits := make(map[string]int)
		for i := 0; i < 1000; i++ {
			s := r.byWeighted(providers)
			hits[s.ID]++
		}
		if hits["p2"] < hits["p1"] {
			t.Errorf("p2 should have more hits than p1 (90 vs 10 weight)")
		}
	})

	t.Run("byLatency", func(t *testing.T) {
		r := &Router{strat: StrategyLatency}
		selected := r.byLatency(providers)
		if selected.ID != "p2" {
			t.Errorf("expected p2 (latency 50), got %s", selected.ID)
		}
	})
}

func TestRouter_SelectProviderForModel(t *testing.T) {
	p1 := &db.ProviderConnection{ID: "p1", Status: "active"}
	p2 := &db.ProviderConnection{ID: "p2", Status: "active"}
	cooldown := time.Now().Add(time.Hour)
	p3 := &db.ProviderConnection{ID: "p3", Status: "active", CooldownUntil: &cooldown}
	
	mock := &mockDB{
		providers: &mockProviderRepo{list: []*db.ProviderConnection{p1, p2, p3}},
		models: &mockModelRepo{list: []*db.Model{
			{ProviderID: "p1", ModelID: "m1"},
			{ProviderID: "p2", ModelID: "m1"},
			{ProviderID: "p3", ModelID: "m1"},
		}},
	}

	r := NewRouter(mock, StrategyPriority)
	selected, err := r.SelectProviderForModel(context.Background(), "m1")
	if err != nil {
		t.Fatal(err)
	}
	if selected.ID == "p3" {
		t.Errorf("should not select provider in cooldown")
	}
}
