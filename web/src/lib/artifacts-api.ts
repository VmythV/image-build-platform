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

export async function listArtifacts(): Promise<ImageArtifact[]> {
  return apiFetch<ImageArtifact[]>("/api/v1/artifacts")
}
