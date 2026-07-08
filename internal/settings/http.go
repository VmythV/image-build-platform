package settings

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/VmythV/image-build-platform/internal/auth"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	repo Repository
}

func NewHandler(repo Repository) Handler {
	return Handler{repo: repo}
}

func (h Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/", h.list)
	r.Put("/{key}", h.update)
	return r
}

func (h Handler) list(w http.ResponseWriter, r *http.Request) {
	settings, err := h.repo.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list settings.", nil)
		return
	}
	data := make([]SettingDTO, 0, len(settings))
	for _, setting := range settings {
		data = append(data, ToDTO(setting))
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (h Handler) update(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication is required.", nil)
		return
	}
	if user.Role != auth.RoleAdmin {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "Permission denied.", nil)
		return
	}

	var input SaveInput
	if !decodeJSON(w, r, &input) {
		return
	}
	setting, err := h.repo.Update(r.Context(), chi.URLParam(r, "key"), input.Value, user.ID, time.Now())
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": ToDTO(setting)})
}

func handleError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Setting not found.", nil)
	case errors.Is(err, ErrValidation):
		writeError(w, http.StatusBadRequest, "VALIDATION_FAILED", err.Error(), nil)
	default:
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Setting operation failed.", nil)
	}
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
