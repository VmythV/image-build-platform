package buildhost

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/VmythV/image-build-platform/internal/credential"
	"github.com/VmythV/image-build-platform/internal/platform/clock"
	"github.com/VmythV/image-build-platform/internal/platform/id"
)

var (
	ErrValidation     = errors.New("build host validation failed")
	ErrInvalidCommand = errors.New("docker command contains unsupported characters")
)

type Service struct {
	repo        Repository
	detector    Detector
	credentials CredentialStore
	encryptor   credential.Encryptor
}

func NewService(repo Repository, detector Detector) Service {
	return NewServiceWithOptions(ServiceOptions{
		Repository: repo,
		Detector:   detector,
	})
}

type ServiceOptions struct {
	Repository  Repository
	Detector    Detector
	Credentials CredentialStore
	Encryptor   credential.Encryptor
}

type CredentialStore interface {
	Create(ctx context.Context, credential credential.Credential) error
	UpdateValue(ctx context.Context, credential credential.Credential) error
	FindByID(ctx context.Context, id string) (credential.Credential, error)
}

func NewServiceWithOptions(opts ServiceOptions) Service {
	detector := opts.Detector
	if detector == nil {
		detector = CommandDetector{}
	}
	return Service{
		repo:        opts.Repository,
		detector:    detector,
		credentials: opts.Credentials,
		encryptor:   opts.Encryptor,
	}
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
	host, sshCredential, err := normalizeInput(input)
	if err != nil {
		return BuildHost{}, err
	}

	now := clock.Now()
	host.ID = id.New()
	host.CreatedBy = actorID
	host.CreatedAt = now
	host.UpdatedAt = now
	host.Status = StatusUnknown

	if sshCredential != nil {
		credentialID, err := s.createSSHCredential(ctx, host, *sshCredential, actorID, now)
		if err != nil {
			return BuildHost{}, err
		}
		host.CredentialID = credentialID
	}

	if err := s.repo.Create(ctx, host); err != nil {
		return BuildHost{}, err
	}
	return s.repo.FindByID(ctx, host.ID)
}

func (s Service) Update(ctx context.Context, hostID string, input SaveInput) (BuildHost, error) {
	existing, err := s.repo.FindByID(ctx, hostID)
	if err != nil {
		return BuildHost{}, err
	}

	updated, sshCredential, err := normalizeInput(input)
	if err != nil {
		return BuildHost{}, err
	}

	updated.ID = existing.ID
	updated.CredentialID = existing.CredentialID
	updated.CreatedBy = existing.CreatedBy
	updated.CreatedAt = existing.CreatedAt
	updated.UpdatedAt = clock.Now()
	updated.Status = StatusUnknown
	if existing.Status == StatusDisabled {
		updated.Status = StatusDisabled
	}
	if updated.ConnectionType == ConnectionLocalDocker {
		updated.CredentialID = ""
	}
	if sshCredential != nil {
		if existing.CredentialID == "" || updated.CredentialID == "" {
			credentialID, err := s.createSSHCredential(ctx, updated, *sshCredential, existing.CreatedBy, updated.UpdatedAt)
			if err != nil {
				return BuildHost{}, err
			}
			updated.CredentialID = credentialID
		} else if err := s.updateSSHCredential(ctx, existing.CredentialID, updated, *sshCredential, updated.UpdatedAt); err != nil {
			return BuildHost{}, err
		}
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

	runtimeHost, err := s.withSSHCredential(ctx, host)
	if err != nil {
		return BuildHost{}, CheckResult{}, err
	}
	result := s.detector.Check(ctx, runtimeHost)
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

func (s Service) SSHCredential(ctx context.Context, host BuildHost) (*SSHCredential, error) {
	return s.sshCredential(ctx, host)
}

func (s Service) withSSHCredential(ctx context.Context, host BuildHost) (BuildHost, error) {
	sshCredential, err := s.sshCredential(ctx, host)
	if err != nil {
		return BuildHost{}, err
	}
	host.SSHCredential = sshCredential
	return host, nil
}

func (s Service) sshCredential(ctx context.Context, host BuildHost) (*SSHCredential, error) {
	if host.ConnectionType != ConnectionSSH || host.CredentialID == "" {
		return nil, nil
	}
	if s.credentials == nil {
		return nil, validationError("SSH credential store is not configured")
	}
	stored, err := s.credentials.FindByID(ctx, host.CredentialID)
	if err != nil {
		return nil, err
	}
	if stored.Type != credential.TypeSSHPrivateKey {
		return nil, validationError("credential is not an SSH private key")
	}
	plaintext, err := s.encryptor.Decrypt(stored.EncryptedValue)
	if err != nil {
		return nil, err
	}

	var sshCredential SSHCredential
	if err := json.Unmarshal(plaintext, &sshCredential); err != nil {
		return nil, fmt.Errorf("decode SSH credential: %w", err)
	}
	return &sshCredential, nil
}

func (s Service) createSSHCredential(ctx context.Context, host BuildHost, sshCredential SSHCredential, actorID string, nowTime time.Time) (string, error) {
	encrypted, err := s.encryptSSHCredential(sshCredential)
	if err != nil {
		return "", err
	}
	credentialID := id.New()
	err = s.credentials.Create(ctx, credential.Credential{
		ID:                credentialID,
		Type:              credential.TypeSSHPrivateKey,
		Name:              host.Username,
		EncryptedValue:    encrypted,
		EncryptionVersion: credential.EncryptionVersion,
		Fingerprint:       credential.Fingerprint(host.Name, host.Address, host.Username, sshCredential.PrivateKey),
		CreatedBy:         actorID,
		CreatedAt:         nowTime,
		UpdatedAt:         nowTime,
	})
	if err != nil {
		return "", err
	}
	return credentialID, nil
}

func (s Service) updateSSHCredential(ctx context.Context, credentialID string, host BuildHost, sshCredential SSHCredential, nowTime time.Time) error {
	encrypted, err := s.encryptSSHCredential(sshCredential)
	if err != nil {
		return err
	}
	return s.credentials.UpdateValue(ctx, credential.Credential{
		ID:                credentialID,
		Name:              host.Username,
		EncryptedValue:    encrypted,
		EncryptionVersion: credential.EncryptionVersion,
		Fingerprint:       credential.Fingerprint(host.Name, host.Address, host.Username, sshCredential.PrivateKey),
		UpdatedAt:         nowTime,
	})
}

func (s Service) encryptSSHCredential(sshCredential SSHCredential) (string, error) {
	if s.credentials == nil {
		return "", validationError("SSH credential store is not configured")
	}
	data, err := json.Marshal(sshCredential)
	if err != nil {
		return "", fmt.Errorf("encode SSH credential: %w", err)
	}
	return s.encryptor.Encrypt(data)
}

func normalizeInput(input SaveInput) (BuildHost, *SSHCredential, error) {
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
	sshCredential, err := normalizeSSHCredential(input.PrivateKey)
	if err != nil {
		return BuildHost{}, nil, err
	}

	if host.Name == "" {
		return BuildHost{}, nil, validationError("name is required")
	}
	if host.ConnectionType == "" {
		host.ConnectionType = ConnectionLocalDocker
	}
	if host.MaxConcurrency == 0 {
		host.MaxConcurrency = 1
	}
	if host.MaxConcurrency < 1 {
		return BuildHost{}, nil, validationError("maxConcurrency must be at least 1")
	}
	if host.DockerCommand == "" {
		host.DockerCommand = DefaultDockerCommand
	}
	if !isSafeCommand(host.DockerCommand) {
		return BuildHost{}, nil, ErrInvalidCommand
	}

	switch host.ConnectionType {
	case ConnectionLocalDocker:
		if host.DockerEndpoint == "" {
			host.DockerEndpoint = DefaultDockerEndpoint
		}
		host.Address = ""
		host.Port = 0
		host.Username = ""
		sshCredential = nil
	case ConnectionSSH:
		if host.Address == "" {
			return BuildHost{}, nil, validationError("address is required for SSH hosts")
		}
		if host.Username == "" {
			return BuildHost{}, nil, validationError("username is required for SSH hosts")
		}
		if host.Port == 0 {
			host.Port = 22
		}
		if host.Port < 1 || host.Port > 65535 {
			return BuildHost{}, nil, validationError("port must be between 1 and 65535")
		}
		host.DockerEndpoint = ""
	default:
		return BuildHost{}, nil, validationError("connectionType must be local_docker or ssh")
	}

	return host, sshCredential, nil
}

func validationError(message string) error {
	return fmt.Errorf("%w: %s", ErrValidation, message)
}

func normalizeSSHCredential(privateKey string) (*SSHCredential, error) {
	privateKey = strings.TrimSpace(privateKey)
	if privateKey == "" {
		return nil, nil
	}
	if len(privateKey) > 128*1024 {
		return nil, validationError("privateKey is too large")
	}
	if !strings.Contains(privateKey, "PRIVATE KEY") {
		return nil, validationError("privateKey must be an SSH private key")
	}
	return &SSHCredential{PrivateKey: privateKey}, nil
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
