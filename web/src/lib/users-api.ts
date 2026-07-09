import { apiFetch } from "@/lib/api"
import { type User } from "@/lib/auth-api"

export type CreateUserInput = {
  username: string
  password: string
  displayName: string
  role: User["role"]
}

export type UpdateUserInput = {
  displayName: string
  role: User["role"]
  status: "active" | "disabled"
}

export async function listUsers(): Promise<User[]> {
  return apiFetch<User[]>("/api/v1/users")
}

export async function createUser(input: CreateUserInput): Promise<User> {
  return apiFetch<User>("/api/v1/users", {
    method: "POST",
    json: input,
  })
}

export async function updateUser(id: string, input: UpdateUserInput): Promise<User> {
  return apiFetch<User>(`/api/v1/users/${id}`, {
    method: "PUT",
    json: input,
  })
}

export async function resetUserPassword(id: string, password: string): Promise<User> {
  return apiFetch<User>(`/api/v1/users/${id}/password`, {
    method: "POST",
    json: { password },
  })
}
