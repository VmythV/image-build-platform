import { apiFetch } from "@/lib/api"

export type ImageType = "java" | "python" | "nodejs" | "mysql" | "base_os" | "database" | "middleware" | "other"
export type ProjectStatus = "active" | "archived"
export type BranchStatus = "active" | "archived"
export type VersionNodeStatus = "draft" | "active" | "archived"

export type ImageProject = {
  id: string
  name: string
  imageType: ImageType
  imageName: string
  namespace?: string | null
  rootImageRef: string
  rootImageSource: string
  defaultRegistryId?: string | null
  defaultArchitecture: string
  labels: string[]
  description?: string | null
  status: ProjectStatus
  ownerId?: string | null
  latestVersionNodeId?: string | null
  latestVersion?: string | null
  createdAt: string
  updatedAt: string
}

export type ImageBranch = {
  id: string
  projectId: string
  name: string
  startNodeId?: string | null
  headNodeId?: string | null
  description?: string | null
  status: BranchStatus
  createdBy?: string | null
  createdAt: string
  updatedAt: string
}

export type VersionNode = {
  id: string
  projectId: string
  branchId: string
  branchName: string
  parentNodeId?: string | null
  version: string
  dockerfile: string
  dockerfileHash: string
  formConfigSnapshot?: string | null
  buildContextRef?: string | null
  description?: string | null
  status: VersionNodeStatus
  latestBuildTaskId?: string | null
  latestArtifactId?: string | null
  graphPosition?: string | null
  createdBy?: string | null
  createdAt: string
  updatedAt: string
}

export type VersionGraph = {
  project: ImageProject
  branches: ImageBranch[]
  nodes: VersionNode[]
  edges: Array<{
    id: string
    source: string
    target: string
    sourceLabel?: string
    targetLabel?: string
  }>
}

export type ProjectInput = {
  name: string
  imageType: ImageType
  imageName: string
  namespace?: string
  rootImageRef: string
  rootImageSource: "external_image"
  defaultRegistryId?: string
  defaultArchitecture: string
  labels: string[]
  description?: string
}

export type BranchInput = {
  name: string
  startNodeId: string
  description?: string
}

export type VersionNodeInput = {
  branchId: string
  parentNodeId?: string
  version: string
  dockerfile: string
  formConfigSnapshot?: string
  description?: string
  status: VersionNodeStatus
}

export async function listImageProjects(): Promise<ImageProject[]> {
  return apiFetch<ImageProject[]>("/api/v1/image-projects")
}

export async function createImageProject(input: ProjectInput): Promise<ImageProject> {
  return apiFetch<ImageProject>("/api/v1/image-projects", {
    method: "POST",
    json: input,
  })
}

export async function archiveImageProject(id: string): Promise<ImageProject> {
  return apiFetch<ImageProject>(`/api/v1/image-projects/${id}/archive`, {
    method: "POST",
  })
}

export async function getVersionGraph(projectId: string): Promise<VersionGraph> {
  return apiFetch<VersionGraph>(`/api/v1/image-projects/${projectId}/graph`)
}

export async function createBranch(projectId: string, input: BranchInput): Promise<ImageBranch> {
  return apiFetch<ImageBranch>(`/api/v1/image-projects/${projectId}/branches`, {
    method: "POST",
    json: input,
  })
}

export async function archiveBranch(projectId: string, branchId: string): Promise<ImageBranch> {
  return apiFetch<ImageBranch>(`/api/v1/image-projects/${projectId}/branches/${branchId}/archive`, {
    method: "POST",
  })
}

export async function createVersionNode(projectId: string, input: VersionNodeInput): Promise<VersionNode> {
  return apiFetch<VersionNode>(`/api/v1/image-projects/${projectId}/version-nodes`, {
    method: "POST",
    json: input,
  })
}
