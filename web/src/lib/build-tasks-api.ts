import { apiFetch } from "@/lib/api"

export type BuildTaskStatus =
  | "created"
  | "queued"
  | "dispatching"
  | "preparing_context"
  | "building"
  | "build_success"
  | "pushing"
  | "push_success"
  | "preparing_context_failed"
  | "dispatch_failed"
  | "build_failed"
  | "push_failed"
  | "canceled"
  | "timeout"

export type BuildTask = {
  id: string
  projectId: string
  projectName: string
  versionNodeId: string
  version: string
  retryOfTaskId?: string | null
  hostId?: string | null
  hostName?: string | null
  requestedHostId?: string | null
  requestedHostName?: string | null
  registryId: string
  registryName: string
  imageName: string
  imageTag: string
  imageRef: string
  architecture: string
  dockerfileSnapshot: string
  dockerfileHash: string
  buildContextRef?: string | null
  buildArgs: Record<string, string>
  buildOptions: Record<string, string>
  schedulerReason?: string | null
  status: BuildTaskStatus
  errorCode?: string | null
  errorMessage?: string | null
  logPath?: string | null
  queuedAt?: string | null
  startedAt?: string | null
  buildStartedAt?: string | null
  buildFinishedAt?: string | null
  pushStartedAt?: string | null
  finishedAt?: string | null
  durationSeconds?: number | null
  createdBy?: string | null
  createdAt: string
  updatedAt: string
}

export type CreateBuildTaskInput = {
  projectId: string
  versionNodeId: string
  registryId?: string
  requestedHostId?: string
  imageName?: string
  imageTag?: string
  architecture?: string
  buildArgs?: Record<string, string>
  buildOptions?: Record<string, string>
}

export type DispatchResult = {
  task: BuildTask
  dispatched: boolean
  reason: string
}

export async function listBuildTasks(): Promise<BuildTask[]> {
  return apiFetch<BuildTask[]>("/api/v1/build-tasks")
}

export async function createBuildTask(input: CreateBuildTaskInput): Promise<BuildTask> {
  return apiFetch<BuildTask>("/api/v1/build-tasks", {
    method: "POST",
    json: input,
  })
}

export async function dispatchBuildTask(id: string): Promise<DispatchResult> {
  return apiFetch<DispatchResult>(`/api/v1/build-tasks/${id}/dispatch`, {
    method: "POST",
  })
}

export async function dispatchNextBuildTask(): Promise<DispatchResult> {
  return apiFetch<DispatchResult>("/api/v1/build-tasks/dispatch-next", {
    method: "POST",
  })
}

export async function cancelBuildTask(id: string): Promise<BuildTask> {
  return apiFetch<BuildTask>(`/api/v1/build-tasks/${id}/cancel`, {
    method: "POST",
  })
}

export async function retryBuildTask(id: string): Promise<BuildTask> {
  return apiFetch<BuildTask>(`/api/v1/build-tasks/${id}/retry`, {
    method: "POST",
  })
}
