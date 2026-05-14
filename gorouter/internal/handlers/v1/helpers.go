package v1

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
)

type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code,omitempty"`
}

func writeError(w http.ResponseWriter, message, errType, code string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{
		Error: ErrorDetail{
			Message: message,
			Type:    errType,
			Code:    code,
		},
	})
}

func writeSSEError(w http.ResponseWriter, message string) {
	data, _ := json.Marshal(map[string]string{"error": message})
	w.Write([]byte("data: "))
	w.Write(data)
	w.Write([]byte("\n\n"))
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

type captureWriter struct {
	code    int
	headers http.Header
	buf     bytes.Buffer
}

func newCaptureWriter() *captureWriter {
	return &captureWriter{code: 200, headers: make(http.Header)}
}

func (cw *captureWriter) Header() http.Header        { return cw.headers }
func (cw *captureWriter) Write(b []byte) (int, error) { return cw.buf.Write(b) }
func (cw *captureWriter) WriteHeader(code int)        { cw.code = code }

func errorTypeForStatus(status int) string {
	switch status {
	case 400:
		return "invalid_request_error"
	case 401:
		return "authentication_error"
	case 403:
		return "permission_error"
	case 429:
		return "rate_limit_error"
	case 500:
		return "internal_error"
	case 502:
		return "upstream_error"
	case 503:
		return "upstream_error"
	default:
		return "server_error"
	}
}

func generateID(prefix string) string {
	b := make([]byte, 16)
	rand.Read(b)
	return prefix + "-" + hex.EncodeToString(b)
}
