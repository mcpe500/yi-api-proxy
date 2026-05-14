package v1

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/gorouter/gorouter/internal/crypto"
	"github.com/gorouter/gorouter/internal/db"
)

type AudioHandler struct {
	DB     db.DatabaseManager
	Logger *slog.Logger
}

func NewAudioHandler(db db.DatabaseManager, log *slog.Logger) *AudioHandler {
	return &AudioHandler{DB: db, Logger: log}
}

func (h *AudioHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, "method not allowed", "invalid_request_error", "method_not_allowed", http.StatusMethodNotAllowed)
		return
	}

	path := r.URL.Path
	if strings.HasSuffix(path, "/speech") {
		h.handleSpeech(w, r)
	} else if strings.HasSuffix(path, "/transcriptions") {
		h.handleTranscriptions(w, r)
	} else {
		http.NotFound(w, r)
	}
}

type SpeechRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
	Voice string `json:"voice"`
}

func (h *AudioHandler) handleSpeech(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, "failed to read body", "invalid_request_error", "read_error", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var req SpeechRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, "invalid JSON", "invalid_request_error", "parse_error", http.StatusBadRequest)
		return
	}

	providerConn, targetModel, err := h.resolveModel(r.Context(), req.Model)
	if err != nil {
		writeError(w, err.Error(), "invalid_request_error", "model_error", http.StatusBadRequest)
		return
	}

	decryptedKey, err := crypto.Decrypt(string(providerConn.EncryptedSecret))
	if err != nil {
		writeError(w, "auth error", "upstream_error", "auth_error", http.StatusServiceUnavailable)
		return
	}

	proxyURL := providerConn.BaseURL + "/v1/audio/speech"
	proxyReq, _ := http.NewRequestWithContext(r.Context(), http.MethodPost, proxyURL, bytes.NewReader(body))
	proxyReq.Header.Set("Content-Type", "application/json")
	proxyReq.Header.Set("Authorization", "Bearer "+decryptedKey)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(proxyReq)
	if err != nil {
		writeError(w, "upstream failed", "upstream_error", "request_failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for k, v := range resp.Header {
		w.Header()[k] = v
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
	_ = targetModel
}

func (h *AudioHandler) handleTranscriptions(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(32 << 20)
	if err != nil {
		writeError(w, "failed to parse multipart form", "invalid_request_error", "parse_error", http.StatusBadRequest)
		return
	}

	model := r.FormValue("model")
	if model == "" {
		writeError(w, "model is required", "invalid_request_error", "invalid_request", http.StatusBadRequest)
		return
	}

	providerConn, _, err := h.resolveModel(r.Context(), model)
	if err != nil {
		writeError(w, err.Error(), "invalid_request_error", "model_error", http.StatusBadRequest)
		return
	}

	decryptedKey, err := crypto.Decrypt(string(providerConn.EncryptedSecret))
	if err != nil {
		writeError(w, "auth error", "upstream_error", "auth_error", http.StatusServiceUnavailable)
		return
	}

	// Forward multipart as-is
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	for name, values := range r.MultipartForm.Value {
		for _, value := range values {
			writer.WriteField(name, value)
		}
	}

	for name, files := range r.MultipartForm.File {
		for _, fileHeader := range files {
			file, _ := fileHeader.Open()
			part, _ := writer.CreateFormFile(name, fileHeader.Filename)
			io.Copy(part, file)
			file.Close()
		}
	}
	writer.Close()

	proxyURL := providerConn.BaseURL + "/v1/audio/transcriptions"
	proxyReq, _ := http.NewRequestWithContext(r.Context(), http.MethodPost, proxyURL, body)
	proxyReq.Header.Set("Content-Type", writer.FormDataContentType())
	proxyReq.Header.Set("Authorization", "Bearer "+decryptedKey)

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(proxyReq)
	if err != nil {
		writeError(w, "upstream failed", "upstream_error", "request_failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for k, v := range resp.Header {
		w.Header()[k] = v
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func (h *AudioHandler) resolveModel(ctx context.Context, modelID string) (*db.ProviderConnection, *db.Model, error) {
	models, err := h.DB.Models().ListEnabled(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load models")
	}

	var targetModel *db.Model
	for _, m := range models {
		if m.ModelID == modelID || m.ModelName == modelID || m.ID == modelID {
			targetModel = m
			break
		}
	}

	if targetModel == nil {
		return nil, nil, fmt.Errorf("model not found: %s", modelID)
	}

	providerConn, err := h.DB.Providers().FindByID(ctx, targetModel.ProviderID)
	if err != nil || providerConn == nil {
		return nil, nil, fmt.Errorf("provider not found")
	}

	return providerConn, targetModel, nil
}
