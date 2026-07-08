package registry

import "time"

const (
	TypeGeneric      = "generic"
	TypeHarbor       = "harbor"
	TypeDockerHub    = "docker_hub"
	TypeAliyun       = "aliyun"
	TypeTencentCloud = "tencent_cloud"

	StatusUnknown     = "unknown"
	StatusAvailable   = "available"
	StatusUnavailable = "unavailable"
	StatusDisabled    = "disabled"
)

type Registry struct {
	ID                    string
	Name                  string
	Type                  string
	Endpoint              string
	Namespace             string
	Region                string
	CredentialID          string
	CredentialName        string
	CredentialFingerprint string
	AllowPull             bool
	AllowPush             bool
	IsDefaultPull         bool
	IsDefaultPush         bool
	TLSVerify             bool
	InsecureHTTP          bool
	Status                string
	LastCheckedAt         *time.Time
	LastCheckResult       string
	LastError             string
	CreatedBy             string
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

type RegistryDTO struct {
	ID                    string  `json:"id"`
	Name                  string  `json:"name"`
	Type                  string  `json:"type"`
	Endpoint              string  `json:"endpoint"`
	Namespace             *string `json:"namespace"`
	Region                *string `json:"region"`
	AllowPull             bool    `json:"allowPull"`
	AllowPush             bool    `json:"allowPush"`
	IsDefaultPull         bool    `json:"isDefaultPull"`
	IsDefaultPush         bool    `json:"isDefaultPush"`
	TLSVerify             bool    `json:"tlsVerify"`
	InsecureHTTP          bool    `json:"insecureHttp"`
	Status                string  `json:"status"`
	LastCheckedAt         *string `json:"lastCheckedAt"`
	LastError             *string `json:"lastError"`
	CredentialConfigured  bool    `json:"credentialConfigured"`
	CredentialUsername    *string `json:"credentialUsername"`
	CredentialFingerprint *string `json:"credentialFingerprint"`
	CreatedBy             *string `json:"createdBy"`
	CreatedAt             string  `json:"createdAt"`
	UpdatedAt             string  `json:"updatedAt"`
}

type SaveInput struct {
	Name          string `json:"name"`
	Type          string `json:"type"`
	Endpoint      string `json:"endpoint"`
	Namespace     string `json:"namespace"`
	Region        string `json:"region"`
	Username      string `json:"username"`
	Password      string `json:"password"`
	AllowPull     bool   `json:"allowPull"`
	AllowPush     bool   `json:"allowPush"`
	IsDefaultPull bool   `json:"isDefaultPull"`
	IsDefaultPush bool   `json:"isDefaultPush"`
	TLSVerify     bool   `json:"tlsVerify"`
	InsecureHTTP  bool   `json:"insecureHttp"`
}

type ListFilter struct {
	Status string
	Type   string
	Page   int
	Size   int
}

type RegistrySecret struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type CheckItem struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

type CheckResult struct {
	Status string     `json:"status"`
	Login  CheckItem  `json:"login"`
	Pull   CheckItem  `json:"pull"`
	Error  *string    `json:"error"`
	Checks []CheckRow `json:"checks,omitempty"`
}

type CheckRow struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

func ToDTO(registry Registry) RegistryDTO {
	var lastCheckedAt *string
	if registry.LastCheckedAt != nil {
		value := registry.LastCheckedAt.UTC().Format(time.RFC3339)
		lastCheckedAt = &value
	}

	return RegistryDTO{
		ID:                    registry.ID,
		Name:                  registry.Name,
		Type:                  registry.Type,
		Endpoint:              registry.Endpoint,
		Namespace:             stringPtr(registry.Namespace),
		Region:                stringPtr(registry.Region),
		AllowPull:             registry.AllowPull,
		AllowPush:             registry.AllowPush,
		IsDefaultPull:         registry.IsDefaultPull,
		IsDefaultPush:         registry.IsDefaultPush,
		TLSVerify:             registry.TLSVerify,
		InsecureHTTP:          registry.InsecureHTTP,
		Status:                registry.Status,
		LastCheckedAt:         lastCheckedAt,
		LastError:             stringPtr(registry.LastError),
		CredentialConfigured:  registry.CredentialID != "",
		CredentialUsername:    stringPtr(registry.CredentialName),
		CredentialFingerprint: stringPtr(registry.CredentialFingerprint),
		CreatedBy:             stringPtr(registry.CreatedBy),
		CreatedAt:             registry.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:             registry.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func stringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
