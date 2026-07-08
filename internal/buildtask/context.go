package buildtask

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type PreparedContext struct {
	ContextRef string
	LogPath    string
}

type contextMetadata struct {
	TaskID         string `json:"taskId"`
	DockerfileHash string `json:"dockerfileHash"`
	Source         string `json:"source"`
	CreatedAt      string `json:"createdAt"`
}

func (s Service) prepareBuildContext(task BuildTask) (PreparedContext, error) {
	if err := validateDockerfileSnapshot(task.DockerfileSnapshot); err != nil {
		return PreparedContext{}, err
	}

	contextPath := filepath.Join(s.contextDir, task.ID)
	if err := os.MkdirAll(contextPath, 0o750); err != nil {
		return PreparedContext{}, fmt.Errorf("create build context: %w", err)
	}
	if err := os.WriteFile(filepath.Join(contextPath, "Dockerfile"), []byte(task.DockerfileSnapshot), 0o640); err != nil {
		return PreparedContext{}, fmt.Errorf("write Dockerfile snapshot: %w", err)
	}

	metadata := contextMetadata{
		TaskID:         task.ID,
		DockerfileHash: task.DockerfileHash,
		Source:         "inline",
		CreatedAt:      time.Now().UTC().Format(time.RFC3339),
	}
	metadataBytes, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return PreparedContext{}, fmt.Errorf("encode context metadata: %w", err)
	}
	if err := os.WriteFile(filepath.Join(contextPath, "metadata.json"), metadataBytes, 0o640); err != nil {
		return PreparedContext{}, fmt.Errorf("write context metadata: %w", err)
	}

	logDir := filepath.Join(s.logDir, "builds")
	if err := os.MkdirAll(logDir, 0o750); err != nil {
		return PreparedContext{}, fmt.Errorf("create build log directory: %w", err)
	}
	logPath := filepath.Join(logDir, task.ID+".log")
	if err := os.WriteFile(logPath, []byte("build log initialized\n"), 0o640); err != nil {
		return PreparedContext{}, fmt.Errorf("initialize build log: %w", err)
	}

	return PreparedContext{ContextRef: contextPath, LogPath: logPath}, nil
}

func validateDockerfileSnapshot(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return validationError("Dockerfile snapshot is required")
	}
	for _, line := range strings.Split(value, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(strings.ToUpper(line), "FROM ") {
			return nil
		}
	}
	return validationError("Dockerfile snapshot must contain a FROM instruction")
}
