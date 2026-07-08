import { apiFetch } from "@/lib/api"

export type User = {
  id: string
  username: string
  displayName: string
  role: "admin" | "maintainer" | "viewer"
  status?: string
  lastLoginAt?: string
  createdAt?: string
  updatedAt?: string
}

export type Credentials = {
  username: string
  password: string
}

export type SetupAdminInput = Credentials & {
  displayName: string
}

export async function getSetupStatus(): Promise<{ initialized: boolean }> {
  return apiFetch<{ initialized: boolean }>("/api/v1/setup/status")
}

export async function setupAdmin(input: SetupAdminInput): Promise<User> {
  const result = await apiFetch<{ user: User }>("/api/v1/setup/admin", {
    method: "POST",
    json: input,
  })
  return result.user
}

export async function login(input: Credentials): Promise<User> {
  const result = await apiFetch<{ user: User }>("/api/v1/auth/login", {
    method: "POST",
    json: input,
  })
  return result.user
}

export async function logout(): Promise<void> {
  await apiFetch<{ success: boolean }>("/api/v1/auth/logout", {
    method: "POST",
  })
}

export async function getCurrentUser(): Promise<User> {
  const result = await apiFetch<{ user: User }>("/api/v1/auth/me")
  return result.user
}
