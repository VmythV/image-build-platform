package buildtask

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
	r.Post("/dispatch-next", h.dispatchNext)
	r.Get("/{id}", h.get)
	r.Post("/{id}/dispatch", h.dispatch)
	r.Post("/{id}/start", h.start)
	r.Post("/{id}/cancel", h.cancel)
	r.Post("/{id}/retry", h.retry)
	r.Get("/{id}/logs", h.logs)
	return r
}

func (h Handler) list(w http.ResponseWriter, r *http.Request) {
	filter := ListFilter{
		Status:        r.URL.Query().Get("status"),
		ProjectID:     r.URL.Query().Get("projectId"),
		VersionNodeID: r.URL.Query().Get("versionNodeId"),
		HostID:        r.URL.Query().Get("hostId"),
		RegistryID:    r.URL.Query().Get("registryId"),
		Page:          parseInt(r.URL.Query().Get("page")),
		PageSize:      parseInt(r.URL.Query().Get("pageSize")),
	}

	tasks, total, err := h.service.List(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list build tasks.", nil)
		return
	}

	data := make([]BuildTaskDTO, 0, len(tasks))
	for _, task := range tasks {
		data = append(data, ToDTO(task))
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
	user, ok := requireMaintainer(w, r)
	if !ok {
		return
	}

	var req CreateInput
	if !decodeJSON(w, r, &req) {
		return
	}

	task, err := h.service.Create(r.Context(), req, user.ID)
	if err != nil {
		handleTaskError(w, err)
		return
	}
	writeData(w, http.StatusCreated, ToDTO(task))
}

func (h Handler) get(w http.ResponseWriter, r *http.Request) {
	task, err := h.service.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		handleTaskError(w, err)
		return
	}
	writeData(w, http.StatusOK, ToDTO(task))
}

func (h Handler) dispatch(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireMaintainer(w, r); !ok {
		return
	}

	task, dispatched, reason, err := h.service.Dispatch(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		handleTaskError(w, err)
		return
	}
	writeData(w, http.StatusOK, DispatchResult{Task: ToDTO(task), Dispatched: dispatched, Reason: reason})
}

func (h Handler) dispatchNext(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireMaintainer(w, r); !ok {
		return
	}

	task, dispatched, reason, err := h.service.DispatchNext(r.Context())
	if err != nil {
		handleTaskError(w, err)
		return
	}
	writeData(w, http.StatusOK, DispatchResult{Task: ToDTO(task), Dispatched: dispatched, Reason: reason})
}

func (h Handler) start(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireMaintainer(w, r); !ok {
		return
	}

	task, err := h.service.Start(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		handleTaskError(w, err)
		return
	}
	writeData(w, http.StatusOK, ToDTO(task))
}

func (h Handler) cancel(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireMaintainer(w, r); !ok {
		return
	}

	task, err := h.service.Cancel(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		handleTaskError(w, err)
		return
	}
	writeData(w, http.StatusOK, ToDTO(task))
}

func (h Handler) retry(w http.ResponseWriter, r *http.Request) {
	user, ok := requireMaintainer(w, r)
	if !ok {
		return
	}

	task, err := h.service.Retry(r.Context(), chi.URLParam(r, "id"), user.ID)
	if err != nil {
		handleTaskError(w, err)
		return
	}
	writeData(w, http.StatusCreated, ToDTO(task))
}

func (h Handler) logs(w http.ResponseWriter, r *http.Request) {
	logs, filename, err := h.service.ReadLogs(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		handleTaskError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", `inline; filename="`+filename+`"`)
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(logs)); err != nil {
		slog.Default().Warn("write build task logs", "error", err)
	}
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

func handleTaskError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Build task resource not found.", nil)
	case errors.Is(err, ErrNoQueuedTask):
		writeError(w, http.StatusNotFound, "NOT_FOUND", "No queued build task.", nil)
	case errors.Is(err, ErrLogsNotFound):
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Build task logs not found.", nil)
	case errors.Is(err, ErrValidation):
		writeError(w, http.StatusBadRequest, "VALIDATION_FAILED", err.Error(), nil)
	case errors.Is(err, ErrInvalidState):
		writeError(w, http.StatusUnprocessableEntity, "INVALID_STATE", "Build task state does not allow this operation.", nil)
	default:
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Build task operation failed.", nil)
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
