import { Activity, CheckCircle2, Clock3, Server, XCircle } from "lucide-react"

const metrics = [
  {
    label: "Running Builds",
    value: "0",
    detail: "No active build tasks",
    icon: Activity,
  },
  {
    label: "Queued Builds",
    value: "0",
    detail: "Scheduler idle",
    icon: Clock3,
  },
  {
    label: "Build Hosts",
    value: "0",
    detail: "Add local or SSH builders",
    icon: Server,
  },
  {
    label: "Recent Failures",
    value: "0",
    detail: "No failures recorded",
    icon: XCircle,
  },
]

const milestones = [
  "Go backend server and health check",
  "React Vite management shell",
  "Shared build commands",
  "Docker and CI foundation",
]

export function DashboardPage() {
  return (
    <div className="space-y-6">
      <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        {metrics.map((metric) => (
          <div key={metric.label} className="rounded-lg border bg-card p-4 text-card-foreground">
            <div className="flex items-center justify-between">
              <p className="text-sm text-muted-foreground">{metric.label}</p>
              <metric.icon className="size-4 text-muted-foreground" aria-hidden="true" />
            </div>
            <div className="mt-3 text-2xl font-semibold">{metric.value}</div>
            <p className="mt-1 text-xs text-muted-foreground">{metric.detail}</p>
          </div>
        ))}
      </section>

      <section className="grid gap-4 xl:grid-cols-[1.25fr_0.75fr]">
        <div className="rounded-lg border bg-card p-5 text-card-foreground">
          <div className="flex items-start justify-between gap-4">
            <div>
              <h2 className="text-base font-semibold">M1 Scaffold Status</h2>
              <p className="mt-1 text-sm text-muted-foreground">
                The first implementation milestone establishes the runnable application shell.
              </p>
            </div>
            <span className="rounded-md bg-secondary px-2 py-1 text-xs text-secondary-foreground">Planning</span>
          </div>

          <div className="mt-5 divide-y rounded-md border">
            {milestones.map((item) => (
              <div key={item} className="flex items-center gap-3 px-3 py-3 text-sm">
                <CheckCircle2 className="size-4 text-muted-foreground" aria-hidden="true" />
                <span>{item}</span>
              </div>
            ))}
          </div>
        </div>

        <div className="rounded-lg border bg-card p-5 text-card-foreground">
          <h2 className="text-base font-semibold">Next Operational Areas</h2>
          <div className="mt-4 space-y-3 text-sm">
            <div>
              <div className="font-medium">Build Hosts</div>
              <p className="text-muted-foreground">Local Docker and SSH builders will be configured here.</p>
            </div>
            <div>
              <div className="font-medium">Registries</div>
              <p className="text-muted-foreground">Registry credentials will be managed with masked values.</p>
            </div>
            <div>
              <div className="font-medium">Version Graph</div>
              <p className="text-muted-foreground">Image versions will use a compact Git-style node graph.</p>
            </div>
          </div>
        </div>
      </section>
    </div>
  )
}
