package buildtask

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

var (
	ErrNotFound     = errors.New("build task not found")
	ErrNoQueuedTask = errors.New("no queued build task")
)

type Repository struct {
	db         *sql.DB
	driverName string
}

type sqlExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type schedulerHost struct {
	ID             string
	Name           string
	Architecture   string
	Status         string
	MaxConcurrency int
	CurrentRunning int
}

func NewRepository(db *sql.DB, driverName string) Repository {
	return Repository{db: db, driverName: driverName}
}

func (r Repository) List(ctx context.Context, filter ListFilter) ([]BuildTask, int, error) {
	where, args := r.filterWhere(filter)
	var total int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM build_tasks t "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count build tasks: %w", err)
	}

	page := normalizePage(filter.Page)
	pageSize := normalizePageSize(filter.PageSize)
	offset := (page - 1) * pageSize
	args = append(args, pageSize, offset)

	query := selectTaskSQL + where + `
ORDER BY t.created_at DESC
LIMIT ` + placeholder(r.driverName, len(args)-1) + ` OFFSET ` + placeholder(r.driverName, len(args))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list build tasks: %w", err)
	}
	defer rows.Close()

	tasks := make([]BuildTask, 0)
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, 0, err
		}
		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate build tasks: %w", err)
	}

	return tasks, total, nil
}

func (r Repository) FindByID(ctx context.Context, taskID string) (BuildTask, error) {
	return r.findByID(ctx, r.db, taskID)
}

func (r Repository) Create(ctx context.Context, task BuildTask) error {
	buildArgs, err := encodeMap(task.BuildArgs)
	if err != nil {
		return err
	}
	buildOptions, err := encodeMap(task.BuildOptions)
	if err != nil {
		return err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin build task creation: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	query := `
INSERT INTO build_tasks (
	id, project_id, version_node_id, retry_of_task_id, host_id, requested_host_id, registry_id,
	image_name, image_tag, image_ref, architecture, dockerfile_snapshot, dockerfile_hash,
	build_context_ref, build_args, build_options, scheduler_reason, status, error_code, error_message,
	log_path, queued_at, started_at, build_started_at, build_finished_at, push_started_at, finished_at,
	duration_seconds, created_by, created_at, updated_at
) VALUES (` + placeholders(r.driverName, 31) + `)`

	_, err = tx.ExecContext(
		ctx,
		query,
		task.ID,
		task.ProjectID,
		task.VersionNodeID,
		nullString(task.RetryOfTaskID),
		nullString(task.HostID),
		nullString(task.RequestedHostID),
		task.RegistryID,
		task.ImageName,
		task.ImageTag,
		task.ImageRef,
		task.Architecture,
		task.DockerfileSnapshot,
		task.DockerfileHash,
		nullString(task.BuildContextRef),
		buildArgs,
		buildOptions,
		nullString(task.SchedulerReason),
		task.Status,
		nullString(task.ErrorCode),
		nullString(task.ErrorMessage),
		nullString(task.LogPath),
		nullTime(task.QueuedAt),
		nullTime(task.StartedAt),
		nullTime(task.BuildStartedAt),
		nullTime(task.BuildFinishedAt),
		nullTime(task.PushStartedAt),
		nullTime(task.FinishedAt),
		nullInt64(task.DurationSeconds),
		nullString(task.CreatedBy),
		formatTime(task.CreatedAt),
		formatTime(task.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("create build task: %w", err)
	}
	if err = r.updateLatestTask(ctx, tx, task.ProjectID, task.VersionNodeID, task.ID, task.UpdatedAt); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit build task creation: %w", err)
	}
	return nil
}

func (r Repository) DispatchTask(ctx context.Context, taskID string, now time.Time) (BuildTask, bool, string, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return BuildTask{}, false, "", fmt.Errorf("begin build task dispatch: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	task, err := r.findByID(ctx, tx, taskID)
	if err != nil {
		return BuildTask{}, false, "", err
	}
	if task.Status != StatusQueued && task.Status != StatusCreated {
		return BuildTask{}, false, "", ErrInvalidState
	}

	host, reason, err := r.selectHost(ctx, tx, task)
	if err != nil {
		if errors.Is(err, ErrNoSchedulableHost) {
			if failErr := r.markDispatchFailed(ctx, tx, task.ID, reason, now); failErr != nil {
				return BuildTask{}, false, "", failErr
			}
			if err = tx.Commit(); err != nil {
				return BuildTask{}, false, "", fmt.Errorf("commit dispatch failure: %w", err)
			}
			updated, findErr := r.FindByID(ctx, task.ID)
			return updated, false, reason, findErr
		}
		return BuildTask{}, false, "", err
	}

	if err = r.incrementHostRunning(ctx, tx, host.ID, now); err != nil {
		return BuildTask{}, false, "", err
	}
	if err = r.markDispatching(ctx, tx, task.ID, host.ID, reason, now); err != nil {
		return BuildTask{}, false, "", err
	}
	if err = tx.Commit(); err != nil {
		return BuildTask{}, false, "", fmt.Errorf("commit build task dispatch: %w", err)
	}

	updated, err := r.FindByID(ctx, task.ID)
	return updated, true, reason, err
}

func (r Repository) DispatchNext(ctx context.Context, now time.Time) (BuildTask, bool, string, error) {
	query := `
SELECT id
FROM build_tasks
WHERE status = ` + placeholder(r.driverName, 1) + `
ORDER BY queued_at ASC, created_at ASC
LIMIT 1`

	var taskID string
	if err := r.db.QueryRowContext(ctx, query, StatusQueued).Scan(&taskID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return BuildTask{}, false, "", ErrNoQueuedTask
		}
		return BuildTask{}, false, "", fmt.Errorf("find next queued build task: %w", err)
	}
	return r.DispatchTask(ctx, taskID, now)
}

func (r Repository) Cancel(ctx context.Context, taskID string, now time.Time) (BuildTask, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return BuildTask{}, fmt.Errorf("begin build task cancel: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	task, err := r.findByID(ctx, tx, taskID)
	if err != nil {
		return BuildTask{}, err
	}
	if terminalStatus(task.Status) {
		return BuildTask{}, ErrInvalidState
	}
	if task.HostID != "" && holdsHostSlot(task.Status) {
		if err = r.decrementHostRunning(ctx, tx, task.HostID, now); err != nil {
			return BuildTask{}, err
		}
	}

	query := `
UPDATE build_tasks
SET status = ` + placeholder(r.driverName, 1) + `,
    error_code = ` + placeholder(r.driverName, 2) + `,
    error_message = ` + placeholder(r.driverName, 3) + `,
    finished_at = ` + placeholder(r.driverName, 4) + `,
    updated_at = ` + placeholder(r.driverName, 5) + `
WHERE id = ` + placeholder(r.driverName, 6)
	result, err := tx.ExecContext(ctx, query, StatusCanceled, "CANCELED", "Build task was canceled.", formatTime(now), formatTime(now), task.ID)
	if err != nil {
		return BuildTask{}, fmt.Errorf("cancel build task: %w", err)
	}
	if err = requireRowsAffected(result); err != nil {
		return BuildTask{}, err
	}
	if err = tx.Commit(); err != nil {
		return BuildTask{}, fmt.Errorf("commit build task cancel: %w", err)
	}
	return r.FindByID(ctx, task.ID)
}

func (r Repository) StartBuild(ctx context.Context, taskID string, contextRef string, logPath string, now time.Time) (BuildTask, error) {
	query := `
UPDATE build_tasks
SET status = ` + placeholder(r.driverName, 1) + `,
    build_context_ref = ` + placeholder(r.driverName, 2) + `,
    log_path = ` + placeholder(r.driverName, 3) + `,
    build_started_at = ` + placeholder(r.driverName, 4) + `,
    started_at = COALESCE(started_at, ` + placeholder(r.driverName, 5) + `),
    updated_at = ` + placeholder(r.driverName, 6) + `
WHERE id = ` + placeholder(r.driverName, 7) + ` AND status = ` + placeholder(r.driverName, 8)
	result, err := r.db.ExecContext(ctx, query, StatusBuilding, contextRef, logPath, formatTime(now), formatTime(now), formatTime(now), taskID, StatusDispatching)
	if err != nil {
		return BuildTask{}, fmt.Errorf("start build task: %w", err)
	}
	if err = requireRowsAffected(result); err != nil {
		return BuildTask{}, ErrInvalidState
	}
	return r.FindByID(ctx, taskID)
}

func (r Repository) FailTask(ctx context.Context, taskID string, status string, code string, message string, now time.Time) (BuildTask, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return BuildTask{}, fmt.Errorf("begin build task failure: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	task, err := r.findByID(ctx, tx, taskID)
	if err != nil {
		return BuildTask{}, err
	}
	if terminalStatus(task.Status) {
		if err = tx.Commit(); err != nil {
			return BuildTask{}, fmt.Errorf("commit terminal build task failure no-op: %w", err)
		}
		return task, nil
	}
	if task.HostID != "" && holdsHostSlot(task.Status) {
		if err = r.decrementHostRunning(ctx, tx, task.HostID, now); err != nil {
			return BuildTask{}, err
		}
	}

	query := `
UPDATE build_tasks
SET status = ` + placeholder(r.driverName, 1) + `,
    error_code = ` + placeholder(r.driverName, 2) + `,
    error_message = ` + placeholder(r.driverName, 3) + `,
    finished_at = ` + placeholder(r.driverName, 4) + `,
    duration_seconds = ` + placeholder(r.driverName, 5) + `,
    updated_at = ` + placeholder(r.driverName, 6) + `
WHERE id = ` + placeholder(r.driverName, 7)
	result, err := tx.ExecContext(ctx, query, status, code, message, formatTime(now), durationSeconds(task, now), formatTime(now), task.ID)
	if err != nil {
		return BuildTask{}, fmt.Errorf("fail build task: %w", err)
	}
	if err = requireRowsAffected(result); err != nil {
		return BuildTask{}, err
	}
	if err = tx.Commit(); err != nil {
		return BuildTask{}, fmt.Errorf("commit build task failure: %w", err)
	}
	return r.FindByID(ctx, task.ID)
}

func (r Repository) CompleteBuild(ctx context.Context, taskID string, success bool, code string, message string, now time.Time) (BuildTask, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return BuildTask{}, fmt.Errorf("begin build task completion: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	task, err := r.findByID(ctx, tx, taskID)
	if err != nil {
		return BuildTask{}, err
	}
	if terminalStatus(task.Status) {
		if err = tx.Commit(); err != nil {
			return BuildTask{}, fmt.Errorf("commit terminal build task completion no-op: %w", err)
		}
		return task, nil
	}
	if task.Status != StatusBuilding {
		return BuildTask{}, ErrInvalidState
	}
	if task.HostID != "" && holdsHostSlot(task.Status) {
		if err = r.decrementHostRunning(ctx, tx, task.HostID, now); err != nil {
			return BuildTask{}, err
		}
	}

	status := StatusBuildFailed
	if success {
		status = StatusBuildSuccess
		code = ""
		message = ""
	}
	query := `
UPDATE build_tasks
SET status = ` + placeholder(r.driverName, 1) + `,
    error_code = ` + placeholder(r.driverName, 2) + `,
    error_message = ` + placeholder(r.driverName, 3) + `,
    build_finished_at = ` + placeholder(r.driverName, 4) + `,
    finished_at = ` + placeholder(r.driverName, 5) + `,
    duration_seconds = ` + placeholder(r.driverName, 6) + `,
    updated_at = ` + placeholder(r.driverName, 7) + `
WHERE id = ` + placeholder(r.driverName, 8)
	result, err := tx.ExecContext(ctx, query, status, nullString(code), nullString(message), formatTime(now), formatTime(now), durationSeconds(task, now), formatTime(now), task.ID)
	if err != nil {
		return BuildTask{}, fmt.Errorf("complete build task: %w", err)
	}
	if err = requireRowsAffected(result); err != nil {
		return BuildTask{}, err
	}
	if err = tx.Commit(); err != nil {
		return BuildTask{}, fmt.Errorf("commit build task completion: %w", err)
	}
	return r.FindByID(ctx, task.ID)
}

func (r Repository) findByID(ctx context.Context, exec sqlExecutor, taskID string) (BuildTask, error) {
	query := selectTaskSQL + `WHERE t.id = ` + placeholder(r.driverName, 1)
	task, err := scanTask(exec.QueryRowContext(ctx, query, taskID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return BuildTask{}, ErrNotFound
		}
		return BuildTask{}, err
	}
	return task, nil
}

func (r Repository) updateLatestTask(ctx context.Context, exec sqlExecutor, projectID string, nodeID string, taskID string, updatedAt time.Time) error {
	projectQuery := `
UPDATE image_projects
SET latest_build_task_id = ` + placeholder(r.driverName, 1) + `,
    updated_at = ` + placeholder(r.driverName, 2) + `
WHERE id = ` + placeholder(r.driverName, 3)
	projectResult, err := exec.ExecContext(ctx, projectQuery, taskID, formatTime(updatedAt), projectID)
	if err != nil {
		return fmt.Errorf("update project latest build task: %w", err)
	}
	if err := requireRowsAffected(projectResult); err != nil {
		return err
	}

	nodeQuery := `
UPDATE image_version_nodes
SET latest_build_task_id = ` + placeholder(r.driverName, 1) + `,
    updated_at = ` + placeholder(r.driverName, 2) + `
WHERE id = ` + placeholder(r.driverName, 3) + ` AND project_id = ` + placeholder(r.driverName, 4)
	nodeResult, err := exec.ExecContext(ctx, nodeQuery, taskID, formatTime(updatedAt), nodeID, projectID)
	if err != nil {
		return fmt.Errorf("update version node latest build task: %w", err)
	}
	return requireRowsAffected(nodeResult)
}

func (r Repository) selectHost(ctx context.Context, exec sqlExecutor, task BuildTask) (schedulerHost, string, error) {
	if task.RequestedHostID != "" {
		host, err := r.findHost(ctx, exec, task.RequestedHostID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return schedulerHost{}, "Requested build host is not available.", ErrNoSchedulableHost
			}
			return schedulerHost{}, "", err
		}
		if reason := rejectHostReason(host, task.Architecture); reason != "" {
			return schedulerHost{}, reason, ErrNoSchedulableHost
		}
		return host, "Requested build host selected.", nil
	}

	query := `
SELECT id, name, architecture, status, max_concurrency, current_running
FROM build_hosts
WHERE deleted_at IS NULL
  AND status != ` + placeholder(r.driverName, 1) + `
  AND current_running < max_concurrency
  AND (architecture IS NULL OR architecture = '' OR architecture = ` + placeholder(r.driverName, 2) + `)
ORDER BY
  CASE WHEN status = ` + placeholder(r.driverName, 3) + ` THEN 0 ELSE 1 END,
  CASE WHEN architecture = ` + placeholder(r.driverName, 4) + ` THEN 0 ELSE 1 END,
  current_running ASC,
  created_at ASC
LIMIT 1`

	host, err := scanSchedulerHost(exec.QueryRowContext(ctx, query, "disabled", task.Architecture, "online", task.Architecture))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return schedulerHost{}, "No build host has available capacity for architecture " + task.Architecture + ".", ErrNoSchedulableHost
		}
		return schedulerHost{}, "", err
	}
	if host.Architecture == "" {
		return host, "Selected build host with unknown architecture; run host check before real execution.", nil
	}
	return host, "Selected available build host for architecture " + task.Architecture + ".", nil
}

func (r Repository) findHost(ctx context.Context, exec sqlExecutor, hostID string) (schedulerHost, error) {
	query := `
SELECT id, name, architecture, status, max_concurrency, current_running
FROM build_hosts
WHERE id = ` + placeholder(r.driverName, 1) + ` AND deleted_at IS NULL`
	return scanSchedulerHost(exec.QueryRowContext(ctx, query, hostID))
}

func (r Repository) incrementHostRunning(ctx context.Context, exec sqlExecutor, hostID string, updatedAt time.Time) error {
	query := `
UPDATE build_hosts
SET current_running = current_running + 1,
    updated_at = ` + placeholder(r.driverName, 1) + `
WHERE id = ` + placeholder(r.driverName, 2) + ` AND deleted_at IS NULL`
	result, err := exec.ExecContext(ctx, query, formatTime(updatedAt), hostID)
	if err != nil {
		return fmt.Errorf("increment build host running count: %w", err)
	}
	return requireRowsAffected(result)
}

func (r Repository) decrementHostRunning(ctx context.Context, exec sqlExecutor, hostID string, updatedAt time.Time) error {
	query := `
UPDATE build_hosts
SET current_running = CASE WHEN current_running > 0 THEN current_running - 1 ELSE 0 END,
    updated_at = ` + placeholder(r.driverName, 1) + `
WHERE id = ` + placeholder(r.driverName, 2) + ` AND deleted_at IS NULL`
	result, err := exec.ExecContext(ctx, query, formatTime(updatedAt), hostID)
	if err != nil {
		return fmt.Errorf("decrement build host running count: %w", err)
	}
	return requireRowsAffected(result)
}

func (r Repository) markDispatching(ctx context.Context, exec sqlExecutor, taskID string, hostID string, reason string, now time.Time) error {
	query := `
UPDATE build_tasks
SET host_id = ` + placeholder(r.driverName, 1) + `,
    status = ` + placeholder(r.driverName, 2) + `,
    scheduler_reason = ` + placeholder(r.driverName, 3) + `,
    error_code = NULL,
    error_message = NULL,
    started_at = ` + placeholder(r.driverName, 4) + `,
    updated_at = ` + placeholder(r.driverName, 5) + `
WHERE id = ` + placeholder(r.driverName, 6)
	result, err := exec.ExecContext(ctx, query, hostID, StatusDispatching, reason, formatTime(now), formatTime(now), taskID)
	if err != nil {
		return fmt.Errorf("mark build task dispatching: %w", err)
	}
	return requireRowsAffected(result)
}

func (r Repository) markDispatchFailed(ctx context.Context, exec sqlExecutor, taskID string, reason string, now time.Time) error {
	query := `
UPDATE build_tasks
SET status = ` + placeholder(r.driverName, 1) + `,
    scheduler_reason = ` + placeholder(r.driverName, 2) + `,
    error_code = ` + placeholder(r.driverName, 3) + `,
    error_message = ` + placeholder(r.driverName, 4) + `,
    finished_at = ` + placeholder(r.driverName, 5) + `,
    updated_at = ` + placeholder(r.driverName, 6) + `
WHERE id = ` + placeholder(r.driverName, 7)
	result, err := exec.ExecContext(ctx, query, StatusDispatchFailed, reason, "DISPATCH_FAILED", reason, formatTime(now), formatTime(now), taskID)
	if err != nil {
		return fmt.Errorf("mark build task dispatch failed: %w", err)
	}
	return requireRowsAffected(result)
}

func (r Repository) filterWhere(filter ListFilter) (string, []any) {
	clauses := make([]string, 0, 5)
	args := make([]any, 0, 5)

	if strings.TrimSpace(filter.Status) != "" {
		args = append(args, strings.TrimSpace(filter.Status))
		clauses = append(clauses, "t.status = "+placeholder(r.driverName, len(args)))
	}
	if strings.TrimSpace(filter.ProjectID) != "" {
		args = append(args, strings.TrimSpace(filter.ProjectID))
		clauses = append(clauses, "t.project_id = "+placeholder(r.driverName, len(args)))
	}
	if strings.TrimSpace(filter.VersionNodeID) != "" {
		args = append(args, strings.TrimSpace(filter.VersionNodeID))
		clauses = append(clauses, "t.version_node_id = "+placeholder(r.driverName, len(args)))
	}
	if strings.TrimSpace(filter.HostID) != "" {
		args = append(args, strings.TrimSpace(filter.HostID))
		clauses = append(clauses, "t.host_id = "+placeholder(r.driverName, len(args)))
	}
	if strings.TrimSpace(filter.RegistryID) != "" {
		args = append(args, strings.TrimSpace(filter.RegistryID))
		clauses = append(clauses, "t.registry_id = "+placeholder(r.driverName, len(args)))
	}

	if len(clauses) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(clauses, " AND "), args
}

const selectTaskSQL = `
SELECT t.id, t.project_id, p.name, t.version_node_id, n.version, t.retry_of_task_id,
       t.host_id, h.name, t.requested_host_id, rh.name, t.registry_id, r.name,
       t.image_name, t.image_tag, t.image_ref, t.architecture, t.dockerfile_snapshot,
       t.dockerfile_hash, t.build_context_ref, t.build_args, t.build_options,
       t.scheduler_reason, t.status, t.error_code, t.error_message, t.log_path,
       t.queued_at, t.started_at, t.build_started_at, t.build_finished_at,
       t.push_started_at, t.finished_at, t.duration_seconds, t.created_by, t.created_at, t.updated_at
FROM build_tasks t
JOIN image_projects p ON p.id = t.project_id
JOIN image_version_nodes n ON n.id = t.version_node_id
JOIN registries r ON r.id = t.registry_id
LEFT JOIN build_hosts h ON h.id = t.host_id
LEFT JOIN build_hosts rh ON rh.id = t.requested_host_id
`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanTask(row rowScanner) (BuildTask, error) {
	var task BuildTask
	var retryOfTaskID, hostID, hostName, requestedHostID, requestedHostName, buildContextRef sql.NullString
	var buildArgs, buildOptions, schedulerReason, errorCode, errorMessage, logPath sql.NullString
	var queuedAt, startedAt, buildStartedAt, buildFinishedAt, pushStartedAt, finishedAt sql.NullString
	var durationSeconds sql.NullInt64
	var createdBy sql.NullString
	var createdAt, updatedAt string

	err := row.Scan(
		&task.ID,
		&task.ProjectID,
		&task.ProjectName,
		&task.VersionNodeID,
		&task.Version,
		&retryOfTaskID,
		&hostID,
		&hostName,
		&requestedHostID,
		&requestedHostName,
		&task.RegistryID,
		&task.RegistryName,
		&task.ImageName,
		&task.ImageTag,
		&task.ImageRef,
		&task.Architecture,
		&task.DockerfileSnapshot,
		&task.DockerfileHash,
		&buildContextRef,
		&buildArgs,
		&buildOptions,
		&schedulerReason,
		&task.Status,
		&errorCode,
		&errorMessage,
		&logPath,
		&queuedAt,
		&startedAt,
		&buildStartedAt,
		&buildFinishedAt,
		&pushStartedAt,
		&finishedAt,
		&durationSeconds,
		&createdBy,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return BuildTask{}, err
	}

	task.RetryOfTaskID = retryOfTaskID.String
	task.HostID = hostID.String
	task.HostName = hostName.String
	task.RequestedHostID = requestedHostID.String
	task.RequestedHostName = requestedHostName.String
	task.BuildContextRef = buildContextRef.String
	task.BuildArgs = decodeMap(buildArgs.String)
	task.BuildOptions = decodeMap(buildOptions.String)
	task.SchedulerReason = schedulerReason.String
	task.ErrorCode = errorCode.String
	task.ErrorMessage = errorMessage.String
	task.LogPath = logPath.String
	task.QueuedAt = parseOptionalTime(queuedAt)
	task.StartedAt = parseOptionalTime(startedAt)
	task.BuildStartedAt = parseOptionalTime(buildStartedAt)
	task.BuildFinishedAt = parseOptionalTime(buildFinishedAt)
	task.PushStartedAt = parseOptionalTime(pushStartedAt)
	task.FinishedAt = parseOptionalTime(finishedAt)
	task.CreatedBy = createdBy.String
	task.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	task.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	if durationSeconds.Valid {
		value := durationSeconds.Int64
		task.DurationSeconds = &value
	}

	return task, nil
}

func scanSchedulerHost(row rowScanner) (schedulerHost, error) {
	var host schedulerHost
	var architecture sql.NullString
	err := row.Scan(&host.ID, &host.Name, &architecture, &host.Status, &host.MaxConcurrency, &host.CurrentRunning)
	if err != nil {
		return schedulerHost{}, err
	}
	host.Architecture = architecture.String
	return host, nil
}

func rejectHostReason(host schedulerHost, architecture string) string {
	if host.Status == "disabled" {
		return "Requested build host is disabled."
	}
	if host.CurrentRunning >= host.MaxConcurrency {
		return "Requested build host has no available concurrency."
	}
	if host.Architecture != "" && architecture != "" && host.Architecture != architecture {
		return "Requested build host architecture " + host.Architecture + " does not match task architecture " + architecture + "."
	}
	return ""
}

func encodeMap(value map[string]string) (string, error) {
	if value == nil {
		value = map[string]string{}
	}
	data, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("encode map: %w", err)
	}
	return string(data), nil
}

func decodeMap(value string) map[string]string {
	if strings.TrimSpace(value) == "" {
		return map[string]string{}
	}
	result := map[string]string{}
	if err := json.Unmarshal([]byte(value), &result); err != nil {
		return map[string]string{}
	}
	return result
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

func durationSeconds(task BuildTask, finishedAt time.Time) int64 {
	startedAt := task.StartedAt
	if startedAt == nil {
		startedAt = task.BuildStartedAt
	}
	if startedAt == nil {
		return int64(finishedAt.Sub(task.CreatedAt).Seconds())
	}
	duration := int64(finishedAt.Sub(*startedAt).Seconds())
	if duration < 0 {
		return 0
	}
	return duration
}

func terminalStatus(status string) bool {
	switch status {
	case StatusBuildSuccess, StatusPushSuccess, StatusPreparingContextFailed, StatusDispatchFailed, StatusBuildFailed, StatusPushFailed, StatusCanceled, StatusTimeout:
		return true
	default:
		return false
	}
}

func holdsHostSlot(status string) bool {
	switch status {
	case StatusDispatching, StatusPreparingContext, StatusBuilding, StatusBuildSuccess, StatusPushing:
		return true
	default:
		return false
	}
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

func nullInt64(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
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
