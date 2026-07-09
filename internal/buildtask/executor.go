package buildtask

import (
	"archive/tar"
	"bytes"
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
	"github.com/VmythV/image-build-platform/internal/registry"
)

type Executor interface {
	Build(ctx context.Context, task BuildTask, host buildhost.BuildHost, contextPath string, logPath string, pullRegistry *registry.Registry, pullSecret *registry.RegistrySecret) error
	Push(ctx context.Context, task BuildTask, host buildhost.BuildHost, pushRegistry registry.Registry, secret *registry.RegistrySecret, logPath string) (PushResult, error)
	Repush(ctx context.Context, operationID string, sourceImageRef string, targetImageRef string, host buildhost.BuildHost, pushRegistry registry.Registry, secret *registry.RegistrySecret, logPath string) (PushResult, error)
	Cancel(taskID string) bool
}

type PushResult struct {
	ImageID   string
	Digest    string
	SizeBytes *int64
}

type LocalDockerExecutor struct {
	mu      sync.Mutex
	cancels map[string]context.CancelFunc
}

func NewLocalDockerExecutor() *LocalDockerExecutor {
	return &LocalDockerExecutor{cancels: map[string]context.CancelFunc{}}
}

func (e *LocalDockerExecutor) Build(ctx context.Context, task BuildTask, host buildhost.BuildHost, contextPath string, logPath string, pullRegistry *registry.Registry, pullSecret *registry.RegistrySecret) error {
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
		return e.buildLocal(buildCtx, task, host, contextPath, pullRegistry, pullSecret, logFile)
	case buildhost.ConnectionSSH:
		return e.buildSSH(buildCtx, task, host, contextPath, pullRegistry, pullSecret, logFile)
	default:
		return fmt.Errorf("unsupported build host connection type %q", host.ConnectionType)
	}
}

func (e *LocalDockerExecutor) buildLocal(ctx context.Context, task BuildTask, host buildhost.BuildHost, contextPath string, pullRegistry *registry.Registry, pullSecret *registry.RegistrySecret, logFile *os.File) error {
	dockerCommand := strings.TrimSpace(host.DockerCommand)
	if dockerCommand == "" {
		dockerCommand = buildhost.DefaultDockerCommand
	}
	env := dockerEnv(host)

	if pullRegistry != nil && pullSecret != nil {
		if err := runLocalDockerCommand(ctx, dockerCommand, []string{"login", pullRegistry.Endpoint, "--username", pullSecret.Username, "--password-stdin"}, env, strings.NewReader(pullSecret.Password), logFile); err != nil {
			return fmt.Errorf("docker login for pull registry failed: %w", err)
		}
		defer func() {
			_ = runLocalDockerCommand(context.Background(), dockerCommand, []string{"logout", pullRegistry.Endpoint}, env, nil, logFile)
		}()
	}

	buildArgs := dockerBuildArgs(task)
	_, _ = fmt.Fprintf(logFile, "\n[%s] docker %s\n", time.Now().UTC().Format(time.RFC3339), strings.Join(buildArgs, " "))
	cmd := exec.CommandContext(ctx, dockerCommand, buildArgs...)
	cmd.Dir = contextPath
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Env = env
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
	inspectCmd.Env = env
	if err := inspectCmd.Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("docker image inspect failed: %w", err)
	}

	_, _ = fmt.Fprintf(logFile, "\n[%s] build finished successfully\n", time.Now().UTC().Format(time.RFC3339))
	return nil
}

func (e *LocalDockerExecutor) buildSSH(ctx context.Context, task BuildTask, host buildhost.BuildHost, contextPath string, pullRegistry *registry.Registry, pullSecret *registry.RegistrySecret, logFile *os.File) error {
	sshHost, cleanup, err := buildhost.PrepareSSHIdentity(host)
	if err != nil {
		return err
	}
	defer cleanup()

	dockerCommand := strings.TrimSpace(host.DockerCommand)
	if dockerCommand == "" {
		dockerCommand = buildhost.DefaultDockerCommand
	}

	remoteDir := remoteBuildDir(task.ID)

	_, _ = fmt.Fprintf(logFile, "\n[%s] ssh upload build context to %s:%s\n", time.Now().UTC().Format(time.RFC3339), sshTarget(sshHost), remoteDir)
	if err := uploadSSHContext(ctx, sshHost, contextPath, remoteDir, logFile); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("upload SSH build context failed: %w", err)
	}
	defer cleanupSSHContext(sshHost, remoteDir, logFile)

	configDir := ""
	if pullRegistry != nil && pullSecret != nil {
		configDir = remoteBuildDir(task.ID) + "-docker-config"
		if err := runSSHCommand(ctx, sshHost, "rm -rf "+shellQuote(configDir)+" && mkdir -p "+shellQuote(configDir), logFile, nil); err != nil {
			return fmt.Errorf("prepare remote docker config: %w", err)
		}
		loginCommand := withDockerConfig(configDir, shellJoin([]string{dockerCommand, "login", pullRegistry.Endpoint, "--username", pullSecret.Username, "--password-stdin"}))
		if err := runSSHCommand(ctx, sshHost, loginCommand, logFile, strings.NewReader(pullSecret.Password)); err != nil {
			cleanupSSHContext(sshHost, configDir, logFile)
			return fmt.Errorf("remote docker login for pull registry failed: %w", err)
		}
		defer func() {
			_ = runSSHCommand(context.Background(), sshHost, withDockerConfig(configDir, shellJoin([]string{dockerCommand, "logout", pullRegistry.Endpoint})), logFile, nil)
			cleanupSSHContext(sshHost, configDir, logFile)
		}()
	}

	buildArgs := dockerBuildArgs(task)
	buildCommand := "cd " + shellQuote(remoteDir) + " && " + shellJoin(append([]string{dockerCommand}, buildArgs...))
	if configDir != "" {
		buildCommand = withDockerConfig(configDir, buildCommand)
	}
	if err := runSSHCommand(ctx, sshHost, buildCommand, logFile, nil); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("remote docker build failed: %w", err)
	}

	inspectArgs := []string{"image", "inspect", task.ImageRef}
	inspectCommand := shellJoin(append([]string{dockerCommand}, inspectArgs...))
	if err := runSSHCommand(ctx, sshHost, inspectCommand, logFile, nil); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("remote docker image inspect failed: %w", err)
	}

	_, _ = fmt.Fprintf(logFile, "\n[%s] remote build finished successfully\n", time.Now().UTC().Format(time.RFC3339))
	return nil
}

func (e *LocalDockerExecutor) Push(ctx context.Context, task BuildTask, host buildhost.BuildHost, pushRegistry registry.Registry, secret *registry.RegistrySecret, logPath string) (PushResult, error) {
	pushCtx, cancel := context.WithCancel(ctx)
	e.register(task.ID, cancel)
	defer e.unregister(task.ID)

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return PushResult{}, fmt.Errorf("open build log: %w", err)
	}
	defer logFile.Close()

	switch host.ConnectionType {
	case buildhost.ConnectionLocalDocker:
		return e.pushLocal(pushCtx, task, host, pushRegistry, secret, logFile)
	case buildhost.ConnectionSSH:
		return e.pushSSH(pushCtx, task, host, pushRegistry, secret, logFile)
	default:
		return PushResult{}, fmt.Errorf("unsupported build host connection type %q", host.ConnectionType)
	}
}

func (e *LocalDockerExecutor) Repush(ctx context.Context, operationID string, sourceImageRef string, targetImageRef string, host buildhost.BuildHost, pushRegistry registry.Registry, secret *registry.RegistrySecret, logPath string) (PushResult, error) {
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return PushResult{}, fmt.Errorf("open repush log: %w", err)
	}
	defer logFile.Close()

	switch host.ConnectionType {
	case buildhost.ConnectionLocalDocker:
		return e.repushLocal(ctx, operationID, sourceImageRef, targetImageRef, host, pushRegistry, secret, logFile)
	case buildhost.ConnectionSSH:
		return e.repushSSH(ctx, operationID, sourceImageRef, targetImageRef, host, pushRegistry, secret, logFile)
	default:
		return PushResult{}, fmt.Errorf("unsupported build host connection type %q", host.ConnectionType)
	}
}

func (e *LocalDockerExecutor) repushLocal(ctx context.Context, operationID string, sourceImageRef string, targetImageRef string, host buildhost.BuildHost, pushRegistry registry.Registry, secret *registry.RegistrySecret, logFile *os.File) (PushResult, error) {
	dockerCommand := strings.TrimSpace(host.DockerCommand)
	if dockerCommand == "" {
		dockerCommand = buildhost.DefaultDockerCommand
	}

	env := dockerEnv(host)
	if sourceImageRef != targetImageRef {
		if err := runLocalDockerCommand(ctx, dockerCommand, []string{"tag", sourceImageRef, targetImageRef}, env, nil, logFile); err != nil {
			if ctx.Err() != nil {
				return PushResult{}, ctx.Err()
			}
			return PushResult{}, fmt.Errorf("docker tag failed: %w", err)
		}
	}

	task := BuildTask{ID: operationID, ImageRef: targetImageRef}
	return e.pushLocal(ctx, task, host, pushRegistry, secret, logFile)
}

func (e *LocalDockerExecutor) repushSSH(ctx context.Context, operationID string, sourceImageRef string, targetImageRef string, host buildhost.BuildHost, pushRegistry registry.Registry, secret *registry.RegistrySecret, logFile *os.File) (PushResult, error) {
	sshHost, cleanup, err := buildhost.PrepareSSHIdentity(host)
	if err != nil {
		return PushResult{}, err
	}
	defer cleanup()

	dockerCommand := strings.TrimSpace(host.DockerCommand)
	if dockerCommand == "" {
		dockerCommand = buildhost.DefaultDockerCommand
	}
	if sourceImageRef != targetImageRef {
		tagCommand := shellJoin([]string{dockerCommand, "tag", sourceImageRef, targetImageRef})
		if err := runSSHCommand(ctx, sshHost, tagCommand, logFile, nil); err != nil {
			if ctx.Err() != nil {
				return PushResult{}, ctx.Err()
			}
			return PushResult{}, fmt.Errorf("remote docker tag failed: %w", err)
		}
	}

	task := BuildTask{ID: operationID, ImageRef: targetImageRef}
	return e.pushSSH(ctx, task, sshHost, pushRegistry, secret, logFile)
}

func (e *LocalDockerExecutor) pushLocal(ctx context.Context, task BuildTask, host buildhost.BuildHost, pushRegistry registry.Registry, secret *registry.RegistrySecret, logFile *os.File) (PushResult, error) {
	dockerCommand := strings.TrimSpace(host.DockerCommand)
	if dockerCommand == "" {
		dockerCommand = buildhost.DefaultDockerCommand
	}

	configDir, err := os.MkdirTemp("", "ibp-push-docker-config-*")
	if err != nil {
		return PushResult{}, fmt.Errorf("create Docker config: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(configDir)
	}()

	env := append(dockerEnv(host), "DOCKER_CONFIG="+configDir)
	if secret != nil && secret.Username != "" && secret.Password != "" {
		if err := runLocalDockerCommand(ctx, dockerCommand, []string{"login", pushRegistry.Endpoint, "--username", secret.Username, "--password-stdin"}, env, strings.NewReader(secret.Password), logFile); err != nil {
			if ctx.Err() != nil {
				return PushResult{}, ctx.Err()
			}
			return PushResult{}, fmt.Errorf("docker login failed: %w", err)
		}
		defer func() {
			_ = runLocalDockerCommand(context.Background(), dockerCommand, []string{"logout", pushRegistry.Endpoint}, env, nil, logFile)
		}()
	}

	if err := runLocalDockerCommand(ctx, dockerCommand, []string{"push", task.ImageRef}, env, nil, logFile); err != nil {
		if ctx.Err() != nil {
			return PushResult{}, ctx.Err()
		}
		return PushResult{}, fmt.Errorf("docker push failed: %w", err)
	}

	inspectArgs := []string{"image", "inspect", "--format", "{{.Id}}|{{if .RepoDigests}}{{index .RepoDigests 0}}{{end}}|{{.Size}}", task.ImageRef}
	output, err := runLocalDockerCommandOutput(ctx, dockerCommand, inspectArgs, env, nil, logFile)
	if err != nil {
		if ctx.Err() != nil {
			return PushResult{}, ctx.Err()
		}
		return PushResult{}, fmt.Errorf("docker image inspect after push failed: %w", err)
	}
	return parsePushInspectOutput(output), nil
}

func (e *LocalDockerExecutor) pushSSH(ctx context.Context, task BuildTask, host buildhost.BuildHost, pushRegistry registry.Registry, secret *registry.RegistrySecret, logFile *os.File) (PushResult, error) {
	sshHost, cleanup, err := buildhost.PrepareSSHIdentity(host)
	if err != nil {
		return PushResult{}, err
	}
	defer cleanup()

	dockerCommand := strings.TrimSpace(host.DockerCommand)
	if dockerCommand == "" {
		dockerCommand = buildhost.DefaultDockerCommand
	}

	configDir := remoteBuildDir(task.ID) + "-docker-config"
	if err := runSSHCommand(ctx, sshHost, "rm -rf "+shellQuote(configDir)+" && mkdir -p "+shellQuote(configDir), logFile, nil); err != nil {
		if ctx.Err() != nil {
			return PushResult{}, ctx.Err()
		}
		return PushResult{}, fmt.Errorf("prepare remote Docker config failed: %w", err)
	}
	defer cleanupSSHContext(sshHost, configDir, logFile)

	if secret != nil && secret.Username != "" && secret.Password != "" {
		loginCommand := withDockerConfig(configDir, shellJoin([]string{dockerCommand, "login", pushRegistry.Endpoint, "--username", secret.Username, "--password-stdin"}))
		if err := runSSHCommand(ctx, sshHost, loginCommand, logFile, strings.NewReader(secret.Password)); err != nil {
			if ctx.Err() != nil {
				return PushResult{}, ctx.Err()
			}
			return PushResult{}, fmt.Errorf("remote docker login failed: %w", err)
		}
		defer func() {
			_ = runSSHCommand(context.Background(), sshHost, withDockerConfig(configDir, shellJoin([]string{dockerCommand, "logout", pushRegistry.Endpoint})), logFile, nil)
		}()
	}

	pushCommand := withDockerConfig(configDir, shellJoin([]string{dockerCommand, "push", task.ImageRef}))
	if err := runSSHCommand(ctx, sshHost, pushCommand, logFile, nil); err != nil {
		if ctx.Err() != nil {
			return PushResult{}, ctx.Err()
		}
		return PushResult{}, fmt.Errorf("remote docker push failed: %w", err)
	}

	inspectCommand := shellJoin([]string{dockerCommand, "image", "inspect", "--format", "{{.Id}}|{{if .RepoDigests}}{{index .RepoDigests 0}}{{end}}|{{.Size}}", task.ImageRef})
	output, err := runSSHCommandWithOutput(ctx, sshHost, inspectCommand, logFile)
	if err != nil {
		if ctx.Err() != nil {
			return PushResult{}, ctx.Err()
		}
		return PushResult{}, fmt.Errorf("remote docker image inspect after push failed: %w", err)
	}
	return parsePushInspectOutput(output), nil
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

func runLocalDockerCommand(ctx context.Context, dockerCommand string, args []string, env []string, stdin io.Reader, logFile *os.File) error {
	_, _ = fmt.Fprintf(logFile, "\n[%s] docker %s\n", time.Now().UTC().Format(time.RFC3339), strings.Join(args, " "))
	cmd := exec.CommandContext(ctx, dockerCommand, args...)
	cmd.Env = env
	cmd.Stdin = stdin
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	return cmd.Run()
}

func runLocalDockerCommandOutput(ctx context.Context, dockerCommand string, args []string, env []string, stdin io.Reader, logFile *os.File) (string, error) {
	_, _ = fmt.Fprintf(logFile, "\n[%s] docker %s\n", time.Now().UTC().Format(time.RFC3339), strings.Join(args, " "))
	var output bytes.Buffer
	cmd := exec.CommandContext(ctx, dockerCommand, args...)
	cmd.Env = env
	cmd.Stdin = stdin
	cmd.Stdout = io.MultiWriter(logFile, &output)
	cmd.Stderr = logFile
	err := cmd.Run()
	return strings.TrimSpace(output.String()), err
}

func runSSHCommand(ctx context.Context, host buildhost.BuildHost, remoteCommand string, logFile *os.File, stdin io.Reader) error {
	_, _ = fmt.Fprintf(logFile, "\n[%s] ssh %s -- %s\n", time.Now().UTC().Format(time.RFC3339), sshTarget(host), remoteCommand)
	cmd := exec.CommandContext(ctx, "ssh", sshArgs(host, remoteCommand)...)
	cmd.Stdin = stdin
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	return cmd.Run()
}

func runSSHCommandWithOutput(ctx context.Context, host buildhost.BuildHost, remoteCommand string, logFile *os.File) (string, error) {
	_, _ = fmt.Fprintf(logFile, "\n[%s] ssh %s -- %s\n", time.Now().UTC().Format(time.RFC3339), sshTarget(host), remoteCommand)
	var output bytes.Buffer
	cmd := exec.CommandContext(ctx, "ssh", sshArgs(host, remoteCommand)...)
	cmd.Stdout = io.MultiWriter(logFile, &output)
	cmd.Stderr = logFile
	err := cmd.Run()
	return strings.TrimSpace(output.String()), err
}

func withDockerConfig(configDir string, command string) string {
	return "DOCKER_CONFIG=" + shellQuote(configDir) + " " + command
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
	args := []string{
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=10",
		"-o", "StrictHostKeyChecking=accept-new",
		"-p", strconv.Itoa(port),
		sshTarget(host),
		"sh -lc " + shellQuote(remoteCommand),
	}
	if host.SSHIdentityFile != "" {
		args = append([]string{"-i", host.SSHIdentityFile, "-o", "IdentitiesOnly=yes"}, args...)
	}
	return args
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

func parsePushInspectOutput(output string) PushResult {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) == 0 {
		return PushResult{}
	}
	parts := strings.Split(strings.TrimSpace(lines[len(lines)-1]), "|")
	result := PushResult{}
	if len(parts) > 0 {
		result.ImageID = strings.TrimSpace(parts[0])
	}
	if len(parts) > 1 {
		repoDigest := strings.TrimSpace(parts[1])
		if _, digest, ok := strings.Cut(repoDigest, "@"); ok {
			result.Digest = strings.TrimSpace(digest)
		} else {
			result.Digest = repoDigest
		}
	}
	if len(parts) > 2 {
		if parsed, err := strconv.ParseInt(strings.TrimSpace(parts[2]), 10, 64); err == nil {
			result.SizeBytes = &parsed
		}
	}
	return result
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
	if network := strings.TrimSpace(task.BuildOptions["network"]); network != "" {
		args = append(args, "--network", network)
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
