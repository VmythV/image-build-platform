package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/VmythV/image-build-platform/internal/auth"
	"github.com/VmythV/image-build-platform/internal/buildhost"
	"github.com/VmythV/image-build-platform/internal/buildtask"
	"github.com/VmythV/image-build-platform/internal/config"
	"github.com/VmythV/image-build-platform/internal/registry"
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

func TestDashboardSummaryRequiresAuth(t *testing.T) {
	router := newAuthTestRouter(t)

	getJSON(t, router, http.MethodGet, "/api/v1/dashboard/summary", "", nil, http.StatusUnauthorized, nil)

	sessionCookie := initializeAdminAndLogin(t, router)
	getJSON(t, router, http.MethodGet, "/api/v1/dashboard/summary", "", sessionCookie, http.StatusOK, func(body map[string]any) {
		data := body["data"].(map[string]any)
		builds := data["builds"].(map[string]any)
		if builds["total"] != float64(0) {
			t.Fatalf("expected zero builds in fresh dashboard, got %v", builds["total"])
		}
		hosts := data["hosts"].(map[string]any)
		if hosts["total"] != float64(1) {
			t.Fatalf("expected default local host in dashboard, got %v", hosts["total"])
		}
	})
}

func TestSettingsAndAuditLogsRequireAuth(t *testing.T) {
	router := newAuthTestRouter(t)

	getJSON(t, router, http.MethodGet, "/api/v1/settings", "", nil, http.StatusUnauthorized, nil)
	getJSON(t, router, http.MethodGet, "/api/v1/audit-logs", "", nil, http.StatusUnauthorized, nil)

	sessionCookie := initializeAdminAndLogin(t, router)
	getJSON(t, router, http.MethodGet, "/api/v1/settings", "", sessionCookie, http.StatusOK, func(body map[string]any) {
		data := body["data"].([]any)
		if len(data) == 0 {
			t.Fatalf("expected default settings")
		}
	})

	getJSON(t, router, http.MethodPut, "/api/v1/settings/build.timeout_minutes", `{"value":"90"}`, sessionCookie, http.StatusOK, func(body map[string]any) {
		data := body["data"].(map[string]any)
		if data["value"] != "90" {
			t.Fatalf("expected updated setting value 90, got %v", data["value"])
		}
	})

	getJSON(t, router, http.MethodGet, "/api/v1/audit-logs", "", sessionCookie, http.StatusOK, func(body map[string]any) {
		data := body["data"].([]any)
		if len(data) == 0 {
			t.Fatalf("expected audit log after setting update")
		}
		log := data[0].(map[string]any)
		if log["action"] != "update_settings" {
			t.Fatalf("expected update_settings audit action, got %v", log["action"])
		}
		if log["resourceId"] != "build.timeout_minutes" {
			t.Fatalf("expected setting key resource id, got %v", log["resourceId"])
		}
	})
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

func TestBuildTasksQueueDispatchCancelAndRetry(t *testing.T) {
	router := newAuthTestRouter(t)

	getJSON(t, router, http.MethodGet, "/api/v1/build-tasks", "", nil, http.StatusUnauthorized, nil)

	sessionCookie := initializeAdminAndLogin(t, router)
	postJSON(t, router, "/api/v1/registries", `{"name":"Push Registry","type":"generic","endpoint":"registry.example.com","namespace":"platform","allowPull":true,"allowPush":true,"isDefaultPull":false,"isDefaultPush":true,"tlsVerify":true,"insecureHttp":false}`, sessionCookie, http.StatusCreated, nil)

	var projectID string
	postJSON(t, router, "/api/v1/image-projects", `{"name":"Java Runtime","imageType":"java","imageName":"java-runtime","namespace":"platform","rootImageRef":"eclipse-temurin:17","rootImageSource":"external_image","defaultArchitecture":"amd64","labels":["java"],"description":"Base Java runtime."}`, sessionCookie, http.StatusCreated, func(rec *httptest.ResponseRecorder) {
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
	postJSON(t, router, "/api/v1/build-tasks", `{"projectId":"`+projectID+`","versionNodeId":"`+rootNodeID+`","architecture":"amd64","imageTag":"root","buildArgs":{"APP_ENV":"test"}}`, sessionCookie, http.StatusCreated, func(rec *httptest.ResponseRecorder) {
		body := decodeJSONBody(t, rec)
		data := body["data"].(map[string]any)
		taskID = data["id"].(string)
		if data["status"] != "queued" {
			t.Fatalf("expected queued status, got %v", data["status"])
		}
		if data["imageRef"] != "registry.example.com/platform/java-runtime:root" {
			t.Fatalf("expected image ref, got %v", data["imageRef"])
		}
	})

	postJSON(t, router, "/api/v1/build-tasks/"+taskID+"/dispatch", "", sessionCookie, http.StatusOK, func(rec *httptest.ResponseRecorder) {
		body := decodeJSONBody(t, rec)
		data := body["data"].(map[string]any)
		if data["dispatched"] != true {
			t.Fatalf("expected dispatched true, got %v", data["dispatched"])
		}
		task := data["task"].(map[string]any)
		if task["status"] != "dispatching" {
			t.Fatalf("expected dispatching status, got %v", task["status"])
		}
		if task["hostId"] == nil {
			t.Fatalf("expected host id after dispatch")
		}
	})

	getJSON(t, router, http.MethodGet, "/api/v1/build-hosts", "", sessionCookie, http.StatusOK, func(body map[string]any) {
		data := body["data"].([]any)
		host := data[0].(map[string]any)
		if host["currentRunning"] != float64(1) {
			t.Fatalf("expected host running count 1, got %v", host["currentRunning"])
		}
	})

	postJSON(t, router, "/api/v1/build-tasks/"+taskID+"/cancel", "", sessionCookie, http.StatusOK, func(rec *httptest.ResponseRecorder) {
		body := decodeJSONBody(t, rec)
		data := body["data"].(map[string]any)
		if data["status"] != "canceled" {
			t.Fatalf("expected canceled status, got %v", data["status"])
		}
	})

	getJSON(t, router, http.MethodGet, "/api/v1/build-hosts", "", sessionCookie, http.StatusOK, func(body map[string]any) {
		data := body["data"].([]any)
		host := data[0].(map[string]any)
		if host["currentRunning"] != float64(0) {
			t.Fatalf("expected host running count 0, got %v", host["currentRunning"])
		}
	})

	var retryTaskID string
	postJSON(t, router, "/api/v1/build-tasks/"+taskID+"/retry", "", sessionCookie, http.StatusCreated, func(rec *httptest.ResponseRecorder) {
		body := decodeJSONBody(t, rec)
		data := body["data"].(map[string]any)
		retryTaskID = data["id"].(string)
		if data["status"] != "queued" {
			t.Fatalf("expected retry queued, got %v", data["status"])
		}
		if data["retryOfTaskId"] != taskID {
			t.Fatalf("expected retryOfTaskId %s, got %v", taskID, data["retryOfTaskId"])
		}
	})

	postJSON(t, router, "/api/v1/build-tasks/dispatch-next", "", sessionCookie, http.StatusOK, func(rec *httptest.ResponseRecorder) {
		body := decodeJSONBody(t, rec)
		data := body["data"].(map[string]any)
		task := data["task"].(map[string]any)
		if task["id"] != retryTaskID {
			t.Fatalf("expected dispatch next retry task %s, got %v", retryTaskID, task["id"])
		}
		if task["status"] != "dispatching" {
			t.Fatalf("expected retry dispatching, got %v", task["status"])
		}
	})

	postJSON(t, router, "/api/v1/build-tasks/"+retryTaskID+"/start", "", sessionCookie, http.StatusOK, func(rec *httptest.ResponseRecorder) {
		body := decodeJSONBody(t, rec)
		data := body["data"].(map[string]any)
		if data["status"] != "building" {
			t.Fatalf("expected building status after start, got %v", data["status"])
		}
		if data["buildContextRef"] == nil {
			t.Fatalf("expected build context ref")
		}
		if data["logPath"] == nil {
			t.Fatalf("expected log path")
		}
	})

	waitForBuildTaskStatus(t, router, retryTaskID, sessionCookie, "push_success")
	getJSON(t, router, http.MethodGet, "/api/v1/build-hosts", "", sessionCookie, http.StatusOK, func(body map[string]any) {
		data := body["data"].([]any)
		host := data[0].(map[string]any)
		if host["currentRunning"] != float64(0) {
			t.Fatalf("expected host running count 0 after build success, got %v", host["currentRunning"])
		}
	})
	getText(t, router, http.MethodGet, "/api/v1/build-tasks/"+retryTaskID+"/logs", sessionCookie, http.StatusOK, func(body string) {
		if !strings.Contains(body, "fake local build completed") {
			t.Fatalf("expected fake build log, got %q", body)
		}
	})
	getText(t, router, http.MethodGet, "/api/v1/build-tasks/"+retryTaskID+"/logs/stream", sessionCookie, http.StatusOK, func(body string) {
		if !strings.Contains(body, "event: log") {
			t.Fatalf("expected SSE log event, got %q", body)
		}
		if !strings.Contains(body, "fake local build completed") {
			t.Fatalf("expected fake build log in stream, got %q", body)
		}
		if !strings.Contains(body, "event: done") || !strings.Contains(body, "data: push_success") {
			t.Fatalf("expected SSE done event, got %q", body)
		}
	})
	getJSON(t, router, http.MethodGet, "/api/v1/artifacts", "", sessionCookie, http.StatusOK, func(body map[string]any) {
		data := body["data"].([]any)
		if len(data) != 1 {
			t.Fatalf("expected one artifact, got %d", len(data))
		}
		artifact := data[0].(map[string]any)
		if artifact["imageRef"] != "registry.example.com/platform/java-runtime:root" {
			t.Fatalf("expected artifact image ref, got %v", artifact["imageRef"])
		}
		if artifact["digest"] != "sha256:fake-digest" {
			t.Fatalf("expected fake digest, got %v", artifact["digest"])
		}
	})
}

func TestBuildTasksCanRunOnSSHHost(t *testing.T) {
	router := newAuthTestRouter(t)
	sessionCookie := initializeAdminAndLogin(t, router)

	postJSON(t, router, "/api/v1/registries", `{"name":"Push Registry","type":"generic","endpoint":"registry.example.com","namespace":"platform","allowPull":true,"allowPush":true,"isDefaultPull":false,"isDefaultPush":true,"tlsVerify":true,"insecureHttp":false}`, sessionCookie, http.StatusCreated, nil)

	var sshHostID string
	postJSON(t, router, "/api/v1/build-hosts", `{"name":"SSH Builder","connectionType":"ssh","address":"192.0.2.10","port":22,"username":"builder","dockerCommand":"docker","maxConcurrency":1,"labels":["remote","amd64"]}`, sessionCookie, http.StatusCreated, func(rec *httptest.ResponseRecorder) {
		body := decodeJSONBody(t, rec)
		data := body["data"].(map[string]any)
		sshHostID = data["id"].(string)
		if data["connectionType"] != "ssh" {
			t.Fatalf("expected ssh host, got %v", data["connectionType"])
		}
	})

	var projectID string
	postJSON(t, router, "/api/v1/image-projects", `{"name":"Python Runtime","imageType":"python","imageName":"python-runtime","namespace":"platform","rootImageRef":"python:3.12-slim","rootImageSource":"external_image","defaultArchitecture":"amd64","labels":["python"],"description":"Base Python runtime."}`, sessionCookie, http.StatusCreated, func(rec *httptest.ResponseRecorder) {
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
	postJSON(t, router, "/api/v1/build-tasks", `{"projectId":"`+projectID+`","versionNodeId":"`+rootNodeID+`","requestedHostId":"`+sshHostID+`","architecture":"amd64","imageTag":"ssh-root"}`, sessionCookie, http.StatusCreated, func(rec *httptest.ResponseRecorder) {
		body := decodeJSONBody(t, rec)
		data := body["data"].(map[string]any)
		taskID = data["id"].(string)
		if data["requestedHostId"] != sshHostID {
			t.Fatalf("expected requestedHostId %s, got %v", sshHostID, data["requestedHostId"])
		}
	})

	postJSON(t, router, "/api/v1/build-tasks/"+taskID+"/start", "", sessionCookie, http.StatusOK, func(rec *httptest.ResponseRecorder) {
		body := decodeJSONBody(t, rec)
		data := body["data"].(map[string]any)
		if data["status"] != "building" {
			t.Fatalf("expected building status after SSH start, got %v", data["status"])
		}
		if data["hostId"] != sshHostID {
			t.Fatalf("expected SSH host id %s, got %v", sshHostID, data["hostId"])
		}
	})

	waitForBuildTaskStatus(t, router, taskID, sessionCookie, "push_success")
	getText(t, router, http.MethodGet, "/api/v1/build-tasks/"+taskID+"/logs", sessionCookie, http.StatusOK, func(body string) {
		if !strings.Contains(body, "on SSH Builder") {
			t.Fatalf("expected SSH host name in fake build log, got %q", body)
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

func getText(t *testing.T, router http.Handler, method string, path string, cookie *http.Cookie, expectedStatus int, assert func(string)) {
	t.Helper()

	req := httptest.NewRequest(method, path, nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != expectedStatus {
		t.Fatalf("expected status %d, got %d: %s", expectedStatus, rec.Code, rec.Body.String())
	}
	if assert != nil {
		assert(rec.Body.String())
	}
}

func waitForBuildTaskStatus(t *testing.T, router http.Handler, taskID string, cookie *http.Cookie, expectedStatus string) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		var currentStatus string
		getJSON(t, router, http.MethodGet, "/api/v1/build-tasks/"+taskID, "", cookie, http.StatusOK, func(body map[string]any) {
			data := body["data"].(map[string]any)
			currentStatus = data["status"].(string)
		})
		if currentStatus == expectedStatus {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for build task %s status %s", taskID, expectedStatus)
}

func decodeJSONBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()

	var response map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return response
}

type fakeBuildExecutor struct{}

func (fakeBuildExecutor) Build(ctx context.Context, task buildtask.BuildTask, host buildhost.BuildHost, contextPath string, logPath string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if _, err := os.Stat(filepath.Join(contextPath, "Dockerfile")); err != nil {
		return err
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return err
	}
	defer logFile.Close()

	_, err = logFile.WriteString("fake local build completed for " + task.ImageRef + " on " + host.Name + "\n")
	return err
}

func (fakeBuildExecutor) Push(ctx context.Context, task buildtask.BuildTask, host buildhost.BuildHost, pushRegistry registry.Registry, secret *registry.RegistrySecret, logPath string) (buildtask.PushResult, error) {
	select {
	case <-ctx.Done():
		return buildtask.PushResult{}, ctx.Err()
	default:
	}

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return buildtask.PushResult{}, err
	}
	defer logFile.Close()

	_, err = logFile.WriteString("fake push completed for " + task.ImageRef + " to " + pushRegistry.Name + " on " + host.Name + "\n")
	if err != nil {
		return buildtask.PushResult{}, err
	}
	size := int64(42)
	return buildtask.PushResult{
		ImageID:   "sha256:fake-image-id",
		Digest:    "sha256:fake-digest",
		SizeBytes: &size,
	}, nil
}

func (fakeBuildExecutor) Cancel(taskID string) bool {
	return true
}
