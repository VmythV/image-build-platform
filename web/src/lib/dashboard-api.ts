import { apiFetch } from "@/lib/api"

export type DashboardSummary = {
  builds: {
    running: number
    queued: number
    failed: number
    total: number
  }
  hosts: {
    online: number
    disabled: number
    total: number
  }
  registries: {
    available: number
    disabled: number
    total: number
  }
  artifacts: {
    pushed: number
    total: number
  }
  projects: {
    active: number
    total: number
  }
  recentTasks: Array<{
    id: string
    imageRef: string
    status: string
    architecture: string
    hostName?: string | null
    createdAt: string
  }>
  recentArtifacts: Array<{
    id: string
    imageRef: string
    digest?: string | null
    architecture: string
    projectName: string
    registryName: string
    createdAt: string
  }>
}

export async function getDashboardSummary(): Promise<DashboardSummary> {
  return apiFetch<DashboardSummary>("/api/v1/dashboard/summary")
}
