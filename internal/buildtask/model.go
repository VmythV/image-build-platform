package buildtask

import "time"

const (
	StatusCreated          = "created"
	StatusQueued           = "queued"
	StatusDispatching      = "dispatching"
	StatusPreparingContext = "preparing_context"
	StatusBuilding         = "building"
	StatusBuildSuccess     = "build_success"
	StatusPushing          = "pushing"
	StatusPushSuccess      = "push_success"

	StatusPreparingContextFailed = "preparing_context_failed"
	StatusDispatchFailed         = "dispatch_failed"
	StatusBuildFailed            = "build_failed"
	StatusPushFailed             = "push_failed"
	StatusCanceled               = "canceled"
	StatusTimeout                = "timeout"
)

type BuildTask struct {
	ID                 string
	ProjectID          string
	ProjectName        string
	VersionNodeID      string
	Version            string
	RetryOfTaskID      string
	HostID             string
	HostName           string
	RequestedHostID    string
	RequestedHostName  string
	RegistryID         string
	RegistryName       string
	ImageName          string
	ImageTag           string
	ImageRef           string
	Architecture       string
	DockerfileSnapshot string
	DockerfileHash     string
	BuildContextRef    string
	BuildArgs          map[string]string
	BuildOptions       map[string]string
	SchedulerReason    string
	Status             string
	ErrorCode          string
	ErrorMessage       string
	LogPath            string
	QueuedAt           *time.Time
	StartedAt          *time.Time
	BuildStartedAt     *time.Time
	BuildFinishedAt    *time.Time
	PushStartedAt      *time.Time
	FinishedAt         *time.Time
	DurationSeconds    *int64
	CreatedBy          string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type BuildTaskDTO struct {
	ID                 string            `json:"id"`
	ProjectID          string            `json:"projectId"`
	ProjectName        string            `json:"projectName"`
	VersionNodeID      string            `json:"versionNodeId"`
	Version            string            `json:"version"`
	RetryOfTaskID      *string           `json:"retryOfTaskId"`
	HostID             *string           `json:"hostId"`
	HostName           *string           `json:"hostName"`
	RequestedHostID    *string           `json:"requestedHostId"`
	RequestedHostName  *string           `json:"requestedHostName"`
	RegistryID         string            `json:"registryId"`
	RegistryName       string            `json:"registryName"`
	ImageName          string            `json:"imageName"`
	ImageTag           string            `json:"imageTag"`
	ImageRef           string            `json:"imageRef"`
	Architecture       string            `json:"architecture"`
	DockerfileSnapshot string            `json:"dockerfileSnapshot"`
	DockerfileHash     string            `json:"dockerfileHash"`
	BuildContextRef    *string           `json:"buildContextRef"`
	BuildArgs          map[string]string `json:"buildArgs"`
	BuildOptions       map[string]string `json:"buildOptions"`
	SchedulerReason    *string           `json:"schedulerReason"`
	Status             string            `json:"status"`
	ErrorCode          *string           `json:"errorCode"`
	ErrorMessage       *string           `json:"errorMessage"`
	LogPath            *string           `json:"logPath"`
	QueuedAt           *string           `json:"queuedAt"`
	StartedAt          *string           `json:"startedAt"`
	BuildStartedAt     *string           `json:"buildStartedAt"`
	BuildFinishedAt    *string           `json:"buildFinishedAt"`
	PushStartedAt      *string           `json:"pushStartedAt"`
	FinishedAt         *string           `json:"finishedAt"`
	DurationSeconds    *int64            `json:"durationSeconds"`
	CreatedBy          *string           `json:"createdBy"`
	CreatedAt          string            `json:"createdAt"`
	UpdatedAt          string            `json:"updatedAt"`
}

type CreateInput struct {
	ProjectID       string            `json:"projectId"`
	VersionNodeID   string            `json:"versionNodeId"`
	RegistryID      string            `json:"registryId"`
	RequestedHostID string            `json:"requestedHostId"`
	ImageName       string            `json:"imageName"`
	ImageTag        string            `json:"imageTag"`
	Architecture    string            `json:"architecture"`
	BuildArgs       map[string]string `json:"buildArgs"`
	BuildOptions    map[string]string `json:"buildOptions"`
}

type ListFilter struct {
	Status        string
	ProjectID     string
	VersionNodeID string
	HostID        string
	RegistryID    string
	Page          int
	PageSize      int
}

type DispatchResult struct {
	Task       BuildTaskDTO `json:"task"`
	Dispatched bool         `json:"dispatched"`
	Reason     string       `json:"reason"`
}

func ToDTO(task BuildTask) BuildTaskDTO {
	return BuildTaskDTO{
		ID:                 task.ID,
		ProjectID:          task.ProjectID,
		ProjectName:        task.ProjectName,
		VersionNodeID:      task.VersionNodeID,
		Version:            task.Version,
		RetryOfTaskID:      stringPtr(task.RetryOfTaskID),
		HostID:             stringPtr(task.HostID),
		HostName:           stringPtr(task.HostName),
		RequestedHostID:    stringPtr(task.RequestedHostID),
		RequestedHostName:  stringPtr(task.RequestedHostName),
		RegistryID:         task.RegistryID,
		RegistryName:       task.RegistryName,
		ImageName:          task.ImageName,
		ImageTag:           task.ImageTag,
		ImageRef:           task.ImageRef,
		Architecture:       task.Architecture,
		DockerfileSnapshot: task.DockerfileSnapshot,
		DockerfileHash:     task.DockerfileHash,
		BuildContextRef:    stringPtr(task.BuildContextRef),
		BuildArgs:          normalizeMap(task.BuildArgs),
		BuildOptions:       normalizeMap(task.BuildOptions),
		SchedulerReason:    stringPtr(task.SchedulerReason),
		Status:             task.Status,
		ErrorCode:          stringPtr(task.ErrorCode),
		ErrorMessage:       stringPtr(task.ErrorMessage),
		LogPath:            stringPtr(task.LogPath),
		QueuedAt:           timePtr(task.QueuedAt),
		StartedAt:          timePtr(task.StartedAt),
		BuildStartedAt:     timePtr(task.BuildStartedAt),
		BuildFinishedAt:    timePtr(task.BuildFinishedAt),
		PushStartedAt:      timePtr(task.PushStartedAt),
		FinishedAt:         timePtr(task.FinishedAt),
		DurationSeconds:    task.DurationSeconds,
		CreatedBy:          stringPtr(task.CreatedBy),
		CreatedAt:          task.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:          task.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func stringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func timePtr(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.UTC().Format(time.RFC3339)
	return &formatted
}

func normalizeMap(value map[string]string) map[string]string {
	if value == nil {
		return map[string]string{}
	}
	return value
}
