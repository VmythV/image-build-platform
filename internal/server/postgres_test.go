package server

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/VmythV/image-build-platform/internal/config"
	"github.com/VmythV/image-build-platform/internal/storage"
)

func TestPostgresSmoke(t *testing.T) {
	dsn := os.Getenv("IBP_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("IBP_TEST_POSTGRES_DSN is not set")
	}

	schema := fmt.Sprintf("ibp_test_%d", time.Now().UnixNano())
	createPostgresSchema(t, dsn, schema)

	tempDir := t.TempDir()
	cfg := config.Default()
	cfg.Database.Driver = "postgres"
	cfg.Database.DSN = postgresDSNWithSearchPath(t, dsn, schema)
	cfg.Storage.DataDir = filepath.Join(tempDir, "data")
	cfg.Storage.LogDir = filepath.Join(tempDir, "logs")
	cfg.Storage.ContextDir = filepath.Join(tempDir, "contexts")
	cfg.Storage.TmpDir = filepath.Join(tempDir, "tmp")
	cfg.Storage.BackupDir = filepath.Join(tempDir, "backups")

	store, err := storage.Open(context.Background(), cfg)
	if err != nil {
		t.Fatalf("open postgres storage: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("close postgres storage: %v", err)
		}
	})

	router, err := New(Options{
		Version:       "test",
		DB:            store.DB,
		DriverName:    store.DriverName,
		SessionTTL:    cfg.Security.SessionTTL,
		SecretKey:     cfg.Security.SecretKey,
		ContextDir:    cfg.Storage.ContextDir,
		LogDir:        cfg.Storage.LogDir,
		BuildExecutor: fakeBuildExecutor{},
	})
	if err != nil {
		t.Fatalf("new postgres server: %v", err)
	}

	sessionCookie := initializeAdminAndLogin(t, router)

	postJSON(t, router, "/api/v1/registries", `{"name":"Push Registry","type":"generic","endpoint":"registry.example.com","namespace":"platform","allowPull":true,"allowPush":true,"isDefaultPull":false,"isDefaultPush":true,"tlsVerify":true,"insecureHttp":false}`, sessionCookie, http.StatusCreated, nil)

	var projectID string
	postJSON(t, router, "/api/v1/image-projects", `{"name":"Postgres Runtime","imageType":"other","imageName":"postgres-runtime","namespace":"platform","rootImageRef":"alpine:3.20","rootImageSource":"external_image","defaultArchitecture":"amd64","labels":["postgres"],"description":"Postgres smoke project."}`, sessionCookie, http.StatusCreated, func(rec *httptest.ResponseRecorder) {
		body := decodeJSONBody(t, rec)
		data := body["data"].(map[string]any)
		projectID = data["id"].(string)
	})

	var rootNodeID string
	getJSON(t, router, http.MethodGet, "/api/v1/image-projects/"+projectID+"/graph", "", sessionCookie, http.StatusOK, func(body map[string]any) {
		data := body["data"].(map[string]any)
		nodes := data["nodes"].([]any)
		rootNodeID = nodes[0].(map[string]any)["id"].(string)
	})

	var taskID string
	postJSON(t, router, "/api/v1/build-tasks", `{"projectId":"`+projectID+`","versionNodeId":"`+rootNodeID+`","architecture":"amd64","imageTag":"pg-smoke"}`, sessionCookie, http.StatusCreated, func(rec *httptest.ResponseRecorder) {
		body := decodeJSONBody(t, rec)
		data := body["data"].(map[string]any)
		taskID = data["id"].(string)
	})

	postJSON(t, router, "/api/v1/build-tasks/"+taskID+"/start", "", sessionCookie, http.StatusOK, nil)
	waitForBuildTaskStatus(t, router, taskID, sessionCookie, "push_success")

	getJSON(t, router, http.MethodGet, "/api/v1/artifacts", "", sessionCookie, http.StatusOK, func(body map[string]any) {
		data := body["data"].([]any)
		if len(data) != 1 {
			t.Fatalf("expected one artifact, got %d", len(data))
		}
	})
}

func createPostgresSchema(t *testing.T, dsn string, schema string) {
	t.Helper()

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open postgres admin connection: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		_ = db.Close()
	})

	if err := db.PingContext(context.Background()); err != nil {
		t.Fatalf("ping postgres: %v", err)
	}
	if _, err := db.ExecContext(context.Background(), "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("create postgres schema: %v", err)
	}
}

func postgresDSNWithSearchPath(t *testing.T, dsn string, schema string) string {
	t.Helper()

	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parse postgres dsn: %v", err)
	}
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}
