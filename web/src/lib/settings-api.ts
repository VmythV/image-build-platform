import { apiFetch } from "@/lib/api"

export type SystemSetting = {
  key: string
  value: string
  valueType: "integer" | "boolean" | "string"
  description: string
  updatedBy?: string | null
  updatedAt: string
}

export async function listSettings(): Promise<SystemSetting[]> {
  return apiFetch<SystemSetting[]>("/api/v1/settings")
}

export async function updateSetting(key: string, value: string): Promise<SystemSetting> {
  return apiFetch<SystemSetting>(`/api/v1/settings/${encodeURIComponent(key)}`, {
    method: "PUT",
    json: { value },
  })
}
