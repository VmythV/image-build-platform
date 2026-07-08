import { apiFetch } from "@/lib/api"

export type BuildHostStatus = "unknown" | "online" | "offline" | "unavailable" | "busy" | "disabled"

export type BuildHost = {
  id: string
  name: string
  connectionType: "local_docker" | "ssh"
  address?: string | null
  port?: number | null
  username?: string | null
  dockerEndpoint?: string | null
  dockerCommand?: string | null
  architecture?: string | null
  os?: string | null
  dockerVersion?: string | null
  buildkitSupported: boolean
  labels: string[]
  maxConcurrency: number
  currentRunning: number
  status: BuildHostStatus
  lastCheckedAt?: string | null
  lastError?: string | null
  createdAt: string
  updatedAt: string
}

export type SaveBuildHostInput = {
  name: string
  connectionType: "local_docker" | "ssh"
  address?: string
  port?: number
  username?: string
  dockerEndpoint?: string
  dockerCommand?: string
  maxConcurrency: number
  labels: string[]
}

export type HostCheckResult = {
  status: BuildHostStatus
  architecture?: string
  os?: string
  dockerVersion?: string
  buildkitSupported: boolean
  checks: Array<{
    name: string
    status: string
    message: string
  }>
  error?: string | null
}

export async function listBuildHosts(): Promise<BuildHost[]> {
  return apiFetch<BuildHost[]>("/api/v1/build-hosts")
}

export async function createBuildHost(input: SaveBuildHostInput): Promise<BuildHost> {
  return apiFetch<BuildHost>("/api/v1/build-hosts", {
    method: "POST",
    json: input,
  })
}

export async function updateBuildHost(id: string, input: SaveBuildHostInput): Promise<BuildHost> {
  return apiFetch<BuildHost>(`/api/v1/build-hosts/${id}`, {
    method: "PUT",
    json: input,
  })
}

export async function deleteBuildHost(id: string): Promise<void> {
  await apiFetch<{ success: boolean }>(`/api/v1/build-hosts/${id}`, {
    method: "DELETE",
  })
}

export async function checkBuildHost(id: string): Promise<{ host: BuildHost; result: HostCheckResult }> {
  return apiFetch<{ host: BuildHost; result: HostCheckResult }>(`/api/v1/build-hosts/${id}/check`, {
    method: "POST",
  })
}

export async function enableBuildHost(id: string): Promise<BuildHost> {
  return apiFetch<BuildHost>(`/api/v1/build-hosts/${id}/enable`, {
    method: "POST",
  })
}

export async function disableBuildHost(id: string): Promise<BuildHost> {
  return apiFetch<BuildHost>(`/api/v1/build-hosts/${id}/disable`, {
    method: "POST",
  })
}
