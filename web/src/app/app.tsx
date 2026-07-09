import { AppShell } from "@/components/layout/app-shell"
import { Button } from "@/components/ui/button"
import { getCurrentUser, getSetupStatus, logout, type User } from "@/lib/auth-api"
import { ArtifactsPage } from "@/pages/artifacts/artifacts-page"
import { LoginPage } from "@/pages/auth/login-page"
import { SetupPage } from "@/pages/auth/setup-page"
import { BuildHostsPage } from "@/pages/build-hosts/build-hosts-page"
import { BuildTasksPage } from "@/pages/build-tasks/build-tasks-page"
import { DashboardPage } from "@/pages/dashboard/dashboard-page"
import { HelpPage } from "@/pages/help/help-page"
import { ImageProjectsPage } from "@/pages/image-projects/image-projects-page"
import { RegistriesPage } from "@/pages/registries/registries-page"
import { SettingsPage } from "@/pages/settings/settings-page"
import { UsersPage } from "@/pages/users/users-page"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { type ReactNode, useState } from "react"

import { useI18n } from "@/lib/i18n"

export type AppView = "dashboard" | "build-hosts" | "registries" | "image-projects" | "build-tasks" | "artifacts" | "users" | "settings" | "help"

export function App() {
  const { t } = useI18n()
  const queryClient = useQueryClient()
  const [activeView, setActiveView] = useState<AppView>("dashboard")
  const setupStatusQuery = useQuery({
    queryKey: ["setup-status"],
    queryFn: getSetupStatus,
  })
  const currentUserQuery = useQuery({
    queryKey: ["auth", "me"],
    queryFn: getCurrentUser,
    enabled: setupStatusQuery.data?.initialized === true,
    retry: false,
  })
  const logoutMutation = useMutation({
    mutationFn: logout,
    onSettled: () => {
      queryClient.removeQueries({ queryKey: ["auth", "me"] })
    },
  })

  function handleAuthenticated(user: User) {
    queryClient.setQueryData(["setup-status"], { initialized: true })
    queryClient.setQueryData(["auth", "me"], user)
  }

  if (setupStatusQuery.isPending) {
    return <StatusScreen title={t("app.connecting.title")} detail={t("app.connecting.detail")} />
  }

  if (setupStatusQuery.isError) {
    return (
      <StatusScreen
        title={t("app.backend_error.title")}
        detail={t("app.backend_error.detail")}
        action={
          <Button variant="outline" size="sm" onClick={() => void setupStatusQuery.refetch()}>
            {t("app.retry")}
          </Button>
        }
      />
    )
  }

  if (!setupStatusQuery.data.initialized) {
    return <SetupPage onCompleted={handleAuthenticated} />
  }

  if (currentUserQuery.isPending) {
    return <StatusScreen title={t("app.session.title")} detail={t("app.session.detail")} />
  }

  if (currentUserQuery.isError) {
    return <LoginPage onLoggedIn={handleAuthenticated} />
  }

  return (
    <AppShell
      activeView={activeView}
      user={currentUserQuery.data}
      logoutPending={logoutMutation.isPending}
      onNavigate={setActiveView}
      onLogout={() => logoutMutation.mutate()}
    >
      {activeView === "dashboard" ? <DashboardPage /> : null}
      {activeView === "build-hosts" ? <BuildHostsPage /> : null}
      {activeView === "registries" ? <RegistriesPage /> : null}
      {activeView === "image-projects" ? <ImageProjectsPage /> : null}
      {activeView === "build-tasks" ? <BuildTasksPage /> : null}
      {activeView === "artifacts" ? <ArtifactsPage /> : null}
      {activeView === "users" ? <UsersPage currentUser={currentUserQuery.data} /> : null}
      {activeView === "settings" ? <SettingsPage /> : null}
      {activeView === "help" ? <HelpPage /> : null}
    </AppShell>
  )
}

type StatusScreenProps = {
  title: string
  detail: string
  action?: ReactNode
}

function StatusScreen({ title, detail, action }: StatusScreenProps) {
  return (
    <main className="flex min-h-screen items-center justify-center bg-background px-4 text-foreground">
      <section className="w-full max-w-[420px] rounded-lg border bg-card p-6 text-card-foreground shadow-sm">
        <h1 className="text-lg font-semibold">{title}</h1>
        <p className="mt-2 text-sm text-muted-foreground">{detail}</p>
        {action ? <div className="mt-4">{action}</div> : null}
      </section>
    </main>
  )
}
