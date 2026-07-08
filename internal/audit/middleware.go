package audit

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/VmythV/image-build-platform/internal/auth"
	"github.com/go-chi/chi/v5/middleware"
)

func Middleware(repo Repository, logger *slog.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(recorder, r)

			if !shouldAudit(r.Method) {
				return
			}
			user, ok := auth.UserFromContext(r.Context())
			if !ok {
				return
			}
			input := recordInput(r, user, recorder.status)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if err := repo.Record(ctx, input, time.Now()); err != nil {
				logger.Warn("record audit log", "error", err)
			}
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func shouldAudit(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func recordInput(r *http.Request, user auth.User, status int) RecordInput {
	resourceType, resourceID := parseResource(r.URL.Path)
	detail, _ := json.Marshal(map[string]any{
		"method": r.Method,
		"path":   r.URL.Path,
		"status": status,
	})

	return RecordInput{
		ActorID:      user.ID,
		ActorName:    user.Username,
		Action:       actionName(r.Method, resourceType),
		ResourceType: resourceType,
		ResourceID:   resourceID,
		IPAddress:    r.RemoteAddr,
		UserAgent:    r.UserAgent(),
		RequestID:    middleware.GetReqID(r.Context()),
		Detail:       string(detail),
	}
}

func parseResource(path string) (string, string) {
	path = strings.Trim(strings.TrimPrefix(path, "/api/v1/"), "/")
	if path == "" {
		return "api", ""
	}
	parts := strings.Split(path, "/")
	resourceType := parts[0]
	resourceID := ""
	if len(parts) > 1 && !knownSubAction(parts[1]) {
		resourceID = parts[1]
	}
	return resourceType, resourceID
}

func knownSubAction(value string) bool {
	switch value {
	case "dispatch-next", "summary":
		return true
	default:
		return false
	}
}

func actionName(method string, resourceType string) string {
	switch method {
	case http.MethodPost:
		return "create_or_action_" + resourceType
	case http.MethodPut, http.MethodPatch:
		return "update_" + resourceType
	case http.MethodDelete:
		return "delete_" + resourceType
	default:
		return strings.ToLower(method) + "_" + resourceType
	}
}
