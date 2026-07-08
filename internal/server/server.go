package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/VmythV/image-build-platform/internal/audit"
	"github.com/VmythV/image-build-platform/internal/auth"
	"github.com/VmythV/image-build-platform/internal/buildhost"
	"github.com/VmythV/image-build-platform/internal/buildtask"
	"github.com/VmythV/image-build-platform/internal/credential"
	"github.com/VmythV/image-build-platform/internal/dashboard"
	"github.com/VmythV/image-build-platform/internal/dockerfile"
	"github.com/VmythV/image-build-platform/internal/imageartifact"
	"github.com/VmythV/image-build-platform/internal/imageproject"
	"github.com/VmythV/image-build-platform/internal/registry"
	"github.com/VmythV/image-build-platform/internal/settings"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Options struct {
	StaticDir     string
	Version       string
	Logger        *slog.Logger
	DB            *sql.DB
	DriverName    string
	SessionTTL    string
	SecureCookie  bool
	SecretKey     string
	ContextDir    string
	LogDir        string
	BuildExecutor buildtask.Executor
}

func New(opts Options) (http.Handler, error) {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	var authHandler auth.Handler
	var authRoutes http.Handler
	var buildHostRoutes http.Handler
	var registryRoutes http.Handler
	var imageProjectRoutes http.Handler
	var dockerfileRoutes http.Handler
	var buildTaskRoutes http.Handler
	var artifactRoutes http.Handler
	var dashboardRoutes http.Handler
	var settingsRoutes http.Handler
	var auditRoutes http.Handler
	var auditRepo audit.Repository
	if opts.DB != nil {
		service, err := auth.NewService(auth.ServiceOptions{
			Repository: auth.NewRepository(opts.DB, opts.DriverName),
			SessionTTL: opts.SessionTTL,
		})
		if err != nil {
			return nil, fmt.Errorf("initialize auth service: %w", err)
		}

		authHandler = auth.NewHandler(auth.HandlerOptions{
			Service:      service,
			SecureCookie: opts.SecureCookie,
		})
		authRoutes = authHandler.Routes()

		buildHostService := buildhost.NewService(
			buildhost.NewRepository(opts.DB, opts.DriverName),
			buildhost.CommandDetector{},
		)
		if err := buildHostService.EnsureDefaultLocalHost(context.Background()); err != nil {
			return nil, fmt.Errorf("ensure default local build host: %w", err)
		}
		buildHostRoutes = buildhost.NewHandler(buildHostService).Routes()
		settingsRepo := settings.NewRepository(opts.DB, opts.DriverName)
		if err := settingsRepo.EnsureDefaults(context.Background(), time.Now()); err != nil {
			return nil, fmt.Errorf("ensure default settings: %w", err)
		}
		settingsRoutes = settings.NewHandler(settingsRepo).Routes()
		auditRepo = audit.NewRepository(opts.DB, opts.DriverName)
		auditRoutes = audit.NewHandler(auditRepo).Routes()

		credentialEncryptor, err := credential.NewEncryptor(opts.SecretKey)
		if err != nil {
			return nil, fmt.Errorf("initialize credential encryption: %w", err)
		}
		registryService := registry.NewService(
			registry.NewRepository(opts.DB, opts.DriverName),
			credential.NewRepository(opts.DB, opts.DriverName),
			credentialEncryptor,
			registry.CommandDetector{},
		)
		registryRoutes = registry.NewHandler(registryService).Routes()
		artifactRepo := imageartifact.NewRepository(opts.DB, opts.DriverName)
		artifactRoutes = imageartifact.NewHandler(artifactRepo).Routes()
		dashboardRoutes = dashboard.NewHandler(dashboard.NewRepository(opts.DB)).Routes()

		imageProjectService := imageproject.NewService(
			imageproject.NewRepository(opts.DB, opts.DriverName),
		)
		imageProjectRoutes = imageproject.NewHandler(imageProjectService).Routes()
		dockerfileRoutes = dockerfile.NewHandler(dockerfile.NewService()).Routes()
		buildTaskService := buildtask.NewServiceWithOptions(buildtask.ServiceOptions{
			Repository:      buildtask.NewRepository(opts.DB, opts.DriverName),
			Projects:        imageproject.NewRepository(opts.DB, opts.DriverName),
			Registries:      registry.NewRepository(opts.DB, opts.DriverName),
			RegistrySecrets: registryService,
			Artifacts:       artifactRepo,
			Hosts:           buildhost.NewRepository(opts.DB, opts.DriverName),
			ContextDir:      opts.ContextDir,
			LogDir:          opts.LogDir,
			Executor:        opts.BuildExecutor,
			Logger:          logger,
		})
		buildTaskRoutes = buildtask.NewHandler(buildTaskService).Routes()
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.CleanPath)
	r.Use(timeoutExceptLogStreams(60 * time.Second))

	r.Get("/healthz", healthHandler(opts.Version, opts.DB))

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/status", statusHandler(opts.Version))
		if authRoutes != nil {
			r.Mount("/", authRoutes)
		}
		if buildHostRoutes != nil {
			r.Group(func(r chi.Router) {
				r.Use(auth.Middleware(authHandler))
				r.Use(audit.Middleware(auditRepo, logger))
				r.Mount("/build-hosts", buildHostRoutes)
			})
		}
		if dashboardRoutes != nil {
			r.Group(func(r chi.Router) {
				r.Use(auth.Middleware(authHandler))
				r.Mount("/dashboard", dashboardRoutes)
			})
		}
		if registryRoutes != nil {
			r.Group(func(r chi.Router) {
				r.Use(auth.Middleware(authHandler))
				r.Use(audit.Middleware(auditRepo, logger))
				r.Mount("/registries", registryRoutes)
			})
		}
		if imageProjectRoutes != nil {
			r.Group(func(r chi.Router) {
				r.Use(auth.Middleware(authHandler))
				r.Use(audit.Middleware(auditRepo, logger))
				r.Mount("/image-projects", imageProjectRoutes)
			})
		}
		if dockerfileRoutes != nil {
			r.Group(func(r chi.Router) {
				r.Use(auth.Middleware(authHandler))
				r.Use(audit.Middleware(auditRepo, logger))
				r.Mount("/dockerfile", dockerfileRoutes)
			})
		}
		if buildTaskRoutes != nil {
			r.Group(func(r chi.Router) {
				r.Use(auth.Middleware(authHandler))
				r.Use(audit.Middleware(auditRepo, logger))
				r.Mount("/build-tasks", buildTaskRoutes)
			})
		}
		if artifactRoutes != nil {
			r.Group(func(r chi.Router) {
				r.Use(auth.Middleware(authHandler))
				r.Mount("/artifacts", artifactRoutes)
			})
		}
		if settingsRoutes != nil {
			r.Group(func(r chi.Router) {
				r.Use(auth.Middleware(authHandler))
				r.Use(audit.Middleware(auditRepo, logger))
				r.Mount("/settings", settingsRoutes)
			})
		}
		if auditRoutes != nil {
			r.Group(func(r chi.Router) {
				r.Use(auth.Middleware(authHandler))
				r.Mount("/audit-logs", auditRoutes)
			})
		}
	})

	if opts.StaticDir != "" {
		r.NotFound(spaFallback(opts.StaticDir, logger))
	}

	return r, nil
}

func timeoutExceptLogStreams(timeout time.Duration) func(http.Handler) http.Handler {
	timeoutMiddleware := middleware.Timeout(timeout)
	return func(next http.Handler) http.Handler {
		timeoutHandler := timeoutMiddleware(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/logs/stream") {
				next.ServeHTTP(w, r)
				return
			}
			timeoutHandler.ServeHTTP(w, r)
		})
	}
}

func healthHandler(version string, db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		databaseStatus := "not_configured"
		if db != nil {
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()
			if err := db.PingContext(ctx); err != nil {
				writeJSON(w, http.StatusServiceUnavailable, map[string]string{
					"status":   "unhealthy",
					"time":     time.Now().UTC().Format(time.RFC3339),
					"version":  version,
					"database": "unavailable",
				})
				return
			}
			databaseStatus = "ok"
		}

		writeJSON(w, http.StatusOK, map[string]string{
			"status":   "ok",
			"time":     time.Now().UTC().Format(time.RFC3339),
			"version":  version,
			"database": databaseStatus,
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
