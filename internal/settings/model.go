package settings

import "time"

const (
	TypeInteger = "integer"
	TypeBoolean = "boolean"
	TypeString  = "string"
)

type Setting struct {
	Key         string
	Value       string
	ValueType   string
	Description string
	UpdatedBy   string
	UpdatedAt   time.Time
}

type SettingDTO struct {
	Key         string  `json:"key"`
	Value       string  `json:"value"`
	ValueType   string  `json:"valueType"`
	Description string  `json:"description"`
	UpdatedBy   *string `json:"updatedBy"`
	UpdatedAt   string  `json:"updatedAt"`
}

type SaveInput struct {
	Value string `json:"value"`
}

type Definition struct {
	Key         string
	Value       string
	ValueType   string
	Description string
}

func Defaults() []Definition {
	return []Definition{
		{Key: "scheduler.global_concurrency", Value: "4", ValueType: TypeInteger, Description: "Maximum number of platform-wide build tasks allowed to run concurrently."},
		{Key: "build.timeout_minutes", Value: "60", ValueType: TypeInteger, Description: "Default timeout budget for a build task execution."},
		{Key: "retention.context_days", Value: "7", ValueType: TypeInteger, Description: "Days to retain generated build contexts after task completion."},
		{Key: "retention.log_days", Value: "30", ValueType: TypeInteger, Description: "Days to retain build log files after task completion."},
		{Key: "security.allow_insecure_registries", Value: "false", ValueType: TypeBoolean, Description: "Whether operators may configure registries with TLS verification disabled or HTTP transport."},
	}
}

func ToDTO(setting Setting) SettingDTO {
	return SettingDTO{
		Key:         setting.Key,
		Value:       setting.Value,
		ValueType:   setting.ValueType,
		Description: setting.Description,
		UpdatedBy:   stringPtr(setting.UpdatedBy),
		UpdatedAt:   setting.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func stringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
