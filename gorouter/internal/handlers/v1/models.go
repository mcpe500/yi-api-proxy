package v1

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorouter/gorouter/internal/db"
)

type OpenAIModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

type ModelsResponse struct {
	Object string       `json:"object"`
	Data   []OpenAIModel `json:"data"`
}

type ModelsHandler struct {
	db db.DatabaseManager
}

func NewModelsHandler(dbManager db.DatabaseManager) *ModelsHandler {
	return &ModelsHandler{db: dbManager}
}

func (h *ModelsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	dbModels, err := h.db.Models().ListEnabled(ctx)
	if err != nil {
		writeError(w, "Failed to list models", "internal_error", "", http.StatusInternalServerError)
		return
	}

	models := make([]OpenAIModel, 0, len(dbModels))
	for _, m := range dbModels {
		models = append(models, OpenAIModel{
			ID:      m.ModelName,
			Object:  "model",
			Created: m.CreatedAt.Unix(),
			OwnedBy: m.Provider,
		})
	}

	if len(models) == 0 {
		models = []OpenAIModel{
			{ID: "gpt-4o", Object: "model", Created: time.Date(2024, 5, 10, 0, 0, 0, 0, time.UTC).Unix(), OwnedBy: "gorouter"},
		}
	}

	response := ModelsResponse{
		Object: "list",
		Data:   models,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func HandleModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, "Method not allowed", "invalid_request_error", "", http.StatusMethodNotAllowed)
		return
	}

	models := []OpenAIModel{
		{ID: "gpt-4o", Object: "model", Created: time.Date(2024, 5, 10, 0, 0, 0, 0, time.UTC).Unix(), OwnedBy: "gorouter"},
	}

	response := ModelsResponse{
		Object: "list",
		Data:   models,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
