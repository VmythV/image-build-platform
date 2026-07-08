import { apiFetch } from "@/lib/api"

export type RegistryStatus = "unknown" | "available" | "unavailable" | "disabled"

export type Registry = {
  id: string
  name: string
  type: "generic" | "harbor" | "docker_hub" | "aliyun" | "tencent_cloud"
  endpoint: string
  namespace?: string | null
  region?: string | null
  allowPull: boolean
  allowPush: boolean
  isDefaultPull: boolean
  isDefaultPush: boolean
  tlsVerify: boolean
  insecureHttp: boolean
  status: RegistryStatus
  lastCheckedAt?: string | null
  lastError?: string | null
  credentialConfigured: boolean
  credentialUsername?: string | null
  credentialFingerprint?: string | null
  createdAt: string
  updatedAt: string
}

export type SaveRegistryInput = {
  name: string
  type: Registry["type"]
  endpoint: string
  namespace?: string
  region?: string
  username?: string
  password?: string
  allowPull: boolean
  allowPush: boolean
  isDefaultPull: boolean
  isDefaultPush: boolean
  tlsVerify: boolean
  insecureHttp: boolean
}

export type RegistryCheckResult = {
  status: RegistryStatus
  login: {
    status: string
    message: string
  }
  pull: {
    status: string
    message: string
  }
  error?: string | null
}

export async function listRegistries(): Promise<Registry[]> {
  return apiFetch<Registry[]>("/api/v1/registries")
}

export async function createRegistry(input: SaveRegistryInput): Promise<Registry> {
  return apiFetch<Registry>("/api/v1/registries", {
    method: "POST",
    json: input,
  })
}

export async function updateRegistry(id: string, input: SaveRegistryInput): Promise<Registry> {
  return apiFetch<Registry>(`/api/v1/registries/${id}`, {
    method: "PUT",
    json: input,
  })
}

export async function deleteRegistry(id: string): Promise<void> {
  await apiFetch<{ success: boolean }>(`/api/v1/registries/${id}`, {
    method: "DELETE",
  })
}

export async function checkRegistry(id: string): Promise<{ registry: Registry; result: RegistryCheckResult }> {
  return apiFetch<{ registry: Registry; result: RegistryCheckResult }>(`/api/v1/registries/${id}/check`, {
    method: "POST",
  })
}

export async function enableRegistry(id: string): Promise<Registry> {
  return apiFetch<Registry>(`/api/v1/registries/${id}/enable`, {
    method: "POST",
  })
}

export async function disableRegistry(id: string): Promise<Registry> {
  return apiFetch<Registry>(`/api/v1/registries/${id}/disable`, {
    method: "POST",
  })
}
