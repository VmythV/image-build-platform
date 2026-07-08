import { AlertTriangle, CheckCircle2, KeyRound, ScrollText, Server, Warehouse } from "lucide-react"
import { type ReactNode } from "react"

export function HelpPage() {
  return (
    <div className="space-y-4">
      <section className="grid gap-4 xl:grid-cols-2">
        <HelpSection
          icon={<Server className="size-4" aria-hidden="true" />}
          title="Build Hosts"
          items={[
            "Use Build Hosts to add local Docker or SSH builders.",
            "Run Check after adding a host to detect architecture, OS, Docker version, and BuildKit support.",
            "A disabled host is excluded from scheduling; re-enable it before assigning builds.",
          ]}
        />
        <HelpSection
          icon={<Warehouse className="size-4" aria-hidden="true" />}
          title="Registries"
          items={[
            "Use Registries to add pull and push destinations.",
            "Credentials are encrypted at rest and are not returned to the browser after saving.",
            "Set one push registry as default so build tasks can resolve image destinations automatically.",
          ]}
        />
        <HelpSection
          icon={<ScrollText className="size-4" aria-hidden="true" />}
          title="Build Logs"
          items={[
            "Open Build Tasks, select a task, then use Stream Logs while a task is running.",
            "Use Load Logs after completion to fetch the full historical log file.",
            "Build failures appear as build_failed; push failures appear as push_failed.",
          ]}
        />
        <HelpSection
          icon={<KeyRound className="size-4" aria-hidden="true" />}
          title="SSH Builders"
          items={[
            "SSH builders can use an encrypted private key saved on the host record or the backend process SSH agent.",
            "The remote user must be able to run Docker and create temporary directories under /tmp.",
            "Remote builds upload a temporary context and clean it up after execution.",
          ]}
        />
      </section>

      <section className="rounded-lg border bg-card text-card-foreground">
        <div className="border-b px-4 py-3">
          <h2 className="text-base font-semibold">Operational Checks</h2>
          <p className="mt-1 text-sm text-muted-foreground">Use these checks when the platform cannot build or push images.</p>
        </div>
        <div className="divide-y">
          <CheckRow title="Host architecture mismatch" detail="Run host Check and compare the detected architecture with the build task architecture." />
          <CheckRow title="Local Docker from container deployment" detail="Mount the host Docker socket and keep Docker Endpoint set to /var/run/docker.sock." />
          <CheckRow title="Registry login failure" detail="Re-enter credentials in Registries, run Check, and verify the registry endpoint does not include a repository path." />
          <CheckRow title="Push failed after build" detail="Open Build Tasks, stream or load logs, then inspect docker login and docker push output." />
          <CheckRow title="No schedulable host" detail="Confirm host status is not disabled, current concurrency is below max concurrency, and architecture matches." />
        </div>
      </section>

      <section className="rounded-lg border border-amber-200 bg-amber-50 p-4 text-amber-900">
        <div className="flex gap-2">
          <AlertTriangle className="mt-0.5 size-4 shrink-0" aria-hidden="true" />
          <div>
            <h2 className="text-sm font-semibold">Docker Socket Risk</h2>
            <p className="mt-1 text-sm">
              Mounting the host Docker socket lets this platform control the host Docker daemon. Only enable it in a trusted environment and restrict access to maintainers and admins.
            </p>
          </div>
        </div>
      </section>
    </div>
  )
}

function HelpSection({ icon, title, items }: { icon: ReactNode; title: string; items: string[] }) {
  return (
    <section className="rounded-lg border bg-card text-card-foreground">
      <div className="flex items-center gap-2 border-b px-4 py-3">
        <span className="text-muted-foreground">{icon}</span>
        <h2 className="text-base font-semibold">{title}</h2>
      </div>
      <div className="space-y-3 p-4">
        {items.map((item) => (
          <div key={item} className="flex gap-2 text-sm">
            <CheckCircle2 className="mt-0.5 size-4 shrink-0 text-emerald-600" aria-hidden="true" />
            <span>{item}</span>
          </div>
        ))}
      </div>
    </section>
  )
}

function CheckRow({ title, detail }: { title: string; detail: string }) {
  return (
    <div className="grid gap-1 px-4 py-3 text-sm md:grid-cols-[260px_minmax(0,1fr)] md:gap-4">
      <div className="font-medium">{title}</div>
      <div className="text-muted-foreground">{detail}</div>
    </div>
  )
}
