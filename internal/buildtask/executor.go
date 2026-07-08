package buildtask

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
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

	switch host.ConnectionType {
	case buildhost.ConnectionLocalDocker:
		return e.buildLocal(buildCtx, task, host, contextPath, logFile)
	case buildhost.ConnectionSSH:
		return e.buildSSH(buildCtx, task, host, contextPath, logFile)
	default:
		return fmt.Errorf("unsupported build host connection type %q", host.ConnectionType)
	}
}

func (e *LocalDockerExecutor) buildLocal(ctx context.Context, task BuildTask, host buildhost.BuildHost, contextPath string, logFile *os.File) error {
	dockerCommand := strings.TrimSpace(host.DockerCommand)
	if dockerCommand == "" {
		dockerCommand = buildhost.DefaultDockerCommand
	}

	buildArgs := dockerBuildArgs(task)
	_, _ = fmt.Fprintf(logFile, "\n[%s] docker %s\n", time.Now().UTC().Format(time.RFC3339), strings.Join(buildArgs, " "))
	cmd := exec.CommandContext(ctx, dockerCommand, buildArgs...)
	cmd.Dir = contextPath
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Env = dockerEnv(host)
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("docker build failed: %w", err)
	}

	inspectArgs := []string{"image", "inspect", task.ImageRef}
	_, _ = fmt.Fprintf(logFile, "\n[%s] docker %s\n", time.Now().UTC().Format(time.RFC3339), strings.Join(inspectArgs, " "))
	inspectCmd := exec.CommandContext(ctx, dockerCommand, inspectArgs...)
	inspectCmd.Stdout = logFile
	inspectCmd.Stderr = logFile
	inspectCmd.Env = dockerEnv(host)
	if err := inspectCmd.Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("docker image inspect failed: %w", err)
	}

	_, _ = fmt.Fprintf(logFile, "\n[%s] build finished successfully\n", time.Now().UTC().Format(time.RFC3339))
	return nil
}

func (e *LocalDockerExecutor) buildSSH(ctx context.Context, task BuildTask, host buildhost.BuildHost, contextPath string, logFile *os.File) error {
	dockerCommand := strings.TrimSpace(host.DockerCommand)
	if dockerCommand == "" {
		dockerCommand = buildhost.DefaultDockerCommand
	}

	remoteDir := remoteBuildDir(task.ID)

	_, _ = fmt.Fprintf(logFile, "\n[%s] ssh upload build context to %s:%s\n", time.Now().UTC().Format(time.RFC3339), sshTarget(host), remoteDir)
	if err := uploadSSHContext(ctx, host, contextPath, remoteDir, logFile); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("upload SSH build context failed: %w", err)
	}
	defer cleanupSSHContext(host, remoteDir, logFile)

	buildArgs := dockerBuildArgs(task)
	buildCommand := "cd " + shellQuote(remoteDir) + " && " + shellJoin(append([]string{dockerCommand}, buildArgs...))
	_, _ = fmt.Fprintf(logFile, "\n[%s] ssh %s -- %s\n", time.Now().UTC().Format(time.RFC3339), sshTarget(host), buildCommand)
	if err := runSSHCommand(ctx, host, buildCommand, logFile, nil); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("remote docker build failed: %w", err)
	}

	inspectArgs := []string{"image", "inspect", task.ImageRef}
	inspectCommand := shellJoin(append([]string{dockerCommand}, inspectArgs...))
	_, _ = fmt.Fprintf(logFile, "\n[%s] ssh %s -- %s\n", time.Now().UTC().Format(time.RFC3339), sshTarget(host), inspectCommand)
	if err := runSSHCommand(ctx, host, inspectCommand, logFile, nil); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("remote docker image inspect failed: %w", err)
	}

	_, _ = fmt.Fprintf(logFile, "\n[%s] remote build finished successfully\n", time.Now().UTC().Format(time.RFC3339))
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

func uploadSSHContext(ctx context.Context, host buildhost.BuildHost, contextPath string, remoteDir string, logFile *os.File) error {
	reader, writer := io.Pipe()
	errCh := make(chan error, 1)
	go func() {
		err := writeTarContext(contextPath, writer)
		_ = writer.CloseWithError(err)
		errCh <- err
	}()

	remoteCommand := "rm -rf " + shellQuote(remoteDir) + " && mkdir -p " + shellQuote(remoteDir) + " && tar -C " + shellQuote(remoteDir) + " -xf -"
	cmdErr := runSSHCommand(ctx, host, remoteCommand, logFile, reader)
	_ = reader.Close()
	tarErr := <-errCh
	if tarErr != nil && cmdErr == nil {
		return fmt.Errorf("archive build context: %w", tarErr)
	}
	if cmdErr != nil {
		return cmdErr
	}
	return nil
}

func writeTarContext(root string, writer io.Writer) error {
	tarWriter := tar.NewWriter(writer)
	defer tarWriter.Close()

	return filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}

		link := ""
		if info.Mode()&os.ModeSymlink != 0 {
			link, err = os.Readlink(path)
			if err != nil {
				return err
			}
		}

		header, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return err
		}
		relativePath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relativePath)
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(tarWriter, file)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
}

func runSSHCommand(ctx context.Context, host buildhost.BuildHost, remoteCommand string, logFile *os.File, stdin io.Reader) error {
	cmd := exec.CommandContext(ctx, "ssh", sshArgs(host, remoteCommand)...)
	cmd.Stdin = stdin
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	return cmd.Run()
}

func cleanupSSHContext(host buildhost.BuildHost, remoteDir string, logFile *os.File) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, _ = fmt.Fprintf(logFile, "\n[%s] ssh cleanup %s:%s\n", time.Now().UTC().Format(time.RFC3339), sshTarget(host), remoteDir)
	if err := runSSHCommand(ctx, host, "rm -rf "+shellQuote(remoteDir), logFile, nil); err != nil {
		_, _ = fmt.Fprintf(logFile, "[%s] remote cleanup failed: %v\n", time.Now().UTC().Format(time.RFC3339), err)
	}
}

func sshArgs(host buildhost.BuildHost, remoteCommand string) []string {
	port := host.Port
	if port == 0 {
		port = 22
	}
	return []string{
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=10",
		"-o", "StrictHostKeyChecking=accept-new",
		"-p", strconv.Itoa(port),
		sshTarget(host),
		"sh -lc " + shellQuote(remoteCommand),
	}
}

func sshTarget(host buildhost.BuildHost) string {
	return host.Username + "@" + host.Address
}

func remoteBuildDir(taskID string) string {
	token := strings.Map(func(value rune) rune {
		switch {
		case value >= 'a' && value <= 'z':
			return value
		case value >= 'A' && value <= 'Z':
			return value
		case value >= '0' && value <= '9':
			return value
		case value == '-' || value == '_' || value == '.':
			return value
		default:
			return '-'
		}
	}, strings.TrimSpace(taskID))
	if token == "" {
		token = "unknown"
	}
	return "/tmp/ibp-build-" + token
}

func shellJoin(args []string) string {
	parts := make([]string, 0, len(args))
	for _, arg := range args {
		parts = append(parts, shellQuote(arg))
	}
	return strings.Join(parts, " ")
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

func dockerEnv(host buildhost.BuildHost) []string {
	env := os.Environ()
	endpoint := strings.TrimSpace(host.DockerEndpoint)
	if endpoint == "" {
		return env
	}
	if strings.HasPrefix(endpoint, "/") {
		endpoint = "unix://" + endpoint
	}
	return append(env, "DOCKER_HOST="+endpoint)
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
