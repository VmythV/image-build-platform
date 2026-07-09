package imageartifact

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/VmythV/image-build-platform/internal/buildhost"
	"github.com/VmythV/image-build-platform/internal/buildtask"
	"github.com/VmythV/image-build-platform/internal/platform/clock"
	"github.com/VmythV/image-build-platform/internal/platform/id"
	"github.com/VmythV/image-build-platform/internal/registry"
)

var ErrValidation = errors.New("image artifact validation failed")

type TaskFinder interface {
	FindByID(ctx context.Context, taskID string) (buildtask.BuildTask, error)
}

type RegistryFinder interface {
	FindByID(ctx context.Context, registryID string) (registry.Registry, error)
}

type RegistrySecretProvider interface {
	Secret(ctx context.Context, registry registry.Registry) (*registry.RegistrySecret, error)
}

type HostFinder interface {
	FindByID(ctx context.Context, hostID string) (buildhost.BuildHost, error)
}

type HostCredentialProvider interface {
	SSHCredential(ctx context.Context, host buildhost.BuildHost) (*buildhost.SSHCredential, error)
}

type RepushExecutor interface {
	Repush(ctx context.Context, operationID string, sourceImageRef string, targetImageRef string, host buildhost.BuildHost, pushRegistry registry.Registry, secret *registry.RegistrySecret, logPath string) (buildtask.PushResult, error)
}

type Service struct {
	repo            Repository
	tasks           TaskFinder
	registries      RegistryFinder
	registrySecrets RegistrySecretProvider
	hosts           HostFinder
	hostCredentials HostCredentialProvider
	executor        RepushExecutor
	logDir          string
}

type ServiceOptions struct {
	Repository      Repository
	Tasks           TaskFinder
	Registries      RegistryFinder
	RegistrySecrets RegistrySecretProvider
	Hosts           HostFinder
	HostCredentials HostCredentialProvider
	Executor        RepushExecutor
	LogDir          string
}

func NewService(repo Repository) Service {
	return NewServiceWithOptions(ServiceOptions{Repository: repo, LogDir: "data/logs"})
}

func NewServiceWithOptions(opts ServiceOptions) Service {
	logDir := strings.TrimSpace(opts.LogDir)
	if logDir == "" {
		logDir = "data/logs"
	}
	return Service{
		repo:            opts.Repository,
		tasks:           opts.Tasks,
		registries:      opts.Registries,
		registrySecrets: opts.RegistrySecrets,
		hosts:           opts.Hosts,
		hostCredentials: opts.HostCredentials,
		executor:        opts.Executor,
		logDir:          logDir,
	}
}

func (s Service) List(ctx context.Context, filter ListFilter) ([]Artifact, int, error) {
	return s.repo.List(ctx, filter)
}

func (s Service) Get(ctx context.Context, artifactID string) (Artifact, error) {
	return s.repo.FindByID(ctx, artifactID)
}

func (s Service) PullCommand(ctx context.Context, artifactID string) (string, error) {
	artifact, err := s.repo.FindByID(ctx, artifactID)
	if err != nil {
		return "", err
	}
	return "docker pull " + artifact.ImageRef, nil
}

func (s Service) Archive(ctx context.Context, artifactID string) (Artifact, error) {
	if err := s.repo.Archive(ctx, artifactID, clock.Now()); err != nil {
		return Artifact{}, err
	}
	return s.repo.FindByID(ctx, artifactID)
}

func (s Service) Deprecate(ctx context.Context, artifactID string) (Artifact, error) {
	if err := s.repo.Deprecate(ctx, artifactID, clock.Now()); err != nil {
		return Artifact{}, err
	}
	return s.repo.FindByID(ctx, artifactID)
}

func (s Service) Repush(ctx context.Context, artifactID string, input RepushInput, actorID string) (Artifact, PushEvent, string, error) {
	if s.tasks == nil || s.registries == nil || s.hosts == nil || s.executor == nil {
		return Artifact{}, PushEvent{}, "", fmt.Errorf("%w: repush dependencies are not configured", ErrValidation)
	}

	artifact, err := s.repo.FindByID(ctx, artifactID)
	if err != nil {
		return Artifact{}, PushEvent{}, "", err
	}
	if strings.TrimSpace(artifact.ImageRef) == "" {
		return Artifact{}, PushEvent{}, "", fmt.Errorf("%w: artifact image ref is empty", ErrValidation)
	}

	task, err := s.tasks.FindByID(ctx, artifact.BuildTaskID)
	if err != nil {
		return Artifact{}, PushEvent{}, "", err
	}
	if strings.TrimSpace(task.HostID) == "" {
		return Artifact{}, PushEvent{}, "", fmt.Errorf("%w: source build task has no build host", ErrValidation)
	}
	host, err := s.hosts.FindByID(ctx, task.HostID)
	if err != nil {
		return Artifact{}, PushEvent{}, "", err
	}
	if s.hostCredentials != nil && host.ConnectionType == buildhost.ConnectionSSH {
		sshCredential, err := s.hostCredentials.SSHCredential(ctx, host)
		if err != nil {
			return Artifact{}, PushEvent{}, "", err
		}
		host.SSHCredential = sshCredential
	}

	targetRegistryID := strings.TrimSpace(input.RegistryID)
	if targetRegistryID == "" {
		targetRegistryID = artifact.RegistryID
	}
	pushRegistry, err := s.registries.FindByID(ctx, targetRegistryID)
	if err != nil {
		return Artifact{}, PushEvent{}, "", err
	}
	if !pushRegistry.AllowPush {
		return Artifact{}, PushEvent{}, "", fmt.Errorf("%w: registry must allow push", ErrValidation)
	}
	if pushRegistry.Status == registry.StatusDisabled {
		return Artifact{}, PushEvent{}, "", fmt.Errorf("%w: registry is disabled", ErrValidation)
	}

	var secret *registry.RegistrySecret
	if s.registrySecrets != nil {
		secret, err = s.registrySecrets.Secret(ctx, pushRegistry)
		if err != nil {
			return Artifact{}, PushEvent{}, "", err
		}
	}

	operationID := id.New()
	targetImageRef := repushTargetImageRef(pushRegistry, task, artifact)
	logPath, err := s.prepareRepushLog(operationID)
	if err != nil {
		return Artifact{}, PushEvent{}, "", err
	}

	startedAt := clock.Now()
	result, pushErr := s.executor.Repush(ctx, operationID, artifact.ImageRef, targetImageRef, host, pushRegistry, secret, logPath)
	finishedAt := clock.Now()
	if pushErr != nil {
		event := PushEvent{
			ID:           operationID,
			ArtifactID:   artifact.ID,
			BuildTaskID:  artifact.BuildTaskID,
			RegistryID:   pushRegistry.ID,
			Status:       PushStatusFailed,
			ErrorMessage: pushErr.Error(),
			StartedAt:    startedAt,
			FinishedAt:   &finishedAt,
			CreatedBy:    actorID,
			CreatedAt:    startedAt,
		}
		if err := s.repo.RecordPushEvent(ctx, event); err != nil {
			return Artifact{}, PushEvent{}, logPath, err
		}
		return Artifact{}, event, logPath, pushErr
	}

	newArtifact := Artifact{
		ID:            id.New(),
		BuildTaskID:   artifact.BuildTaskID,
		ProjectID:     artifact.ProjectID,
		VersionNodeID: artifact.VersionNodeID,
		RegistryID:    pushRegistry.ID,
		ImageRef:      targetImageRef,
		ImageID:       result.ImageID,
		Digest:        result.Digest,
		Tag:           artifact.Tag,
		Architecture:  artifact.Architecture,
		SizeBytes:     result.SizeBytes,
		Status:        StatusAvailable,
		Pushed:        true,
		PushedAt:      &finishedAt,
		CreatedAt:     finishedAt,
		UpdatedAt:     finishedAt,
	}
	event := PushEvent{
		ID:          operationID,
		ArtifactID:  newArtifact.ID,
		BuildTaskID: artifact.BuildTaskID,
		RegistryID:  pushRegistry.ID,
		Status:      PushStatusSuccess,
		StartedAt:   startedAt,
		FinishedAt:  &finishedAt,
		CreatedBy:   actorID,
		CreatedAt:   startedAt,
	}
	if err := s.repo.RecordPushed(ctx, newArtifact, event); err != nil {
		return Artifact{}, PushEvent{}, logPath, err
	}

	created, err := s.repo.FindByID(ctx, newArtifact.ID)
	if err != nil {
		return Artifact{}, PushEvent{}, logPath, err
	}
	return created, event, logPath, nil
}

func (s Service) prepareRepushLog(operationID string) (string, error) {
	logDir := filepath.Join(s.logDir, "artifacts")
	if err := os.MkdirAll(logDir, 0o750); err != nil {
		return "", fmt.Errorf("create artifact log directory: %w", err)
	}
	logPath := filepath.Join(logDir, operationID+".log")
	if err := os.WriteFile(logPath, []byte("artifact repush log initialized\n"), 0o640); err != nil {
		return "", fmt.Errorf("initialize artifact repush log: %w", err)
	}
	return logPath, nil
}

func repushTargetImageRef(pushRegistry registry.Registry, task buildtask.BuildTask, artifact Artifact) string {
	imageName := strings.Trim(strings.TrimSpace(task.ImageName), "/")
	if imageName == "" {
		imageName = imagePathFromRef(artifact.ImageRef)
	}
	tag := strings.TrimSpace(artifact.Tag)
	if tag == "" {
		tag = strings.TrimSpace(task.ImageTag)
	}
	if tag == "" {
		return strings.TrimRight(pushRegistry.Endpoint, "/") + "/" + imageName
	}
	return strings.TrimRight(pushRegistry.Endpoint, "/") + "/" + imageName + ":" + tag
}

func imagePathFromRef(imageRef string) string {
	value := strings.TrimSpace(imageRef)
	value = strings.TrimPrefix(value, "http://")
	value = strings.TrimPrefix(value, "https://")
	if slash := strings.Index(value, "/"); slash >= 0 && slash+1 < len(value) {
		value = value[slash+1:]
	}
	if at := strings.Index(value, "@"); at >= 0 {
		value = value[:at]
	}
	return strings.Trim(value, "/")
}
