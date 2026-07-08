package server

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Options struct {
	StaticDir string
	Version   string
	Logger    *slog.Logger
}

func New(opts Options) http.Handler {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.CleanPath)
	r.Use(middleware.Timeout(60 * time.Second))

	r.Get("/healthz", healthHandler(opts.Version))

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/status", statusHandler(opts.Version))
	})

	if opts.StaticDir != "" {
		r.NotFound(spaFallback(opts.StaticDir, logger))
	}

	return r
}

func healthHandler(version string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"status":  "ok",
			"time":    time.Now().UTC().Format(time.RFC3339),
			"version": version,
		})
	}
}

func statusHandler(version string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"data": map[string]string{
				"status":  "running",
				"version": version,
			},
		})
	}
}

func spaFallback(staticDir string, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			writeJSON(w, http.StatusNotFound, map[string]any{
				"error": map[string]any{
					"code":    "NOT_FOUND",
					"message": "Resource not found.",
					"details": nil,
				},
			})
			return
		}

		file, ok := staticFile(staticDir, r.URL.Path)
		if ok {
			http.ServeFile(w, r, file)
			return
		}

		index := filepath.Join(staticDir, "index.html")
		if _, err := os.Stat(index); err == nil {
			http.ServeFile(w, r, index)
			return
		}

		logger.Debug("frontend static assets not found", "staticDir", staticDir)
		writeJSON(w, http.StatusNotFound, map[string]any{
			"error": map[string]any{
				"code":    "FRONTEND_NOT_BUILT",
				"message": "Frontend assets are not available. Run the frontend build first.",
				"details": nil,
			},
		})
	}
}

func staticFile(staticDir string, requestPath string) (string, bool) {
	cleanPath := strings.TrimPrefix(path.Clean("/"+requestPath), "/")
	if cleanPath == "." || cleanPath == "" {
		return "", false
	}

	root, err := filepath.Abs(staticDir)
	if err != nil {
		return "", false
	}

	candidate, err := filepath.Abs(filepath.Join(staticDir, filepath.FromSlash(cleanPath)))
	if err != nil {
		return "", false
	}

	rel, err := filepath.Rel(root, candidate)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}

	info, err := os.Stat(candidate)
	if err != nil || info.IsDir() {
		return "", false
	}

	return candidate, true
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil && !errors.Is(err, http.ErrHandlerTimeout) {
		slog.Default().Error("write json response", "error", err)
	}
}
