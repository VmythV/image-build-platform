package dockerfile

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type Handler struct {
	service Service
}

func NewHandler(service Service) Handler {
	return Handler{service: service}
}

func (h Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Post("/generate", h.generate)
	r.Post("/validate", h.validate)
	return r
}

func (h Handler) generate(w http.ResponseWriter, r *http.Request) {
	var req GenerateRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	generated, err := h.service.Generate(req)
	if err != nil {
		if errors.Is(err, ErrValidation) {
			writeError(w, http.StatusBadRequest, "VALIDATION_FAILED", err.Error(), nil)
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to generate Dockerfile.", nil)
		return
	}
	writeData(w, http.StatusOK, map[string]string{"dockerfile": generated})
}

func (h Handler) validate(w http.ResponseWriter, r *http.Request) {
	var req ValidateRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	writeData(w, http.StatusOK, h.service.Validate(req.Dockerfile))
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid JSON request body.", nil)
		return false
	}
	return true
}

func writeData(w http.ResponseWriter, status int, data any) {
	writeJSON(w, status, map[string]any{"data": data})
}

func writeError(w http.ResponseWriter, status int, code string, message string, details any) {
	writeJSON(w, status, map[string]any{
		"error": map[string]any{
			"code":    code,
			"message": message,
			"details": details,
		},
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		slog.Default().Warn("write json response", "error", err)
	}
}
