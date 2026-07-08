package buildhost

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/VmythV/image-build-platform/internal/platform/clock"
	"github.com/VmythV/image-build-platform/internal/platform/id"
)

var (
	ErrValidation     = errors.New("build host validation failed")
	ErrInvalidCommand = errors.New("docker command contains unsupported characters")
)

type Service struct {
	repo     Repository
	detector Detector
}

func NewService(repo Repository, detector Detector) Service {
	if detector == nil {
		detector = CommandDetector{}
	}
	return Service{repo: repo, detector: detector}
}

func (s Service) EnsureDefaultLocalHost(ctx context.Context) error {
	count, err := s.repo.CountLocalHosts(ctx)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	now := clock.Now()
	return s.repo.Create(ctx, BuildHost{
		ID:             id.New(),
		Name:           "Local Docker",
		ConnectionType: ConnectionLocalDocker,
		DockerEndpoint: DefaultDockerEndpoint,
		DockerCommand:  DefaultDockerCommand,
		Labels:         []string{"local"},
		MaxConcurrency: 1,
		Status:         StatusUnknown,
		CreatedAt:      now,
		UpdatedAt:      now,
	})
}

func (s Service) List(ctx context.Context, filter ListFilter) ([]BuildHost, int, error) {
	return s.repo.List(ctx, filter)
}

func (s Service) Get(ctx context.Context, hostID string) (BuildHost, error) {
	return s.repo.FindByID(ctx, hostID)
}

func (s Service) Create(ctx context.Context, input SaveInput, actorID string) (BuildHost, error) {
	host, err := normalizeInput(input)
	if err != nil {
		return BuildHost{}, err
	}

	now := clock.Now()
	host.ID = id.New()
	host.CreatedBy = actorID
	host.CreatedAt = now
	host.UpdatedAt = now
	host.Status = StatusUnknown

	if err := s.repo.Create(ctx, host); err != nil {
		return BuildHost{}, err
	}
	return host, nil
}

func (s Service) Update(ctx context.Context, hostID string, input SaveInput) (BuildHost, error) {
	existing, err := s.repo.FindByID(ctx, hostID)
	if err != nil {
		return BuildHost{}, err
	}

	updated, err := normalizeInput(input)
	if err != nil {
		return BuildHost{}, err
	}

	updated.ID = existing.ID
	updated.CreatedBy = existing.CreatedBy
	updated.CreatedAt = existing.CreatedAt
	updated.UpdatedAt = clock.Now()
	updated.Status = StatusUnknown
	if existing.Status == StatusDisabled {
		updated.Status = StatusDisabled
	}

	if err := s.repo.Update(ctx, updated); err != nil {
		return BuildHost{}, err
	}
	return s.repo.FindByID(ctx, hostID)
}

func (s Service) Delete(ctx context.Context, hostID string) error {
	return s.repo.Delete(ctx, hostID, clock.Now())
}

func (s Service) Enable(ctx context.Context, hostID string) (BuildHost, error) {
	if err := s.repo.SetStatus(ctx, hostID, StatusUnknown, clock.Now()); err != nil {
		return BuildHost{}, err
	}
	return s.repo.FindByID(ctx, hostID)
}

func (s Service) Disable(ctx context.Context, hostID string) (BuildHost, error) {
	if err := s.repo.SetStatus(ctx, hostID, StatusDisabled, clock.Now()); err != nil {
		return BuildHost{}, err
	}
	return s.repo.FindByID(ctx, hostID)
}

func (s Service) Check(ctx context.Context, hostID string) (BuildHost, CheckResult, error) {
	host, err := s.repo.FindByID(ctx, hostID)
	if err != nil {
		return BuildHost{}, CheckResult{}, err
	}
	if host.Status == StatusDisabled {
		result := failedCheck(StatusDisabled, "status", "Build host is disabled.")
		return host, result, nil
	}

	result := s.detector.Check(ctx, host)
	checkedAt := clock.Now()
	rawResult, err := json.Marshal(result)
	if err != nil {
		return BuildHost{}, CheckResult{}, fmt.Errorf("encode check result: %w", err)
	}
	if err := s.repo.UpdateCheckResult(ctx, hostID, result, checkedAt, string(rawResult)); err != nil {
		return BuildHost{}, CheckResult{}, err
	}

	updated, err := s.repo.FindByID(ctx, hostID)
	if err != nil {
		return BuildHost{}, CheckResult{}, err
	}
	return updated, result, nil
}

func normalizeInput(input SaveInput) (BuildHost, error) {
	host := BuildHost{
		Name:           strings.TrimSpace(input.Name),
		ConnectionType: strings.TrimSpace(input.ConnectionType),
		Address:        strings.TrimSpace(input.Address),
		Port:           input.Port,
		Username:       strings.TrimSpace(input.Username),
		DockerEndpoint: strings.TrimSpace(input.DockerEndpoint),
		DockerCommand:  strings.TrimSpace(input.DockerCommand),
		MaxConcurrency: input.MaxConcurrency,
		Labels:         normalizeLabels(input.Labels),
	}

	if host.Name == "" {
		return BuildHost{}, validationError("name is required")
	}
	if host.ConnectionType == "" {
		host.ConnectionType = ConnectionLocalDocker
	}
	if host.MaxConcurrency == 0 {
		host.MaxConcurrency = 1
	}
	if host.MaxConcurrency < 1 {
		return BuildHost{}, validationError("maxConcurrency must be at least 1")
	}
	if host.DockerCommand == "" {
		host.DockerCommand = DefaultDockerCommand
	}
	if !isSafeCommand(host.DockerCommand) {
		return BuildHost{}, ErrInvalidCommand
	}

	switch host.ConnectionType {
	case ConnectionLocalDocker:
		if host.DockerEndpoint == "" {
			host.DockerEndpoint = DefaultDockerEndpoint
		}
		host.Address = ""
		host.Port = 0
		host.Username = ""
	case ConnectionSSH:
		if host.Address == "" {
			return BuildHost{}, validationError("address is required for SSH hosts")
		}
		if host.Username == "" {
			return BuildHost{}, validationError("username is required for SSH hosts")
		}
		if host.Port == 0 {
			host.Port = 22
		}
		if host.Port < 1 || host.Port > 65535 {
			return BuildHost{}, validationError("port must be between 1 and 65535")
		}
		host.DockerEndpoint = ""
	default:
		return BuildHost{}, validationError("connectionType must be local_docker or ssh")
	}

	return host, nil
}

func validationError(message string) error {
	return fmt.Errorf("%w: %s", ErrValidation, message)
}

func normalizeLabels(labels []string) []string {
	seen := make(map[string]struct{}, len(labels))
	normalized := make([]string, 0, len(labels))
	for _, label := range labels {
		label = strings.TrimSpace(label)
		if label == "" {
			continue
		}
		if _, ok := seen[label]; ok {
			continue
		}
		seen[label] = struct{}{}
		normalized = append(normalized, label)
	}
	return normalized
}

var safeCommandPattern = regexp.MustCompile(`^[A-Za-z0-9_./-]+$`)

func isSafeCommand(value string) bool {
	return safeCommandPattern.MatchString(value)
}
