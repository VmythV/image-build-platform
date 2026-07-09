import { apiFetch } from "@/lib/api"

export type ImageArtifact = {
  id: string
  buildTaskId: string
  projectId: string
  projectName: string
  versionNodeId: string
  version: string
  registryId: string
  registryName: string
  imageRef: string
  imageId?: string | null
  digest?: string | null
  tag: string
  architecture: string
  sizeBytes?: number | null
  status: string
  pushed: boolean
  pushedAt?: string | null
  deprecated: boolean
  createdAt: string
  updatedAt: string
}

export type ArtifactPushEvent = {
  id: string
  artifactId: string
  buildTaskId?: string | null
  registryId: string
  status: string
  errorMessage?: string | null
  startedAt: string
  finishedAt?: string | null
  createdBy?: string | null
  createdAt: string
}

export type ArtifactRepushResult = {
  artifact: ImageArtifact
  event: ArtifactPushEvent
  logPath?: string | null
}

export async function listArtifacts(): Promise<ImageArtifact[]> {
  return apiFetch<ImageArtifact[]>("/api/v1/artifacts")
}

export async function getArtifactPullCommand(id: string): Promise<string> {
  const result = await apiFetch<{ command: string }>(`/api/v1/artifacts/${id}/pull-command`)
  return result.command
}

export async function repushArtifact(id: string, registryId?: string): Promise<ArtifactRepushResult> {
  return apiFetch<ArtifactRepushResult>(`/api/v1/artifacts/${id}/repush`, {
    method: "POST",
    json: { registryId: registryId ?? "" },
  })
}

export async function archiveArtifact(id: string): Promise<ImageArtifact> {
  return apiFetch<ImageArtifact>(`/api/v1/artifacts/${id}/archive`, {
    method: "POST",
  })
}

export async function deprecateArtifact(id: string): Promise<ImageArtifact> {
  return apiFetch<ImageArtifact>(`/api/v1/artifacts/${id}/deprecate`, {
    method: "POST",
  })
}
