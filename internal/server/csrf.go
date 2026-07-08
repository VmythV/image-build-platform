package server

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
)

const (
	csrfCookieName  = "ibp_csrf_token"
	csrfHeaderName  = "X-CSRF-Token"
	csrfTokenMaxAge = 24 * 60 * 60
)

type csrfProtection struct {
	enabled      bool
	secureCookie bool
}

func (c csrfProtection) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !c.enabled || isSafeMethod(r.Method) || isCSRFExemptPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		cookie, err := r.Cookie(csrfCookieName)
		if err != nil || cookie.Value == "" || r.Header.Get(csrfHeaderName) == "" || !sameToken(cookie.Value, r.Header.Get(csrfHeaderName)) {
			writeJSON(w, http.StatusForbidden, map[string]any{
				"error": map[string]any{
					"code":    "CSRF_FAILED",
					"message": "CSRF token is missing or invalid.",
					"details": nil,
				},
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (c csrfProtection) handleToken(w http.ResponseWriter, r *http.Request) {
	token := ""
	if cookie, err := r.Cookie(csrfCookieName); err == nil {
		token = cookie.Value
	}
	if token == "" {
		generated, err := generateCSRFToken()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error": map[string]any{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to create CSRF token.",
					"details": nil,
				},
			})
			return
		}
		token = generated
		http.SetCookie(w, &http.Cookie{
			Name:     csrfCookieName,
			Value:    token,
			Path:     "/",
			MaxAge:   csrfTokenMaxAge,
			SameSite: http.SameSiteLaxMode,
			Secure:   c.secureCookie,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": map[string]string{
			"token": token,
		},
	})
}

func generateCSRFToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func isSafeMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions
}

func isCSRFExemptPath(path string) bool {
	return path == "/api/v1/setup/admin" || path == "/api/v1/auth/login"
}

func sameToken(left string, right string) bool {
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}
