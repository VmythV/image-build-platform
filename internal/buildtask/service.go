package buildtask

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/VmythV/image-build-platform/internal/buildhost"
	"github.com/VmythV/image-build-platform/internal/imageartifact"
	"github.com/VmythV/image-build-platform/internal/imageproject"
	"github.com/VmythV/image-build-platform/internal/platform/clock"
	"github.com/VmythV/image-build-platform/internal/platform/id"
	"github.com/VmythV/image-build-platform/internal/registry"
	systemsettings "github.com/VmythV/image-build-platform/internal/settings"
)

var (
	ErrValidation        = errors.New("build task validation failed")
	ErrInvalidState      = errors.New("build task state transition is invalid")
	ErrNoSchedulableHost = errors.New("no schedulable build host")
	ErrLogsNotFound      = errors.New("build task logs not found")
)

type Service struct {
	repo                 Repository
	projects             imageproject.Repository
	registries           registry.Repository
	secrets              RegistrySecretProvider
	artifacts            ArtifactRecorder
	hosts                buildhost.Repository
	settings             RuntimeSettings
	executor             Executor
	contextDir           string
	logDir               string
	buildTimeout         time.Duration
	maxGlobalConcurrency int
	logger               *slog.Logger
}

func NewService(repo Repository, projects imageproject.Repository, registries registry.Repository) Service {
	return NewServiceWithOptions(ServiceOptions{
		Repository: repo,
		Projects:   projects,
		Registries: registries,
		ContextDir: "data/contexts",
		LogDir:     "data/logs",
	})
}

type ServiceOptions struct {
	Repository           Repository
	Projects             imageproject.Repository
	Registries           registry.Repository
	RegistrySecrets      RegistrySecretProvider
	Artifacts            ArtifactRecorder
	Hosts                buildhost.Repository
	Settings             RuntimeSettings
	Executor             Executor
	ContextDir           string
	LogDir               string
	DefaultBuildTimeout  time.Duration
	MaxGlobalConcurrency int
	Logger               *slog.Logger
}

type RuntimeSettings interface {
	FindByKey(ctx context.Context, key string) (systemsettings.Setting, error)
}

type RegistrySecretProvider interface {
	Secret(ctx context.Context, registry registry.Registry) (*registry.RegistrySecret, error)
}

type ArtifactRecorder interface {
	RecordPushed(ctx context.Context, artifact imageartifact.Artifact, event imageartifact.PushEvent) error
}

func NewServiceWithOptions(opts ServiceOptions) Service {
	executor := opts.Executor
	if executor == nil {
		executor = NewLocalDockerExecutor()
	}
	contextDir := strings.TrimSpace(opts.ContextDir)
	if contextDir == "" {
		contextDir = "data/contexts"
	}
	logDir := strings.TrimSpace(opts.LogDir)
	if logDir == "" {
		logDir = "data/logs"
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	buildTimeout := opts.DefaultBuildTimeout
	if buildTimeout <= 0 {
		buildTimeout = time.Hour
	}
	maxGlobalConcurrency := opts.MaxGlobalConcurrency
	if maxGlobalConcurrency <= 0 {
		maxGlobalConcurrency = 4
	}

	return Service{
		repo:                 opts.Repository,
		projects:             opts.Projects,
		registries:           opts.Registries,
		secrets:              opts.RegistrySecrets,
		artifacts:            opts.Artifacts,
		hosts:                opts.Hosts,
		settings:             opts.Settings,
		executor:             executor,
		contextDir:           contextDir,
		logDir:               logDir,
		buildTimeout:         buildTimeout,
		maxGlobalConcurrency: maxGlobalConcurrency,
		logger:               logger,
	}
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
	return s.repo.DispatchTaskWithOptions(ctx, taskID, clock.Now(), s.dispatchOptions(ctx))
}

func (s Service) DispatchNext(ctx context.Context) (BuildTask, bool, string, error) {
	return s.repo.DispatchNextWithOptions(ctx, clock.Now(), s.dispatchOptions(ctx))
}

func (s Service) Cancel(ctx context.Context, taskID string) (BuildTask, error) {
	s.executor.Cancel(taskID)
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

func (s Service) Start(ctx context.Context, taskID string) (BuildTask, error) {
	task, err := s.repo.FindByID(ctx, taskID)
	if err != nil {
		return BuildTask{}, err
	}
	if task.Status == StatusQueued || task.Status == StatusCreated {
		var dispatched bool
		task, dispatched, _, err = s.repo.DispatchTaskWithOptions(ctx, task.ID, clock.Now(), s.dispatchOptions(ctx))
		if err != nil {
			return BuildTask{}, err
		}
		if !dispatched {
			return task, nil
		}
	}
	if task.Status == StatusDispatchFailed {
		return task, nil
	}
	if task.Status != StatusDispatching {
		return BuildTask{}, ErrInvalidState
	}

	host, err := s.hosts.FindByID(ctx, task.HostID)
	if err != nil {
		if errors.Is(err, buildhost.ErrNotFound) {
			return s.repo.FailTask(ctx, task.ID, StatusDispatchFailed, "HOST_NOT_FOUND", "Build host was not found.", clock.Now())
		}
		return BuildTask{}, err
	}
	if host.ConnectionType != buildhost.ConnectionLocalDocker && host.ConnectionType != buildhost.ConnectionSSH {
		return s.repo.FailTask(ctx, task.ID, StatusDispatchFailed, "UNSUPPORTED_HOST", "Only local Docker and SSH build hosts are supported.", clock.Now())
	}

	prepared, err := s.prepareBuildContext(task)
	if err != nil {
		return s.repo.FailTask(ctx, task.ID, StatusPreparingContextFailed, "PREPARE_CONTEXT_FAILED", err.Error(), clock.Now())
	}

	started, err := s.repo.StartBuild(ctx, task.ID, prepared.ContextRef, prepared.LogPath, clock.Now())
	if err != nil {
		return BuildTask{}, err
	}

	go s.runBuild(started.ID)
	return started, nil
}

func (s Service) ReadLogs(ctx context.Context, taskID string) (string, string, error) {
	_, logPath, filename, err := s.LogFile(ctx, taskID)
	if err != nil {
		return "", "", err
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", "", ErrLogsNotFound
		}
		return "", "", err
	}
	return string(data), filename, nil
}

func (s Service) LogFile(ctx context.Context, taskID string) (BuildTask, string, string, error) {
	task, err := s.repo.FindByID(ctx, taskID)
	if err != nil {
		return BuildTask{}, "", "", err
	}
	if task.LogPath == "" {
		return BuildTask{}, "", "", ErrLogsNotFound
	}
	cleanLogRoot, err := filepath.Abs(s.logDir)
	if err != nil {
		return BuildTask{}, "", "", err
	}
	logPath, err := filepath.Abs(task.LogPath)
	if err != nil {
		return BuildTask{}, "", "", err
	}
	if !strings.HasPrefix(logPath, cleanLogRoot+string(os.PathSeparator)) && logPath != cleanLogRoot {
		return BuildTask{}, "", "", ErrLogsNotFound
	}
	return task, logPath, filepath.Base(logPath), nil
}

func (s Service) runBuild(taskID string) {
	dbCtx := context.Background()
	runCtx := dbCtx
	cancel := func() {}
	if timeout := s.effectiveBuildTimeout(dbCtx); timeout > 0 {
		runCtx, cancel = context.WithTimeout(dbCtx, timeout)
	}
	defer cancel()

	task, err := s.repo.FindByID(dbCtx, taskID)
	if err != nil {
		s.logger.Warn("load build task for execution", "task_id", taskID, "error", err)
		return
	}
	host, err := s.hosts.FindByID(dbCtx, task.HostID)
	if err != nil {
		s.completeBuild(dbCtx, task.ID, fmt.Errorf("load build host: %w", err))
		return
	}
	contextPath := filepath.Join(s.contextDir, task.ID)
	err = s.executor.Build(runCtx, task, host, contextPath, task.LogPath)
	if err != nil {
		if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
			s.timeoutTask(dbCtx, task.ID)
			return
		}
		s.completeBuild(dbCtx, task.ID, err)
		return
	}
	s.pushBuild(runCtx, dbCtx, task, host)
}

func (s Service) completeBuild(ctx context.Context, taskID string, buildErr error) {
	now := clock.Now()
	if buildErr != nil {
		if _, err := s.repo.CompleteBuild(ctx, taskID, false, "BUILD_FAILED", buildErr.Error(), now); err != nil {
			s.logger.Warn("mark build task failed", "task_id", taskID, "error", err)
		}
		return
	}
	if _, err := s.repo.CompleteBuild(ctx, taskID, true, "", "", now); err != nil {
		s.logger.Warn("mark build task success", "task_id", taskID, "error", err)
	}
}

func (s Service) pushBuild(execCtx context.Context, dbCtx context.Context, task BuildTask, host buildhost.BuildHost) {
	pushing, err := s.repo.StartPush(dbCtx, task.ID, clock.Now())
	if err != nil {
		s.logger.Warn("start build task push", "task_id", task.ID, "error", err)
		return
	}

	pushRegistry, err := s.registries.FindByID(dbCtx, pushing.RegistryID)
	if err != nil {
		s.completePush(dbCtx, pushing.ID, fmt.Errorf("load push registry: %w", err))
		return
	}
	if !pushRegistry.AllowPush {
		s.completePush(dbCtx, pushing.ID, errors.New("registry does not allow push"))
		return
	}

	var secret *registry.RegistrySecret
	if s.secrets != nil {
		secret, err = s.secrets.Secret(dbCtx, pushRegistry)
		if err != nil {
			s.completePush(dbCtx, pushing.ID, fmt.Errorf("load push registry credential: %w", err))
			return
		}
	}

	pushStartedAt := clock.Now()
	result, err := s.executor.Push(execCtx, pushing, host, pushRegistry, secret, pushing.LogPath)
	if err != nil {
		if errors.Is(execCtx.Err(), context.DeadlineExceeded) {
			s.timeoutTask(dbCtx, pushing.ID)
			return
		}
		s.completePush(dbCtx, pushing.ID, err)
		return
	}

	if s.artifacts != nil {
		if err := s.recordPushedArtifact(dbCtx, pushing, result, pushStartedAt); err != nil {
			s.completePush(dbCtx, pushing.ID, fmt.Errorf("record image artifact: %w", err))
			return
		}
	}

	if _, err := s.repo.CompletePush(dbCtx, pushing.ID, true, "", "", clock.Now()); err != nil {
		s.logger.Warn("mark build task push success", "task_id", pushing.ID, "error", err)
	}
}

func (s Service) timeoutTask(ctx context.Context, taskID string) {
	if _, err := s.repo.FailTask(ctx, taskID, StatusTimeout, "TASK_TIMEOUT", "Build task exceeded its timeout budget.", clock.Now()); err != nil {
		s.logger.Warn("mark build task timed out", "task_id", taskID, "error", err)
	}
}

func (s Service) completePush(ctx context.Context, taskID string, pushErr error) {
	if _, err := s.repo.CompletePush(ctx, taskID, false, "PUSH_FAILED", pushErr.Error(), clock.Now()); err != nil {
		s.logger.Warn("mark build task push failed", "task_id", taskID, "error", err)
	}
}

func (s Service) recordPushedArtifact(ctx context.Context, task BuildTask, result PushResult, pushStartedAt time.Time) error {
	now := clock.Now()
	artifactID := id.New()
	return s.artifacts.RecordPushed(ctx, imageartifact.Artifact{
		ID:            artifactID,
		BuildTaskID:   task.ID,
		ProjectID:     task.ProjectID,
		VersionNodeID: task.VersionNodeID,
		RegistryID:    task.RegistryID,
		ImageRef:      task.ImageRef,
		ImageID:       result.ImageID,
		Digest:        result.Digest,
		Tag:           task.ImageTag,
		Architecture:  task.Architecture,
		SizeBytes:     result.SizeBytes,
		Status:        imageartifact.StatusAvailable,
		Pushed:        true,
		PushedAt:      &now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}, imageartifact.PushEvent{
		ID:          id.New(),
		ArtifactID:  artifactID,
		BuildTaskID: task.ID,
		RegistryID:  task.RegistryID,
		Status:      imageartifact.PushStatusSuccess,
		StartedAt:   pushStartedAt,
		FinishedAt:  &now,
		CreatedBy:   task.CreatedBy,
		CreatedAt:   now,
	})
}

func (s Service) dispatchOptions(ctx context.Context) DispatchOptions {
	return DispatchOptions{
		EnforceGlobalConcurrency: true,
		MaxGlobalConcurrency:     s.effectiveGlobalConcurrency(ctx),
	}
}

func (s Service) effectiveGlobalConcurrency(ctx context.Context) int {
	value := s.settingInt(ctx, "scheduler.global_concurrency", s.maxGlobalConcurrency)
	if value < 0 {
		return s.maxGlobalConcurrency
	}
	return value
}

func (s Service) effectiveBuildTimeout(ctx context.Context) time.Duration {
	defaultMinutes := int(s.buildTimeout / time.Minute)
	if defaultMinutes <= 0 {
		defaultMinutes = 60
	}
	minutes := s.settingInt(ctx, "build.timeout_minutes", defaultMinutes)
	if minutes <= 0 {
		return 0
	}
	return time.Duration(minutes) * time.Minute
}

func (s Service) settingInt(ctx context.Context, key string, fallback int) int {
	if s.settings == nil {
		return fallback
	}
	setting, err := s.settings.FindByKey(ctx, key)
	if err != nil {
		if !errors.Is(err, systemsettings.ErrNotFound) {
			s.logger.Warn("load runtime setting", "key", key, "error", err)
		}
		return fallback
	}
	value, err := strconv.Atoi(strings.TrimSpace(setting.Value))
	if err != nil {
		s.logger.Warn("parse runtime setting", "key", key, "value", setting.Value, "error", err)
		return fallback
	}
	return value
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
