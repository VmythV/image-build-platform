package imageartifact

import "time"

const (
	StatusAvailable = "available"
	StatusArchived  = "archived"

	PushStatusSuccess = "success"
	PushStatusFailed  = "failed"
)

type Artifact struct {
	ID            string
	BuildTaskID   string
	ProjectID     string
	ProjectName   string
	VersionNodeID string
	Version       string
	RegistryID    string
	RegistryName  string
	ImageRef      string
	ImageID       string
	Digest        string
	Tag           string
	Architecture  string
	SizeBytes     *int64
	Status        string
	Pushed        bool
	PushedAt      *time.Time
	Deprecated    bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type PushEvent struct {
	ID           string
	ArtifactID   string
	BuildTaskID  string
	RegistryID   string
	Status       string
	ErrorMessage string
	StartedAt    time.Time
	FinishedAt   *time.Time
	CreatedBy    string
	CreatedAt    time.Time
}

type PushEventDTO struct {
	ID           string  `json:"id"`
	ArtifactID   string  `json:"artifactId"`
	BuildTaskID  *string `json:"buildTaskId"`
	RegistryID   string  `json:"registryId"`
	Status       string  `json:"status"`
	ErrorMessage *string `json:"errorMessage"`
	StartedAt    string  `json:"startedAt"`
	FinishedAt   *string `json:"finishedAt"`
	CreatedBy    *string `json:"createdBy"`
	CreatedAt    string  `json:"createdAt"`
}

type RepushInput struct {
	RegistryID string `json:"registryId"`
}

type RepushResultDTO struct {
	Artifact ArtifactDTO  `json:"artifact"`
	Event    PushEventDTO `json:"event"`
	LogPath  *string      `json:"logPath"`
}

type PullCommandDTO struct {
	Command string `json:"command"`
}

type ArtifactDTO struct {
	ID            string  `json:"id"`
	BuildTaskID   string  `json:"buildTaskId"`
	ProjectID     string  `json:"projectId"`
	ProjectName   string  `json:"projectName"`
	VersionNodeID string  `json:"versionNodeId"`
	Version       string  `json:"version"`
	RegistryID    string  `json:"registryId"`
	RegistryName  string  `json:"registryName"`
	ImageRef      string  `json:"imageRef"`
	ImageID       *string `json:"imageId"`
	Digest        *string `json:"digest"`
	Tag           string  `json:"tag"`
	Architecture  string  `json:"architecture"`
	SizeBytes     *int64  `json:"sizeBytes"`
	Status        string  `json:"status"`
	Pushed        bool    `json:"pushed"`
	PushedAt      *string `json:"pushedAt"`
	Deprecated    bool    `json:"deprecated"`
	CreatedAt     string  `json:"createdAt"`
	UpdatedAt     string  `json:"updatedAt"`
}

type ListFilter struct {
	ProjectID  string
	RegistryID string
	Status     string
	Page       int
	PageSize   int
}

func ToDTO(artifact Artifact) ArtifactDTO {
	return ArtifactDTO{
		ID:            artifact.ID,
		BuildTaskID:   artifact.BuildTaskID,
		ProjectID:     artifact.ProjectID,
		ProjectName:   artifact.ProjectName,
		VersionNodeID: artifact.VersionNodeID,
		Version:       artifact.Version,
		RegistryID:    artifact.RegistryID,
		RegistryName:  artifact.RegistryName,
		ImageRef:      artifact.ImageRef,
		ImageID:       stringPtr(artifact.ImageID),
		Digest:        stringPtr(artifact.Digest),
		Tag:           artifact.Tag,
		Architecture:  artifact.Architecture,
		SizeBytes:     artifact.SizeBytes,
		Status:        artifact.Status,
		Pushed:        artifact.Pushed,
		PushedAt:      timePtr(artifact.PushedAt),
		Deprecated:    artifact.Deprecated,
		CreatedAt:     artifact.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:     artifact.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func EventToDTO(event PushEvent) PushEventDTO {
	return PushEventDTO{
		ID:           event.ID,
		ArtifactID:   event.ArtifactID,
		BuildTaskID:  stringPtr(event.BuildTaskID),
		RegistryID:   event.RegistryID,
		Status:       event.Status,
		ErrorMessage: stringPtr(event.ErrorMessage),
		StartedAt:    event.StartedAt.UTC().Format(time.RFC3339),
		FinishedAt:   timePtr(event.FinishedAt),
		CreatedBy:    stringPtr(event.CreatedBy),
		CreatedAt:    event.CreatedAt.UTC().Format(time.RFC3339),
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
