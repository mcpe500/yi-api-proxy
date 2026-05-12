package provider

import (
	"context"

	"github.com/gorouter/gorouter/internal/db"
)

type ModelProvider interface {
	ListModels(ctx context.Context) ([]*db.Model, error)
	GetModel(ctx context.Context, id string) (*db.Model, error)
	CreateModel(ctx context.Context, model *db.Model) error
	UpdateModel(ctx context.Context, model *db.Model) error
	DeleteModel(ctx context.Context, id string) error
}

type modelProvider struct {
	db db.DatabaseManager
}

func NewModelProvider(dbManager db.DatabaseManager) ModelProvider {
	return &modelProvider{db: dbManager}
}

func (p *modelProvider) ListModels(ctx context.Context) ([]*db.Model, error) {
	return p.db.Models().List(ctx)
}

func (p *modelProvider) GetModel(ctx context.Context, id string) (*db.Model, error) {
	return p.db.Models().FindByID(ctx, id)
}

func (p *modelProvider) CreateModel(ctx context.Context, model *db.Model) error {
	return p.db.Models().Create(ctx, model)
}

func (p *modelProvider) UpdateModel(ctx context.Context, model *db.Model) error {
	return p.db.Models().Update(ctx, model)
}

func (p *modelProvider) DeleteModel(ctx context.Context, id string) error {
	return p.db.Models().Delete(ctx, id)
}