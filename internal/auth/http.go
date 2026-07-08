package auth

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

type Handler struct {
	service      Service
	secureCookie bool
}

type HandlerOptions struct {
	Service      Service
	SecureCookie bool
}

func NewHandler(opts HandlerOptions) Handler {
	return Handler{
		service:      opts.Service,
		secureCookie: opts.SecureCookie,
	}
}

func (h Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/setup/status", h.setupStatus)
	r.Post("/setup/admin", h.initializeAdmin)
	r.Post("/auth/login", h.login)
	r.Post("/auth/logout", h.logout)
	r.Get("/auth/me", h.me)
	return r
}

func (h Handler) setupStatus(w http.ResponseWriter, r *http.Request) {
	initialized, err := h.service.IsInitialized(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to check setup status.", nil)
		return
	}
	writeData(w, http.StatusOK, map[string]bool{"initialized": initialized})
}

func (h Handler) initializeAdmin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username    string `json:"username"`
		Password    string `json:"password"`
		DisplayName string `json:"displayName"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	user, err := h.service.InitializeAdmin(r.Context(), req.Username, req.Password, req.DisplayName)
	if err != nil {
		switch {
		case errors.Is(err, ErrAlreadyInitialized):
			writeError(w, http.StatusConflict, "CONFLICT", "System is already initialized.", nil)
		default:
			writeError(w, http.StatusBadRequest, "VALIDATION_FAILED", err.Error(), nil)
		}
		return
	}

	writeData(w, http.StatusCreated, map[string]any{"user": ToUserDTO(user)})
}

func (h Handler) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	user, token, expiresAt, err := h.service.Login(r.Context(), req.Username, req.Password, r.UserAgent(), clientIP(r))
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			writeError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid username or password.", nil)
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Login failed.", nil)
		return
	}

	http.SetCookie(w, h.sessionCookie(token, expiresAt))
	writeData(w, http.StatusOK, map[string]any{"user": ToUserDTO(user)})
}

func (h Handler) logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(SessionCookieName)
	if err == nil {
		if logoutErr := h.service.Logout(r.Context(), cookie.Value); logoutErr != nil {
			slog.Default().Warn("logout failed", "error", logoutErr)
		}
	}

	http.SetCookie(w, h.expiredSessionCookie())
	writeData(w, http.StatusOK, map[string]bool{"success": true})
}

func (h Handler) me(w http.ResponseWriter, r *http.Request) {
	user, err := h.userFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication is required.", nil)
		return
	}
	writeData(w, http.StatusOK, map[string]any{"user": ToUserDTO(user)})
}

func (h Handler) userFromRequest(r *http.Request) (User, error) {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		return User{}, ErrUnauthenticated
	}
	return h.service.CurrentUser(r.Context(), cookie.Value)
}

func (h Handler) sessionCookie(token string, expiresAt time.Time) *http.Cookie {
	return &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteLaxMode,
	}
}

func (h Handler) expiredSessionCookie() *http.Cookie {
	return &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteLaxMode,
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

func clientIP(r *http.Request) string {
	if value := r.Header.Get("X-Forwarded-For"); value != "" {
		return value
	}
	if value := r.Header.Get("X-Real-IP"); value != "" {
		return value
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
