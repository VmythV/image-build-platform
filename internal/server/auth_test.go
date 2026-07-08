package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/VmythV/image-build-platform/internal/auth"
	"github.com/VmythV/image-build-platform/internal/config"
	"github.com/VmythV/image-build-platform/internal/storage"
)

func TestAuthSetupLoginMeLogout(t *testing.T) {
	router := newAuthTestRouter(t)

	getJSON(t, router, http.MethodGet, "/api/v1/setup/status", "", nil, http.StatusOK, func(body map[string]any) {
		data := body["data"].(map[string]any)
		if data["initialized"] != false {
			t.Fatalf("expected initialized false, got %v", data["initialized"])
		}
	})

	postJSON(t, router, "/api/v1/setup/admin", `{"username":"admin","password":"ChangeMe123!","displayName":"Administrator"}`, nil, http.StatusCreated, nil)

	getJSON(t, router, http.MethodGet, "/api/v1/setup/status", "", nil, http.StatusOK, func(body map[string]any) {
		data := body["data"].(map[string]any)
		if data["initialized"] != true {
			t.Fatalf("expected initialized true, got %v", data["initialized"])
		}
	})

	var sessionCookie *http.Cookie
	postJSON(t, router, "/api/v1/auth/login", `{"username":"admin","password":"ChangeMe123!"}`, nil, http.StatusOK, func(rec *httptest.ResponseRecorder) {
		for _, cookie := range rec.Result().Cookies() {
			if cookie.Name == auth.SessionCookieName {
				sessionCookie = cookie
			}
		}
		if sessionCookie == nil {
			t.Fatalf("expected session cookie")
		}
	})

	getJSON(t, router, http.MethodGet, "/api/v1/auth/me", "", sessionCookie, http.StatusOK, func(body map[string]any) {
		data := body["data"].(map[string]any)
		user := data["user"].(map[string]any)
		if user["username"] != "admin" {
			t.Fatalf("expected username admin, got %v", user["username"])
		}
	})

	postJSON(t, router, "/api/v1/auth/logout", "", sessionCookie, http.StatusOK, nil)
	getJSON(t, router, http.MethodGet, "/api/v1/auth/me", "", sessionCookie, http.StatusUnauthorized, nil)
}

func newAuthTestRouter(t *testing.T) http.Handler {
	t.Helper()

	tempDir := t.TempDir()
	cfg := config.Default()
	cfg.Database.Driver = "sqlite"
	cfg.Database.DSN = filepath.Join(tempDir, "app.db")
	cfg.Storage.DataDir = filepath.Join(tempDir, "data")
	cfg.Storage.LogDir = filepath.Join(tempDir, "logs")
	cfg.Storage.ContextDir = filepath.Join(tempDir, "contexts")
	cfg.Storage.TmpDir = filepath.Join(tempDir, "tmp")
	cfg.Storage.BackupDir = filepath.Join(tempDir, "backups")

	store, err := storage.Open(context.Background(), cfg)
	if err != nil {
		t.Fatalf("open storage: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("close storage: %v", err)
		}
	})

	router, err := New(Options{
		Version:    "test",
		DB:         store.DB,
		DriverName: store.DriverName,
		SessionTTL: cfg.Security.SessionTTL,
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	return router
}

func postJSON(t *testing.T, router http.Handler, path string, body string, cookie *http.Cookie, expectedStatus int, assert func(*httptest.ResponseRecorder)) {
	t.Helper()
	getJSONRecorder(t, router, http.MethodPost, path, body, cookie, expectedStatus, assert)
}

func getJSON(t *testing.T, router http.Handler, method string, path string, body string, cookie *http.Cookie, expectedStatus int, assert func(map[string]any)) {
	t.Helper()
	getJSONRecorder(t, router, method, path, body, cookie, expectedStatus, func(rec *httptest.ResponseRecorder) {
		if assert == nil {
			return
		}
		var response map[string]any
		if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		assert(response)
	})
}

func getJSONRecorder(t *testing.T, router http.Handler, method string, path string, body string, cookie *http.Cookie, expectedStatus int, assert func(*httptest.ResponseRecorder)) {
	t.Helper()

	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != expectedStatus {
		t.Fatalf("expected status %d, got %d: %s", expectedStatus, rec.Code, rec.Body.String())
	}
	if assert != nil {
		assert(rec)
	}
}
