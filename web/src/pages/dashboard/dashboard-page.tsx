import { useQuery } from "@tanstack/react-query"
import { Activity, Box, Clock3, GitBranch, Server, Warehouse, XCircle } from "lucide-react"
import { type ReactNode } from "react"

import { getDashboardSummary, type DashboardSummary } from "@/lib/dashboard-api"
import { cn } from "@/lib/utils"

export function DashboardPage() {
  const summaryQuery = useQuery({
    queryKey: ["dashboard", "summary"],
    queryFn: getDashboardSummary,
    refetchInterval: 5000,
  })

  if (summaryQuery.isPending) {
    return <StateBlock title="Loading dashboard" detail="Fetching platform summary." />
  }

  if (summaryQuery.isError) {
    return <StateBlock title="Failed to load dashboard" detail="Please retry after checking the backend service." />
  }

  const summary = summaryQuery.data
  const metrics = dashboardMetrics(summary)

  return (
    <div className="space-y-6">
      <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        {metrics.map((metric) => (
          <Metric key={metric.label} {...metric} />
        ))}
      </section>

      <section className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_420px]">
        <Panel title="Recent Build Tasks" detail="Latest queue and execution activity.">
          {summary.recentTasks.length === 0 ? (
            <EmptyLine>No build tasks yet.</EmptyLine>
          ) : (
            <div className="divide-y">
              {summary.recentTasks.map((task) => (
                <div key={task.id} className="grid gap-2 px-4 py-3 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center">
                  <div className="min-w-0">
                    <div className="truncate text-sm font-medium">{task.imageRef}</div>
                    <div className="mt-1 flex flex-wrap gap-2 text-xs text-muted-foreground">
                      <span>{task.architecture}</span>
                      <span>{task.hostName || "No host"}</span>
                      <span>{formatDate(task.createdAt)}</span>
                    </div>
                  </div>
                  <StatusPill status={task.status} />
                </div>
              ))}
            </div>
          )}
        </Panel>

        <div className="space-y-4">
          <Panel title="Capacity" detail="Current configured platform resources.">
            <div className="grid gap-2 p-4">
              <SummaryRow icon={<Server className="size-4" aria-hidden="true" />} label="Build hosts" value={`${summary.hosts.online}/${summary.hosts.total} online`} />
              <SummaryRow icon={<Warehouse className="size-4" aria-hidden="true" />} label="Registries" value={`${summary.registries.available}/${summary.registries.total} available`} />
              <SummaryRow icon={<GitBranch className="size-4" aria-hidden="true" />} label="Image projects" value={`${summary.projects.active}/${summary.projects.total} active`} />
              <SummaryRow icon={<Box className="size-4" aria-hidden="true" />} label="Artifacts" value={`${summary.artifacts.pushed}/${summary.artifacts.total} pushed`} />
            </div>
          </Panel>

          <Panel title="Recent Artifacts" detail="Newest pushed images.">
            {summary.recentArtifacts.length === 0 ? (
              <EmptyLine>No pushed artifacts yet.</EmptyLine>
            ) : (
              <div className="divide-y">
                {summary.recentArtifacts.map((artifact) => (
                  <div key={artifact.id} className="px-4 py-3">
                    <div className="break-all text-sm font-medium">{artifact.imageRef}</div>
                    <div className="mt-1 flex flex-wrap gap-2 text-xs text-muted-foreground">
                      <span>{artifact.projectName}</span>
                      <span>{artifact.registryName}</span>
                      <span>{artifact.architecture}</span>
                    </div>
                    {artifact.digest ? <div className="mt-1 truncate font-mono text-xs text-muted-foreground">{artifact.digest}</div> : null}
                  </div>
                ))}
              </div>
            )}
          </Panel>
        </div>
      </section>
    </div>
  )
}

function dashboardMetrics(summary: DashboardSummary) {
  return [
    {
      label: "Running Builds",
      value: summary.builds.running,
      detail: summary.builds.running === 0 ? "No active build tasks" : "Build or push in progress",
      icon: <Activity className="size-4" aria-hidden="true" />,
    },
    {
      label: "Queued Builds",
      value: summary.builds.queued,
      detail: summary.builds.queued === 0 ? "Scheduler idle" : "Waiting for capacity",
      icon: <Clock3 className="size-4" aria-hidden="true" />,
    },
    {
      label: "Build Hosts",
      value: summary.hosts.total,
      detail: `${summary.hosts.online} online, ${summary.hosts.disabled} disabled`,
      icon: <Server className="size-4" aria-hidden="true" />,
    },
    {
      label: "Recent Failures",
      value: summary.builds.failed,
      detail: summary.builds.failed === 0 ? "No failures recorded" : "Open Build Tasks for logs",
      icon: <XCircle className="size-4" aria-hidden="true" />,
    },
  ]
}

function Metric({ label, value, detail, icon }: { label: string; value: number; detail: string; icon: ReactNode }) {
  return (
    <div className="rounded-lg border bg-card p-4 text-card-foreground">
      <div className="flex items-center justify-between">
        <p className="text-sm text-muted-foreground">{label}</p>
        <span className="text-muted-foreground">{icon}</span>
      </div>
      <div className="mt-3 text-2xl font-semibold">{value}</div>
      <p className="mt-1 text-xs text-muted-foreground">{detail}</p>
    </div>
  )
}

function Panel({ title, detail, children }: { title: string; detail: string; children: ReactNode }) {
  return (
    <section className="rounded-lg border bg-card text-card-foreground">
      <div className="border-b px-4 py-3">
        <h2 className="text-base font-semibold">{title}</h2>
        <p className="mt-1 text-sm text-muted-foreground">{detail}</p>
      </div>
      {children}
    </section>
  )
}

function SummaryRow({ icon, label, value }: { icon: ReactNode; label: string; value: string }) {
  return (
    <div className="flex items-center justify-between gap-3 rounded-md border bg-background px-3 py-2 text-sm">
      <div className="flex min-w-0 items-center gap-2 text-muted-foreground">
        {icon}
        <span>{label}</span>
      </div>
      <span className="font-medium">{value}</span>
    </div>
  )
}

function StatusPill({ status }: { status: string }) {
  return <span className={cn("inline-flex items-center rounded-md px-2 py-1 text-xs font-medium", statusClass(status))}>{status}</span>
}

function EmptyLine({ children }: { children: ReactNode }) {
  return <div className="px-4 py-8 text-center text-sm text-muted-foreground">{children}</div>
}

function StateBlock({ title, detail }: { title: string; detail: string }) {
  return (
    <div className="rounded-lg border bg-card p-8 text-center text-card-foreground">
      <h2 className="text-base font-semibold">{title}</h2>
      <p className="mt-1 text-sm text-muted-foreground">{detail}</p>
    </div>
  )
}

function statusClass(status: string) {
  if (status === "queued") {
    return "bg-sky-50 text-sky-700"
  }
  if (status === "dispatching" || status === "building" || status === "pushing" || status === "preparing_context") {
    return "bg-amber-50 text-amber-700"
  }
  if (status === "build_success" || status === "push_success") {
    return "bg-emerald-50 text-emerald-700"
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
