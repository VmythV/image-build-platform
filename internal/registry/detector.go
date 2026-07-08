package registry

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

type Detector interface {
	Check(ctx context.Context, registry Registry, secret *RegistrySecret) CheckResult
}

type CommandDetector struct {
	Timeout       time.Duration
	DockerCommand string
}

func (d CommandDetector) Check(ctx context.Context, registry Registry, secret *RegistrySecret) CheckResult {
	timeout := d.Timeout
	if timeout <= 0 {
		timeout = 20 * time.Second
	}

	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if registry.Status == StatusDisabled {
		return failedCheck(StatusDisabled, "Registry is disabled.")
	}

	if secret != nil && secret.Username != "" && secret.Password != "" {
		return d.checkDockerLogin(checkCtx, registry, *secret)
	}

	return d.checkAnonymousEndpoint(checkCtx, registry)
}

func (d CommandDetector) checkDockerLogin(ctx context.Context, registry Registry, secret RegistrySecret) CheckResult {
	dockerCommand := d.DockerCommand
	if dockerCommand == "" {
		dockerCommand = "docker"
	}

	configDir, err := os.MkdirTemp("", "ibp-registry-check-*")
	if err != nil {
		return failedCheck(StatusUnavailable, "Failed to create temporary Docker config.")
	}
	defer func() {
		_ = os.RemoveAll(configDir)
	}()

	cmd := exec.CommandContext(ctx, dockerCommand, "login", registry.Endpoint, "--username", secret.Username, "--password-stdin")
	cmd.Env = append(os.Environ(), "DOCKER_CONFIG="+configDir)
	cmd.Stdin = strings.NewReader(secret.Password)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return failedCheck(StatusUnavailable, commandErrorMessage("Registry login failed.", err, string(output)))
	}

	logoutCmd := exec.CommandContext(ctx, dockerCommand, "logout", registry.Endpoint)
	logoutCmd.Env = append(os.Environ(), "DOCKER_CONFIG="+configDir)
	_ = logoutCmd.Run()

	return CheckResult{
		Status: StatusAvailable,
		Login:  CheckItem{Status: "success", Message: "Login succeeded."},
		Pull:   CheckItem{Status: "skipped", Message: "No test image provided."},
	}
}

func (d CommandDetector) checkAnonymousEndpoint(ctx context.Context, registry Registry) CheckResult {
	scheme := "https"
	if registry.InsecureHTTP {
		scheme = "http"
	}
	url := scheme + "://" + strings.TrimRight(registry.Endpoint, "/") + "/v2/"

	transport := http.DefaultTransport.(*http.Transport).Clone()
	if !registry.TLSVerify {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec
	}
	client := http.Client{Transport: transport, Timeout: 15 * time.Second}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return failedCheck(StatusUnavailable, "Registry endpoint is invalid.")
	}
	resp, err := client.Do(req)
	if err != nil {
		return failedCheck(StatusUnavailable, commandErrorMessage("Registry endpoint check failed.", err, ""))
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusUnauthorized {
		return CheckResult{
			Status: StatusAvailable,
			Login:  CheckItem{Status: "skipped", Message: "No credential configured."},
			Pull:   CheckItem{Status: "skipped", Message: "No test image provided."},
		}
	}

	return failedCheck(StatusUnavailable, fmt.Sprintf("Registry returned HTTP %d.", resp.StatusCode))
}

func failedCheck(status string, message string) CheckResult {
	errorMessage := message
	return CheckResult{
		Status: status,
		Login:  CheckItem{Status: "failed", Message: message},
		Pull:   CheckItem{Status: "skipped", Message: "No test image provided."},
		Error:  &errorMessage,
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
	output = redactDockerLoginOutput(output)
	if len(output) > 600 {
		output = output[:600]
	}
	return prefix + " " + output
}

func redactDockerLoginOutput(output string) string {
	output = strings.ReplaceAll(output, "\r", "\n")
	lines := strings.Split(output, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.Contains(strings.ToLower(line), "password") {
			continue
		}
		line = strings.TrimSpace(line)
		if line != "" {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, " ")
}
