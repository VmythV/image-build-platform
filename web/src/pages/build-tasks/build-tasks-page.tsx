import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { ArchiveX, GitBranch, Loader2, Play, RotateCcw, Server, XCircle } from "lucide-react"
import { type ReactNode, useMemo, useState } from "react"

import { Button } from "@/components/ui/button"
import {
  cancelBuildTask,
  dispatchBuildTask,
  dispatchNextBuildTask,
  listBuildTasks,
  retryBuildTask,
  type BuildTask,
  type BuildTaskStatus,
} from "@/lib/build-tasks-api"
import { cn } from "@/lib/utils"

const retryableStatuses: BuildTaskStatus[] = [
  "preparing_context_failed",
  "dispatch_failed",
  "build_failed",
  "push_failed",
  "canceled",
  "timeout",
]

const cancelableStatuses: BuildTaskStatus[] = ["created", "queued", "dispatching", "preparing_context", "building", "pushing"]

export function BuildTasksPage() {
  const queryClient = useQueryClient()
  const [selectedTaskId, setSelectedTaskId] = useState<string | null>(null)

  const tasksQuery = useQuery({
    queryKey: ["build-tasks"],
    queryFn: listBuildTasks,
  })

  const tasks = tasksQuery.data ?? []
  const selectedTask = useMemo(() => tasks.find((task) => task.id === selectedTaskId) ?? tasks[0] ?? null, [selectedTaskId, tasks])

  const invalidateTasks = () => queryClient.invalidateQueries({ queryKey: ["build-tasks"] })

  const dispatchNextMutation = useMutation({
    mutationFn: dispatchNextBuildTask,
    onSuccess: (result) => {
      setSelectedTaskId(result.task.id)
      return invalidateTasks()
    },
  })
  const dispatchMutation = useMutation({
    mutationFn: dispatchBuildTask,
    onSuccess: (result) => {
      setSelectedTaskId(result.task.id)
      return invalidateTasks()
    },
  })
  const cancelMutation = useMutation({
    mutationFn: cancelBuildTask,
    onSuccess: (task) => {
      setSelectedTaskId(task.id)
      return invalidateTasks()
    },
  })
  const retryMutation = useMutation({
    mutationFn: retryBuildTask,
    onSuccess: (task) => {
      setSelectedTaskId(task.id)
      return invalidateTasks()
    },
  })

  const queuedCount = tasks.filter((task) => task.status === "queued").length
  const dispatchingCount = tasks.filter((task) => task.status === "dispatching").length
  const failedCount = tasks.filter((task) => task.status.endsWith("_failed")).length

  return (
    <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_440px]">
      <section className="space-y-4">
        <div className="grid gap-3 sm:grid-cols-3">
          <Metric title="Queued" value={queuedCount} />
          <Metric title="Dispatching" value={dispatchingCount} />
          <Metric title="Failed" value={failedCount} />
        </div>

        <section className="rounded-lg border bg-card text-card-foreground">
          <div className="flex flex-col gap-3 border-b p-4 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <h2 className="text-base font-semibold">Build Queue</h2>
              <p className="mt-1 text-sm text-muted-foreground">Tasks created from image version nodes.</p>
            </div>
            <Button size="sm" onClick={() => dispatchNextMutation.mutate()} disabled={dispatchNextMutation.isPending || queuedCount === 0}>
              {dispatchNextMutation.isPending ? <Loader2 className="animate-spin" aria-hidden="true" /> : <Play aria-hidden="true" />}
              Dispatch Next
            </Button>
          </div>

          {tasksQuery.isPending ? (
            <StateBlock title="Loading build tasks" detail="Fetching queue state." />
          ) : tasksQuery.isError ? (
            <StateBlock title="Failed to load build tasks" detail="Please retry after checking the backend service." />
          ) : tasks.length === 0 ? (
            <StateBlock title="No build tasks" detail="Create a build task from an image version node." />
          ) : (
            <div className="divide-y">
              {tasks.map((task) => (
                <button
                  key={task.id}
                  type="button"
                  className={cn("block w-full px-4 py-3 text-left transition-colors hover:bg-accent", selectedTask?.id === task.id && "bg-accent")}
                  onClick={() => setSelectedTaskId(task.id)}
                >
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <div className="min-w-0">
                      <div className="truncate text-sm font-medium">{task.imageRef}</div>
                      <div className="mt-1 flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
                        <span>{task.projectName}</span>
                        <span>{task.version}</span>
                        <span>{task.architecture}</span>
                      </div>
                    </div>
                    <StatusPill status={task.status} />
                  </div>
                  <div className="mt-2 flex flex-wrap gap-1">
                    <Pill>{task.registryName}</Pill>
                    <Pill>{task.hostName || task.requestedHostName || "No host"}</Pill>
                    <Pill>{formatDate(task.createdAt)}</Pill>
                  </div>
                </button>
              ))}
            </div>
          )}
        </section>
      </section>

      <TaskDetail
        task={selectedTask}
        dispatchPending={dispatchMutation.isPending}
        cancelPending={cancelMutation.isPending}
        retryPending={retryMutation.isPending}
        onDispatch={(task) => dispatchMutation.mutate(task.id)}
        onCancel={(task) => cancelMutation.mutate(task.id)}
        onRetry={(task) => retryMutation.mutate(task.id)}
      />
    </div>
  )
}

function TaskDetail({
  task,
  dispatchPending,
  cancelPending,
  retryPending,
  onDispatch,
  onCancel,
  onRetry,
}: {
  task: BuildTask | null
  dispatchPending: boolean
  cancelPending: boolean
  retryPending: boolean
  onDispatch: (task: BuildTask) => void
  onCancel: (task: BuildTask) => void
  onRetry: (task: BuildTask) => void
}) {
  return (
    <aside className="rounded-lg border bg-card p-4 text-card-foreground">
      <h2 className="text-base font-semibold">Task Detail</h2>
      {task ? (
        <div className="mt-4 space-y-4">
          <div className="space-y-2">
            <div className="flex flex-wrap items-center gap-2">
              <StatusPill status={task.status} />
              <Pill>{task.architecture}</Pill>
              <Pill>{task.registryName}</Pill>
            </div>
            <h3 className="break-all text-sm font-medium">{task.imageRef}</h3>
            <p className="text-xs text-muted-foreground">{task.projectName} / {task.version}</p>
          </div>

          <div className="grid grid-cols-2 gap-2">
            <Button variant="outline" size="sm" onClick={() => onDispatch(task)} disabled={task.status !== "queued" || dispatchPending}>
              {dispatchPending ? <Loader2 className="animate-spin" aria-hidden="true" /> : <Play aria-hidden="true" />}
              Dispatch
            </Button>
            <Button variant="outline" size="sm" onClick={() => onCancel(task)} disabled={!cancelableStatuses.includes(task.status) || cancelPending}>
              {cancelPending ? <Loader2 className="animate-spin" aria-hidden="true" /> : <XCircle aria-hidden="true" />}
              Cancel
            </Button>
            <Button variant="outline" size="sm" onClick={() => onRetry(task)} disabled={!retryableStatuses.includes(task.status) || retryPending}>
              {retryPending ? <Loader2 className="animate-spin" aria-hidden="true" /> : <RotateCcw aria-hidden="true" />}
              Retry
            </Button>
          </div>

          <div className="space-y-2 rounded-md border bg-background p-3 text-sm">
            <DetailRow icon={<GitBranch className="size-4" aria-hidden="true" />} label="Version Node" value={task.versionNodeId} />
            <DetailRow icon={<Server className="size-4" aria-hidden="true" />} label="Host" value={task.hostName || task.requestedHostName || "-"} />
            <DetailRow icon={<ArchiveX className="size-4" aria-hidden="true" />} label="Retry Of" value={task.retryOfTaskId || "-"} />
          </div>

          {task.schedulerReason ? <InfoBlock title="Scheduler" detail={task.schedulerReason} /> : null}
          {task.errorMessage ? <InfoBlock title={task.errorCode || "Error"} detail={task.errorMessage} tone="danger" /> : null}

          <section>
            <h3 className="mb-2 text-sm font-semibold">Dockerfile Snapshot</h3>
            <pre className="max-h-[360px] overflow-auto rounded-md border bg-background p-3 text-xs">{task.dockerfileSnapshot}</pre>
          </section>
        </div>
      ) : (
        <p className="mt-2 text-sm text-muted-foreground">Select a build task to inspect queue state and Dockerfile snapshot.</p>
      )}
    </aside>
  )
}

function Metric({ title, value }: { title: string; value: number }) {
  return (
    <div className="rounded-lg border bg-card p-4 text-card-foreground">
      <div className="text-sm text-muted-foreground">{title}</div>
      <div className="mt-1 text-2xl font-semibold">{value}</div>
    </div>
  )
}

function DetailRow({ icon, label, value }: { icon: ReactNode; label: string; value: string }) {
  return (
    <div className="flex items-start gap-2">
      <span className="mt-0.5 text-muted-foreground">{icon}</span>
      <div className="min-w-0">
        <div className="text-xs text-muted-foreground">{label}</div>
        <div className="break-all text-sm">{value}</div>
      </div>
    </div>
  )
}

function InfoBlock({ title, detail, tone = "default" }: { title: string; detail: string; tone?: "default" | "danger" }) {
  return (
    <div className={cn("rounded-md border px-3 py-2 text-sm", tone === "danger" ? "border-red-200 bg-red-50 text-red-700" : "bg-background")}>
      <div className="font-medium">{title}</div>
      <div className="mt-1">{detail}</div>
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

function StatusPill({ status }: { status: BuildTaskStatus }) {
  return (
    <span className={cn("inline-flex items-center rounded-md px-2 py-1 text-xs font-medium", statusClass(status))}>
      {status}
    </span>
  )
}

function Pill({ children }: { children: ReactNode }) {
  return <span className="inline-flex items-center rounded-md bg-secondary px-1.5 py-0.5 text-xs text-secondary-foreground">{children}</span>
}

function statusClass(status: BuildTaskStatus) {
  if (status === "queued") {
    return "bg-sky-50 text-sky-700"
  }
  if (status === "dispatching" || status === "building" || status === "pushing" || status === "preparing_context") {
    return "bg-amber-50 text-amber-700"
  }
  if (status === "build_success" || status === "push_success") {
    return "bg-emerald-50 text-emerald-700"
  }
  if (status === "canceled") {
    return "bg-zinc-100 text-zinc-700"
  }
  if (status.endsWith("_failed") || status === "timeout") {
    return "bg-red-50 text-red-700"
  }
  return "bg-secondary text-secondary-foreground"
}

function formatDate(value?: string | null) {
  if (!value) {
    return "-"
  }
  return new Date(value).toLocaleString()
}
