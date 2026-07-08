import {
  Boxes,
  GitBranch,
  HardDrive,
  HelpCircle,
  Home,
  Image,
  ListChecks,
  LogOut,
  Settings,
  UserCircle,
  Warehouse,
} from "lucide-react"
import { type PropsWithChildren } from "react"

import { type AppView } from "@/app/app"
import { Button } from "@/components/ui/button"
import { type User } from "@/lib/auth-api"
import { cn } from "@/lib/utils"

const navigation = [
  { id: "dashboard", label: "Dashboard", icon: Home },
  { id: "build-hosts", label: "Build Hosts", icon: HardDrive },
  { id: "registries", label: "Registries", icon: Warehouse },
  { id: "image-projects", label: "Image Projects", icon: GitBranch },
  { id: "build-tasks", label: "Build Tasks", icon: ListChecks, disabled: true },
  { id: "artifacts", label: "Artifacts", icon: Image, disabled: true },
  { id: "settings", label: "Settings", icon: Settings, disabled: true },
  { id: "help", label: "Help", icon: HelpCircle, disabled: true },
]

type AppShellProps = PropsWithChildren<{
  activeView: AppView
  user: User
  logoutPending?: boolean
  onNavigate: (view: AppView) => void
  onLogout: () => void
}>

const viewTitles: Record<AppView, { title: string; subtitle: string }> = {
  dashboard: { title: "Dashboard", subtitle: "Platform overview" },
  "build-hosts": { title: "Build Hosts", subtitle: "Local and SSH builder capacity" },
  registries: { title: "Registries", subtitle: "Pull and push destinations" },
  "image-projects": { title: "Image Projects", subtitle: "Root images, versions, and branches" },
}

export function AppShell({ children, activeView, user, logoutPending = false, onNavigate, onLogout }: AppShellProps) {
  const title = viewTitles[activeView]

  return (
    <div className="min-h-screen bg-background text-foreground">
      <aside className="fixed inset-y-0 left-0 hidden w-64 border-r bg-sidebar px-3 py-4 lg:block">
        <div className="mb-6 flex items-center gap-2 px-2">
          <div className="flex size-9 items-center justify-center rounded-md bg-primary text-primary-foreground">
            <Boxes className="size-5" aria-hidden="true" />
          </div>
          <div>
            <div className="text-sm font-semibold">Image Build</div>
            <div className="text-xs text-muted-foreground">Platform Console</div>
          </div>
        </div>

        <nav className="space-y-1">
          {navigation.map((item) => (
            <button
              key={item.label}
              type="button"
              disabled={item.disabled}
              onClick={() => {
                if (!item.disabled) {
                  onNavigate(item.id as AppView)
                }
              }}
              className={cn(
                "flex h-9 w-full items-center gap-2 rounded-md px-2 text-sm text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground",
                item.id === activeView && "bg-accent text-accent-foreground",
                item.disabled && "cursor-not-allowed opacity-55 hover:bg-transparent hover:text-muted-foreground",
              )}
            >
              <item.icon className="size-4" aria-hidden="true" />
              <span>{item.label}</span>
            </button>
          ))}
        </nav>
      </aside>

      <div className="lg:pl-64">
        <header className="sticky top-0 z-10 flex h-14 items-center justify-between border-b bg-background/95 px-4 backdrop-blur lg:px-6">
          <div>
            <h1 className="text-base font-semibold">{title.title}</h1>
            <p className="text-xs text-muted-foreground">{title.subtitle}</p>
          </div>
          <div className="flex items-center gap-2">
            <div className="ml-2 hidden items-center gap-2 border-l pl-3 sm:flex">
              <UserCircle className="size-4 text-muted-foreground" aria-hidden="true" />
              <span className="max-w-40 truncate text-sm">{user.displayName || user.username}</span>
            </div>
            <Button variant="ghost" size="icon" onClick={onLogout} disabled={logoutPending} aria-label="Logout">
              <LogOut aria-hidden="true" />
            </Button>
          </div>
        </header>

        <main className="p-4 lg:p-6">{children}</main>
      </div>
    </div>
  )
}
