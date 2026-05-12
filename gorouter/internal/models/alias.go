package models

import (
	"context"
	"strings"

	"github.com/gorouter/gorouter/internal/db"
)

type ModelResolver struct {
	DB db.DatabaseManager
}

func (r *ModelResolver) Resolve(ctx context.Context, modelName string, userID string) (string, string, error) {
	if modelName == "" {
		return "", "", nil
	}

	if alias, err := r.DB.Aliases().FindByName(ctx, modelName); err == nil && alias != nil {
		if alias.UserID == "" || alias.UserID == userID {
			return alias.TargetID, alias.Provider, nil
		}
	}

	if strings.Contains(modelName, "/") {
		parts := strings.SplitN(modelName, "/", 2)
		return modelName, parts[0], nil
	}

	provider := inferProvider(modelName)
	return modelName, provider, nil
}

func (r *ModelResolver) ListAliases(ctx context.Context, userID string) ([]*db.ModelAlias, error) {
	var result []*db.ModelAlias

	global, err := r.DB.Aliases().ListGlobal(ctx)
	if err != nil {
		return nil, err
	}
	result = append(result, global...)

	userAliases, err := r.DB.Aliases().FindByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	result = append(result, userAliases...)

	return result, nil
}

func inferProvider(modelName string) string {
	modelName = strings.ToLower(modelName)

	switch {
	case strings.HasPrefix(modelName, "gpt-"):
		return "openai"
	case strings.HasPrefix(modelName, "glm-"):
		return "glm"
	case strings.HasPrefix(modelName, "claude-"):
		return "anthropic"
	case strings.HasPrefix(modelName, "gemini-"):
		return "google"
	case strings.HasPrefix(modelName, "mistral-"):
		return "mistral"
	case strings.HasPrefix(modelName, "llama-"):
		return "ollama"
	case strings.HasPrefix(modelName, "qwen-"):
		return "qwen"
	default:
		return ""
	}
}
