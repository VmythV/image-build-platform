package buildhost

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type Detector interface {
	Check(ctx context.Context, host BuildHost) CheckResult
}

type CommandDetector struct {
	Timeout time.Duration
}

func (d CommandDetector) Check(ctx context.Context, host BuildHost) CheckResult {
	timeout := d.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}

	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	switch host.ConnectionType {
	case ConnectionLocalDocker:
		return d.checkLocalDocker(checkCtx, host)
	case ConnectionSSH:
		return d.checkSSH(checkCtx, host)
	default:
		return failedCheck(StatusUnavailable, "connection_type", fmt.Sprintf("Unsupported connection type %q.", host.ConnectionType))
	}
}

func (d CommandDetector) checkLocalDocker(ctx context.Context, host BuildHost) CheckResult {
	dockerCommand := valueOrDefault(host.DockerCommand, DefaultDockerCommand)
	output, err := runLocalDocker(ctx, host, dockerCommand, "version", "--format", "{{.Server.Os}}|{{.Server.Arch}}|{{.Server.Version}}")
	if err != nil {
		return failedCheck(StatusUnavailable, "docker", commandErrorMessage("Docker is not available.", err, output))
	}

	osName, arch, version := parseDockerVersionOutput(output)
	buildkitSupported := localBuildxAvailable(ctx, host, dockerCommand)

	return CheckResult{
		Status:            StatusOnline,
		Architecture:      normalizeArch(arch),
		OS:                normalizeOS(valueOrDefault(osName, runtime.GOOS)),
		DockerVersion:     version,
		BuildkitSupported: buildkitSupported,
		Checks: []CheckItem{
			{Name: "docker", Status: "success", Message: "Docker is available."},
			{Name: "architecture", Status: "success", Message: "Architecture detected."},
		},
	}
}

func (d CommandDetector) checkSSH(ctx context.Context, host BuildHost) CheckResult {
	if strings.TrimSpace(host.Address) == "" || strings.TrimSpace(host.Username) == "" {
		return failedCheck(StatusUnavailable, "ssh", "SSH address and username are required.")
	}

	dockerCommand := valueOrDefault(host.DockerCommand, DefaultDockerCommand)
	remoteCommand := fmt.Sprintf(
		`OS=$(uname -s); ARCH=$(uname -m); VERSION=$(%[1]s version --format '{{.Server.Os}}|{{.Server.Arch}}|{{.Server.Version}}'); BUILDKIT=false; if %[1]s buildx version >/dev/null 2>&1; then BUILDKIT=true; fi; printf 'host_os=%%s\nhost_arch=%%s\ndocker=%%s\nbuildkit=%%s\n' "$OS" "$ARCH" "$VERSION" "$BUILDKIT"`,
		dockerCommand,
	)
	output, err := runSSH(ctx, host, remoteCommand)
	if err != nil {
		return failedCheck(StatusOffline, "ssh", commandErrorMessage("SSH or remote Docker check failed.", err, output))
	}

	values := parseKeyValueOutput(output)
	osName, arch, version := parseDockerVersionOutput(values["docker"])
	if osName == "" {
		osName = values["host_os"]
	}
	if arch == "" {
		arch = values["host_arch"]
	}

	return CheckResult{
		Status:            StatusOnline,
		Architecture:      normalizeArch(arch),
		OS:                normalizeOS(osName),
		DockerVersion:     version,
		BuildkitSupported: values["buildkit"] == "true",
		Checks: []CheckItem{
			{Name: "ssh", Status: "success", Message: "SSH connection is available."},
			{Name: "docker", Status: "success", Message: "Remote Docker is available."},
		},
	}
}

func runLocalDocker(ctx context.Context, host BuildHost, command string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Env = dockerEnv(host)
	output, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

func localBuildxAvailable(ctx context.Context, host BuildHost, dockerCommand string) bool {
	_, err := runLocalDocker(ctx, host, dockerCommand, "buildx", "version")
	return err == nil
}

func dockerEnv(host BuildHost) []string {
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

func runSSH(ctx context.Context, host BuildHost, remoteCommand string) (string, error) {
	runtimeHost, cleanup, err := PrepareSSHIdentity(host)
	if err != nil {
		return "", err
	}
	defer cleanup()

	port := host.Port
	if port == 0 {
		port = 22
	}
	target := runtimeHost.Username + "@" + runtimeHost.Address
	args := []string{
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=10",
		"-o", "StrictHostKeyChecking=accept-new",
		"-p", strconv.Itoa(port),
		target,
		"sh -lc " + shellQuote(remoteCommand),
	}
	if runtimeHost.SSHIdentityFile != "" {
		args = append([]string{"-i", runtimeHost.SSHIdentityFile, "-o", "IdentitiesOnly=yes"}, args...)
	}

	cmd := exec.CommandContext(ctx, "ssh", args...)
	output, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

func failedCheck(status string, name string, message string) CheckResult {
	errorMessage := message
	return CheckResult{
		Status: status,
		Checks: []CheckItem{
			{Name: name, Status: "failed", Message: message},
		},
		Error: &errorMessage,
	}
}

func commandErrorMessage(prefix string, err error, output string) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return prefix + " Operation timed out."
	}
	output = strings.TrimSpace(output)
	if output == "" {
		return prefix + " " + err.Error()
	}
	if len(output) > 600 {
		output = output[:600]
	}
	return prefix + " " + output
}

func parseDockerVersionOutput(output string) (string, string, string) {
	parts := strings.Split(strings.TrimSpace(output), "|")
	if len(parts) != 3 {
		return "", "", ""
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), strings.TrimSpace(parts[2])
}

func parseKeyValueOutput(output string) map[string]string {
	values := make(map[string]string)
	for _, line := range strings.Split(output, "\n") {
		key, value, ok := strings.Cut(line, "=")
		if ok {
			values[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}
	return values
}

func normalizeArch(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "x86_64", "amd64":
		return "amd64"
	case "aarch64", "arm64":
		return "arm64"
	case "armv7l", "armv7":
		return "arm/v7"
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func normalizeOS(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func valueOrDefault(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}
