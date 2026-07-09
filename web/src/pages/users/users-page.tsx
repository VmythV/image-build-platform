import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { KeyRound, Loader2, Save, UserPlus, Users } from "lucide-react"
import { type FormEvent, useEffect, useMemo, useState } from "react"

import { Button } from "@/components/ui/button"
import { type User } from "@/lib/auth-api"
import { createUser, listUsers, resetUserPassword, updateUser } from "@/lib/users-api"

type UserDraft = {
  displayName: string
  role: User["role"]
  status: "active" | "disabled"
}

const defaultCreateForm = {
  username: "",
  displayName: "",
  password: "",
  role: "viewer" as User["role"],
}

export function UsersPage({ currentUser }: { currentUser: User }) {
  const queryClient = useQueryClient()
  const [createForm, setCreateForm] = useState(defaultCreateForm)
  const [drafts, setDrafts] = useState<Record<string, UserDraft>>({})
  const [passwordDrafts, setPasswordDrafts] = useState<Record<string, string>>({})
  const [message, setMessage] = useState("")

  const usersQuery = useQuery({
    queryKey: ["users"],
    queryFn: listUsers,
  })

  useEffect(() => {
    const users = usersQuery.data ?? []
    setDrafts(
      Object.fromEntries(
        users.map((user) => [
          user.id,
          {
            displayName: user.displayName,
            role: user.role,
            status: (user.status === "disabled" ? "disabled" : "active") as "active" | "disabled",
          },
        ]),
      ),
    )
  }, [usersQuery.data])

  const stats = useMemo(() => summarizeUsers(usersQuery.data ?? []), [usersQuery.data])

  const createMutation = useMutation({
    mutationFn: createUser,
    onSuccess: (user) => {
      setCreateForm(defaultCreateForm)
      setMessage(`Created ${user.username}`)
      return queryClient.invalidateQueries({ queryKey: ["users"] })
    },
    onError: (error) => setMessage(error instanceof Error ? error.message : "Failed to create user."),
  })

  const updateMutation = useMutation({
    mutationFn: ({ id, draft }: { id: string; draft: UserDraft }) => updateUser(id, draft),
    onSuccess: (user) => {
      setMessage(`Saved ${user.username}`)
      return queryClient.invalidateQueries({ queryKey: ["users"] })
    },
    onError: (error) => setMessage(error instanceof Error ? error.message : "Failed to update user."),
  })

  const resetMutation = useMutation({
    mutationFn: ({ id, password }: { id: string; password: string }) => resetUserPassword(id, password),
    onSuccess: (user) => {
      setPasswordDrafts((current) => ({ ...current, [user.id]: "" }))
      setMessage(`Password reset for ${user.username}`)
    },
    onError: (error) => setMessage(error instanceof Error ? error.message : "Failed to reset password."),
  })

  function handleCreate(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    createMutation.mutate(createForm)
  }

  function setDraft(id: string, patch: Partial<UserDraft>) {
    setDrafts((current) => ({ ...current, [id]: { ...current[id], ...patch } }))
  }

  if (currentUser.role !== "admin") {
    return (
      <section className="rounded-lg border bg-card p-6 text-card-foreground">
        <h2 className="text-base font-semibold">Users</h2>
        <p className="mt-1 text-sm text-muted-foreground">Only administrators can manage users.</p>
      </section>
    )
  }

  return (
    <div className="space-y-4">
      <section className="grid gap-3 md:grid-cols-3">
        <Metric title="Users" value={stats.total} />
        <Metric title="Admins" value={stats.admins} />
        <Metric title="Disabled" value={stats.disabled} />
      </section>

      <section className="rounded-lg border bg-card text-card-foreground">
        <div className="flex flex-col gap-2 border-b p-4 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h2 className="flex items-center gap-2 text-base font-semibold">
              <Users className="size-4" aria-hidden="true" />
              User Management
            </h2>
            <p className="mt-1 text-sm text-muted-foreground">Create operators and manage console access.</p>
          </div>
          {message ? <div className="text-sm text-muted-foreground">{message}</div> : null}
        </div>

        <form className="grid gap-3 border-b p-4 lg:grid-cols-[1fr_1fr_1fr_180px_auto]" onSubmit={handleCreate}>
          <input className={inputClassName} placeholder="Username" value={createForm.username} onChange={(event) => setCreateForm((current) => ({ ...current, username: event.target.value }))} />
          <input className={inputClassName} placeholder="Display name" value={createForm.displayName} onChange={(event) => setCreateForm((current) => ({ ...current, displayName: event.target.value }))} />
          <input className={inputClassName} type="password" placeholder="Initial password" value={createForm.password} onChange={(event) => setCreateForm((current) => ({ ...current, password: event.target.value }))} />
          <select className={inputClassName} value={createForm.role} onChange={(event) => setCreateForm((current) => ({ ...current, role: event.target.value as User["role"] }))}>
            <option value="viewer">Viewer</option>
            <option value="maintainer">Maintainer</option>
            <option value="admin">Admin</option>
          </select>
          <Button type="submit" disabled={createMutation.isPending || !createForm.username || !createForm.password}>
            {createMutation.isPending ? <Loader2 className="animate-spin" aria-hidden="true" /> : <UserPlus aria-hidden="true" />}
            Create
          </Button>
        </form>

        {usersQuery.isPending ? (
          <StateBlock title="Loading users" detail="Fetching user accounts." />
        ) : usersQuery.isError ? (
          <StateBlock title="Failed to load users" detail="Please retry after checking the backend service." />
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full min-w-[900px] border-collapse text-left text-sm">
              <thead className="border-b bg-muted/60 text-xs uppercase text-muted-foreground">
                <tr>
                  <th className="px-4 py-3 font-medium">User</th>
                  <th className="px-4 py-3 font-medium">Role</th>
                  <th className="px-4 py-3 font-medium">Status</th>
                  <th className="px-4 py-3 font-medium">Last Login</th>
                  <th className="px-4 py-3 font-medium">Password</th>
                  <th className="px-4 py-3 text-right font-medium">Actions</th>
                </tr>
              </thead>
              <tbody>
                {(usersQuery.data ?? []).map((user) => {
                  const draft = drafts[user.id] ?? { displayName: user.displayName, role: user.role, status: user.status === "disabled" ? "disabled" : "active" }
                  const disableSelf = user.id === currentUser.id
                  return (
                    <tr key={user.id} className="border-b last:border-b-0">
                      <td className="px-4 py-3">
                        <div className="font-medium">{user.username}</div>
                        <input className="mt-2 h-9 w-full rounded-md border bg-background px-2 text-sm outline-none focus:border-ring focus:ring-2 focus:ring-ring/20" value={draft.displayName} onChange={(event) => setDraft(user.id, { displayName: event.target.value })} />
                      </td>
                      <td className="px-4 py-3">
                        <select className={inputClassName} value={draft.role} onChange={(event) => setDraft(user.id, { role: event.target.value as User["role"] })} disabled={disableSelf}>
                          <option value="viewer">Viewer</option>
                          <option value="maintainer">Maintainer</option>
                          <option value="admin">Admin</option>
                        </select>
                      </td>
                      <td className="px-4 py-3">
                        <select className={inputClassName} value={draft.status} onChange={(event) => setDraft(user.id, { status: event.target.value as "active" | "disabled" })} disabled={disableSelf}>
                          <option value="active">Active</option>
                          <option value="disabled">Disabled</option>
                        </select>
                      </td>
                      <td className="px-4 py-3 text-muted-foreground">{formatDate(user.lastLoginAt)}</td>
                      <td className="px-4 py-3">
                        <input
                          className={inputClassName}
                          type="password"
                          placeholder="New password"
                          value={passwordDrafts[user.id] ?? ""}
                          onChange={(event) => setPasswordDrafts((current) => ({ ...current, [user.id]: event.target.value }))}
                        />
                      </td>
                      <td className="px-4 py-3">
                        <div className="flex justify-end gap-2">
                          <Button variant="outline" size="sm" onClick={() => updateMutation.mutate({ id: user.id, draft })} disabled={updateMutation.isPending}>
                            {updateMutation.isPending ? <Loader2 className="animate-spin" aria-hidden="true" /> : <Save aria-hidden="true" />}
                            Save
                          </Button>
                          <Button variant="outline" size="sm" onClick={() => resetMutation.mutate({ id: user.id, password: passwordDrafts[user.id] ?? "" })} disabled={resetMutation.isPending || !(passwordDrafts[user.id] ?? "")}>
                            {resetMutation.isPending ? <Loader2 className="animate-spin" aria-hidden="true" /> : <KeyRound aria-hidden="true" />}
                            Reset
                          </Button>
                        </div>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </div>
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

function StateBlock({ title, detail }: { title: string; detail: string }) {
  return (
    <div className="p-8 text-center">
      <h3 className="text-base font-semibold">{title}</h3>
      <p className="mt-1 text-sm text-muted-foreground">{detail}</p>
    </div>
  )
}

function summarizeUsers(users: User[]) {
  return {
    total: users.length,
    admins: users.filter((user) => user.role === "admin").length,
    disabled: users.filter((user) => user.status === "disabled").length,
  }
}

function formatDate(value?: string | null) {
  if (!value) {
    return "-"
  }
  return new Date(value).toLocaleString()
}

const inputClassName = "h-10 w-full rounded-md border bg-background px-3 text-sm outline-none transition-colors focus:border-ring focus:ring-2 focus:ring-ring/20"
