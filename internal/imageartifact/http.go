package imageartifact

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
	r.Get("/{id}", h.get)
	r.Get("/{id}/pull-command", h.pullCommand)
	r.Post("/{id}/repush", h.repush)
	r.Post("/{id}/archive", h.archive)
	r.Post("/{id}/deprecate", h.deprecate)
	return r
}

func (h Handler) list(w http.ResponseWriter, r *http.Request) {
	filter := ListFilter{
		ProjectID:  r.URL.Query().Get("projectId"),
		RegistryID: r.URL.Query().Get("registryId"),
		Status:     r.URL.Query().Get("status"),
		Page:       parseInt(r.URL.Query().Get("page")),
		PageSize:   parseInt(r.URL.Query().Get("pageSize")),
	}

	artifacts, total, err := h.service.List(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list image artifacts.", nil)
		return
	}

	data := make([]ArtifactDTO, 0, len(artifacts))
	for _, artifact := range artifacts {
		data = append(data, ToDTO(artifact))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": data,
		"pagination": map[string]int{
			"page":     normalizePage(filter.Page),
			"pageSize": normalizePageSize(filter.PageSize),
			"total":    total,
		},
	})
}

func (h Handler) get(w http.ResponseWriter, r *http.Request) {
	artifact, err := h.service.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": ToDTO(artifact)})
}

func (h Handler) pullCommand(w http.ResponseWriter, r *http.Request) {
	command, err := h.service.PullCommand(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": PullCommandDTO{Command: command}})
}

func (h Handler) repush(w http.ResponseWriter, r *http.Request) {
	user, ok := requireMaintainer(w, r)
	if !ok {
		return
	}

	var input RepushInput
	if r.Body != nil && r.ContentLength != 0 {
		if !decodeJSON(w, r, &input) {
			return
		}
	}

	artifact, event, logPath, err := h.service.Repush(r.Context(), chi.URLParam(r, "id"), input, user.ID)
	if err != nil {
		if event.ID != "" {
			writeError(w, http.StatusBadGateway, "REPUSH_FAILED", err.Error(), map[string]any{
				"event":   EventToDTO(event),
				"logPath": logPath,
			})
			return
		}
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": RepushResultDTO{
		Artifact: ToDTO(artifact),
		Event:    EventToDTO(event),
		LogPath:  stringPtr(logPath),
	}})
}

func (h Handler) archive(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireMaintainer(w, r); !ok {
		return
	}
	artifact, err := h.service.Archive(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": ToDTO(artifact)})
}

func (h Handler) deprecate(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireMaintainer(w, r); !ok {
		return
	}
	artifact, err := h.service.Deprecate(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": ToDTO(artifact)})
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

func requireMaintainer(w http.ResponseWriter, r *http.Request) (auth.User, bool) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication is required.", nil)
		return auth.User{}, false
	}
	if user.Role != auth.RoleAdmin && user.Role != auth.RoleMaintainer {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "Permission denied.", nil)
		return auth.User{}, false
	}
	return user, true
}

func handleError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Image artifact not found.", nil)
	case errors.Is(err, ErrValidation):
		writeError(w, http.StatusBadRequest, "VALIDATION_FAILED", err.Error(), nil)
	default:
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Image artifact operation failed.", nil)
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
