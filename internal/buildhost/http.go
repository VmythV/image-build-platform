package buildhost

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/VmythV/image-build-platform/internal/auth"
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
	r.Get("/", h.list)
	r.Post("/", h.create)
	r.Get("/{id}", h.get)
	r.Put("/{id}", h.update)
	r.Delete("/{id}", h.delete)
	r.Post("/{id}/check", h.check)
	r.Post("/{id}/enable", h.enable)
	r.Post("/{id}/disable", h.disable)
	return r
}

func (h Handler) list(w http.ResponseWriter, r *http.Request) {
	filter := ListFilter{
		Status:         r.URL.Query().Get("status"),
		Architecture:   r.URL.Query().Get("architecture"),
		ConnectionType: r.URL.Query().Get("connectionType"),
		Page:           parseInt(r.URL.Query().Get("page")),
		PageSize:       parseInt(r.URL.Query().Get("pageSize")),
	}

	hosts, total, err := h.service.List(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list build hosts.", nil)
		return
	}

	data := make([]BuildHostDTO, 0, len(hosts))
	for _, host := range hosts {
		data = append(data, ToDTO(host))
	}

	page := normalizePage(filter.Page)
	pageSize := normalizePageSize(filter.PageSize)
	writeJSON(w, http.StatusOK, map[string]any{
		"data": data,
		"pagination": map[string]int{
			"page":     page,
			"pageSize": pageSize,
			"total":    total,
		},
	})
}

func (h Handler) create(w http.ResponseWriter, r *http.Request) {
	user, ok := requireAdmin(w, r)
	if !ok {
		return
	}

	var req SaveInput
	if !decodeJSON(w, r, &req) {
		return
	}

	host, err := h.service.Create(r.Context(), req, user.ID)
	if err != nil {
		handleHostError(w, err)
		return
	}

	writeData(w, http.StatusCreated, ToDTO(host))
}

func (h Handler) get(w http.ResponseWriter, r *http.Request) {
	host, err := h.service.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		handleHostError(w, err)
		return
	}
	writeData(w, http.StatusOK, ToDTO(host))
}

func (h Handler) update(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdmin(w, r); !ok {
		return
	}

	var req SaveInput
	if !decodeJSON(w, r, &req) {
		return
	}

	host, err := h.service.Update(r.Context(), chi.URLParam(r, "id"), req)
	if err != nil {
		handleHostError(w, err)
		return
	}
	writeData(w, http.StatusOK, ToDTO(host))
}

func (h Handler) delete(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdmin(w, r); !ok {
		return
	}

	if err := h.service.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		handleHostError(w, err)
		return
	}
	writeData(w, http.StatusOK, map[string]bool{"success": true})
}

func (h Handler) check(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdmin(w, r); !ok {
		return
	}

	host, result, err := h.service.Check(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		handleHostError(w, err)
		return
	}
	writeData(w, http.StatusOK, map[string]any{
		"host":   ToDTO(host),
		"result": result,
	})
}

func (h Handler) enable(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdmin(w, r); !ok {
		return
	}

	host, err := h.service.Enable(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		handleHostError(w, err)
		return
	}
	writeData(w, http.StatusOK, ToDTO(host))
}

func (h Handler) disable(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdmin(w, r); !ok {
		return
	}

	host, err := h.service.Disable(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		handleHostError(w, err)
		return
	}
	writeData(w, http.StatusOK, ToDTO(host))
}

func requireAdmin(w http.ResponseWriter, r *http.Request) (auth.User, bool) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication is required.", nil)
		return auth.User{}, false
	}
	if user.Role != auth.RoleAdmin {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "Permission denied.", nil)
		return auth.User{}, false
	}
	return user, true
}

func handleHostError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Build host not found.", nil)
	case errors.Is(err, ErrValidation), errors.Is(err, ErrInvalidCommand):
		writeError(w, http.StatusBadRequest, "VALIDATION_FAILED", err.Error(), nil)
	default:
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Build host operation failed.", nil)
	}
}

func parseInt(value string) int {
	if value == "" {
		return 0
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return parsed
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
