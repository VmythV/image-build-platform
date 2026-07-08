import { type Dispatch, type FormEvent, type ReactNode, type SetStateAction, useMemo, useState } from "react"
import {
  AlertTriangle,
  CheckCircle2,
  Edit3,
  KeyRound,
  Loader2,
  Plus,
  Power,
  RefreshCw,
  ShieldCheck,
  Trash2,
  Warehouse,
  XCircle,
} from "lucide-react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import { Button } from "@/components/ui/button"
import {
  checkRegistry,
  createRegistry,
  deleteRegistry,
  disableRegistry,
  enableRegistry,
  listRegistries,
  updateRegistry,
  type Registry,
  type SaveRegistryInput,
} from "@/lib/registries-api"
import { cn } from "@/lib/utils"

type RegistryFormState = {
  name: string
  type: Registry["type"]
  endpoint: string
  namespace: string
  region: string
  username: string
  password: string
  allowPull: boolean
  allowPush: boolean
  isDefaultPull: boolean
  isDefaultPush: boolean
  tlsVerify: boolean
  insecureHttp: boolean
}

const defaultForm: RegistryFormState = {
  name: "Internal Registry",
  type: "generic",
  endpoint: "",
  namespace: "",
  region: "",
  username: "",
  password: "",
  allowPull: true,
  allowPush: true,
  isDefaultPull: false,
  isDefaultPush: true,
  tlsVerify: true,
  insecureHttp: false,
}

export function RegistriesPage() {
  const queryClient = useQueryClient()
  const [form, setForm] = useState(defaultForm)
  const [editingRegistry, setEditingRegistry] = useState<Registry | null>(null)
  const [showForm, setShowForm] = useState(false)
  const [errorMessage, setErrorMessage] = useState("")
  const [checkingID, setCheckingID] = useState<string | null>(null)

  const registriesQuery = useQuery({
    queryKey: ["registries"],
    queryFn: listRegistries,
  })

  const saveMutation = useMutation({
    mutationFn: (input: SaveRegistryInput) =>
      editingRegistry ? updateRegistry(editingRegistry.id, input) : createRegistry(input),
    onSuccess: () => {
      closeForm()
      return queryClient.invalidateQueries({ queryKey: ["registries"] })
    },
  })

  const checkMutation = useMutation({
    mutationFn: checkRegistry,
    onSettled: () => {
      setCheckingID(null)
      return queryClient.invalidateQueries({ queryKey: ["registries"] })
    },
  })

  const toggleMutation = useMutation({
    mutationFn: ({ registry }: { registry: Registry }) =>
      registry.status === "disabled" ? enableRegistry(registry.id) : disableRegistry(registry.id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["registries"] }),
  })

  const deleteMutation = useMutation({
    mutationFn: deleteRegistry,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["registries"] }),
  })

  const stats = useMemo(() => summarizeRegistries(registriesQuery.data ?? []), [registriesQuery.data])

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setErrorMessage("")
    try {
      await saveMutation.mutateAsync(toSaveInput(form))
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : "保存镜像仓库失败。")
    }
  }

  function startCreate() {
    setEditingRegistry(null)
    setForm(defaultForm)
    setErrorMessage("")
    setShowForm(true)
  }

  function startEdit(registry: Registry) {
    setEditingRegistry(registry)
    setForm({
      name: registry.name,
      type: registry.type,
      endpoint: registry.endpoint,
      namespace: registry.namespace ?? "",
      region: registry.region ?? "",
      username: registry.credentialUsername ?? "",
      password: "",
      allowPull: registry.allowPull,
      allowPush: registry.allowPush,
      isDefaultPull: registry.isDefaultPull,
      isDefaultPush: registry.isDefaultPush,
      tlsVerify: registry.tlsVerify,
      insecureHttp: registry.insecureHttp,
    })
    setErrorMessage("")
    setShowForm(true)
  }

  function closeForm() {
    setShowForm(false)
    setEditingRegistry(null)
    setForm(defaultForm)
    setErrorMessage("")
  }

  return (
    <div className="space-y-6">
      <section className="grid gap-4 md:grid-cols-3">
        <Metric label="Registries" value={String(stats.total)} detail="Configured endpoints" icon={Warehouse} />
        <Metric label="Available" value={String(stats.available)} detail="Last check succeeded" icon={CheckCircle2} />
        <Metric label="Default Push" value={stats.defaultPush || "-"} detail="Target for built images" icon={ShieldCheck} />
      </section>

      <section className="rounded-lg border bg-card p-5 text-card-foreground">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
          <div>
            <h2 className="text-base font-semibold">Registries</h2>
            <p className="mt-1 max-w-3xl text-sm text-muted-foreground">
              Registry credentials are encrypted at rest and never returned to the browser after saving.
            </p>
          </div>
          <Button size="sm" onClick={startCreate}>
            <Plus aria-hidden="true" />
            Add Registry
          </Button>
        </div>

        {showForm ? (
          <form className="mt-5 grid gap-4 rounded-md border bg-background p-4" onSubmit={handleSubmit}>
            <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
              <Field label="Name">
                <input className={inputClassName} value={form.name} onChange={(event) => updateForm(setForm, "name", event.target.value)} required />
              </Field>

              <Field label="Type">
                <select className={inputClassName} value={form.type} onChange={(event) => updateForm(setForm, "type", event.target.value)}>
                  <option value="generic">Generic</option>
                  <option value="harbor">Harbor</option>
                  <option value="docker_hub">Docker Hub</option>
                  <option value="aliyun">Aliyun</option>
                  <option value="tencent_cloud">Tencent Cloud</option>
                </select>
              </Field>

              <Field label="Endpoint">
                <input
                  className={inputClassName}
                  placeholder="registry.example.com"
                  value={form.endpoint}
                  onChange={(event) => updateForm(setForm, "endpoint", event.target.value)}
                  required
                />
              </Field>

              <Field label="Namespace">
                <input className={inputClassName} value={form.namespace} onChange={(event) => updateForm(setForm, "namespace", event.target.value)} />
              </Field>

              <Field label="Username">
                <input
                  className={inputClassName}
                  autoComplete="username"
                  value={form.username}
                  onChange={(event) => updateForm(setForm, "username", event.target.value)}
                />
              </Field>

              <Field label={editingRegistry ? "New Password or Token" : "Password or Token"}>
                <input
                  className={inputClassName}
                  autoComplete="new-password"
                  type="password"
                  value={form.password}
                  onChange={(event) => updateForm(setForm, "password", event.target.value)}
                />
              </Field>

              <Field label="Region">
                <input className={inputClassName} value={form.region} onChange={(event) => updateForm(setForm, "region", event.target.value)} />
              </Field>
            </div>

            <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
              <Toggle label="Allow Pull" checked={form.allowPull} onChange={(checked) => updateFormBool(setForm, "allowPull", checked)} />
              <Toggle label="Allow Push" checked={form.allowPush} onChange={(checked) => updateFormBool(setForm, "allowPush", checked)} />
              <Toggle label="Default Pull" checked={form.isDefaultPull} onChange={(checked) => updateFormBool(setForm, "isDefaultPull", checked)} />
              <Toggle label="Default Push" checked={form.isDefaultPush} onChange={(checked) => updateFormBool(setForm, "isDefaultPush", checked)} />
              <Toggle label="TLS Verify" checked={form.tlsVerify} onChange={(checked) => updateFormBool(setForm, "tlsVerify", checked)} />
              <Toggle label="Insecure HTTP" checked={form.insecureHttp} onChange={(checked) => updateFormBool(setForm, "insecureHttp", checked)} />
            </div>

            {form.insecureHttp ? (
              <div className="flex gap-2 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-800">
                <AlertTriangle className="mt-0.5 size-4 shrink-0" aria-hidden="true" />
                <span>Use insecure HTTP only for trusted internal networks or test registries.</span>
              </div>
            ) : null}

            {editingRegistry?.credentialConfigured && !form.password ? (
              <div className="flex gap-2 rounded-md border px-3 py-2 text-sm text-muted-foreground">
                <KeyRound className="mt-0.5 size-4 shrink-0" aria-hidden="true" />
                <span>Leave the password empty to keep the existing encrypted credential.</span>
              </div>
            ) : null}

            {errorMessage ? (
              <div className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
                {errorMessage}
              </div>
            ) : null}

            <div className="flex justify-end gap-2">
              <Button type="button" variant="outline" onClick={closeForm}>
                Cancel
              </Button>
              <Button type="submit" disabled={saveMutation.isPending}>
                {saveMutation.isPending ? <Loader2 className="animate-spin" aria-hidden="true" /> : <Plus aria-hidden="true" />}
                {editingRegistry ? "Save Registry" : "Create Registry"}
              </Button>
            </div>
          </form>
        ) : null}
      </section>

      <section className="rounded-lg border bg-card text-card-foreground">
        {registriesQuery.isPending ? (
          <StateBlock title="Loading registries" detail="Fetching registry configuration." />
        ) : registriesQuery.isError ? (
          <StateBlock title="Failed to load registries" detail="Please retry after checking the backend service." />
        ) : registriesQuery.data.length === 0 ? (
          <StateBlock title="No registries" detail="Add a registry before creating image projects and build tasks." />
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full min-w-[960px] border-collapse text-left text-sm">
              <thead className="border-b bg-muted/60 text-xs uppercase text-muted-foreground">
                <tr>
                  <th className="px-4 py-3 font-medium">Registry</th>
                  <th className="px-4 py-3 font-medium">Usage</th>
                  <th className="px-4 py-3 font-medium">Credential</th>
                  <th className="px-4 py-3 font-medium">Security</th>
                  <th className="px-4 py-3 font-medium">Status</th>
                  <th className="px-4 py-3 font-medium">Last Check</th>
                  <th className="px-4 py-3 text-right font-medium">Actions</th>
                </tr>
              </thead>
              <tbody>
                {registriesQuery.data.map((registry) => (
                  <tr key={registry.id} className="border-b last:border-b-0">
                    <td className="px-4 py-3">
                      <div className="font-medium">{registry.name}</div>
                      <div className="mt-1 text-xs text-muted-foreground">
                        {registry.endpoint}
                        {registry.namespace ? `/${registry.namespace}` : ""}
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex flex-wrap gap-1">
                        {registry.allowPull ? <Pill>pull</Pill> : null}
                        {registry.allowPush ? <Pill>push</Pill> : null}
                        {registry.isDefaultPull ? <Pill>default pull</Pill> : null}
                        {registry.isDefaultPush ? <Pill>default push</Pill> : null}
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <div>{registry.credentialConfigured ? registry.credentialUsername || "configured" : "anonymous"}</div>
                      {registry.credentialFingerprint ? (
                        <div className="mt-1 text-xs text-muted-foreground">{registry.credentialFingerprint}</div>
                      ) : null}
                    </td>
                    <td className="px-4 py-3 text-muted-foreground">
                      <div>TLS {registry.tlsVerify ? "verify" : "skip verify"}</div>
                      <div className="mt-1 text-xs">{registry.insecureHttp ? "HTTP allowed" : "HTTPS"}</div>
                    </td>
                    <td className="px-4 py-3">
                      <StatusBadge status={registry.status} />
                      {registry.lastError ? <div className="mt-1 max-w-48 truncate text-xs text-red-700">{registry.lastError}</div> : null}
                    </td>
                    <td className="px-4 py-3 text-muted-foreground">{formatDate(registry.lastCheckedAt)}</td>
                    <td className="px-4 py-3">
                      <div className="flex justify-end gap-2">
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => {
                            setCheckingID(registry.id)
                            checkMutation.mutate(registry.id)
                          }}
                          disabled={checkingID === registry.id}
                        >
                          {checkingID === registry.id ? <Loader2 className="animate-spin" aria-hidden="true" /> : <RefreshCw aria-hidden="true" />}
                          Check
                        </Button>
                        <Button variant="ghost" size="icon" onClick={() => startEdit(registry)} aria-label="Edit registry">
                          <Edit3 aria-hidden="true" />
                        </Button>
                        <Button variant="ghost" size="icon" onClick={() => toggleMutation.mutate({ registry })} aria-label={registry.status === "disabled" ? "Enable registry" : "Disable registry"}>
                          <Power aria-hidden="true" />
                        </Button>
                        <Button variant="ghost" size="icon" onClick={() => deleteMutation.mutate(registry.id)} aria-label="Delete registry">
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

function Metric({ label, value, detail, icon: Icon }: { label: string; value: string; detail: string; icon: typeof Warehouse }) {
  return (
    <div className="rounded-lg border bg-card p-4 text-card-foreground">
      <div className="flex items-center justify-between">
        <p className="text-sm text-muted-foreground">{label}</p>
        <Icon className="size-4 text-muted-foreground" aria-hidden="true" />
      </div>
      <div className="mt-3 truncate text-2xl font-semibold">{value}</div>
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

function Toggle({ label, checked, onChange }: { label: string; checked: boolean; onChange: (checked: boolean) => void }) {
  return (
    <label className="flex h-10 items-center gap-2 rounded-md border bg-background px-3 text-sm">
      <input type="checkbox" checked={checked} onChange={(event) => onChange(event.target.checked)} />
      <span>{label}</span>
    </label>
  )
}

function Pill({ children }: { children: ReactNode }) {
  return <span className="rounded-md bg-secondary px-1.5 py-0.5 text-xs text-secondary-foreground">{children}</span>
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

function StatusBadge({ status }: { status: Registry["status"] }) {
  const style = {
    available: "border-emerald-200 bg-emerald-50 text-emerald-700",
    disabled: "border-slate-200 bg-slate-50 text-slate-700",
    unavailable: "border-red-200 bg-red-50 text-red-700",
    unknown: "border-zinc-200 bg-zinc-50 text-zinc-700",
  }[status]
  const Icon = status === "available" ? CheckCircle2 : status === "unknown" || status === "disabled" ? AlertTriangle : XCircle

  return (
    <span className={cn("inline-flex h-7 items-center gap-1.5 rounded-md border px-2 text-xs font-medium", style)}>
      <Icon className="size-3.5" aria-hidden="true" />
      {status}
    </span>
  )
}

function summarizeRegistries(registries: Registry[]) {
  return registries.reduce(
    (summary, registry) => ({
      total: summary.total + 1,
      available: summary.available + (registry.status === "available" ? 1 : 0),
      defaultPush: registry.isDefaultPush ? registry.name : summary.defaultPush,
    }),
    { total: 0, available: 0, defaultPush: "" },
  )
}

function toSaveInput(form: RegistryFormState): SaveRegistryInput {
  return {
    name: form.name,
    type: form.type,
    endpoint: form.endpoint,
    namespace: form.namespace,
    region: form.region,
    username: form.username,
    password: form.password,
    allowPull: form.allowPull,
    allowPush: form.allowPush,
    isDefaultPull: form.isDefaultPull,
    isDefaultPush: form.isDefaultPush,
    tlsVerify: form.tlsVerify,
    insecureHttp: form.insecureHttp,
  }
}

function updateForm(setForm: Dispatch<SetStateAction<RegistryFormState>>, key: keyof RegistryFormState, value: string) {
  setForm((current) => ({ ...current, [key]: value }))
}

function updateFormBool(setForm: Dispatch<SetStateAction<RegistryFormState>>, key: keyof RegistryFormState, value: boolean) {
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
