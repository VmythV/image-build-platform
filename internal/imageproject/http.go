package imageproject

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
	r.Get("/", h.listProjects)
	r.Post("/", h.createProject)
	r.Get("/{projectID}", h.getProject)
	r.Put("/{projectID}", h.updateProject)
	r.Post("/{projectID}/archive", h.archiveProject)
	r.Get("/{projectID}/graph", h.graph)
	r.Get("/{projectID}/branches", h.listBranches)
	r.Post("/{projectID}/branches", h.createBranch)
	r.Post("/{projectID}/branches/{branchID}/archive", h.archiveBranch)
	r.Post("/{projectID}/version-nodes", h.createNode)
	r.Get("/{projectID}/version-nodes/{leftNodeID}/diff/{rightNodeID}", h.diffNodes)
	r.Get("/{projectID}/version-nodes/{nodeID}", h.getNode)
	r.Put("/{projectID}/version-nodes/{nodeID}", h.updateNode)
	return r
}

func (h Handler) listProjects(w http.ResponseWriter, r *http.Request) {
	filter := ProjectFilter{
		ImageType: r.URL.Query().Get("imageType"),
		Status:    r.URL.Query().Get("status"),
		Keyword:   r.URL.Query().Get("keyword"),
		Page:      parseInt(r.URL.Query().Get("page")),
		PageSize:  parseInt(r.URL.Query().Get("pageSize")),
	}
	projects, total, err := h.service.ListProjects(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list image projects.", nil)
		return
	}
	data := make([]ProjectDTO, 0, len(projects))
	for _, project := range projects {
		data = append(data, ToProjectDTO(project))
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

func (h Handler) createProject(w http.ResponseWriter, r *http.Request) {
	user, ok := requireMaintainer(w, r)
	if !ok {
		return
	}
	var req ProjectInput
	if !decodeJSON(w, r, &req) {
		return
	}
	project, err := h.service.CreateProject(r.Context(), req, user.ID)
	if err != nil {
		handleError(w, err)
		return
	}
	writeData(w, http.StatusCreated, ToProjectDTO(project))
}

func (h Handler) getProject(w http.ResponseWriter, r *http.Request) {
	project, err := h.service.GetProject(r.Context(), chi.URLParam(r, "projectID"))
	if err != nil {
		handleError(w, err)
		return
	}
	writeData(w, http.StatusOK, ToProjectDTO(project))
}

func (h Handler) updateProject(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireMaintainer(w, r); !ok {
		return
	}
	var req ProjectInput
	if !decodeJSON(w, r, &req) {
		return
	}
	project, err := h.service.UpdateProject(r.Context(), chi.URLParam(r, "projectID"), req)
	if err != nil {
		handleError(w, err)
		return
	}
	writeData(w, http.StatusOK, ToProjectDTO(project))
}

func (h Handler) archiveProject(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireMaintainer(w, r); !ok {
		return
	}
	project, err := h.service.ArchiveProject(r.Context(), chi.URLParam(r, "projectID"))
	if err != nil {
		handleError(w, err)
		return
	}
	writeData(w, http.StatusOK, ToProjectDTO(project))
}

func (h Handler) graph(w http.ResponseWriter, r *http.Request) {
	graph, err := h.service.Graph(r.Context(), chi.URLParam(r, "projectID"), GraphFilter{
		Branch: r.URL.Query().Get("branch"),
		Status: r.URL.Query().Get("status"),
	})
	if err != nil {
		handleError(w, err)
		return
	}
	writeData(w, http.StatusOK, graph)
}

func (h Handler) listBranches(w http.ResponseWriter, r *http.Request) {
	branches, err := h.service.ListBranches(r.Context(), chi.URLParam(r, "projectID"))
	if err != nil {
		handleError(w, err)
		return
	}
	data := make([]BranchDTO, 0, len(branches))
	for _, branch := range branches {
		data = append(data, ToBranchDTO(branch))
	}
	writeData(w, http.StatusOK, data)
}

func (h Handler) createBranch(w http.ResponseWriter, r *http.Request) {
	user, ok := requireMaintainer(w, r)
	if !ok {
		return
	}
	var req BranchInput
	if !decodeJSON(w, r, &req) {
		return
	}
	branch, err := h.service.CreateBranch(r.Context(), chi.URLParam(r, "projectID"), req, user.ID)
	if err != nil {
		handleError(w, err)
		return
	}
	writeData(w, http.StatusCreated, ToBranchDTO(branch))
}

func (h Handler) archiveBranch(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireMaintainer(w, r); !ok {
		return
	}
	branch, err := h.service.ArchiveBranch(r.Context(), chi.URLParam(r, "projectID"), chi.URLParam(r, "branchID"))
	if err != nil {
		handleError(w, err)
		return
	}
	writeData(w, http.StatusOK, ToBranchDTO(branch))
}

func (h Handler) createNode(w http.ResponseWriter, r *http.Request) {
	user, ok := requireMaintainer(w, r)
	if !ok {
		return
	}
	var req VersionNodeInput
	if !decodeJSON(w, r, &req) {
		return
	}
	node, err := h.service.CreateNode(r.Context(), chi.URLParam(r, "projectID"), req, user.ID)
	if err != nil {
		handleError(w, err)
		return
	}
	writeData(w, http.StatusCreated, ToVersionNodeDTO(node))
}

func (h Handler) getNode(w http.ResponseWriter, r *http.Request) {
	node, err := h.service.GetNode(r.Context(), chi.URLParam(r, "projectID"), chi.URLParam(r, "nodeID"))
	if err != nil {
		handleError(w, err)
		return
	}
	writeData(w, http.StatusOK, ToVersionNodeDTO(node))
}

func (h Handler) diffNodes(w http.ResponseWriter, r *http.Request) {
	diff, err := h.service.DiffNodes(r.Context(), chi.URLParam(r, "projectID"), chi.URLParam(r, "leftNodeID"), chi.URLParam(r, "rightNodeID"))
	if err != nil {
		handleError(w, err)
		return
	}
	writeData(w, http.StatusOK, diff)
}

func (h Handler) updateNode(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireMaintainer(w, r); !ok {
		return
	}
	var req VersionNodeInput
	if !decodeJSON(w, r, &req) {
		return
	}
	node, err := h.service.UpdateNode(r.Context(), chi.URLParam(r, "projectID"), chi.URLParam(r, "nodeID"), req)
	if err != nil {
		handleError(w, err)
		return
	}
	writeData(w, http.StatusOK, ToVersionNodeDTO(node))
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
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Image project resource not found.", nil)
	case errors.Is(err, ErrValidation):
		writeError(w, http.StatusBadRequest, "VALIDATION_FAILED", err.Error(), nil)
	default:
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Image project operation failed.", nil)
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
