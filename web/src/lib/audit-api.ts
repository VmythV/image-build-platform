import { apiFetch } from "@/lib/api"

export type AuditLog = {
  id: string
  actorId?: string | null
  actorName?: string | null
  action: string
  resourceType: string
  resourceId?: string | null
  resourceName?: string | null
  ipAddress?: string | null
  userAgent?: string | null
  requestId?: string | null
  detail?: string | null
  createdAt: string
}

export async function listAuditLogs(): Promise<AuditLog[]> {
  return apiFetch<AuditLog[]>("/api/v1/audit-logs")
}
