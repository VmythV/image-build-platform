import { type Dispatch, type FormEvent, type ReactNode, type SetStateAction, useMemo, useState } from "react"
import {
  AlertTriangle,
  CheckCircle2,
  HardDrive,
  Loader2,
  type LucideIcon,
  Plus,
  Power,
  RefreshCw,
  Server,
  Trash2,
  XCircle,
} from "lucide-react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import { Button } from "@/components/ui/button"
import {
  checkBuildHost,
  createBuildHost,
  deleteBuildHost,
  disableBuildHost,
  enableBuildHost,
  listBuildHosts,
  type BuildHost,
  type SaveBuildHostInput,
} from "@/lib/build-hosts-api"
import { cn } from "@/lib/utils"

type HostFormState = {
  name: string
  connectionType: "local_docker" | "ssh"
  dockerEndpoint: string
  dockerCommand: string
  address: string
  port: string
  username: string
  maxConcurrency: string
  labels: string
}

const defaultForm: HostFormState = {
  name: "Local Docker",
  connectionType: "local_docker",
  dockerEndpoint: "/var/run/docker.sock",
  dockerCommand: "docker",
  address: "",
  port: "22",
  username: "",
  maxConcurrency: "1",
  labels: "local",
}

export function BuildHostsPage() {
  const queryClient = useQueryClient()
  const [form, setForm] = useState(defaultForm)
  const [showForm, setShowForm] = useState(false)
  const [errorMessage, setErrorMessage] = useState("")
  const [checkingHostID, setCheckingHostID] = useState<string | null>(null)

  const hostsQuery = useQuery({
    queryKey: ["build-hosts"],
    queryFn: listBuildHosts,
  })

  const createMutation = useMutation({
    mutationFn: createBuildHost,
    onSuccess: () => {
      setShowForm(false)
      setForm(defaultForm)
      return queryClient.invalidateQueries({ queryKey: ["build-hosts"] })
    },
  })

  const checkMutation = useMutation({
    mutationFn: checkBuildHost,
    onSettled: () => {
      setCheckingHostID(null)
      return queryClient.invalidateQueries({ queryKey: ["build-hosts"] })
    },
  })

  const toggleMutation = useMutation({
    mutationFn: ({ host }: { host: BuildHost }) =>
      host.status === "disabled" ? enableBuildHost(host.id) : disableBuildHost(host.id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["build-hosts"] }),
  })

  const deleteMutation = useMutation({
    mutationFn: deleteBuildHost,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["build-hosts"] }),
  })

  const hostStats = useMemo(() => summarizeHosts(hostsQuery.data ?? []), [hostsQuery.data])

  async function handleCreate(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setErrorMessage("")

    try {
      await createMutation.mutateAsync(toSaveInput(form))
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : "创建构建主机失败。")
    }
  }

  function handleConnectionTypeChange(connectionType: "local_docker" | "ssh") {
    setForm((current) => ({
      ...current,
      connectionType,
      name: connectionType === "local_docker" ? "Local Docker" : "SSH Builder",
      labels: connectionType === "local_docker" ? "local" : "remote",
    }))
  }

  return (
    <div className="space-y-6">
      <section className="grid gap-4 md:grid-cols-3">
        <Metric label="Total Hosts" value={String(hostStats.total)} detail="Registered build capacity" icon={Server} />
        <Metric label="Online" value={String(hostStats.online)} detail="Ready for future scheduling" icon={CheckCircle2} />
        <Metric label="Disabled" value={String(hostStats.disabled)} detail="Excluded from scheduling" icon={Power} />
      </section>

      <section className="rounded-lg border bg-card p-5 text-card-foreground">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
          <div>
            <h2 className="text-base font-semibold">Build Hosts</h2>
            <p className="mt-1 max-w-3xl text-sm text-muted-foreground">
              Local Docker uses the server Docker CLI. In Docker deployments, mount the host Docker socket only when
              this platform should control the host Docker daemon.
            </p>
          </div>
          <Button size="sm" onClick={() => setShowForm((value) => !value)}>
            <Plus aria-hidden="true" />
            Add Host
          </Button>
        </div>

        {showForm ? (
          <form className="mt-5 grid gap-4 rounded-md border bg-background p-4" onSubmit={handleCreate}>
            <div className="flex flex-wrap gap-2">
              <button
                type="button"
                className={cn(
                  "inline-flex h-9 items-center gap-2 rounded-md border px-3 text-sm",
                  form.connectionType === "local_docker" && "border-primary bg-primary text-primary-foreground",
                )}
                onClick={() => handleConnectionTypeChange("local_docker")}
              >
                <HardDrive className="size-4" aria-hidden="true" />
                Local Docker
              </button>
              <button
                type="button"
                className={cn(
                  "inline-flex h-9 items-center gap-2 rounded-md border px-3 text-sm",
                  form.connectionType === "ssh" && "border-primary bg-primary text-primary-foreground",
                )}
                onClick={() => handleConnectionTypeChange("ssh")}
              >
                <Server className="size-4" aria-hidden="true" />
                SSH
              </button>
            </div>

            <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
              <Field label="Name">
                <input className={inputClassName} value={form.name} onChange={(event) => updateForm(setForm, "name", event.target.value)} required />
              </Field>

              <Field label="Docker Command">
                <input
                  className={inputClassName}
                  value={form.dockerCommand}
                  onChange={(event) => updateForm(setForm, "dockerCommand", event.target.value)}
                  required
                />
              </Field>

              <Field label="Max Concurrency">
                <input
                  className={inputClassName}
                  min={1}
                  type="number"
                  value={form.maxConcurrency}
                  onChange={(event) => updateForm(setForm, "maxConcurrency", event.target.value)}
                  required
                />
              </Field>

              <Field label="Labels">
                <input className={inputClassName} value={form.labels} onChange={(event) => updateForm(setForm, "labels", event.target.value)} />
              </Field>

              {form.connectionType === "local_docker" ? (
                <Field label="Docker Endpoint">
                  <input
                    className={inputClassName}
                    value={form.dockerEndpoint}
                    onChange={(event) => updateForm(setForm, "dockerEndpoint", event.target.value)}
                  />
                </Field>
              ) : (
                <>
                  <Field label="Address">
                    <input
                      className={inputClassName}
                      value={form.address}
                      onChange={(event) => updateForm(setForm, "address", event.target.value)}
                      required
                    />
                  </Field>
                  <Field label="Port">
                    <input
                      className={inputClassName}
                      min={1}
                      max={65535}
                      type="number"
                      value={form.port}
                      onChange={(event) => updateForm(setForm, "port", event.target.value)}
                      required
                    />
                  </Field>
                  <Field label="Username">
                    <input
                      className={inputClassName}
                      value={form.username}
                      onChange={(event) => updateForm(setForm, "username", event.target.value)}
                      required
                    />
                  </Field>
                </>
              )}
            </div>

            {form.connectionType === "ssh" ? (
              <div className="flex gap-2 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-800">
                <AlertTriangle className="mt-0.5 size-4 shrink-0" aria-hidden="true" />
                <span>SSH detection currently uses keys or agent configuration available to the backend process.</span>
              </div>
            ) : null}

            {errorMessage ? (
              <div className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
                {errorMessage}
              </div>
            ) : null}

            <div className="flex justify-end gap-2">
              <Button type="button" variant="outline" onClick={() => setShowForm(false)}>
                Cancel
              </Button>
              <Button type="submit" disabled={createMutation.isPending}>
                {createMutation.isPending ? <Loader2 className="animate-spin" aria-hidden="true" /> : <Plus aria-hidden="true" />}
                Create Host
              </Button>
            </div>
          </form>
        ) : null}
      </section>

      <section className="rounded-lg border bg-card text-card-foreground">
        {hostsQuery.isPending ? (
          <StateBlock title="Loading build hosts" detail="Fetching configured build hosts." />
        ) : hostsQuery.isError ? (
          <StateBlock title="Failed to load build hosts" detail="Please retry after checking the backend service." />
        ) : hostsQuery.data.length === 0 ? (
          <StateBlock title="No build hosts" detail="Add a local or SSH host to start building images." />
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full min-w-[920px] border-collapse text-left text-sm">
              <thead className="border-b bg-muted/60 text-xs uppercase text-muted-foreground">
                <tr>
                  <th className="px-4 py-3 font-medium">Host</th>
                  <th className="px-4 py-3 font-medium">Connection</th>
                  <th className="px-4 py-3 font-medium">Architecture</th>
                  <th className="px-4 py-3 font-medium">Docker</th>
                  <th className="px-4 py-3 font-medium">Concurrency</th>
                  <th className="px-4 py-3 font-medium">Status</th>
                  <th className="px-4 py-3 font-medium">Last Check</th>
                  <th className="px-4 py-3 text-right font-medium">Actions</th>
                </tr>
              </thead>
              <tbody>
                {hostsQuery.data.map((host) => (
                  <tr key={host.id} className="border-b last:border-b-0">
                    <td className="px-4 py-3">
                      <div className="font-medium">{host.name}</div>
                      <div className="mt-1 flex flex-wrap gap-1">
                        {host.labels.map((label) => (
                          <span key={label} className="rounded-md bg-secondary px-1.5 py-0.5 text-xs text-secondary-foreground">
                            {label}
                          </span>
                        ))}
                      </div>
                    </td>
                    <td className="px-4 py-3 text-muted-foreground">
                      <div>{host.connectionType === "local_docker" ? "Local Docker" : "SSH"}</div>
                      <div className="mt-1 max-w-44 truncate text-xs">{host.connectionType === "ssh" ? host.address : host.dockerEndpoint}</div>
                    </td>
                    <td className="px-4 py-3">{host.architecture ?? "-"}</td>
                    <td className="px-4 py-3">
                      <div>{host.dockerVersion ?? "-"}</div>
                      <div className="mt-1 text-xs text-muted-foreground">BuildKit {host.buildkitSupported ? "yes" : "unknown"}</div>
                    </td>
                    <td className="px-4 py-3">
                      {host.currentRunning}/{host.maxConcurrency}
                    </td>
                    <td className="px-4 py-3">
                      <StatusBadge status={host.status} />
                      {host.lastError ? <div className="mt-1 max-w-48 truncate text-xs text-red-700">{host.lastError}</div> : null}
                    </td>
                    <td className="px-4 py-3 text-muted-foreground">{formatDate(host.lastCheckedAt)}</td>
                    <td className="px-4 py-3">
                      <div className="flex justify-end gap-2">
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => {
                            setCheckingHostID(host.id)
                            checkMutation.mutate(host.id)
                          }}
                          disabled={checkingHostID === host.id}
                        >
                          {checkingHostID === host.id ? <Loader2 className="animate-spin" aria-hidden="true" /> : <RefreshCw aria-hidden="true" />}
                          Check
                        </Button>
                        <Button variant="ghost" size="icon" onClick={() => toggleMutation.mutate({ host })} aria-label={host.status === "disabled" ? "Enable host" : "Disable host"}>
                          <Power aria-hidden="true" />
                        </Button>
                        <Button variant="ghost" size="icon" onClick={() => deleteMutation.mutate(host.id)} aria-label="Delete host">
                          <Trash2 aria-hidden="true" />
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

function Metric({ label, value, detail, icon: Icon }: { label: string; value: string; detail: string; icon: LucideIcon }) {
  return (
    <div className="rounded-lg border bg-card p-4 text-card-foreground">
      <div className="flex items-center justify-between">
        <p className="text-sm text-muted-foreground">{label}</p>
        <Icon className="size-4 text-muted-foreground" aria-hidden="true" />
      </div>
      <div className="mt-3 text-2xl font-semibold">{value}</div>
      <p className="mt-1 text-xs text-muted-foreground">{detail}</p>
    </div>
  )
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <label className="block space-y-2 text-sm font-medium">
      <span>{label}</span>
      {children}
    </label>
  )
}

function StateBlock({ title, detail }: { title: string; detail: string }) {
  return (
    <div className="flex min-h-48 items-center justify-center p-6 text-center">
      <div>
        <h3 className="text-base font-semibold">{title}</h3>
        <p className="mt-1 text-sm text-muted-foreground">{detail}</p>
      </div>
    </div>
  )
}

function StatusBadge({ status }: { status: BuildHost["status"] }) {
  const style = {
    online: "border-emerald-200 bg-emerald-50 text-emerald-700",
    disabled: "border-slate-200 bg-slate-50 text-slate-700",
    unavailable: "border-red-200 bg-red-50 text-red-700",
    offline: "border-red-200 bg-red-50 text-red-700",
    busy: "border-amber-200 bg-amber-50 text-amber-700",
    unknown: "border-zinc-200 bg-zinc-50 text-zinc-700",
  }[status]

  const Icon = status === "online" ? CheckCircle2 : status === "unknown" || status === "disabled" ? AlertTriangle : XCircle

  return (
    <span className={cn("inline-flex h-7 items-center gap-1.5 rounded-md border px-2 text-xs font-medium", style)}>
      <Icon className="size-3.5" aria-hidden="true" />
      {status}
    </span>
  )
}

function summarizeHosts(hosts: BuildHost[]) {
  return hosts.reduce(
    (summary, host) => ({
      total: summary.total + 1,
      online: summary.online + (host.status === "online" ? 1 : 0),
      disabled: summary.disabled + (host.status === "disabled" ? 1 : 0),
    }),
    { total: 0, online: 0, disabled: 0 },
  )
}

function toSaveInput(form: HostFormState): SaveBuildHostInput {
  return {
    name: form.name,
    connectionType: form.connectionType,
    address: form.address,
    port: Number(form.port || 0),
    username: form.username,
    dockerEndpoint: form.dockerEndpoint,
    dockerCommand: form.dockerCommand,
    maxConcurrency: Number(form.maxConcurrency || 1),
    labels: form.labels
      .split(",")
      .map((label) => label.trim())
      .filter(Boolean),
  }
}

function updateForm(
  setForm: Dispatch<SetStateAction<HostFormState>>,
  key: keyof HostFormState,
  value: string,
) {
  setForm((current) => ({ ...current, [key]: value }))
}

function formatDate(value?: string | null) {
  if (!value) {
    return "-"
  }
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(new Date(value))
}

const inputClassName =
  "h-10 w-full rounded-md border bg-background px-3 text-sm outline-none transition-colors focus:border-ring focus:ring-2 focus:ring-ring/20"
