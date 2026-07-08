package buildhost

import "time"

const (
	ConnectionLocalDocker = "local_docker"
	ConnectionSSH         = "ssh"

	StatusUnknown     = "unknown"
	StatusOnline      = "online"
	StatusOffline     = "offline"
	StatusUnavailable = "unavailable"
	StatusDisabled    = "disabled"

	DefaultDockerEndpoint = "/var/run/docker.sock"
	DefaultDockerCommand  = "docker"
)

type BuildHost struct {
	ID                string
	Name              string
	ConnectionType    string
	Address           string
	Port              int
	Username          string
	CredentialID      string
	DockerEndpoint    string
	DockerCommand     string
	Architecture      string
	OS                string
	DockerVersion     string
	BuildkitSupported bool
	Labels            []string
	MaxConcurrency    int
	CurrentRunning    int
	Status            string
	LastCheckedAt     *time.Time
	LastCheckResult   string
	LastError         string
	CreatedBy         string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type BuildHostDTO struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	ConnectionType    string   `json:"connectionType"`
	Address           *string  `json:"address"`
	Port              *int     `json:"port"`
	Username          *string  `json:"username"`
	DockerEndpoint    *string  `json:"dockerEndpoint"`
	DockerCommand     *string  `json:"dockerCommand"`
	Architecture      *string  `json:"architecture"`
	OS                *string  `json:"os"`
	DockerVersion     *string  `json:"dockerVersion"`
	BuildkitSupported bool     `json:"buildkitSupported"`
	Labels            []string `json:"labels"`
	MaxConcurrency    int      `json:"maxConcurrency"`
	CurrentRunning    int      `json:"currentRunning"`
	Status            string   `json:"status"`
	LastCheckedAt     *string  `json:"lastCheckedAt"`
	LastError         *string  `json:"lastError"`
	CreatedBy         *string  `json:"createdBy"`
	CreatedAt         string   `json:"createdAt"`
	UpdatedAt         string   `json:"updatedAt"`
}

type SaveInput struct {
	Name           string   `json:"name"`
	ConnectionType string   `json:"connectionType"`
	Address        string   `json:"address"`
	Port           int      `json:"port"`
	Username       string   `json:"username"`
	DockerEndpoint string   `json:"dockerEndpoint"`
	DockerCommand  string   `json:"dockerCommand"`
	MaxConcurrency int      `json:"maxConcurrency"`
	Labels         []string `json:"labels"`
}

type ListFilter struct {
	Status         string
	Architecture   string
	ConnectionType string
	Page           int
	PageSize       int
}

type CheckItem struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type CheckResult struct {
	Status            string      `json:"status"`
	Architecture      string      `json:"architecture,omitempty"`
	OS                string      `json:"os,omitempty"`
	DockerVersion     string      `json:"dockerVersion,omitempty"`
	BuildkitSupported bool        `json:"buildkitSupported"`
	DiskFreeBytes     *int64      `json:"diskFreeBytes,omitempty"`
	Checks            []CheckItem `json:"checks"`
	Error             *string     `json:"error"`
}

func ToDTO(host BuildHost) BuildHostDTO {
	var lastCheckedAt *string
	if host.LastCheckedAt != nil {
		value := host.LastCheckedAt.UTC().Format(time.RFC3339)
		lastCheckedAt = &value
	}

	return BuildHostDTO{
		ID:                host.ID,
		Name:              host.Name,
		ConnectionType:    host.ConnectionType,
		Address:           stringPtr(host.Address),
		Port:              intPtr(host.Port),
		Username:          stringPtr(host.Username),
		DockerEndpoint:    stringPtr(host.DockerEndpoint),
		DockerCommand:     stringPtr(host.DockerCommand),
		Architecture:      stringPtr(host.Architecture),
		OS:                stringPtr(host.OS),
		DockerVersion:     stringPtr(host.DockerVersion),
		BuildkitSupported: host.BuildkitSupported,
		Labels:            host.Labels,
		MaxConcurrency:    host.MaxConcurrency,
		CurrentRunning:    host.CurrentRunning,
		Status:            host.Status,
		LastCheckedAt:     lastCheckedAt,
		LastError:         stringPtr(host.LastError),
		CreatedBy:         stringPtr(host.CreatedBy),
		CreatedAt:         host.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:         host.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func stringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func intPtr(value int) *int {
	if value == 0 {
		return nil
	}
	return &value
}
