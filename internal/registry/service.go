package registry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/VmythV/image-build-platform/internal/credential"
	"github.com/VmythV/image-build-platform/internal/platform/clock"
	"github.com/VmythV/image-build-platform/internal/platform/id"
)

var ErrValidation = errors.New("registry validation failed")

type Service struct {
	repo        Repository
	credentials credential.Repository
	encryptor   credential.Encryptor
	detector    Detector
}

func NewService(repo Repository, credentials credential.Repository, encryptor credential.Encryptor, detector Detector) Service {
	if detector == nil {
		detector = CommandDetector{}
	}
	return Service{repo: repo, credentials: credentials, encryptor: encryptor, detector: detector}
}

func (s Service) List(ctx context.Context, filter ListFilter) ([]Registry, int, error) {
	return s.repo.List(ctx, filter)
}

func (s Service) Get(ctx context.Context, registryID string) (Registry, error) {
	return s.repo.FindByID(ctx, registryID)
}

func (s Service) Create(ctx context.Context, input SaveInput, actorID string) (Registry, error) {
	registry, secret, err := normalizeInput(input, false)
	if err != nil {
		return Registry{}, err
	}

	now := clock.Now()
	registry.ID = id.New()
	registry.CreatedBy = actorID
	registry.CreatedAt = now
	registry.UpdatedAt = now
	registry.Status = StatusUnknown

	if secret != nil {
		credentialID, err := s.createCredential(ctx, registry.Name, *secret, actorID, now)
		if err != nil {
			return Registry{}, err
		}
		registry.CredentialID = credentialID
	}

	if err := s.repo.Create(ctx, registry); err != nil {
		return Registry{}, err
	}
	if err := s.clearDefaults(ctx, registry); err != nil {
		return Registry{}, err
	}

	return s.repo.FindByID(ctx, registry.ID)
}

func (s Service) Update(ctx context.Context, registryID string, input SaveInput) (Registry, error) {
	existing, err := s.repo.FindByID(ctx, registryID)
	if err != nil {
		return Registry{}, err
	}

	updated, secret, err := normalizeInput(input, true)
	if err != nil {
		return Registry{}, err
	}

	now := clock.Now()
	updated.ID = existing.ID
	updated.CredentialID = existing.CredentialID
	updated.CreatedBy = existing.CreatedBy
	updated.CreatedAt = existing.CreatedAt
	updated.UpdatedAt = now
	updated.Status = StatusUnknown
	if existing.Status == StatusDisabled {
		updated.Status = StatusDisabled
	}

	if secret != nil {
		if existing.CredentialID == "" {
			credentialID, err := s.createCredential(ctx, updated.Name, *secret, existing.CreatedBy, now)
			if err != nil {
				return Registry{}, err
			}
			updated.CredentialID = credentialID
		} else if err := s.updateCredential(ctx, existing.CredentialID, updated.Name, *secret, now); err != nil {
			return Registry{}, err
		}
	}

	if err := s.repo.Update(ctx, updated); err != nil {
		return Registry{}, err
	}
	if err := s.clearDefaults(ctx, updated); err != nil {
		return Registry{}, err
	}

	return s.repo.FindByID(ctx, registryID)
}

func (s Service) Delete(ctx context.Context, registryID string) error {
	return s.repo.Delete(ctx, registryID, clock.Now())
}

func (s Service) Enable(ctx context.Context, registryID string) (Registry, error) {
	if err := s.repo.SetStatus(ctx, registryID, StatusUnknown, clock.Now()); err != nil {
		return Registry{}, err
	}
	return s.repo.FindByID(ctx, registryID)
}

func (s Service) Disable(ctx context.Context, registryID string) (Registry, error) {
	if err := s.repo.SetStatus(ctx, registryID, StatusDisabled, clock.Now()); err != nil {
		return Registry{}, err
	}
	return s.repo.FindByID(ctx, registryID)
}

func (s Service) Check(ctx context.Context, registryID string) (Registry, CheckResult, error) {
	registry, err := s.repo.FindByID(ctx, registryID)
	if err != nil {
		return Registry{}, CheckResult{}, err
	}
	if registry.Status == StatusDisabled {
		result := failedCheck(StatusDisabled, "Registry is disabled.")
		return registry, result, nil
	}

	secret, err := s.registrySecret(ctx, registry)
	if err != nil {
		return Registry{}, CheckResult{}, err
	}
	result := s.detector.Check(ctx, registry, secret)
	checkedAt := clock.Now()
	rawResult, err := json.Marshal(result)
	if err != nil {
		return Registry{}, CheckResult{}, fmt.Errorf("encode registry check result: %w", err)
	}
	if err := s.repo.UpdateCheckResult(ctx, registryID, result, checkedAt, string(rawResult)); err != nil {
		return Registry{}, CheckResult{}, err
	}

	updated, err := s.repo.FindByID(ctx, registryID)
	if err != nil {
		return Registry{}, CheckResult{}, err
	}
	return updated, result, nil
}

func (s Service) Secret(ctx context.Context, registry Registry) (*RegistrySecret, error) {
	return s.registrySecret(ctx, registry)
}

func (s Service) createCredential(ctx context.Context, registryName string, secret RegistrySecret, actorID string, nowTime time.Time) (string, error) {
	encrypted, err := s.encryptSecret(secret)
	if err != nil {
		return "", err
	}
	credentialID := id.New()
	err = s.credentials.Create(ctx, credential.Credential{
		ID:                credentialID,
		Type:              credential.TypeRegistryPassword,
		Name:              secret.Username,
		EncryptedValue:    encrypted,
		EncryptionVersion: credential.EncryptionVersion,
		Fingerprint:       credential.Fingerprint(registryName, secret.Username, secret.Password),
		CreatedBy:         actorID,
		CreatedAt:         nowTime,
		UpdatedAt:         nowTime,
	})
	if err != nil {
		return "", err
	}
	return credentialID, nil
}

func (s Service) updateCredential(ctx context.Context, credentialID string, registryName string, secret RegistrySecret, nowTime time.Time) error {
	encrypted, err := s.encryptSecret(secret)
	if err != nil {
		return err
	}
	return s.credentials.UpdateValue(ctx, credential.Credential{
		ID:                credentialID,
		Name:              secret.Username,
		EncryptedValue:    encrypted,
		EncryptionVersion: credential.EncryptionVersion,
		Fingerprint:       credential.Fingerprint(registryName, secret.Username, secret.Password),
		UpdatedAt:         nowTime,
	})
}

func (s Service) encryptSecret(secret RegistrySecret) (string, error) {
	data, err := json.Marshal(secret)
	if err != nil {
		return "", fmt.Errorf("encode registry credential: %w", err)
	}
	return s.encryptor.Encrypt(data)
}

func (s Service) registrySecret(ctx context.Context, registry Registry) (*RegistrySecret, error) {
	if registry.CredentialID == "" {
		return nil, nil
	}
	stored, err := s.credentials.FindByID(ctx, registry.CredentialID)
	if err != nil {
		return nil, err
	}
	plaintext, err := s.encryptor.Decrypt(stored.EncryptedValue)
	if err != nil {
		return nil, err
	}

	var secret RegistrySecret
	if err := json.Unmarshal(plaintext, &secret); err != nil {
		return nil, fmt.Errorf("decode registry credential: %w", err)
	}
	return &secret, nil
}

func (s Service) clearDefaults(ctx context.Context, registry Registry) error {
	if registry.IsDefaultPull {
		if err := s.repo.ClearDefaultPull(ctx, registry.ID); err != nil {
			return err
		}
	}
	if registry.IsDefaultPush {
		if err := s.repo.ClearDefaultPush(ctx, registry.ID); err != nil {
			return err
		}
	}
	return nil
}

func normalizeInput(input SaveInput, allowEmptyPassword bool) (Registry, *RegistrySecret, error) {
	registry := Registry{
		Name:          strings.TrimSpace(input.Name),
		Type:          strings.TrimSpace(input.Type),
		Endpoint:      normalizeEndpoint(input.Endpoint),
		Namespace:     strings.Trim(strings.TrimSpace(input.Namespace), "/"),
		Region:        strings.TrimSpace(input.Region),
		AllowPull:     input.AllowPull,
		AllowPush:     input.AllowPush,
		IsDefaultPull: input.IsDefaultPull,
		IsDefaultPush: input.IsDefaultPush,
		TLSVerify:     input.TLSVerify,
		InsecureHTTP:  input.InsecureHTTP,
	}

	if registry.Name == "" {
		return Registry{}, nil, validationError("name is required")
	}
	if registry.Type == "" {
		registry.Type = TypeGeneric
	}
	if !validType(registry.Type) {
		return Registry{}, nil, validationError("type is invalid")
	}
	if registry.Endpoint == "" {
		return Registry{}, nil, validationError("endpoint is required")
	}
	if !registry.AllowPull && !registry.AllowPush {
		return Registry{}, nil, validationError("allowPull or allowPush must be enabled")
	}
	if registry.IsDefaultPull && !registry.AllowPull {
		return Registry{}, nil, validationError("default pull registry must allow pull")
	}
	if registry.IsDefaultPush && !registry.AllowPush {
		return Registry{}, nil, validationError("default push registry must allow push")
	}

	username := strings.TrimSpace(input.Username)
	password := input.Password
	var secret *RegistrySecret
	if username != "" || password != "" {
		if username == "" {
			return Registry{}, nil, validationError("username is required when password is provided")
		}
		if password == "" && !allowEmptyPassword {
			return Registry{}, nil, validationError("password is required when username is provided")
		}
		if password != "" {
			secret = &RegistrySecret{Username: username, Password: password}
		}
	}

	return registry, secret, nil
}

func normalizeEndpoint(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimRight(value, "/")
	if parsed, err := url.Parse(value); err == nil && parsed.Host != "" {
		value = parsed.Host
	}
	return strings.TrimRight(value, "/")
}

func validType(value string) bool {
	switch value {
	case TypeGeneric, TypeHarbor, TypeDockerHub, TypeAliyun, TypeTencentCloud:
		return true
	default:
		return false
	}
}

func validationError(message string) error {
	return fmt.Errorf("%w: %s", ErrValidation, message)
}
