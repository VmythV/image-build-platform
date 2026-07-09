import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Archive, CheckCircle2, Copy, Image, Loader2, RefreshCw, Server, TriangleAlert } from "lucide-react"
import { type ReactNode, useMemo, useState } from "react"

import { Button } from "@/components/ui/button"
import { archiveArtifact, deprecateArtifact, getArtifactPullCommand, listArtifacts, repushArtifact, type ImageArtifact } from "@/lib/artifacts-api"
import { listRegistries } from "@/lib/registries-api"
import { cn } from "@/lib/utils"

export function ArtifactsPage() {
  const queryClient = useQueryClient()
  const [copiedArtifactId, setCopiedArtifactId] = useState<string | null>(null)
  const [selectedRegistries, setSelectedRegistries] = useState<Record<string, string>>({})
  const [message, setMessage] = useState("")
  const artifactsQuery = useQuery({
    queryKey: ["artifacts"],
    queryFn: listArtifacts,
    refetchInterval: 5000,
  })
  const registriesQuery = useQuery({
    queryKey: ["registries"],
    queryFn: listRegistries,
  })

  const artifacts = artifactsQuery.data ?? []
  const pushRegistries = (registriesQuery.data ?? []).filter((registry) => registry.allowPush && registry.status !== "disabled")
  const stats = useMemo(() => summarizeArtifacts(artifacts), [artifacts])

  async function copyPullCommand(artifact: ImageArtifact) {
    const command = await getArtifactPullCommand(artifact.id)
    await navigator.clipboard.writeText(command)
    setCopiedArtifactId(artifact.id)
    window.setTimeout(() => setCopiedArtifactId((current) => (current === artifact.id ? null : current)), 1800)
  }

  const repushMutation = useMutation({
    mutationFn: ({ artifact, registryId }: { artifact: ImageArtifact; registryId: string }) => repushArtifact(artifact.id, registryId),
    onSuccess: (result) => {
      setMessage(`Repushed: ${result.artifact.imageRef}`)
      return queryClient.invalidateQueries({ queryKey: ["artifacts"] })
    },
    onError: (error) => {
      setMessage(error instanceof Error ? error.message : "Failed to repush artifact.")
    },
  })
  const archiveMutation = useMutation({
    mutationFn: archiveArtifact,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["artifacts"] }),
  })
  const deprecateMutation = useMutation({
    mutationFn: deprecateArtifact,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["artifacts"] }),
  })

  return (
    <div className="space-y-4">
      <section className="grid gap-3 md:grid-cols-3">
        <Metric title="Artifacts" value={stats.total} icon={<Image className="size-4" aria-hidden="true" />} />
        <Metric title="Architectures" value={stats.architectures} icon={<Server className="size-4" aria-hidden="true" />} />
        <Metric title="Pushed" value={stats.pushed} icon={<CheckCircle2 className="size-4" aria-hidden="true" />} />
      </section>

      <section className="rounded-lg border bg-card text-card-foreground">
        <div className="flex flex-col gap-2 border-b p-4 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h2 className="text-base font-semibold">Image Artifacts</h2>
            <p className="mt-1 text-sm text-muted-foreground">Images produced and pushed by build tasks.</p>
          </div>
          {message ? <div className="text-sm text-muted-foreground">{message}</div> : null}
        </div>

        {artifactsQuery.isPending ? (
          <StateBlock title="Loading artifacts" detail="Fetching pushed image records." />
        ) : artifactsQuery.isError ? (
          <StateBlock title="Failed to load artifacts" detail="Please retry after checking the backend service." />
        ) : artifacts.length === 0 ? (
          <StateBlock title="No artifacts" detail="Build and push an image to create the first artifact." />
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full min-w-[980px] border-collapse text-left text-sm">
              <thead className="border-b bg-muted/60 text-xs uppercase text-muted-foreground">
                <tr>
                  <th className="px-4 py-3 font-medium">Image</th>
                  <th className="px-4 py-3 font-medium">Project</th>
                  <th className="px-4 py-3 font-medium">Registry</th>
                  <th className="px-4 py-3 font-medium">Digest</th>
                  <th className="px-4 py-3 font-medium">Size</th>
                  <th className="px-4 py-3 font-medium">Pushed</th>
                  <th className="px-4 py-3 font-medium">Repush Target</th>
                  <th className="px-4 py-3 text-right font-medium">Actions</th>
                </tr>
              </thead>
              <tbody>
                {artifacts.map((artifact) => (
                  <tr key={artifact.id} className="border-b last:border-b-0">
                    <td className="px-4 py-3">
                      <div className="max-w-[320px] break-all font-medium">{artifact.imageRef}</div>
                      <div className="mt-1 flex flex-wrap gap-1">
                        <Pill>{artifact.architecture}</Pill>
                        <Pill>{artifact.status}</Pill>
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <div>{artifact.projectName}</div>
                      <div className="mt-1 text-xs text-muted-foreground">{artifact.version}</div>
                    </td>
                    <td className="px-4 py-3">{artifact.registryName}</td>
                    <td className="px-4 py-3">
                      <div className="max-w-[220px] truncate font-mono text-xs">{artifact.digest ?? "-"}</div>
                      {artifact.imageId ? <div className="mt-1 max-w-[220px] truncate font-mono text-xs text-muted-foreground">{artifact.imageId}</div> : null}
                    </td>
                    <td className="px-4 py-3">{formatBytes(artifact.sizeBytes)}</td>
                    <td className="px-4 py-3 text-muted-foreground">{formatDate(artifact.pushedAt ?? artifact.createdAt)}</td>
                    <td className="px-4 py-3">
                      <select
                        className="h-9 w-full rounded-md border bg-background px-2 text-sm outline-none focus:border-ring focus:ring-2 focus:ring-ring/20"
                        value={selectedRegistries[artifact.id] ?? ""}
                        onChange={(event) => setSelectedRegistries((current) => ({ ...current, [artifact.id]: event.target.value }))}
                      >
                        <option value="">Current registry</option>
                        {pushRegistries.map((registry) => (
                          <option key={registry.id} value={registry.id}>
                            {registry.name}
                          </option>
                        ))}
                      </select>
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex flex-wrap justify-end gap-2">
                        <Button variant="outline" size="sm" onClick={() => void copyPullCommand(artifact)}>
                          <Copy aria-hidden="true" />
                          {copiedArtifactId === artifact.id ? "Copied" : "Pull"}
                        </Button>
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => repushMutation.mutate({ artifact, registryId: selectedRegistries[artifact.id] ?? "" })}
                          disabled={repushMutation.isPending || artifact.status === "archived"}
                        >
                          {repushMutation.isPending ? <Loader2 className="animate-spin" aria-hidden="true" /> : <RefreshCw aria-hidden="true" />}
                          Repush
                        </Button>
                        <Button variant="outline" size="sm" onClick={() => deprecateMutation.mutate(artifact.id)} disabled={deprecateMutation.isPending || artifact.deprecated}>
                          <TriangleAlert aria-hidden="true" />
                          {artifact.deprecated ? "Deprecated" : "Deprecate"}
                        </Button>
                        <Button variant="outline" size="sm" onClick={() => archiveMutation.mutate(artifact.id)} disabled={archiveMutation.isPending || artifact.status === "archived"}>
                          <Archive aria-hidden="true" />
                          Archive
                        </Button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </div>
  )
}

function Metric({ title, value, icon }: { title: string; value: number; icon: ReactNode }) {
  return (
    <div className="rounded-lg border bg-card p-4 text-card-foreground">
      <div className="flex items-center justify-between text-sm text-muted-foreground">
        <span>{title}</span>
        {icon}
      </div>
      <div className="mt-1 text-2xl font-semibold">{value}</div>
    </div>
  )
}

function StateBlock({ title, detail }: { title: string; detail: string }) {
  return (
    <div className="p-8 text-center">
      <h3 className="text-base font-semibold">{title}</h3>
      <p className="mt-1 text-sm text-muted-foreground">{detail}</p>
    </div>
  )
}

function Pill({ children }: { children: ReactNode }) {
  return <span className={cn("inline-flex items-center rounded-md bg-secondary px-1.5 py-0.5 text-xs text-secondary-foreground")}>{children}</span>
}

function summarizeArtifacts(artifacts: ImageArtifact[]) {
  const architectures = new Set(artifacts.map((artifact) => artifact.architecture).filter(Boolean))
  return {
    total: artifacts.length,
    architectures: architectures.size,
    pushed: artifacts.filter((artifact) => artifact.pushed).length,
  }
}

function formatBytes(value?: number | null) {
  if (!value) {
    return "-"
  }
  return new Intl.NumberFormat(undefined, { maximumFractionDigits: 1, notation: "compact" }).format(value) + "B"
}

function formatDate(value?: string | null) {
  if (!value) {
    return "-"
  }
  return new Date(value).toLocaleString()
}
