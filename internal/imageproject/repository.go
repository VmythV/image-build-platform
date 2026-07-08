package imageproject

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var ErrNotFound = errors.New("image project resource not found")

type Repository struct {
	db         *sql.DB
	driverName string
}

type sqlExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func NewRepository(db *sql.DB, driverName string) Repository {
	return Repository{db: db, driverName: driverName}
}

func (r Repository) CreateProjectWithInitial(ctx context.Context, project Project, branch Branch, node VersionNode) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin project creation: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if err = r.insertProject(ctx, tx, project); err != nil {
		return err
	}
	if err = r.insertBranch(ctx, tx, branch); err != nil {
		return err
	}
	if err = r.insertNode(ctx, tx, node); err != nil {
		return err
	}
	if err = r.updateBranchHead(ctx, tx, branch.ID, node.ID, node.ID, node.UpdatedAt); err != nil {
		return err
	}
	if err = r.updateProjectLatest(ctx, tx, project.ID, node.ID, node.UpdatedAt); err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit project creation: %w", err)
	}
	return nil
}

func (r Repository) ListProjects(ctx context.Context, filter ProjectFilter) ([]Project, int, error) {
	where, args := r.projectWhere(filter)
	var total int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM image_projects p "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count image projects: %w", err)
	}

	page := normalizePage(filter.Page)
	pageSize := normalizePageSize(filter.PageSize)
	offset := (page - 1) * pageSize
	args = append(args, pageSize, offset)

	query := selectProjectSQL + where + `
ORDER BY p.updated_at DESC
LIMIT ` + placeholder(r.driverName, len(args)-1) + ` OFFSET ` + placeholder(r.driverName, len(args))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list image projects: %w", err)
	}
	defer rows.Close()

	projects := make([]Project, 0)
	for rows.Next() {
		project, err := scanProject(rows)
		if err != nil {
			return nil, 0, err
		}
		projects = append(projects, project)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate image projects: %w", err)
	}
	return projects, total, nil
}

func (r Repository) FindProject(ctx context.Context, projectID string) (Project, error) {
	query := selectProjectSQL + `WHERE p.id = ` + placeholder(r.driverName, 1) + ` AND p.deleted_at IS NULL`
	project, err := scanProject(r.db.QueryRowContext(ctx, query, projectID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Project{}, ErrNotFound
		}
		return Project{}, err
	}
	return project, nil
}

func (r Repository) UpdateProject(ctx context.Context, project Project) error {
	labels, err := encodeLabels(project.Labels)
	if err != nil {
		return err
	}
	query := `
UPDATE image_projects
SET name = ` + placeholder(r.driverName, 1) + `,
    image_type = ` + placeholder(r.driverName, 2) + `,
    image_name = ` + placeholder(r.driverName, 3) + `,
    namespace = ` + placeholder(r.driverName, 4) + `,
    root_image_ref = ` + placeholder(r.driverName, 5) + `,
    default_registry_id = ` + placeholder(r.driverName, 6) + `,
    default_architecture = ` + placeholder(r.driverName, 7) + `,
    labels = ` + placeholder(r.driverName, 8) + `,
    description = ` + placeholder(r.driverName, 9) + `,
    updated_at = ` + placeholder(r.driverName, 10) + `
WHERE id = ` + placeholder(r.driverName, 11) + ` AND deleted_at IS NULL`

	result, err := r.db.ExecContext(ctx, query, project.Name, project.ImageType, project.ImageName, nullString(project.Namespace), project.RootImageRef, nullString(project.DefaultRegistryID), project.DefaultArchitecture, labels, nullString(project.Description), formatTime(project.UpdatedAt), project.ID)
	if err != nil {
		return fmt.Errorf("update image project: %w", err)
	}
	return requireRowsAffected(result)
}

func (r Repository) ArchiveProject(ctx context.Context, projectID string, archivedAt time.Time) error {
	query := `
UPDATE image_projects
SET status = ` + placeholder(r.driverName, 1) + `,
    updated_at = ` + placeholder(r.driverName, 2) + `
WHERE id = ` + placeholder(r.driverName, 3) + ` AND deleted_at IS NULL`
	result, err := r.db.ExecContext(ctx, query, ProjectStatusArchived, formatTime(archivedAt), projectID)
	if err != nil {
		return fmt.Errorf("archive image project: %w", err)
	}
	return requireRowsAffected(result)
}

func (r Repository) ListBranches(ctx context.Context, projectID string) ([]Branch, error) {
	query := selectBranchSQL + `
WHERE project_id = ` + placeholder(r.driverName, 1) + ` AND deleted_at IS NULL
ORDER BY name ASC`
	rows, err := r.db.QueryContext(ctx, query, projectID)
	if err != nil {
		return nil, fmt.Errorf("list image branches: %w", err)
	}
	defer rows.Close()

	branches := make([]Branch, 0)
	for rows.Next() {
		branch, err := scanBranch(rows)
		if err != nil {
			return nil, err
		}
		branches = append(branches, branch)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate image branches: %w", err)
	}
	return branches, nil
}

func (r Repository) FindBranch(ctx context.Context, projectID string, branchID string) (Branch, error) {
	query := selectBranchSQL + `
WHERE project_id = ` + placeholder(r.driverName, 1) + ` AND id = ` + placeholder(r.driverName, 2) + ` AND deleted_at IS NULL`
	branch, err := scanBranch(r.db.QueryRowContext(ctx, query, projectID, branchID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Branch{}, ErrNotFound
		}
		return Branch{}, err
	}
	return branch, nil
}

func (r Repository) CreateBranch(ctx context.Context, branch Branch) error {
	return r.insertBranch(ctx, r.db, branch)
}

func (r Repository) ArchiveBranch(ctx context.Context, projectID string, branchID string, archivedAt time.Time) error {
	query := `
UPDATE image_branches
SET status = ` + placeholder(r.driverName, 1) + `,
    updated_at = ` + placeholder(r.driverName, 2) + `
WHERE project_id = ` + placeholder(r.driverName, 3) + ` AND id = ` + placeholder(r.driverName, 4) + ` AND deleted_at IS NULL`
	result, err := r.db.ExecContext(ctx, query, BranchStatusArchived, formatTime(archivedAt), projectID, branchID)
	if err != nil {
		return fmt.Errorf("archive image branch: %w", err)
	}
	return requireRowsAffected(result)
}

func (r Repository) CreateNode(ctx context.Context, node VersionNode) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin version node creation: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if err = r.insertNode(ctx, tx, node); err != nil {
		return err
	}
	if err = r.updateBranchHead(ctx, tx, node.BranchID, "", node.ID, node.UpdatedAt); err != nil {
		return err
	}
	if err = r.updateProjectLatest(ctx, tx, node.ProjectID, node.ID, node.UpdatedAt); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit version node creation: %w", err)
	}
	return nil
}

func (r Repository) FindNode(ctx context.Context, projectID string, nodeID string) (VersionNode, error) {
	query := selectNodeSQL + `
WHERE n.project_id = ` + placeholder(r.driverName, 1) + ` AND n.id = ` + placeholder(r.driverName, 2) + ` AND n.deleted_at IS NULL`
	node, err := scanNode(r.db.QueryRowContext(ctx, query, projectID, nodeID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return VersionNode{}, ErrNotFound
		}
		return VersionNode{}, err
	}
	return node, nil
}

func (r Repository) ListNodes(ctx context.Context, projectID string, filter GraphFilter) ([]VersionNode, error) {
	args := []any{projectID}
	clauses := []string{"n.project_id = " + placeholder(r.driverName, 1), "n.deleted_at IS NULL"}
	if strings.TrimSpace(filter.Status) != "" {
		args = append(args, strings.TrimSpace(filter.Status))
		clauses = append(clauses, "n.status = "+placeholder(r.driverName, len(args)))
	}
	if strings.TrimSpace(filter.Branch) != "" {
		args = append(args, strings.TrimSpace(filter.Branch))
		clauses = append(clauses, "b.name = "+placeholder(r.driverName, len(args)))
	}

	query := selectNodeSQL + `
WHERE ` + strings.Join(clauses, " AND ") + `
ORDER BY n.created_at ASC, n.id ASC`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list version nodes: %w", err)
	}
	defer rows.Close()

	nodes := make([]VersionNode, 0)
	for rows.Next() {
		node, err := scanNode(rows)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate version nodes: %w", err)
	}
	return nodes, nil
}

func (r Repository) UpdateNode(ctx context.Context, node VersionNode) error {
	query := `
UPDATE image_version_nodes
SET version = ` + placeholder(r.driverName, 1) + `,
    dockerfile = ` + placeholder(r.driverName, 2) + `,
    dockerfile_hash = ` + placeholder(r.driverName, 3) + `,
    form_config_snapshot = ` + placeholder(r.driverName, 4) + `,
    description = ` + placeholder(r.driverName, 5) + `,
    status = ` + placeholder(r.driverName, 6) + `,
    updated_at = ` + placeholder(r.driverName, 7) + `
WHERE project_id = ` + placeholder(r.driverName, 8) + ` AND id = ` + placeholder(r.driverName, 9) + ` AND deleted_at IS NULL`
	result, err := r.db.ExecContext(ctx, query, node.Version, node.Dockerfile, node.DockerfileHash, nullString(node.FormConfigSnapshot), nullString(node.Description), node.Status, formatTime(node.UpdatedAt), node.ProjectID, node.ID)
	if err != nil {
		return fmt.Errorf("update version node: %w", err)
	}
	return requireRowsAffected(result)
}

func (r Repository) insertProject(ctx context.Context, exec sqlExecutor, project Project) error {
	labels, err := encodeLabels(project.Labels)
	if err != nil {
		return err
	}
	query := `
INSERT INTO image_projects (
	id, name, image_type, image_name, namespace, root_image_ref, root_image_source,
	source_project_id, source_version_node_id, default_registry_id, default_architecture,
	labels, description, status, owner_id, latest_version_node_id, latest_build_task_id,
	created_at, updated_at, deleted_at
) VALUES (` + placeholders(r.driverName, 20) + `)`
	_, err = exec.ExecContext(ctx, query, project.ID, project.Name, project.ImageType, project.ImageName, nullString(project.Namespace), project.RootImageRef, project.RootImageSource, nullString(project.SourceProjectID), nullString(project.SourceVersionNodeID), nullString(project.DefaultRegistryID), project.DefaultArchitecture, labels, nullString(project.Description), project.Status, nullString(project.OwnerID), nullString(project.LatestVersionNodeID), nullString(project.LatestBuildTaskID), formatTime(project.CreatedAt), formatTime(project.UpdatedAt), nil)
	if err != nil {
		return fmt.Errorf("create image project: %w", err)
	}
	return nil
}

func (r Repository) insertBranch(ctx context.Context, exec sqlExecutor, branch Branch) error {
	query := `
INSERT INTO image_branches (
	id, project_id, name, start_node_id, head_node_id, description, status,
	created_by, created_at, updated_at, deleted_at
) VALUES (` + placeholders(r.driverName, 11) + `)`
	_, err := exec.ExecContext(ctx, query, branch.ID, branch.ProjectID, branch.Name, nullString(branch.StartNodeID), nullString(branch.HeadNodeID), nullString(branch.Description), branch.Status, nullString(branch.CreatedBy), formatTime(branch.CreatedAt), formatTime(branch.UpdatedAt), nil)
	if err != nil {
		return fmt.Errorf("create image branch: %w", err)
	}
	return nil
}

func (r Repository) insertNode(ctx context.Context, exec sqlExecutor, node VersionNode) error {
	query := `
INSERT INTO image_version_nodes (
	id, project_id, branch_id, parent_node_id, version, dockerfile, dockerfile_hash,
	form_config_snapshot, build_context_ref, description, status, latest_build_task_id,
	latest_artifact_id, graph_position, created_by, created_at, updated_at, deleted_at
) VALUES (` + placeholders(r.driverName, 18) + `)`
	_, err := exec.ExecContext(ctx, query, node.ID, node.ProjectID, node.BranchID, nullString(node.ParentNodeID), node.Version, node.Dockerfile, node.DockerfileHash, nullString(node.FormConfigSnapshot), nullString(node.BuildContextRef), nullString(node.Description), node.Status, nullString(node.LatestBuildTaskID), nullString(node.LatestArtifactID), nullString(node.GraphPosition), nullString(node.CreatedBy), formatTime(node.CreatedAt), formatTime(node.UpdatedAt), nil)
	if err != nil {
		return fmt.Errorf("create version node: %w", err)
	}
	return nil
}

func (r Repository) updateBranchHead(ctx context.Context, exec sqlExecutor, branchID string, startNodeID string, headNodeID string, updatedAt time.Time) error {
	query := `
UPDATE image_branches
SET head_node_id = ` + placeholder(r.driverName, 1) + `,
    start_node_id = COALESCE(start_node_id, ` + placeholder(r.driverName, 2) + `),
    updated_at = ` + placeholder(r.driverName, 3) + `
WHERE id = ` + placeholder(r.driverName, 4)
	result, err := exec.ExecContext(ctx, query, nullString(headNodeID), nullString(startNodeID), formatTime(updatedAt), branchID)
	if err != nil {
		return fmt.Errorf("update branch head: %w", err)
	}
	return requireRowsAffected(result)
}

func (r Repository) updateProjectLatest(ctx context.Context, exec sqlExecutor, projectID string, nodeID string, updatedAt time.Time) error {
	query := `
UPDATE image_projects
SET latest_version_node_id = ` + placeholder(r.driverName, 1) + `,
    updated_at = ` + placeholder(r.driverName, 2) + `
WHERE id = ` + placeholder(r.driverName, 3)
	result, err := exec.ExecContext(ctx, query, nullString(nodeID), formatTime(updatedAt), projectID)
	if err != nil {
		return fmt.Errorf("update project latest node: %w", err)
	}
	return requireRowsAffected(result)
}

func (r Repository) projectWhere(filter ProjectFilter) (string, []any) {
	clauses := []string{"p.deleted_at IS NULL"}
	args := make([]any, 0, 3)
	if strings.TrimSpace(filter.Status) != "" {
		args = append(args, strings.TrimSpace(filter.Status))
		clauses = append(clauses, "p.status = "+placeholder(r.driverName, len(args)))
	}
	if strings.TrimSpace(filter.ImageType) != "" {
		args = append(args, strings.TrimSpace(filter.ImageType))
		clauses = append(clauses, "p.image_type = "+placeholder(r.driverName, len(args)))
	}
	if strings.TrimSpace(filter.Keyword) != "" {
		args = append(args, "%"+strings.ToLower(strings.TrimSpace(filter.Keyword))+"%")
		clauses = append(clauses, "(LOWER(p.name) LIKE "+placeholder(r.driverName, len(args))+" OR LOWER(p.image_name) LIKE "+placeholder(r.driverName, len(args))+")")
	}
	return "WHERE " + strings.Join(clauses, " AND "), args
}

const selectProjectSQL = `
SELECT p.id, p.name, p.image_type, p.image_name, p.namespace, p.root_image_ref, p.root_image_source,
       p.source_project_id, p.source_version_node_id, p.default_registry_id, p.default_architecture,
       p.labels, p.description, p.status, p.owner_id, p.latest_version_node_id, p.latest_build_task_id,
       latest.version, p.created_at, p.updated_at
FROM image_projects p
LEFT JOIN image_version_nodes latest ON latest.id = p.latest_version_node_id
`

const selectBranchSQL = `
SELECT id, project_id, name, start_node_id, head_node_id, description, status, created_by, created_at, updated_at
FROM image_branches
`

const selectNodeSQL = `
SELECT n.id, n.project_id, n.branch_id, b.name, n.parent_node_id, n.version, n.dockerfile,
       n.dockerfile_hash, n.form_config_snapshot, n.build_context_ref, n.description, n.status,
       n.latest_build_task_id, n.latest_artifact_id, n.graph_position, n.created_by, n.created_at, n.updated_at
FROM image_version_nodes n
JOIN image_branches b ON b.id = n.branch_id
`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanProject(row rowScanner) (Project, error) {
	var project Project
	var namespace, sourceProjectID, sourceVersionNodeID, defaultRegistryID, description, ownerID, latestVersionNodeID, latestBuildTaskID, latestVersion sql.NullString
	var labels string
	var createdAt, updatedAt string
	err := row.Scan(&project.ID, &project.Name, &project.ImageType, &project.ImageName, &namespace, &project.RootImageRef, &project.RootImageSource, &sourceProjectID, &sourceVersionNodeID, &defaultRegistryID, &project.DefaultArchitecture, &labels, &description, &project.Status, &ownerID, &latestVersionNodeID, &latestBuildTaskID, &latestVersion, &createdAt, &updatedAt)
	if err != nil {
		return Project{}, err
	}
	project.Namespace = namespace.String
	project.SourceProjectID = sourceProjectID.String
	project.SourceVersionNodeID = sourceVersionNodeID.String
	project.DefaultRegistryID = defaultRegistryID.String
	project.Description = description.String
	project.OwnerID = ownerID.String
	project.LatestVersionNodeID = latestVersionNodeID.String
	project.LatestBuildTaskID = latestBuildTaskID.String
	project.LatestVersion = latestVersion.String
	project.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	project.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	if err := json.Unmarshal([]byte(labels), &project.Labels); err != nil {
		project.Labels = []string{}
	}
	return project, nil
}

func scanBranch(row rowScanner) (Branch, error) {
	var branch Branch
	var startNodeID, headNodeID, description, createdBy sql.NullString
	var createdAt, updatedAt string
	err := row.Scan(&branch.ID, &branch.ProjectID, &branch.Name, &startNodeID, &headNodeID, &description, &branch.Status, &createdBy, &createdAt, &updatedAt)
	if err != nil {
		return Branch{}, err
	}
	branch.StartNodeID = startNodeID.String
	branch.HeadNodeID = headNodeID.String
	branch.Description = description.String
	branch.CreatedBy = createdBy.String
	branch.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	branch.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return branch, nil
}

func scanNode(row rowScanner) (VersionNode, error) {
	var node VersionNode
	var parentNodeID, formConfigSnapshot, buildContextRef, description, latestBuildTaskID, latestArtifactID, graphPosition, createdBy sql.NullString
	var createdAt, updatedAt string
	err := row.Scan(&node.ID, &node.ProjectID, &node.BranchID, &node.BranchName, &parentNodeID, &node.Version, &node.Dockerfile, &node.DockerfileHash, &formConfigSnapshot, &buildContextRef, &description, &node.Status, &latestBuildTaskID, &latestArtifactID, &graphPosition, &createdBy, &createdAt, &updatedAt)
	if err != nil {
		return VersionNode{}, err
	}
	node.ParentNodeID = parentNodeID.String
	node.FormConfigSnapshot = formConfigSnapshot.String
	node.BuildContextRef = buildContextRef.String
	node.Description = description.String
	node.LatestBuildTaskID = latestBuildTaskID.String
	node.LatestArtifactID = latestArtifactID.String
	node.GraphPosition = graphPosition.String
	node.CreatedBy = createdBy.String
	node.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	node.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return node, nil
}

func encodeLabels(labels []string) (string, error) {
	if labels == nil {
		labels = []string{}
	}
	data, err := json.Marshal(labels)
	if err != nil {
		return "", fmt.Errorf("encode labels: %w", err)
	}
	return string(data), nil
}

func requireRowsAffected(result sql.Result) error {
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil
	}
	if rowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func normalizePage(page int) int {
	if page < 1 {
		return 1
	}
	return page
}

func normalizePageSize(pageSize int) int {
	if pageSize < 1 {
		return 20
	}
	if pageSize > 100 {
		return 100
	}
	return pageSize
}

func placeholder(driverName string, position int) string {
	if driverName == "pgx" {
		return "$" + strconv.Itoa(position)
	}
	return "?"
}

func placeholders(driverName string, count int) string {
	values := make([]byte, 0, count*4)
	for i := 1; i <= count; i++ {
		if i > 1 {
			values = append(values, ',', ' ')
		}
		values = append(values, placeholder(driverName, i)...)
	}
	return string(values)
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339)
}

func nullString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
