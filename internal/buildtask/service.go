package buildtask

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/VmythV/image-build-platform/internal/imageproject"
	"github.com/VmythV/image-build-platform/internal/platform/clock"
	"github.com/VmythV/image-build-platform/internal/platform/id"
	"github.com/VmythV/image-build-platform/internal/registry"
)

var (
	ErrValidation        = errors.New("build task validation failed")
	ErrInvalidState      = errors.New("build task state transition is invalid")
	ErrNoSchedulableHost = errors.New("no schedulable build host")
)

type Service struct {
	repo       Repository
	projects   imageproject.Repository
	registries registry.Repository
}

func NewService(repo Repository, projects imageproject.Repository, registries registry.Repository) Service {
	return Service{repo: repo, projects: projects, registries: registries}
}

func (s Service) List(ctx context.Context, filter ListFilter) ([]BuildTask, int, error) {
	return s.repo.List(ctx, filter)
}

func (s Service) Get(ctx context.Context, taskID string) (BuildTask, error) {
	return s.repo.FindByID(ctx, taskID)
}

func (s Service) Create(ctx context.Context, input CreateInput, actorID string) (BuildTask, error) {
	normalized, err := s.normalizeCreateInput(ctx, input)
	if err != nil {
		return BuildTask{}, err
	}

	now := clock.Now()
	task := BuildTask{
		ID:                 id.New(),
		ProjectID:          normalized.project.ID,
		VersionNodeID:      normalized.node.ID,
		RequestedHostID:    normalized.requestedHostID,
		RegistryID:         normalized.registry.ID,
		ImageName:          normalized.imageName,
		ImageTag:           normalized.imageTag,
		ImageRef:           normalized.imageRef,
		Architecture:       normalized.architecture,
		DockerfileSnapshot: normalized.node.Dockerfile,
		DockerfileHash:     normalized.node.DockerfileHash,
		BuildContextRef:    normalized.node.BuildContextRef,
		BuildArgs:          normalized.buildArgs,
		BuildOptions:       normalized.buildOptions,
		Status:             StatusQueued,
		QueuedAt:           &now,
		CreatedBy:          actorID,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	if err := s.repo.Create(ctx, task); err != nil {
		return BuildTask{}, err
	}
	return s.repo.FindByID(ctx, task.ID)
}

func (s Service) Dispatch(ctx context.Context, taskID string) (BuildTask, bool, string, error) {
	return s.repo.DispatchTask(ctx, taskID, clock.Now())
}

func (s Service) DispatchNext(ctx context.Context) (BuildTask, bool, string, error) {
	return s.repo.DispatchNext(ctx, clock.Now())
}

func (s Service) Cancel(ctx context.Context, taskID string) (BuildTask, error) {
	return s.repo.Cancel(ctx, taskID, clock.Now())
}

func (s Service) Retry(ctx context.Context, taskID string, actorID string) (BuildTask, error) {
	existing, err := s.repo.FindByID(ctx, taskID)
	if err != nil {
		return BuildTask{}, err
	}
	if !retryableStatus(existing.Status) {
		return BuildTask{}, ErrInvalidState
	}

	now := clock.Now()
	retry := BuildTask{
		ID:                 id.New(),
		ProjectID:          existing.ProjectID,
		VersionNodeID:      existing.VersionNodeID,
		RetryOfTaskID:      existing.ID,
		RequestedHostID:    existing.RequestedHostID,
		RegistryID:         existing.RegistryID,
		ImageName:          existing.ImageName,
		ImageTag:           existing.ImageTag,
		ImageRef:           existing.ImageRef,
		Architecture:       existing.Architecture,
		DockerfileSnapshot: existing.DockerfileSnapshot,
		DockerfileHash:     existing.DockerfileHash,
		BuildContextRef:    existing.BuildContextRef,
		BuildArgs:          existing.BuildArgs,
		BuildOptions:       existing.BuildOptions,
		Status:             StatusQueued,
		QueuedAt:           &now,
		CreatedBy:          actorID,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := s.repo.Create(ctx, retry); err != nil {
		return BuildTask{}, err
	}
	return s.repo.FindByID(ctx, retry.ID)
}

type normalizedCreateInput struct {
	project         imageproject.Project
	node            imageproject.VersionNode
	registry        registry.Registry
	requestedHostID string
	imageName       string
	imageTag        string
	imageRef        string
	architecture    string
	buildArgs       map[string]string
	buildOptions    map[string]string
}

func (s Service) normalizeCreateInput(ctx context.Context, input CreateInput) (normalizedCreateInput, error) {
	projectID := strings.TrimSpace(input.ProjectID)
	nodeID := strings.TrimSpace(input.VersionNodeID)
	if projectID == "" {
		return normalizedCreateInput{}, validationError("projectId is required")
	}
	if nodeID == "" {
		return normalizedCreateInput{}, validationError("versionNodeId is required")
	}

	project, err := s.projects.FindProject(ctx, projectID)
	if err != nil {
		if errors.Is(err, imageproject.ErrNotFound) {
			return normalizedCreateInput{}, ErrNotFound
		}
		return normalizedCreateInput{}, err
	}
	if project.Status != imageproject.ProjectStatusActive {
		return normalizedCreateInput{}, validationError("project must be active")
	}

	node, err := s.projects.FindNode(ctx, project.ID, nodeID)
	if err != nil {
		if errors.Is(err, imageproject.ErrNotFound) {
			return normalizedCreateInput{}, ErrNotFound
		}
		return normalizedCreateInput{}, err
	}
	if node.Status == imageproject.NodeStatusArchived {
		return normalizedCreateInput{}, validationError("version node is archived")
	}

	pushRegistry, err := s.resolveRegistry(ctx, strings.TrimSpace(input.RegistryID), project.DefaultRegistryID)
	if err != nil {
		return normalizedCreateInput{}, err
	}

	architecture := strings.TrimSpace(input.Architecture)
	if architecture == "" {
		architecture = strings.TrimSpace(project.DefaultArchitecture)
	}
	if architecture == "" {
		architecture = "amd64"
	}
	if !architecturePattern.MatchString(architecture) {
		return normalizedCreateInput{}, validationError("architecture is invalid")
	}

	imageName := strings.Trim(strings.TrimSpace(input.ImageName), "/")
	if imageName == "" {
		imageName = defaultImagePath(project, pushRegistry)
	}
	if !imageNamePattern.MatchString(imageName) || strings.Contains(imageName, "//") {
		return normalizedCreateInput{}, validationError("imageName is invalid")
	}

	imageTag := strings.TrimSpace(input.ImageTag)
	if imageTag == "" {
		imageTag = node.Version
	}
	if !imageTagPattern.MatchString(imageTag) {
		return normalizedCreateInput{}, validationError("imageTag is invalid")
	}

	buildArgs, err := normalizeBuildArgs(input.BuildArgs)
	if err != nil {
		return normalizedCreateInput{}, err
	}
	buildOptions := normalizeBuildOptions(input.BuildOptions)

	return normalizedCreateInput{
		project:         project,
		node:            node,
		registry:        pushRegistry,
		requestedHostID: strings.TrimSpace(input.RequestedHostID),
		imageName:       imageName,
		imageTag:        imageTag,
		imageRef:        buildImageRef(pushRegistry.Endpoint, imageName, imageTag),
		architecture:    architecture,
		buildArgs:       buildArgs,
		buildOptions:    buildOptions,
	}, nil
}

func (s Service) resolveRegistry(ctx context.Context, requestedID string, projectDefaultID string) (registry.Registry, error) {
	registryID := requestedID
	if registryID == "" {
		registryID = strings.TrimSpace(projectDefaultID)
	}

	var selected registry.Registry
	var err error
	if registryID != "" {
		selected, err = s.registries.FindByID(ctx, registryID)
	} else {
		selected, err = s.registries.FindDefaultPush(ctx)
	}
	if err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			return registry.Registry{}, validationError("registryId is required when no default push registry is configured")
		}
		return registry.Registry{}, err
	}
	if !selected.AllowPush {
		return registry.Registry{}, validationError("registry must allow push")
	}
	if selected.Status == registry.StatusDisabled {
		return registry.Registry{}, validationError("registry is disabled")
	}
	return selected, nil
}

func defaultImagePath(project imageproject.Project, selected registry.Registry) string {
	imageName := strings.Trim(strings.TrimSpace(project.ImageName), "/")
	namespace := strings.Trim(strings.TrimSpace(project.Namespace), "/")
	if namespace == "" {
		namespace = strings.Trim(strings.TrimSpace(selected.Namespace), "/")
	}
	if namespace == "" {
		return imageName
	}
	return namespace + "/" + imageName
}

func buildImageRef(endpoint string, imageName string, imageTag string) string {
	return strings.TrimRight(endpoint, "/") + "/" + strings.Trim(imageName, "/") + ":" + imageTag
}

func normalizeBuildArgs(input map[string]string) (map[string]string, error) {
	result := make(map[string]string, len(input))
	for key, value := range input {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if !buildArgPattern.MatchString(key) {
			return nil, validationError("buildArgs contains invalid key " + key)
		}
		result[key] = strings.TrimSpace(value)
	}
	return result, nil
}

func normalizeBuildOptions(input map[string]string) map[string]string {
	result := make(map[string]string, len(input))
	for key, value := range input {
		key = strings.TrimSpace(key)
		if key != "" {
			result[key] = strings.TrimSpace(value)
		}
	}
	return result
}

func retryableStatus(status string) bool {
	switch status {
	case StatusPreparingContextFailed, StatusDispatchFailed, StatusBuildFailed, StatusPushFailed, StatusCanceled, StatusTimeout:
		return true
	default:
		return false
	}
}

func validationError(message string) error {
	return fmt.Errorf("%w: %s", ErrValidation, message)
}

var (
	architecturePattern = regexp.MustCompile(`^[A-Za-z0-9_./-]+$`)
	imageNamePattern    = regexp.MustCompile(`^[a-z0-9][a-z0-9._/-]*[a-z0-9]$|^[a-z0-9]$`)
	imageTagPattern     = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$`)
	buildArgPattern     = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)
