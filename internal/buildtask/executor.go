package buildtask

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/VmythV/image-build-platform/internal/buildhost"
)

type Executor interface {
	Build(ctx context.Context, task BuildTask, host buildhost.BuildHost, contextPath string, logPath string) error
	Cancel(taskID string) bool
}

type LocalDockerExecutor struct {
	mu      sync.Mutex
	cancels map[string]context.CancelFunc
}

func NewLocalDockerExecutor() *LocalDockerExecutor {
	return &LocalDockerExecutor{cancels: map[string]context.CancelFunc{}}
}

func (e *LocalDockerExecutor) Build(ctx context.Context, task BuildTask, host buildhost.BuildHost, contextPath string, logPath string) error {
	buildCtx, cancel := context.WithCancel(ctx)
	e.register(task.ID, cancel)
	defer e.unregister(task.ID)

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return fmt.Errorf("open build log: %w", err)
	}
	defer logFile.Close()

	dockerCommand := strings.TrimSpace(host.DockerCommand)
	if dockerCommand == "" {
		dockerCommand = buildhost.DefaultDockerCommand
	}

	buildArgs := dockerBuildArgs(task)
	_, _ = fmt.Fprintf(logFile, "\n[%s] docker %s\n", time.Now().UTC().Format(time.RFC3339), strings.Join(buildArgs, " "))
	cmd := exec.CommandContext(buildCtx, dockerCommand, buildArgs...)
	cmd.Dir = contextPath
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Env = os.Environ()
	if err := cmd.Run(); err != nil {
		if buildCtx.Err() != nil {
			return buildCtx.Err()
		}
		return fmt.Errorf("docker build failed: %w", err)
	}

	inspectArgs := []string{"image", "inspect", task.ImageRef}
	_, _ = fmt.Fprintf(logFile, "\n[%s] docker %s\n", time.Now().UTC().Format(time.RFC3339), strings.Join(inspectArgs, " "))
	inspectCmd := exec.CommandContext(buildCtx, dockerCommand, inspectArgs...)
	inspectCmd.Stdout = logFile
	inspectCmd.Stderr = logFile
	inspectCmd.Env = os.Environ()
	if err := inspectCmd.Run(); err != nil {
		if buildCtx.Err() != nil {
			return buildCtx.Err()
		}
		return fmt.Errorf("docker image inspect failed: %w", err)
	}

	_, _ = fmt.Fprintf(logFile, "\n[%s] build finished successfully\n", time.Now().UTC().Format(time.RFC3339))
	return nil
}

func (e *LocalDockerExecutor) Cancel(taskID string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	cancel, ok := e.cancels[taskID]
	if !ok {
		return false
	}
	cancel()
	return true
}

func (e *LocalDockerExecutor) register(taskID string, cancel context.CancelFunc) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.cancels == nil {
		e.cancels = map[string]context.CancelFunc{}
	}
	e.cancels[taskID] = cancel
}

func (e *LocalDockerExecutor) unregister(taskID string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.cancels, taskID)
}

func dockerBuildArgs(task BuildTask) []string {
	args := []string{"build"}
	if platform := dockerPlatform(task.Architecture); platform != "" {
		args = append(args, "--platform", platform)
	}
	if strings.EqualFold(task.BuildOptions["pull"], "true") {
		args = append(args, "--pull")
	}
	if strings.EqualFold(task.BuildOptions["noCache"], "true") || strings.EqualFold(task.BuildOptions["no-cache"], "true") {
		args = append(args, "--no-cache")
	}
	if target := strings.TrimSpace(task.BuildOptions["target"]); target != "" {
		args = append(args, "--target", target)
	}

	for _, key := range sortedMapKeys(task.BuildArgs) {
		args = append(args, "--build-arg", key+"="+task.BuildArgs[key])
	}

	args = append(args, "-t", task.ImageRef, "-f", "Dockerfile", ".")
	return args
}

func dockerPlatform(architecture string) string {
	architecture = strings.TrimSpace(architecture)
	if architecture == "" {
		return ""
	}
	if strings.Contains(architecture, "/") {
		return architecture
	}
	return "linux/" + architecture
}

func sortedMapKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
