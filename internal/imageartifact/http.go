package imageartifact

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

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
	r.Get("/{id}", h.get)
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

	artifacts, total, err := h.repo.List(r.Context(), filter)
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
	artifact, err := h.repo.FindByID(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Image artifact not found.", nil)
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load image artifact.", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": ToDTO(artifact)})
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
