package audit

import "time"

type Log struct {
	ID           string
	ActorID      string
	ActorName    string
	Action       string
	ResourceType string
	ResourceID   string
	ResourceName string
	IPAddress    string
	UserAgent    string
	RequestID    string
	Detail       string
	CreatedAt    time.Time
}

type LogDTO struct {
	ID           string  `json:"id"`
	ActorID      *string `json:"actorId"`
	ActorName    *string `json:"actorName"`
	Action       string  `json:"action"`
	ResourceType string  `json:"resourceType"`
	ResourceID   *string `json:"resourceId"`
	ResourceName *string `json:"resourceName"`
	IPAddress    *string `json:"ipAddress"`
	UserAgent    *string `json:"userAgent"`
	RequestID    *string `json:"requestId"`
	Detail       *string `json:"detail"`
	CreatedAt    string  `json:"createdAt"`
}

type ListFilter struct {
	ActorID      string
	Action       string
	ResourceType string
	Page         int
	PageSize     int
}

type RecordInput struct {
	ActorID      string
	ActorName    string
	Action       string
	ResourceType string
	ResourceID   string
	ResourceName string
	IPAddress    string
	UserAgent    string
	RequestID    string
	Detail       string
}

func ToDTO(log Log) LogDTO {
	return LogDTO{
		ID:           log.ID,
		ActorID:      stringPtr(log.ActorID),
		ActorName:    stringPtr(log.ActorName),
		Action:       log.Action,
		ResourceType: log.ResourceType,
		ResourceID:   stringPtr(log.ResourceID),
		ResourceName: stringPtr(log.ResourceName),
		IPAddress:    stringPtr(log.IPAddress),
		UserAgent:    stringPtr(log.UserAgent),
		RequestID:    stringPtr(log.RequestID),
		Detail:       stringPtr(log.Detail),
		CreatedAt:    log.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func stringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
