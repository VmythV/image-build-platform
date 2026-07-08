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

func TestBuildHostsRequireAuthAndCRUD(t *testing.T) {
	router := newAuthTestRouter(t)

	getJSON(t, router, http.MethodGet, "/api/v1/build-hosts", "", nil, http.StatusUnauthorized, nil)

	sessionCookie := initializeAdminAndLogin(t, router)
	getJSON(t, router, http.MethodGet, "/api/v1/build-hosts", "", sessionCookie, http.StatusOK, func(body map[string]any) {
		data := body["data"].([]any)
		if len(data) != 1 {
			t.Fatalf("expected default local build host, got %d hosts", len(data))
		}
		host := data[0].(map[string]any)
		if host["connectionType"] != "local_docker" {
			t.Fatalf("expected local_docker host, got %v", host["connectionType"])
		}
	})

	var sshHostID string
	postJSON(t, router, "/api/v1/build-hosts", `{"name":"SSH Builder","connectionType":"ssh","address":"192.0.2.10","port":22,"username":"builder","dockerCommand":"docker","maxConcurrency":1,"labels":["remote","arm64"]}`, sessionCookie, http.StatusCreated, func(rec *httptest.ResponseRecorder) {
		body := decodeJSONBody(t, rec)
		data := body["data"].(map[string]any)
		sshHostID = data["id"].(string)
		if data["connectionType"] != "ssh" {
			t.Fatalf("expected ssh host, got %v", data["connectionType"])
		}
	})

	postJSON(t, router, "/api/v1/build-hosts/"+sshHostID+"/disable", "", sessionCookie, http.StatusOK, func(rec *httptest.ResponseRecorder) {
		body := decodeJSONBody(t, rec)
		data := body["data"].(map[string]any)
		if data["status"] != "disabled" {
			t.Fatalf("expected disabled status, got %v", data["status"])
		}
	})

	postJSON(t, router, "/api/v1/build-hosts/"+sshHostID+"/enable", "", sessionCookie, http.StatusOK, func(rec *httptest.ResponseRecorder) {
		body := decodeJSONBody(t, rec)
		data := body["data"].(map[string]any)
		if data["status"] != "unknown" {
			t.Fatalf("expected unknown status, got %v", data["status"])
		}
	})

	getJSON(t, router, http.MethodDelete, "/api/v1/build-hosts/"+sshHostID, "", sessionCookie, http.StatusOK, nil)
	getJSON(t, router, http.MethodGet, "/api/v1/build-hosts/"+sshHostID, "", sessionCookie, http.StatusNotFound, nil)
}

func TestRegistriesRequireAuthAndCRUD(t *testing.T) {
	router := newAuthTestRouter(t)

	getJSON(t, router, http.MethodGet, "/api/v1/registries", "", nil, http.StatusUnauthorized, nil)

	sessionCookie := initializeAdminAndLogin(t, router)
	var registryID string
	postJSON(t, router, "/api/v1/registries", `{"name":"Internal Registry","type":"generic","endpoint":"registry.example.com","namespace":"platform","username":"robot","password":"registry-secret","allowPull":true,"allowPush":true,"isDefaultPull":false,"isDefaultPush":true,"tlsVerify":true,"insecureHttp":false}`, sessionCookie, http.StatusCreated, func(rec *httptest.ResponseRecorder) {
		if strings.Contains(rec.Body.String(), "registry-secret") {
			t.Fatalf("registry response leaked password")
		}
		body := decodeJSONBody(t, rec)
		data := body["data"].(map[string]any)
		registryID = data["id"].(string)
		if data["credentialConfigured"] != true {
			t.Fatalf("expected credentialConfigured true, got %v", data["credentialConfigured"])
		}
		if data["credentialUsername"] != "robot" {
			t.Fatalf("expected credential username robot, got %v", data["credentialUsername"])
		}
	})

	getJSONRecorder(t, router, http.MethodPut, "/api/v1/registries/"+registryID, `{"name":"Internal Registry Updated","type":"generic","endpoint":"registry.example.com","namespace":"platform","username":"robot","password":"","allowPull":true,"allowPush":true,"isDefaultPull":true,"isDefaultPush":true,"tlsVerify":true,"insecureHttp":false}`, sessionCookie, http.StatusOK, func(rec *httptest.ResponseRecorder) {
		body := decodeJSONBody(t, rec)
		data := body["data"].(map[string]any)
		if data["credentialConfigured"] != true {
			t.Fatalf("expected credential to remain configured")
		}
		if data["isDefaultPull"] != true {
			t.Fatalf("expected default pull true, got %v", data["isDefaultPull"])
		}
	})

	postJSON(t, router, "/api/v1/registries/"+registryID+"/disable", "", sessionCookie, http.StatusOK, func(rec *httptest.ResponseRecorder) {
		body := decodeJSONBody(t, rec)
		data := body["data"].(map[string]any)
		if data["status"] != "disabled" {
			t.Fatalf("expected disabled status, got %v", data["status"])
		}
	})

	postJSON(t, router, "/api/v1/registries/"+registryID+"/enable", "", sessionCookie, http.StatusOK, func(rec *httptest.ResponseRecorder) {
		body := decodeJSONBody(t, rec)
		data := body["data"].(map[string]any)
		if data["status"] != "unknown" {
			t.Fatalf("expected unknown status, got %v", data["status"])
		}
	})

	getJSON(t, router, http.MethodDelete, "/api/v1/registries/"+registryID, "", sessionCookie, http.StatusOK, nil)
	getJSON(t, router, http.MethodGet, "/api/v1/registries/"+registryID, "", sessionCookie, http.StatusNotFound, nil)
}

func TestImageProjectsBranchesAndVersionNodes(t *testing.T) {
	router := newAuthTestRouter(t)

	getJSON(t, router, http.MethodGet, "/api/v1/image-projects", "", nil, http.StatusUnauthorized, nil)

	sessionCookie := initializeAdminAndLogin(t, router)
	var projectID string
	postJSON(t, router, "/api/v1/image-projects", `{"name":"Java Runtime","imageType":"java","imageName":"java-runtime","namespace":"platform","rootImageRef":"eclipse-temurin:17","rootImageSource":"external_image","defaultArchitecture":"amd64","labels":["java","jdk17"],"description":"Base Java runtime."}`, sessionCookie, http.StatusCreated, func(rec *httptest.ResponseRecorder) {
		body := decodeJSONBody(t, rec)
		data := body["data"].(map[string]any)
		projectID = data["id"].(string)
		if data["latestVersion"] != "root" {
			t.Fatalf("expected latest root version, got %v", data["latestVersion"])
		}
	})

	var rootNodeID string
	var mainBranchID string
	getJSON(t, router, http.MethodGet, "/api/v1/image-projects/"+projectID+"/graph", "", sessionCookie, http.StatusOK, func(body map[string]any) {
		data := body["data"].(map[string]any)
		branches := data["branches"].([]any)
		if len(branches) != 1 {
			t.Fatalf("expected main branch, got %d branches", len(branches))
		}
		mainBranch := branches[0].(map[string]any)
		mainBranchID = mainBranch["id"].(string)
		if mainBranch["name"] != "main" {
			t.Fatalf("expected main branch, got %v", mainBranch["name"])
		}
		nodes := data["nodes"].([]any)
		if len(nodes) != 1 {
			t.Fatalf("expected root node, got %d nodes", len(nodes))
		}
		rootNode := nodes[0].(map[string]any)
		rootNodeID = rootNode["id"].(string)
		if rootNode["version"] != "root" {
			t.Fatalf("expected root node version, got %v", rootNode["version"])
		}
	})

	var branchID string
	postJSON(t, router, "/api/v1/image-projects/"+projectID+"/branches", `{"name":"jdk21","startNodeId":"`+rootNodeID+`","description":"Java 21 line."}`, sessionCookie, http.StatusCreated, func(rec *httptest.ResponseRecorder) {
		body := decodeJSONBody(t, rec)
		data := body["data"].(map[string]any)
		branchID = data["id"].(string)
		if data["headNodeId"] != rootNodeID {
			t.Fatalf("expected branch head root, got %v", data["headNodeId"])
		}
	})

	var childNodeID string
	postJSON(t, router, "/api/v1/image-projects/"+projectID+"/version-nodes", `{"branchId":"`+branchID+`","parentNodeId":"`+rootNodeID+`","version":"jdk21-v1","dockerfile":"FROM eclipse-temurin:21\nRUN java -version\n","description":"Java 21 runtime.","status":"active"}`, sessionCookie, http.StatusCreated, func(rec *httptest.ResponseRecorder) {
		body := decodeJSONBody(t, rec)
		data := body["data"].(map[string]any)
		childNodeID = data["id"].(string)
		if data["parentNodeId"] != rootNodeID {
			t.Fatalf("expected parent root, got %v", data["parentNodeId"])
		}
	})

	getJSON(t, router, http.MethodGet, "/api/v1/image-projects/"+projectID+"/graph", "", sessionCookie, http.StatusOK, func(body map[string]any) {
		data := body["data"].(map[string]any)
		branches := data["branches"].([]any)
		nodes := data["nodes"].([]any)
		edges := data["edges"].([]any)
		if len(branches) != 2 {
			t.Fatalf("expected two branches, got %d", len(branches))
		}
		if len(nodes) != 2 {
			t.Fatalf("expected two nodes, got %d", len(nodes))
		}
		if len(edges) != 1 {
			t.Fatalf("expected one edge, got %d", len(edges))
		}
	})

	getJSON(t, router, http.MethodGet, "/api/v1/image-projects/"+projectID+"/version-nodes/"+rootNodeID+"/diff/"+childNodeID, "", sessionCookie, http.StatusOK, func(body map[string]any) {
		data := body["data"].(map[string]any)
		diff := data["unifiedDiff"].(string)
		if !strings.Contains(diff, "+RUN java -version") {
			t.Fatalf("expected Dockerfile diff to include added java command, got %q", diff)
		}
	})

	postJSON(t, router, "/api/v1/image-projects/"+projectID+"/branches/"+mainBranchID+"/archive", "", sessionCookie, http.StatusOK, func(rec *httptest.ResponseRecorder) {
		body := decodeJSONBody(t, rec)
		data := body["data"].(map[string]any)
		if data["status"] != "archived" {
			t.Fatalf("expected branch archived, got %v", data["status"])
		}
	})
}

func TestDockerfileGenerateAndValidateRequireAuth(t *testing.T) {
	router := newAuthTestRouter(t)

	postJSON(t, router, "/api/v1/dockerfile/validate", `{"dockerfile":"FROM ubuntu:24.04\n"}`, nil, http.StatusUnauthorized, nil)

	sessionCookie := initializeAdminAndLogin(t, router)
	postJSON(t, router, "/api/v1/dockerfile/validate", `{"dockerfile":"RUN echo hi\n"}`, sessionCookie, http.StatusOK, func(rec *httptest.ResponseRecorder) {
		body := decodeJSONBody(t, rec)
		data := body["data"].(map[string]any)
		if data["valid"] != false {
			t.Fatalf("expected invalid Dockerfile, got %v", data["valid"])
		}
		errors := data["errors"].([]any)
		if len(errors) == 0 {
			t.Fatalf("expected validation errors")
		}
	})

	postJSON(t, router, "/api/v1/dockerfile/generate", `{"baseImage":"ubuntu:24.04","packages":["curl","ca-certificates"],"workdir":"/app","expose":[8080],"cmd":["./server"],"environment":{"APP_ENV":"test"},"copy":[],"entrypoint":[],"args":{},"labels":{}}`, sessionCookie, http.StatusOK, func(rec *httptest.ResponseRecorder) {
		body := decodeJSONBody(t, rec)
		data := body["data"].(map[string]any)
		dockerfile := data["dockerfile"].(string)
		if !strings.Contains(dockerfile, "FROM ubuntu:24.04") {
			t.Fatalf("expected generated Dockerfile FROM, got %q", dockerfile)
		}
		if !strings.Contains(dockerfile, "RUN apt-get update && apt-get install -y --no-install-recommends curl ca-certificates") {
			t.Fatalf("expected generated Dockerfile package install, got %q", dockerfile)
		}
	})
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
		SecretKey:  cfg.Security.SecretKey,
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	return router
}

func initializeAdminAndLogin(t *testing.T, router http.Handler) *http.Cookie {
	t.Helper()

	postJSON(t, router, "/api/v1/setup/admin", `{"username":"admin","password":"ChangeMe123!","displayName":"Administrator"}`, nil, http.StatusCreated, nil)

	var sessionCookie *http.Cookie
	postJSON(t, router, "/api/v1/auth/login", `{"username":"admin","password":"ChangeMe123!"}`, nil, http.StatusOK, func(rec *httptest.ResponseRecorder) {
		for _, cookie := range rec.Result().Cookies() {
			if cookie.Name == auth.SessionCookieName {
				sessionCookie = cookie
			}
		}
	})
	if sessionCookie == nil {
		t.Fatalf("expected session cookie")
	}
	return sessionCookie
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

func decodeJSONBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()

	var response map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return response
}
