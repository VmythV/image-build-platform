package dashboard

import (
	"context"
	"database/sql"
	"fmt"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) Repository {
	return Repository{db: db}
}

func (r Repository) Summary(ctx context.Context) (Summary, error) {
	builds, err := r.buildCounts(ctx)
	if err != nil {
		return Summary{}, err
	}
	hosts, err := r.hostCounts(ctx)
	if err != nil {
		return Summary{}, err
	}
	registries, err := r.registryCounts(ctx)
	if err != nil {
		return Summary{}, err
	}
	artifacts, err := r.artifactCounts(ctx)
	if err != nil {
		return Summary{}, err
	}
	projects, err := r.projectCounts(ctx)
	if err != nil {
		return Summary{}, err
	}
	recentTasks, err := r.recentTasks(ctx)
	if err != nil {
		return Summary{}, err
	}
	recentArtifacts, err := r.recentArtifacts(ctx)
	if err != nil {
		return Summary{}, err
	}

	return Summary{
		Builds:          builds,
		Hosts:           hosts,
		Registries:      registries,
		Artifacts:       artifacts,
		Projects:        projects,
		RecentTasks:     recentTasks,
		RecentArtifacts: recentArtifacts,
	}, nil
}

func (r Repository) buildCounts(ctx context.Context) (BuildCounts, error) {
	query := `
SELECT
	COALESCE(SUM(CASE WHEN status IN ('dispatching', 'preparing_context', 'building', 'pushing') THEN 1 ELSE 0 END), 0),
	COALESCE(SUM(CASE WHEN status = 'queued' THEN 1 ELSE 0 END), 0),
	COALESCE(SUM(CASE WHEN status IN ('preparing_context_failed', 'dispatch_failed', 'build_failed', 'push_failed', 'timeout') THEN 1 ELSE 0 END), 0),
	COUNT(*)
FROM build_tasks`

	var counts BuildCounts
	if err := r.db.QueryRowContext(ctx, query).Scan(&counts.Running, &counts.Queued, &counts.Failed, &counts.Total); err != nil {
		return BuildCounts{}, fmt.Errorf("count builds: %w", err)
	}
	return counts, nil
}

func (r Repository) hostCounts(ctx context.Context) (HostCounts, error) {
	query := `
SELECT
	COALESCE(SUM(CASE WHEN status = 'online' THEN 1 ELSE 0 END), 0),
	COALESCE(SUM(CASE WHEN status = 'disabled' THEN 1 ELSE 0 END), 0),
	COUNT(*)
FROM build_hosts
WHERE deleted_at IS NULL`

	var counts HostCounts
	if err := r.db.QueryRowContext(ctx, query).Scan(&counts.Online, &counts.Disabled, &counts.Total); err != nil {
		return HostCounts{}, fmt.Errorf("count build hosts: %w", err)
	}
	return counts, nil
}

func (r Repository) registryCounts(ctx context.Context) (RegistryCounts, error) {
	query := `
SELECT
	COALESCE(SUM(CASE WHEN status = 'available' THEN 1 ELSE 0 END), 0),
	COALESCE(SUM(CASE WHEN status = 'disabled' THEN 1 ELSE 0 END), 0),
	COUNT(*)
FROM registries
WHERE deleted_at IS NULL`

	var counts RegistryCounts
	if err := r.db.QueryRowContext(ctx, query).Scan(&counts.Available, &counts.Disabled, &counts.Total); err != nil {
		return RegistryCounts{}, fmt.Errorf("count registries: %w", err)
	}
	return counts, nil
}

func (r Repository) artifactCounts(ctx context.Context) (ArtifactCounts, error) {
	query := `
SELECT
	COALESCE(SUM(CASE WHEN pushed = 1 THEN 1 ELSE 0 END), 0),
	COUNT(*)
FROM image_artifacts
WHERE deleted_at IS NULL`

	var counts ArtifactCounts
	if err := r.db.QueryRowContext(ctx, query).Scan(&counts.Pushed, &counts.Total); err != nil {
		return ArtifactCounts{}, fmt.Errorf("count image artifacts: %w", err)
	}
	return counts, nil
}

func (r Repository) projectCounts(ctx context.Context) (ProjectCounts, error) {
	query := `
SELECT
	COALESCE(SUM(CASE WHEN status = 'active' THEN 1 ELSE 0 END), 0),
	COUNT(*)
FROM image_projects
WHERE deleted_at IS NULL`

	var counts ProjectCounts
	if err := r.db.QueryRowContext(ctx, query).Scan(&counts.Active, &counts.Total); err != nil {
		return ProjectCounts{}, fmt.Errorf("count image projects: %w", err)
	}
	return counts, nil
}

func (r Repository) recentTasks(ctx context.Context) ([]RecentTask, error) {
	query := `
SELECT t.id, t.image_ref, t.status, t.architecture, h.name, t.created_at
FROM build_tasks t
LEFT JOIN build_hosts h ON h.id = t.host_id
ORDER BY t.created_at DESC
LIMIT 6`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list recent build tasks: %w", err)
	}
	defer rows.Close()

	tasks := make([]RecentTask, 0)
	for rows.Next() {
		var task RecentTask
		var hostName sql.NullString
		if err := rows.Scan(&task.ID, &task.ImageRef, &task.Status, &task.Architecture, &hostName, &task.CreatedAt); err != nil {
			return nil, err
		}
		if hostName.Valid {
			task.HostName = &hostName.String
		}
		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recent build tasks: %w", err)
	}
	return tasks, nil
}

func (r Repository) recentArtifacts(ctx context.Context) ([]RecentArtifact, error) {
	query := `
SELECT a.id, a.image_ref, a.digest, a.architecture, p.name, r.name, a.created_at
FROM image_artifacts a
JOIN image_projects p ON p.id = a.project_id
JOIN registries r ON r.id = a.registry_id
WHERE a.deleted_at IS NULL
ORDER BY a.created_at DESC
LIMIT 6`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list recent image artifacts: %w", err)
	}
	defer rows.Close()

	artifacts := make([]RecentArtifact, 0)
	for rows.Next() {
		var artifact RecentArtifact
		var digest sql.NullString
		if err := rows.Scan(&artifact.ID, &artifact.ImageRef, &digest, &artifact.Architecture, &artifact.ProjectName, &artifact.RegistryName, &artifact.CreatedAt); err != nil {
			return nil, err
		}
		if digest.Valid {
			artifact.Digest = &digest.String
		}
		artifacts = append(artifacts, artifact)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recent image artifacts: %w", err)
	}
	return artifacts, nil
}
