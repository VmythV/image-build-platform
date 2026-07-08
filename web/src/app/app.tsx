import { AppShell } from "@/components/layout/app-shell"
import { Button } from "@/components/ui/button"
import { getCurrentUser, getSetupStatus, logout, type User } from "@/lib/auth-api"
import { LoginPage } from "@/pages/auth/login-page"
import { SetupPage } from "@/pages/auth/setup-page"
import { BuildHostsPage } from "@/pages/build-hosts/build-hosts-page"
import { DashboardPage } from "@/pages/dashboard/dashboard-page"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { type ReactNode, useState } from "react"

export type AppView = "dashboard" | "build-hosts"

export function App() {
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
    return <StatusScreen title="正在连接平台" detail="正在检查初始化状态。" />
  }

  if (setupStatusQuery.isError) {
    return (
      <StatusScreen
        title="无法连接后端服务"
        detail="请确认服务已经启动，并且前端 API 地址配置正确。"
        action={
          <Button variant="outline" size="sm" onClick={() => void setupStatusQuery.refetch()}>
            重试
          </Button>
        }
      />
    )
  }

  if (!setupStatusQuery.data.initialized) {
    return <SetupPage onCompleted={handleAuthenticated} />
  }

  if (currentUserQuery.isPending) {
    return <StatusScreen title="正在加载会话" detail="正在读取当前登录用户。" />
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
      {activeView === "dashboard" ? <DashboardPage /> : <BuildHostsPage />}
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
