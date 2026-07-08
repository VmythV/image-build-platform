import { apiFetch } from "@/lib/api"

export type DockerfileGenerateInput = {
  baseImage: string
  environment: Record<string, string>
  workdir: string
  packages: string[]
  copy: Array<{ source: string; target: string }>
  expose: number[]
  cmd: string[]
  entrypoint: string[]
  args: Record<string, string>
  labels: Record<string, string>
}

export type DockerfileValidationResult = {
  valid: boolean
  warnings: string[]
  errors: string[]
}

export async function generateDockerfile(input: DockerfileGenerateInput): Promise<string> {
  const result = await apiFetch<{ dockerfile: string }>("/api/v1/dockerfile/generate", {
    method: "POST",
    json: input,
  })
  return result.dockerfile
}

export async function validateDockerfile(dockerfile: string): Promise<DockerfileValidationResult> {
  return apiFetch<DockerfileValidationResult>("/api/v1/dockerfile/validate", {
    method: "POST",
    json: { dockerfile },
  })
}
