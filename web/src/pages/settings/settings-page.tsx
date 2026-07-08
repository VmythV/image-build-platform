import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Activity, Loader2, Save, Settings as SettingsIcon, ShieldCheck } from "lucide-react"
import { type FormEvent, useEffect, useMemo, useState } from "react"

import { Button } from "@/components/ui/button"
import { listAuditLogs, type AuditLog } from "@/lib/audit-api"
import { listSettings, updateSetting, type SystemSetting } from "@/lib/settings-api"

export function SettingsPage() {
  const queryClient = useQueryClient()
  const settingsQuery = useQuery({
    queryKey: ["settings"],
    queryFn: listSettings,
  })
  const auditQuery = useQuery({
    queryKey: ["audit-logs"],
    queryFn: listAuditLogs,
    refetchInterval: 5000,
  })
  const [drafts, setDrafts] = useState<Record<string, string>>({})
  const [message, setMessage] = useState("")

  useEffect(() => {
    if (settingsQuery.data) {
      setDrafts(Object.fromEntries(settingsQuery.data.map((setting) => [setting.key, setting.value])))
    }
  }, [settingsQuery.data])

  const updateMutation = useMutation({
    mutationFn: ({ key, value }: { key: string; value: string }) => updateSetting(key, value),
    onSuccess: () => {
      setMessage("Settings saved.")
      void queryClient.invalidateQueries({ queryKey: ["settings"] })
      void queryClient.invalidateQueries({ queryKey: ["audit-logs"] })
    },
    onError: (error) => {
      setMessage(error instanceof Error ? error.message : "Failed to save setting.")
    },
  })

  const dirtySettings = useMemo(
    () => (settingsQuery.data ?? []).filter((setting) => drafts[setting.key] !== undefined && drafts[setting.key] !== setting.value),
    [drafts, settingsQuery.data],
  )

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setMessage("")
    for (const setting of dirtySettings) {
      updateMutation.mutate({ key: setting.key, value: drafts[setting.key] })
    }
  }

  return (
    <div className="space-y-4">
      <section className="rounded-lg border bg-card text-card-foreground">
        <div className="flex items-center gap-2 border-b px-4 py-3">
          <SettingsIcon className="size-4 text-muted-foreground" aria-hidden="true" />
          <div>
            <h2 className="text-base font-semibold">System Settings</h2>
            <p className="mt-1 text-sm text-muted-foreground">Admin-managed defaults used by platform operations.</p>
          </div>
        </div>

        {settingsQuery.isPending ? (
          <StateBlock title="Loading settings" detail="Fetching system settings." />
        ) : settingsQuery.isError ? (
          <StateBlock title="Failed to load settings" detail="Please retry after checking the backend service." />
        ) : (
          <form onSubmit={handleSubmit}>
            <div className="divide-y">
              {settingsQuery.data.map((setting) => (
                <SettingRow
                  key={setting.key}
                  setting={setting}
                  value={drafts[setting.key] ?? setting.value}
                  onChange={(value) => setDrafts((current) => ({ ...current, [setting.key]: value }))}
                />
              ))}
            </div>
            <div className="flex flex-wrap items-center justify-between gap-3 border-t px-4 py-3">
              <div className="text-sm text-muted-foreground">{message || `${dirtySettings.length} unsaved change${dirtySettings.length === 1 ? "" : "s"}.`}</div>
              <Button type="submit" size="sm" disabled={dirtySettings.length === 0 || updateMutation.isPending}>
                {updateMutation.isPending ? <Loader2 className="animate-spin" aria-hidden="true" /> : <Save aria-hidden="true" />}
                Save Changes
              </Button>
            </div>
          </form>
        )}
      </section>

      <section className="rounded-lg border bg-card text-card-foreground">
        <div className="flex items-center gap-2 border-b px-4 py-3">
          <Activity className="size-4 text-muted-foreground" aria-hidden="true" />
          <div>
            <h2 className="text-base font-semibold">Recent Audit Logs</h2>
            <p className="mt-1 text-sm text-muted-foreground">Recent authenticated write operations.</p>
          </div>
        </div>
        {auditQuery.isPending ? (
          <StateBlock title="Loading audit logs" detail="Fetching recent events." />
        ) : auditQuery.isError ? (
          <StateBlock title="Failed to load audit logs" detail="Only admins can view audit logs." />
        ) : auditQuery.data.length === 0 ? (
          <StateBlock title="No audit logs" detail="Write operations will appear here." />
        ) : (
          <div className="divide-y">
            {auditQuery.data.slice(0, 12).map((log) => (
              <AuditRow key={log.id} log={log} />
            ))}
          </div>
        )}
      </section>
    </div>
  )
}

function SettingRow({ setting, value, onChange }: { setting: SystemSetting; value: string; onChange: (value: string) => void }) {
  return (
    <div className="grid gap-3 px-4 py-3 lg:grid-cols-[minmax(0,1fr)_260px] lg:items-center">
      <div className="min-w-0">
        <div className="break-all text-sm font-medium">{setting.key}</div>
        <div className="mt-1 text-sm text-muted-foreground">{setting.description}</div>
        <div className="mt-1 text-xs text-muted-foreground">Updated {formatDate(setting.updatedAt)}</div>
      </div>
      {setting.valueType === "boolean" ? (
        <label className="inline-flex items-center gap-2 text-sm">
          <input type="checkbox" checked={value === "true"} onChange={(event) => onChange(String(event.target.checked))} />
          Enabled
        </label>
      ) : (
        <input
          className="h-10 w-full rounded-md border bg-background px-3 text-sm outline-none transition-colors focus:border-ring focus:ring-2 focus:ring-ring/20"
          min={0}
          type={setting.valueType === "integer" ? "number" : "text"}
          value={value}
          onChange={(event) => onChange(event.target.value)}
        />
      )}
    </div>
  )
}

function AuditRow({ log }: { log: AuditLog }) {
  return (
    <div className="grid gap-2 px-4 py-3 text-sm lg:grid-cols-[220px_minmax(0,1fr)_180px] lg:items-center">
      <div className="flex items-center gap-2">
        <ShieldCheck className="size-4 text-muted-foreground" aria-hidden="true" />
        <div>
          <div className="font-medium">{log.action}</div>
          <div className="text-xs text-muted-foreground">{log.actorName || "system"}</div>
        </div>
      </div>
      <div className="min-w-0">
        <div className="truncate">{log.resourceType}</div>
        {log.resourceId ? <div className="mt-1 truncate font-mono text-xs text-muted-foreground">{log.resourceId}</div> : null}
      </div>
      <div className="text-xs text-muted-foreground lg:text-right">{formatDate(log.createdAt)}</div>
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

function formatDate(value?: string | null) {
  if (!value) {
    return "-"
  }
  return new Date(value).toLocaleString()
}
