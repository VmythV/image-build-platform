package audit

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

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
	return r
}

func (h Handler) list(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication is required.", nil)
		return
	}
	if user.Role != auth.RoleAdmin {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "Permission denied.", nil)
		return
	}

	filter := ListFilter{
		ActorID:      r.URL.Query().Get("actorId"),
		Action:       r.URL.Query().Get("action"),
		ResourceType: r.URL.Query().Get("resourceType"),
		Page:         parseInt(r.URL.Query().Get("page")),
		PageSize:     parseInt(r.URL.Query().Get("pageSize")),
	}
	logs, total, err := h.repo.List(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list audit logs.", nil)
		return
	}

	data := make([]LogDTO, 0, len(logs))
	for _, log := range logs {
		data = append(data, ToDTO(log))
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
