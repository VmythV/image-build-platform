package imageartifact

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrNotFound = errors.New("image artifact not found")

type Repository struct {
	db         *sql.DB
	driverName string
}

func NewRepository(db *sql.DB, driverName string) Repository {
	return Repository{db: db, driverName: driverName}
}

func (r Repository) List(ctx context.Context, filter ListFilter) ([]Artifact, int, error) {
	where, args := r.filterWhere(filter)
	var total int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM image_artifacts a "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count image artifacts: %w", err)
	}

	page := normalizePage(filter.Page)
	pageSize := normalizePageSize(filter.PageSize)
	offset := (page - 1) * pageSize
	args = append(args, pageSize, offset)

	query := selectArtifactSQL + where + `
ORDER BY a.created_at DESC
LIMIT ` + placeholder(r.driverName, len(args)-1) + ` OFFSET ` + placeholder(r.driverName, len(args))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list image artifacts: %w", err)
	}
	defer rows.Close()

	artifacts := make([]Artifact, 0)
	for rows.Next() {
		artifact, err := scanArtifact(rows)
		if err != nil {
			return nil, 0, err
		}
		artifacts = append(artifacts, artifact)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate image artifacts: %w", err)
	}
	return artifacts, total, nil
}

func (r Repository) FindByID(ctx context.Context, artifactID string) (Artifact, error) {
	query := selectArtifactSQL + `WHERE a.id = ` + placeholder(r.driverName, 1) + ` AND a.deleted_at IS NULL`
	artifact, err := scanArtifact(r.db.QueryRowContext(ctx, query, artifactID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Artifact{}, ErrNotFound
		}
		return Artifact{}, err
	}
	return artifact, nil
}

func (r Repository) RecordPushed(ctx context.Context, artifact Artifact, event PushEvent) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin image artifact creation: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	artifactQuery := `
INSERT INTO image_artifacts (
	id, build_task_id, project_id, version_node_id, registry_id, image_ref,
	image_id, digest, tag, architecture, size_bytes, status, pushed, pushed_at,
	deprecated, created_at, updated_at, deleted_at
) VALUES (` + placeholders(r.driverName, 18) + `)`
	_, err = tx.ExecContext(
		ctx,
		artifactQuery,
		artifact.ID,
		artifact.BuildTaskID,
		artifact.ProjectID,
		artifact.VersionNodeID,
		artifact.RegistryID,
		artifact.ImageRef,
		nullString(artifact.ImageID),
		nullString(artifact.Digest),
		artifact.Tag,
		artifact.Architecture,
		nullInt64Ptr(artifact.SizeBytes),
		artifact.Status,
		boolInt(artifact.Pushed),
		nullTime(artifact.PushedAt),
		boolInt(artifact.Deprecated),
		formatTime(artifact.CreatedAt),
		formatTime(artifact.UpdatedAt),
		nil,
	)
	if err != nil {
		return fmt.Errorf("create image artifact: %w", err)
	}

	eventQuery := `
INSERT INTO artifact_push_events (
	id, artifact_id, build_task_id, registry_id, status, error_message,
	started_at, finished_at, created_by, created_at
) VALUES (` + placeholders(r.driverName, 10) + `)`
	_, err = tx.ExecContext(
		ctx,
		eventQuery,
		event.ID,
		event.ArtifactID,
		nullString(event.BuildTaskID),
		event.RegistryID,
		event.Status,
		nullString(event.ErrorMessage),
		formatTime(event.StartedAt),
		nullTime(event.FinishedAt),
		nullString(event.CreatedBy),
		formatTime(event.CreatedAt),
	)
	if err != nil {
		return fmt.Errorf("create artifact push event: %w", err)
	}

	nodeQuery := `
UPDATE image_version_nodes
SET latest_artifact_id = ` + placeholder(r.driverName, 1) + `,
    updated_at = ` + placeholder(r.driverName, 2) + `
WHERE id = ` + placeholder(r.driverName, 3) + ` AND project_id = ` + placeholder(r.driverName, 4)
	if _, err = tx.ExecContext(ctx, nodeQuery, artifact.ID, formatTime(artifact.UpdatedAt), artifact.VersionNodeID, artifact.ProjectID); err != nil {
		return fmt.Errorf("update version node latest artifact: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit image artifact creation: %w", err)
	}
	return nil
}

func (r Repository) RecordPushEvent(ctx context.Context, event PushEvent) error {
	query := `
INSERT INTO artifact_push_events (
	id, artifact_id, build_task_id, registry_id, status, error_message,
	started_at, finished_at, created_by, created_at
) VALUES (` + placeholders(r.driverName, 10) + `)`
	_, err := r.db.ExecContext(
		ctx,
		query,
		event.ID,
		event.ArtifactID,
		nullString(event.BuildTaskID),
		event.RegistryID,
		event.Status,
		nullString(event.ErrorMessage),
		formatTime(event.StartedAt),
		nullTime(event.FinishedAt),
		nullString(event.CreatedBy),
		formatTime(event.CreatedAt),
	)
	if err != nil {
		return fmt.Errorf("create artifact push event: %w", err)
	}
	return nil
}

func (r Repository) Archive(ctx context.Context, artifactID string, archivedAt time.Time) error {
	query := `
UPDATE image_artifacts
SET status = ` + placeholder(r.driverName, 1) + `,
    updated_at = ` + placeholder(r.driverName, 2) + `
WHERE id = ` + placeholder(r.driverName, 3) + ` AND deleted_at IS NULL`
	result, err := r.db.ExecContext(ctx, query, StatusArchived, formatTime(archivedAt), artifactID)
	if err != nil {
		return fmt.Errorf("archive image artifact: %w", err)
	}
	return requireRowsAffected(result)
}

func (r Repository) Deprecate(ctx context.Context, artifactID string, deprecatedAt time.Time) error {
	query := `
UPDATE image_artifacts
SET deprecated = ` + placeholder(r.driverName, 1) + `,
    updated_at = ` + placeholder(r.driverName, 2) + `
WHERE id = ` + placeholder(r.driverName, 3) + ` AND deleted_at IS NULL`
	result, err := r.db.ExecContext(ctx, query, boolInt(true), formatTime(deprecatedAt), artifactID)
	if err != nil {
		return fmt.Errorf("deprecate image artifact: %w", err)
	}
	return requireRowsAffected(result)
}

func (r Repository) filterWhere(filter ListFilter) (string, []any) {
	clauses := []string{"a.deleted_at IS NULL"}
	args := make([]any, 0, 3)

	if strings.TrimSpace(filter.ProjectID) != "" {
		args = append(args, strings.TrimSpace(filter.ProjectID))
		clauses = append(clauses, "a.project_id = "+placeholder(r.driverName, len(args)))
	}
	if strings.TrimSpace(filter.RegistryID) != "" {
		args = append(args, strings.TrimSpace(filter.RegistryID))
		clauses = append(clauses, "a.registry_id = "+placeholder(r.driverName, len(args)))
	}
	if strings.TrimSpace(filter.Status) != "" {
		args = append(args, strings.TrimSpace(filter.Status))
		clauses = append(clauses, "a.status = "+placeholder(r.driverName, len(args)))
	}

	return "WHERE " + strings.Join(clauses, " AND "), args
}

const selectArtifactSQL = `
SELECT a.id, a.build_task_id, a.project_id, p.name, a.version_node_id, n.version,
       a.registry_id, r.name, a.image_ref, a.image_id, a.digest, a.tag, a.architecture,
       a.size_bytes, a.status, a.pushed, a.pushed_at, a.deprecated, a.created_at, a.updated_at
FROM image_artifacts a
JOIN image_projects p ON p.id = a.project_id
JOIN image_version_nodes n ON n.id = a.version_node_id
JOIN registries r ON r.id = a.registry_id
`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanArtifact(row rowScanner) (Artifact, error) {
	var artifact Artifact
	var imageID, digest sql.NullString
	var sizeBytes sql.NullInt64
	var pushedAt sql.NullString
	var createdAt, updatedAt string
	var pushed, deprecated int

	err := row.Scan(
		&artifact.ID,
		&artifact.BuildTaskID,
		&artifact.ProjectID,
		&artifact.ProjectName,
		&artifact.VersionNodeID,
		&artifact.Version,
		&artifact.RegistryID,
		&artifact.RegistryName,
		&artifact.ImageRef,
		&imageID,
		&digest,
		&artifact.Tag,
		&artifact.Architecture,
		&sizeBytes,
		&artifact.Status,
		&pushed,
		&pushedAt,
		&deprecated,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return Artifact{}, err
	}

	artifact.ImageID = imageID.String
	artifact.Digest = digest.String
	if sizeBytes.Valid {
		value := sizeBytes.Int64
		artifact.SizeBytes = &value
	}
	artifact.Pushed = pushed != 0
	artifact.Deprecated = deprecated != 0
	artifact.PushedAt = parseOptionalTime(pushedAt)
	artifact.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	artifact.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return artifact, nil
}

func parseOptionalTime(value sql.NullString) *time.Time {
	if !value.Valid {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, value.String)
	if err != nil {
		return nil
	}
	return &parsed
}

func normalizePage(page int) int {
	if page < 1 {
		return 1
	}
	return page
}

func normalizePageSize(pageSize int) int {
	if pageSize < 1 {
		return 50
	}
	if pageSize > 200 {
		return 200
	}
	return pageSize
}

func placeholders(driverName string, count int) string {
	parts := make([]string, count)
	for i := range parts {
		parts[i] = placeholder(driverName, i+1)
	}
	return strings.Join(parts, ", ")
}

func placeholder(driverName string, index int) string {
	if driverName == "postgres" || driverName == "pgx" {
		return fmt.Sprintf("$%d", index)
	}
	return "?"
}

func nullString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func nullTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return formatTime(*value)
}

func nullInt64Ptr(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339)
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
