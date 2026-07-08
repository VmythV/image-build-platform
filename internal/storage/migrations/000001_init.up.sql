CREATE TABLE IF NOT EXISTS users (
	id TEXT PRIMARY KEY,
	username TEXT NOT NULL UNIQUE,
	display_name TEXT NOT NULL,
	password_hash TEXT NOT NULL,
	role TEXT NOT NULL,
	status TEXT NOT NULL,
	last_login_at TEXT NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	deleted_at TEXT NULL
);

CREATE INDEX IF NOT EXISTS idx_users_username ON users (username);
CREATE INDEX IF NOT EXISTS idx_users_role ON users (role);
CREATE INDEX IF NOT EXISTS idx_users_status ON users (status);

CREATE TABLE IF NOT EXISTS sessions (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	token_hash TEXT NOT NULL UNIQUE,
	user_agent TEXT NULL,
	ip_address TEXT NULL,
	expires_at TEXT NOT NULL,
	created_at TEXT NOT NULL,
	last_seen_at TEXT NULL
);

CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions (user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions (expires_at);
CREATE INDEX IF NOT EXISTS idx_sessions_token_hash ON sessions (token_hash);

CREATE TABLE IF NOT EXISTS credentials (
	id TEXT PRIMARY KEY,
	type TEXT NOT NULL,
	name TEXT NOT NULL,
	encrypted_value TEXT NOT NULL,
	encryption_version INTEGER NOT NULL,
	fingerprint TEXT NULL,
	created_by TEXT NULL REFERENCES users(id) ON DELETE SET NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_credentials_type ON credentials (type);
CREATE INDEX IF NOT EXISTS idx_credentials_created_by ON credentials (created_by);

CREATE TABLE IF NOT EXISTS build_hosts (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	connection_type TEXT NOT NULL,
	address TEXT NULL,
	port INTEGER NULL,
	username TEXT NULL,
	credential_id TEXT NULL REFERENCES credentials(id) ON DELETE SET NULL,
	docker_endpoint TEXT NULL,
	docker_command TEXT NULL,
	architecture TEXT NULL,
	os TEXT NULL,
	docker_version TEXT NULL,
	buildkit_supported INTEGER NOT NULL DEFAULT 0,
	labels TEXT NOT NULL DEFAULT '[]',
	max_concurrency INTEGER NOT NULL DEFAULT 1,
	current_running INTEGER NOT NULL DEFAULT 0,
	status TEXT NOT NULL,
	last_checked_at TEXT NULL,
	last_check_result TEXT NULL,
	last_error TEXT NULL,
	created_by TEXT NULL REFERENCES users(id) ON DELETE SET NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	deleted_at TEXT NULL
);

CREATE INDEX IF NOT EXISTS idx_build_hosts_connection_type ON build_hosts (connection_type);
CREATE INDEX IF NOT EXISTS idx_build_hosts_architecture ON build_hosts (architecture);
CREATE INDEX IF NOT EXISTS idx_build_hosts_status ON build_hosts (status);
CREATE INDEX IF NOT EXISTS idx_build_hosts_deleted_at ON build_hosts (deleted_at);

CREATE TABLE IF NOT EXISTS registries (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	type TEXT NOT NULL,
	endpoint TEXT NOT NULL,
	namespace TEXT NULL,
	region TEXT NULL,
	credential_id TEXT NULL REFERENCES credentials(id) ON DELETE SET NULL,
	allow_pull INTEGER NOT NULL DEFAULT 1,
	allow_push INTEGER NOT NULL DEFAULT 1,
	is_default_pull INTEGER NOT NULL DEFAULT 0,
	is_default_push INTEGER NOT NULL DEFAULT 0,
	tls_verify INTEGER NOT NULL DEFAULT 1,
	insecure_http INTEGER NOT NULL DEFAULT 0,
	status TEXT NOT NULL,
	last_checked_at TEXT NULL,
	last_check_result TEXT NULL,
	last_error TEXT NULL,
	created_by TEXT NULL REFERENCES users(id) ON DELETE SET NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	deleted_at TEXT NULL
);

CREATE INDEX IF NOT EXISTS idx_registries_type ON registries (type);
CREATE INDEX IF NOT EXISTS idx_registries_endpoint ON registries (endpoint);
CREATE INDEX IF NOT EXISTS idx_registries_status ON registries (status);
CREATE INDEX IF NOT EXISTS idx_registries_default_push ON registries (is_default_push);
CREATE INDEX IF NOT EXISTS idx_registries_deleted_at ON registries (deleted_at);

CREATE TABLE IF NOT EXISTS image_projects (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	image_type TEXT NOT NULL,
	image_name TEXT NOT NULL,
	namespace TEXT NULL,
	root_image_ref TEXT NOT NULL,
	root_image_source TEXT NOT NULL,
	source_project_id TEXT NULL REFERENCES image_projects(id) ON DELETE SET NULL,
	source_version_node_id TEXT NULL,
	default_registry_id TEXT NULL REFERENCES registries(id) ON DELETE SET NULL,
	default_architecture TEXT NOT NULL,
	labels TEXT NOT NULL DEFAULT '[]',
	description TEXT NULL,
	status TEXT NOT NULL,
	owner_id TEXT NULL REFERENCES users(id) ON DELETE SET NULL,
	latest_version_node_id TEXT NULL,
	latest_build_task_id TEXT NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	deleted_at TEXT NULL
);

CREATE INDEX IF NOT EXISTS idx_image_projects_name ON image_projects (name);
CREATE INDEX IF NOT EXISTS idx_image_projects_image_type ON image_projects (image_type);
CREATE INDEX IF NOT EXISTS idx_image_projects_image_name ON image_projects (image_name);
CREATE INDEX IF NOT EXISTS idx_image_projects_owner_id ON image_projects (owner_id);
CREATE INDEX IF NOT EXISTS idx_image_projects_status ON image_projects (status);
CREATE INDEX IF NOT EXISTS idx_image_projects_deleted_at ON image_projects (deleted_at);

CREATE TABLE IF NOT EXISTS image_branches (
	id TEXT PRIMARY KEY,
	project_id TEXT NOT NULL REFERENCES image_projects(id) ON DELETE CASCADE,
	name TEXT NOT NULL,
	start_node_id TEXT NULL,
	head_node_id TEXT NULL,
	description TEXT NULL,
	status TEXT NOT NULL,
	created_by TEXT NULL REFERENCES users(id) ON DELETE SET NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	deleted_at TEXT NULL,
	UNIQUE (project_id, name)
);

CREATE INDEX IF NOT EXISTS idx_image_branches_project_id ON image_branches (project_id);
CREATE INDEX IF NOT EXISTS idx_image_branches_project_name ON image_branches (project_id, name);
CREATE INDEX IF NOT EXISTS idx_image_branches_status ON image_branches (status);

CREATE TABLE IF NOT EXISTS image_version_nodes (
	id TEXT PRIMARY KEY,
	project_id TEXT NOT NULL REFERENCES image_projects(id) ON DELETE CASCADE,
	branch_id TEXT NOT NULL REFERENCES image_branches(id) ON DELETE RESTRICT,
	parent_node_id TEXT NULL REFERENCES image_version_nodes(id) ON DELETE SET NULL,
	version TEXT NOT NULL,
	dockerfile TEXT NOT NULL,
	dockerfile_hash TEXT NOT NULL,
	form_config_snapshot TEXT NULL,
	build_context_ref TEXT NULL,
	description TEXT NULL,
	status TEXT NOT NULL,
	latest_build_task_id TEXT NULL,
	latest_artifact_id TEXT NULL,
	graph_position TEXT NULL,
	created_by TEXT NULL REFERENCES users(id) ON DELETE SET NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	deleted_at TEXT NULL
);

CREATE INDEX IF NOT EXISTS idx_image_version_nodes_project_id ON image_version_nodes (project_id);
CREATE INDEX IF NOT EXISTS idx_image_version_nodes_branch_id ON image_version_nodes (branch_id);
CREATE INDEX IF NOT EXISTS idx_image_version_nodes_parent_node_id ON image_version_nodes (parent_node_id);
CREATE INDEX IF NOT EXISTS idx_image_version_nodes_project_version ON image_version_nodes (project_id, version);
CREATE INDEX IF NOT EXISTS idx_image_version_nodes_status ON image_version_nodes (status);

CREATE TABLE IF NOT EXISTS build_tasks (
	id TEXT PRIMARY KEY,
	project_id TEXT NOT NULL REFERENCES image_projects(id) ON DELETE RESTRICT,
	version_node_id TEXT NOT NULL REFERENCES image_version_nodes(id) ON DELETE RESTRICT,
	retry_of_task_id TEXT NULL REFERENCES build_tasks(id) ON DELETE SET NULL,
	host_id TEXT NULL REFERENCES build_hosts(id) ON DELETE SET NULL,
	requested_host_id TEXT NULL REFERENCES build_hosts(id) ON DELETE SET NULL,
	registry_id TEXT NOT NULL REFERENCES registries(id) ON DELETE RESTRICT,
	image_name TEXT NOT NULL,
	image_tag TEXT NOT NULL,
	image_ref TEXT NOT NULL,
	architecture TEXT NOT NULL,
	dockerfile_snapshot TEXT NOT NULL,
	dockerfile_hash TEXT NOT NULL,
	build_context_ref TEXT NULL,
	build_args TEXT NULL,
	build_options TEXT NULL,
	scheduler_reason TEXT NULL,
	status TEXT NOT NULL,
	error_code TEXT NULL,
	error_message TEXT NULL,
	log_path TEXT NULL,
	queued_at TEXT NULL,
	started_at TEXT NULL,
	build_started_at TEXT NULL,
	build_finished_at TEXT NULL,
	push_started_at TEXT NULL,
	finished_at TEXT NULL,
	duration_seconds INTEGER NULL,
	created_by TEXT NULL REFERENCES users(id) ON DELETE SET NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_build_tasks_project_id ON build_tasks (project_id);
CREATE INDEX IF NOT EXISTS idx_build_tasks_version_node_id ON build_tasks (version_node_id);
CREATE INDEX IF NOT EXISTS idx_build_tasks_host_id ON build_tasks (host_id);
CREATE INDEX IF NOT EXISTS idx_build_tasks_registry_id ON build_tasks (registry_id);
CREATE INDEX IF NOT EXISTS idx_build_tasks_status ON build_tasks (status);
CREATE INDEX IF NOT EXISTS idx_build_tasks_created_by ON build_tasks (created_by);
CREATE INDEX IF NOT EXISTS idx_build_tasks_created_at ON build_tasks (created_at);
CREATE INDEX IF NOT EXISTS idx_build_tasks_retry_of_task_id ON build_tasks (retry_of_task_id);

CREATE TABLE IF NOT EXISTS image_artifacts (
	id TEXT PRIMARY KEY,
	build_task_id TEXT NOT NULL REFERENCES build_tasks(id) ON DELETE RESTRICT,
	project_id TEXT NOT NULL REFERENCES image_projects(id) ON DELETE RESTRICT,
	version_node_id TEXT NOT NULL REFERENCES image_version_nodes(id) ON DELETE RESTRICT,
	registry_id TEXT NOT NULL REFERENCES registries(id) ON DELETE RESTRICT,
	image_ref TEXT NOT NULL,
	image_id TEXT NULL,
	digest TEXT NULL,
	tag TEXT NOT NULL,
	architecture TEXT NOT NULL,
	size_bytes INTEGER NULL,
	status TEXT NOT NULL,
	pushed INTEGER NOT NULL DEFAULT 0,
	pushed_at TEXT NULL,
	deprecated INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	deleted_at TEXT NULL
);

CREATE INDEX IF NOT EXISTS idx_image_artifacts_build_task_id ON image_artifacts (build_task_id);
CREATE INDEX IF NOT EXISTS idx_image_artifacts_project_id ON image_artifacts (project_id);
CREATE INDEX IF NOT EXISTS idx_image_artifacts_version_node_id ON image_artifacts (version_node_id);
CREATE INDEX IF NOT EXISTS idx_image_artifacts_registry_id ON image_artifacts (registry_id);
CREATE INDEX IF NOT EXISTS idx_image_artifacts_image_ref ON image_artifacts (image_ref);
CREATE INDEX IF NOT EXISTS idx_image_artifacts_digest ON image_artifacts (digest);
CREATE INDEX IF NOT EXISTS idx_image_artifacts_status ON image_artifacts (status);

CREATE TABLE IF NOT EXISTS artifact_push_events (
	id TEXT PRIMARY KEY,
	artifact_id TEXT NOT NULL REFERENCES image_artifacts(id) ON DELETE CASCADE,
	build_task_id TEXT NULL REFERENCES build_tasks(id) ON DELETE SET NULL,
	registry_id TEXT NOT NULL REFERENCES registries(id) ON DELETE RESTRICT,
	status TEXT NOT NULL,
	error_message TEXT NULL,
	started_at TEXT NOT NULL,
	finished_at TEXT NULL,
	created_by TEXT NULL REFERENCES users(id) ON DELETE SET NULL,
	created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_artifact_push_events_artifact_id ON artifact_push_events (artifact_id);
CREATE INDEX IF NOT EXISTS idx_artifact_push_events_registry_id ON artifact_push_events (registry_id);
CREATE INDEX IF NOT EXISTS idx_artifact_push_events_created_at ON artifact_push_events (created_at);

CREATE TABLE IF NOT EXISTS audit_logs (
	id TEXT PRIMARY KEY,
	actor_id TEXT NULL REFERENCES users(id) ON DELETE SET NULL,
	actor_name TEXT NULL,
	action TEXT NOT NULL,
	resource_type TEXT NOT NULL,
	resource_id TEXT NULL,
	resource_name TEXT NULL,
	ip_address TEXT NULL,
	user_agent TEXT NULL,
	request_id TEXT NULL,
	detail TEXT NULL,
	created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_audit_logs_actor_id ON audit_logs (actor_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs (action);
CREATE INDEX IF NOT EXISTS idx_audit_logs_resource ON audit_logs (resource_type, resource_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs (created_at);
CREATE INDEX IF NOT EXISTS idx_audit_logs_request_id ON audit_logs (request_id);

CREATE TABLE IF NOT EXISTS system_settings (
	key TEXT PRIMARY KEY,
	value TEXT NOT NULL,
	value_type TEXT NOT NULL,
	description TEXT NULL,
	updated_by TEXT NULL REFERENCES users(id) ON DELETE SET NULL,
	updated_at TEXT NOT NULL
);
