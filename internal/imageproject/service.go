package imageproject

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/VmythV/image-build-platform/internal/platform/clock"
	"github.com/VmythV/image-build-platform/internal/platform/id"
)

var ErrValidation = errors.New("image project validation failed")

type Service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return Service{repo: repo}
}

func (s Service) ListProjects(ctx context.Context, filter ProjectFilter) ([]Project, int, error) {
	return s.repo.ListProjects(ctx, filter)
}

func (s Service) GetProject(ctx context.Context, projectID string) (Project, error) {
	return s.repo.FindProject(ctx, projectID)
}

func (s Service) CreateProject(ctx context.Context, input ProjectInput, actorID string) (Project, error) {
	project, err := normalizeProjectInput(input)
	if err != nil {
		return Project{}, err
	}

	now := clock.Now()
	project.ID = id.New()
	project.Status = ProjectStatusActive
	project.OwnerID = actorID
	project.CreatedAt = now
	project.UpdatedAt = now

	branch := Branch{
		ID:        id.New(),
		ProjectID: project.ID,
		Name:      MainBranchName,
		Status:    BranchStatusActive,
		CreatedBy: actorID,
		CreatedAt: now,
		UpdatedAt: now,
	}
	rootDockerfile := "FROM " + project.RootImageRef + "\n"
	rootNode := VersionNode{
		ID:             id.New(),
		ProjectID:      project.ID,
		BranchID:       branch.ID,
		Version:        "root",
		Dockerfile:     rootDockerfile,
		DockerfileHash: dockerfileHash(rootDockerfile),
		Description:    "Initial version from " + project.RootImageRef,
		Status:         NodeStatusActive,
		GraphPosition:  `{"x":80,"y":120}`,
		CreatedBy:      actorID,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := s.repo.CreateProjectWithInitial(ctx, project, branch, rootNode); err != nil {
		return Project{}, err
	}

	return s.repo.FindProject(ctx, project.ID)
}

func (s Service) UpdateProject(ctx context.Context, projectID string, input ProjectInput) (Project, error) {
	existing, err := s.repo.FindProject(ctx, projectID)
	if err != nil {
		return Project{}, err
	}

	updated, err := normalizeProjectInput(input)
	if err != nil {
		return Project{}, err
	}
	updated.ID = existing.ID
	updated.RootImageSource = existing.RootImageSource
	updated.SourceProjectID = existing.SourceProjectID
	updated.SourceVersionNodeID = existing.SourceVersionNodeID
	updated.Status = existing.Status
	updated.OwnerID = existing.OwnerID
	updated.LatestVersionNodeID = existing.LatestVersionNodeID
	updated.LatestBuildTaskID = existing.LatestBuildTaskID
	updated.CreatedAt = existing.CreatedAt
	updated.UpdatedAt = clock.Now()

	if err := s.repo.UpdateProject(ctx, updated); err != nil {
		return Project{}, err
	}
	return s.repo.FindProject(ctx, projectID)
}

func (s Service) ArchiveProject(ctx context.Context, projectID string) (Project, error) {
	if err := s.repo.ArchiveProject(ctx, projectID, clock.Now()); err != nil {
		return Project{}, err
	}
	return s.repo.FindProject(ctx, projectID)
}

func (s Service) Graph(ctx context.Context, projectID string, filter GraphFilter) (VersionGraph, error) {
	project, err := s.repo.FindProject(ctx, projectID)
	if err != nil {
		return VersionGraph{}, err
	}
	branches, err := s.repo.ListBranches(ctx, projectID)
	if err != nil {
		return VersionGraph{}, err
	}
	nodes, err := s.repo.ListNodes(ctx, projectID, filter)
	if err != nil {
		return VersionGraph{}, err
	}

	branchDTOs := make([]BranchDTO, 0, len(branches))
	for _, branch := range branches {
		branchDTOs = append(branchDTOs, ToBranchDTO(branch))
	}
	nodeDTOs := make([]VersionNodeDTO, 0, len(nodes))
	edges := make([]GraphEdge, 0, len(nodes))
	for _, node := range nodes {
		nodeDTOs = append(nodeDTOs, ToVersionNodeDTO(node))
		if node.ParentNodeID != "" {
			edges = append(edges, GraphEdge{
				ID:          node.ParentNodeID + "->" + node.ID,
				Source:      node.ParentNodeID,
				Target:      node.ID,
				TargetLabel: node.BranchName,
			})
		}
	}

	return VersionGraph{
		Project:  ToProjectDTO(project),
		Branches: branchDTOs,
		Nodes:    nodeDTOs,
		Edges:    edges,
	}, nil
}

func (s Service) ListBranches(ctx context.Context, projectID string) ([]Branch, error) {
	if _, err := s.repo.FindProject(ctx, projectID); err != nil {
		return nil, err
	}
	return s.repo.ListBranches(ctx, projectID)
}

func (s Service) CreateBranch(ctx context.Context, projectID string, input BranchInput, actorID string) (Branch, error) {
	if _, err := s.repo.FindProject(ctx, projectID); err != nil {
		return Branch{}, err
	}
	input.Name = strings.TrimSpace(input.Name)
	input.StartNodeID = strings.TrimSpace(input.StartNodeID)
	input.Description = strings.TrimSpace(input.Description)
	if input.Name == "" {
		return Branch{}, validationError("name is required")
	}
	if input.StartNodeID == "" {
		return Branch{}, validationError("startNodeId is required")
	}
	if _, err := s.repo.FindNode(ctx, projectID, input.StartNodeID); err != nil {
		return Branch{}, err
	}

	now := clock.Now()
	branch := Branch{
		ID:          id.New(),
		ProjectID:   projectID,
		Name:        input.Name,
		StartNodeID: input.StartNodeID,
		HeadNodeID:  input.StartNodeID,
		Description: input.Description,
		Status:      BranchStatusActive,
		CreatedBy:   actorID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.repo.CreateBranch(ctx, branch); err != nil {
		return Branch{}, err
	}
	return s.repo.FindBranch(ctx, projectID, branch.ID)
}

func (s Service) ArchiveBranch(ctx context.Context, projectID string, branchID string) (Branch, error) {
	if err := s.repo.ArchiveBranch(ctx, projectID, branchID, clock.Now()); err != nil {
		return Branch{}, err
	}
	return s.repo.FindBranch(ctx, projectID, branchID)
}

func (s Service) CreateNode(ctx context.Context, projectID string, input VersionNodeInput, actorID string) (VersionNode, error) {
	branch, err := s.repo.FindBranch(ctx, projectID, strings.TrimSpace(input.BranchID))
	if err != nil {
		return VersionNode{}, err
	}

	node, err := normalizeNodeInput(input)
	if err != nil {
		return VersionNode{}, err
	}
	parentID := strings.TrimSpace(input.ParentNodeID)
	if parentID == "" {
		parentID = branch.HeadNodeID
	}
	if parentID == "" {
		return VersionNode{}, validationError("parentNodeId is required")
	}
	if _, err := s.repo.FindNode(ctx, projectID, parentID); err != nil {
		return VersionNode{}, err
	}

	now := clock.Now()
	node.ID = id.New()
	node.ProjectID = projectID
	node.BranchID = branch.ID
	node.ParentNodeID = parentID
	node.DockerfileHash = dockerfileHash(node.Dockerfile)
	node.CreatedBy = actorID
	node.CreatedAt = now
	node.UpdatedAt = now

	if err := s.repo.CreateNode(ctx, node); err != nil {
		return VersionNode{}, err
	}
	return s.repo.FindNode(ctx, projectID, node.ID)
}

func (s Service) GetNode(ctx context.Context, projectID string, nodeID string) (VersionNode, error) {
	return s.repo.FindNode(ctx, projectID, nodeID)
}

func (s Service) UpdateNode(ctx context.Context, projectID string, nodeID string, input VersionNodeInput) (VersionNode, error) {
	existing, err := s.repo.FindNode(ctx, projectID, nodeID)
	if err != nil {
		return VersionNode{}, err
	}
	updated, err := normalizeNodeInput(input)
	if err != nil {
		return VersionNode{}, err
	}
	updated.ID = existing.ID
	updated.ProjectID = existing.ProjectID
	updated.BranchID = existing.BranchID
	updated.ParentNodeID = existing.ParentNodeID
	updated.DockerfileHash = dockerfileHash(updated.Dockerfile)
	updated.CreatedBy = existing.CreatedBy
	updated.CreatedAt = existing.CreatedAt
	updated.UpdatedAt = clock.Now()

	if err := s.repo.UpdateNode(ctx, updated); err != nil {
		return VersionNode{}, err
	}
	return s.repo.FindNode(ctx, projectID, nodeID)
}

func (s Service) DiffNodes(ctx context.Context, projectID string, leftNodeID string, rightNodeID string) (DockerfileDiff, error) {
	left, err := s.repo.FindNode(ctx, projectID, leftNodeID)
	if err != nil {
		return DockerfileDiff{}, err
	}
	right, err := s.repo.FindNode(ctx, projectID, rightNodeID)
	if err != nil {
		return DockerfileDiff{}, err
	}
	return DockerfileDiff{
		LeftNodeID:      left.ID,
		RightNodeID:     right.ID,
		LeftDockerfile:  left.Dockerfile,
		RightDockerfile: right.Dockerfile,
		UnifiedDiff:     unifiedDiff(left.Version, left.Dockerfile, right.Version, right.Dockerfile),
	}, nil
}

func normalizeProjectInput(input ProjectInput) (Project, error) {
	project := Project{
		Name:                strings.TrimSpace(input.Name),
		ImageType:           strings.TrimSpace(input.ImageType),
		ImageName:           strings.TrimSpace(input.ImageName),
		Namespace:           strings.Trim(strings.TrimSpace(input.Namespace), "/"),
		RootImageRef:        strings.TrimSpace(input.RootImageRef),
		RootImageSource:     strings.TrimSpace(input.RootImageSource),
		DefaultRegistryID:   strings.TrimSpace(input.DefaultRegistryID),
		DefaultArchitecture: strings.TrimSpace(input.DefaultArchitecture),
		Labels:              normalizeLabels(input.Labels),
		Description:         strings.TrimSpace(input.Description),
	}
	if project.Name == "" {
		return Project{}, validationError("name is required")
	}
	if project.ImageType == "" {
		project.ImageType = ImageTypeOther
	}
	if !validImageType(project.ImageType) {
		return Project{}, validationError("imageType is invalid")
	}
	if project.ImageName == "" {
		return Project{}, validationError("imageName is required")
	}
	if project.RootImageRef == "" {
		return Project{}, validationError("rootImageRef is required")
	}
	if project.RootImageSource == "" {
		project.RootImageSource = RootSourceExternalImage
	}
	if project.RootImageSource != RootSourceExternalImage {
		return Project{}, validationError("rootImageSource currently supports external_image only")
	}
	if project.DefaultArchitecture == "" {
		project.DefaultArchitecture = "amd64"
	}
	return project, nil
}

func normalizeNodeInput(input VersionNodeInput) (VersionNode, error) {
	node := VersionNode{
		BranchID:           strings.TrimSpace(input.BranchID),
		Version:            strings.TrimSpace(input.Version),
		Dockerfile:         strings.TrimSpace(input.Dockerfile),
		FormConfigSnapshot: strings.TrimSpace(input.FormConfigSnapshot),
		Description:        strings.TrimSpace(input.Description),
		Status:             strings.TrimSpace(input.Status),
	}
	if node.BranchID == "" {
		return VersionNode{}, validationError("branchId is required")
	}
	if node.Version == "" {
		return VersionNode{}, validationError("version is required")
	}
	if node.Dockerfile == "" {
		return VersionNode{}, validationError("dockerfile is required")
	}
	if !hasDockerfileFrom(node.Dockerfile) {
		return VersionNode{}, validationError("dockerfile must contain a FROM instruction")
	}
	if node.Status == "" {
		node.Status = NodeStatusActive
	}
	if node.Status != NodeStatusDraft && node.Status != NodeStatusActive && node.Status != NodeStatusArchived {
		return VersionNode{}, validationError("status is invalid")
	}
	if !strings.HasSuffix(node.Dockerfile, "\n") {
		node.Dockerfile += "\n"
	}
	return node, nil
}

func hasDockerfileFrom(dockerfile string) bool {
	for _, line := range strings.Split(dockerfile, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToUpper(line), "FROM ") {
			return true
		}
	}
	return false
}

func dockerfileHash(dockerfile string) string {
	sum := sha256.Sum256([]byte(dockerfile))
	return hex.EncodeToString(sum[:])
}

func validImageType(value string) bool {
	switch value {
	case ImageTypeJava, ImageTypePython, ImageTypeNodeJS, ImageTypeMySQL, ImageTypeBaseOS, ImageTypeDatabase, ImageTypeMiddleware, ImageTypeOther:
		return true
	default:
		return false
	}
}

func normalizeLabels(labels []string) []string {
	seen := make(map[string]struct{}, len(labels))
	normalized := make([]string, 0, len(labels))
	for _, label := range labels {
		label = strings.TrimSpace(label)
		if label == "" {
			continue
		}
		if _, ok := seen[label]; ok {
			continue
		}
		seen[label] = struct{}{}
		normalized = append(normalized, label)
	}
	return normalized
}

func validationError(message string) error {
	return fmt.Errorf("%w: %s", ErrValidation, message)
}

func unifiedDiff(leftName string, leftValue string, rightName string, rightValue string) string {
	leftLines := splitLines(leftValue)
	rightLines := splitLines(rightValue)
	maxLength := len(leftLines)
	if len(rightLines) > maxLength {
		maxLength = len(rightLines)
	}

	lines := []string{"--- " + leftName, "+++ " + rightName, "@@ @@"}
	for i := 0; i < maxLength; i++ {
		var leftLine, rightLine string
		if i < len(leftLines) {
			leftLine = leftLines[i]
		}
		if i < len(rightLines) {
			rightLine = rightLines[i]
		}
		switch {
		case i >= len(leftLines):
			lines = append(lines, "+"+rightLine)
		case i >= len(rightLines):
			lines = append(lines, "-"+leftLine)
		case leftLine == rightLine:
			lines = append(lines, " "+leftLine)
		default:
			lines = append(lines, "-"+leftLine, "+"+rightLine)
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

func splitLines(value string) []string {
	value = strings.TrimRight(value, "\n")
	if value == "" {
		return []string{}
	}
	return strings.Split(value, "\n")
}
