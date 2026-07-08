package imageproject

import "time"

const (
	ImageTypeJava       = "java"
	ImageTypePython     = "python"
	ImageTypeNodeJS     = "nodejs"
	ImageTypeMySQL      = "mysql"
	ImageTypeBaseOS     = "base_os"
	ImageTypeDatabase   = "database"
	ImageTypeMiddleware = "middleware"
	ImageTypeOther      = "other"

	RootSourceExternalImage = "external_image"
	RootSourceInternal      = "internal_artifact"
	RootSourceVersionNode   = "version_node"

	ProjectStatusActive   = "active"
	ProjectStatusArchived = "archived"

	BranchStatusActive   = "active"
	BranchStatusArchived = "archived"

	NodeStatusDraft    = "draft"
	NodeStatusActive   = "active"
	NodeStatusArchived = "archived"

	MainBranchName = "main"
)

type Project struct {
	ID                  string
	Name                string
	ImageType           string
	ImageName           string
	Namespace           string
	RootImageRef        string
	RootImageSource     string
	SourceProjectID     string
	SourceVersionNodeID string
	DefaultRegistryID   string
	DefaultArchitecture string
	Labels              []string
	Description         string
	Status              string
	OwnerID             string
	LatestVersionNodeID string
	LatestBuildTaskID   string
	LatestVersion       string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type Branch struct {
	ID          string
	ProjectID   string
	Name        string
	StartNodeID string
	HeadNodeID  string
	Description string
	Status      string
	CreatedBy   string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type VersionNode struct {
	ID                 string
	ProjectID          string
	BranchID           string
	BranchName         string
	ParentNodeID       string
	Version            string
	Dockerfile         string
	DockerfileHash     string
	FormConfigSnapshot string
	BuildContextRef    string
	Description        string
	Status             string
	LatestBuildTaskID  string
	LatestArtifactID   string
	GraphPosition      string
	CreatedBy          string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type ProjectDTO struct {
	ID                  string   `json:"id"`
	Name                string   `json:"name"`
	ImageType           string   `json:"imageType"`
	ImageName           string   `json:"imageName"`
	Namespace           *string  `json:"namespace"`
	RootImageRef        string   `json:"rootImageRef"`
	RootImageSource     string   `json:"rootImageSource"`
	DefaultRegistryID   *string  `json:"defaultRegistryId"`
	DefaultArchitecture string   `json:"defaultArchitecture"`
	Labels              []string `json:"labels"`
	Description         *string  `json:"description"`
	Status              string   `json:"status"`
	OwnerID             *string  `json:"ownerId"`
	LatestVersionNodeID *string  `json:"latestVersionNodeId"`
	LatestVersion       *string  `json:"latestVersion"`
	CreatedAt           string   `json:"createdAt"`
	UpdatedAt           string   `json:"updatedAt"`
}

type BranchDTO struct {
	ID          string  `json:"id"`
	ProjectID   string  `json:"projectId"`
	Name        string  `json:"name"`
	StartNodeID *string `json:"startNodeId"`
	HeadNodeID  *string `json:"headNodeId"`
	Description *string `json:"description"`
	Status      string  `json:"status"`
	CreatedBy   *string `json:"createdBy"`
	CreatedAt   string  `json:"createdAt"`
	UpdatedAt   string  `json:"updatedAt"`
}

type VersionNodeDTO struct {
	ID                 string  `json:"id"`
	ProjectID          string  `json:"projectId"`
	BranchID           string  `json:"branchId"`
	BranchName         string  `json:"branchName"`
	ParentNodeID       *string `json:"parentNodeId"`
	Version            string  `json:"version"`
	Dockerfile         string  `json:"dockerfile"`
	DockerfileHash     string  `json:"dockerfileHash"`
	FormConfigSnapshot *string `json:"formConfigSnapshot"`
	BuildContextRef    *string `json:"buildContextRef"`
	Description        *string `json:"description"`
	Status             string  `json:"status"`
	LatestBuildTaskID  *string `json:"latestBuildTaskId"`
	LatestArtifactID   *string `json:"latestArtifactId"`
	GraphPosition      *string `json:"graphPosition"`
	CreatedBy          *string `json:"createdBy"`
	CreatedAt          string  `json:"createdAt"`
	UpdatedAt          string  `json:"updatedAt"`
}

type ProjectInput struct {
	Name                string   `json:"name"`
	ImageType           string   `json:"imageType"`
	ImageName           string   `json:"imageName"`
	Namespace           string   `json:"namespace"`
	RootImageRef        string   `json:"rootImageRef"`
	RootImageSource     string   `json:"rootImageSource"`
	DefaultRegistryID   string   `json:"defaultRegistryId"`
	DefaultArchitecture string   `json:"defaultArchitecture"`
	Labels              []string `json:"labels"`
	Description         string   `json:"description"`
}

type BranchInput struct {
	Name        string `json:"name"`
	StartNodeID string `json:"startNodeId"`
	Description string `json:"description"`
}

type VersionNodeInput struct {
	BranchID           string `json:"branchId"`
	ParentNodeID       string `json:"parentNodeId"`
	Version            string `json:"version"`
	Dockerfile         string `json:"dockerfile"`
	FormConfigSnapshot string `json:"formConfigSnapshot"`
	Description        string `json:"description"`
	Status             string `json:"status"`
}

type ProjectFilter struct {
	ImageType string
	Status    string
	Keyword   string
	Page      int
	PageSize  int
}

type GraphFilter struct {
	Branch string
	Status string
}

type VersionGraph struct {
	Project  ProjectDTO       `json:"project"`
	Branches []BranchDTO      `json:"branches"`
	Nodes    []VersionNodeDTO `json:"nodes"`
	Edges    []GraphEdge      `json:"edges"`
}

type GraphEdge struct {
	ID          string `json:"id"`
	Source      string `json:"source"`
	Target      string `json:"target"`
	SourceLabel string `json:"sourceLabel,omitempty"`
	TargetLabel string `json:"targetLabel,omitempty"`
}

type DockerfileDiff struct {
	LeftNodeID      string `json:"leftNodeId"`
	RightNodeID     string `json:"rightNodeId"`
	LeftDockerfile  string `json:"leftDockerfile"`
	RightDockerfile string `json:"rightDockerfile"`
	UnifiedDiff     string `json:"unifiedDiff"`
}

func ToProjectDTO(project Project) ProjectDTO {
	return ProjectDTO{
		ID:                  project.ID,
		Name:                project.Name,
		ImageType:           project.ImageType,
		ImageName:           project.ImageName,
		Namespace:           stringPtr(project.Namespace),
		RootImageRef:        project.RootImageRef,
		RootImageSource:     project.RootImageSource,
		DefaultRegistryID:   stringPtr(project.DefaultRegistryID),
		DefaultArchitecture: project.DefaultArchitecture,
		Labels:              project.Labels,
		Description:         stringPtr(project.Description),
		Status:              project.Status,
		OwnerID:             stringPtr(project.OwnerID),
		LatestVersionNodeID: stringPtr(project.LatestVersionNodeID),
		LatestVersion:       stringPtr(project.LatestVersion),
		CreatedAt:           project.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:           project.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func ToBranchDTO(branch Branch) BranchDTO {
	return BranchDTO{
		ID:          branch.ID,
		ProjectID:   branch.ProjectID,
		Name:        branch.Name,
		StartNodeID: stringPtr(branch.StartNodeID),
		HeadNodeID:  stringPtr(branch.HeadNodeID),
		Description: stringPtr(branch.Description),
		Status:      branch.Status,
		CreatedBy:   stringPtr(branch.CreatedBy),
		CreatedAt:   branch.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:   branch.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func ToVersionNodeDTO(node VersionNode) VersionNodeDTO {
	return VersionNodeDTO{
		ID:                 node.ID,
		ProjectID:          node.ProjectID,
		BranchID:           node.BranchID,
		BranchName:         node.BranchName,
		ParentNodeID:       stringPtr(node.ParentNodeID),
		Version:            node.Version,
		Dockerfile:         node.Dockerfile,
		DockerfileHash:     node.DockerfileHash,
		FormConfigSnapshot: stringPtr(node.FormConfigSnapshot),
		BuildContextRef:    stringPtr(node.BuildContextRef),
		Description:        stringPtr(node.Description),
		Status:             node.Status,
		LatestBuildTaskID:  stringPtr(node.LatestBuildTaskID),
		LatestArtifactID:   stringPtr(node.LatestArtifactID),
		GraphPosition:      stringPtr(node.GraphPosition),
		CreatedBy:          stringPtr(node.CreatedBy),
		CreatedAt:          node.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:          node.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func stringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
