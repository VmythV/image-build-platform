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
  type LucideIcon,
  UserCircle,
  UserCog,
  Warehouse,
} from "lucide-react"
import { type PropsWithChildren } from "react"

import { type AppView } from "@/app/app"
import { Button } from "@/components/ui/button"
import { type User } from "@/lib/auth-api"
import { useI18n } from "@/lib/i18n"
import { cn } from "@/lib/utils"

const navigation: Array<{ id: AppView; labelKey: string; icon: LucideIcon; adminOnly?: boolean; disabled?: boolean }> = [
  { id: "dashboard", labelKey: "nav.dashboard", icon: Home },
  { id: "build-hosts", labelKey: "nav.build-hosts", icon: HardDrive },
  { id: "registries", labelKey: "nav.registries", icon: Warehouse },
  { id: "image-projects", labelKey: "nav.image-projects", icon: GitBranch },
  { id: "build-tasks", labelKey: "nav.build-tasks", icon: ListChecks },
  { id: "artifacts", labelKey: "nav.artifacts", icon: Image },
  { id: "users", labelKey: "nav.users", icon: UserCog, adminOnly: true },
  { id: "settings", labelKey: "nav.settings", icon: Settings },
  { id: "help", labelKey: "nav.help", icon: HelpCircle },
]

type AppShellProps = PropsWithChildren<{
  activeView: AppView
  user: User
  logoutPending?: boolean
  onNavigate: (view: AppView) => void
  onLogout: () => void
}>

const viewTitles: Record<AppView, { titleKey: string; subtitleKey: string }> = {
  dashboard: { titleKey: "view.dashboard.title", subtitleKey: "view.dashboard.subtitle" },
  "build-hosts": { titleKey: "view.build-hosts.title", subtitleKey: "view.build-hosts.subtitle" },
  registries: { titleKey: "view.registries.title", subtitleKey: "view.registries.subtitle" },
  "image-projects": { titleKey: "view.image-projects.title", subtitleKey: "view.image-projects.subtitle" },
  "build-tasks": { titleKey: "view.build-tasks.title", subtitleKey: "view.build-tasks.subtitle" },
  artifacts: { titleKey: "view.artifacts.title", subtitleKey: "view.artifacts.subtitle" },
  users: { titleKey: "view.users.title", subtitleKey: "view.users.subtitle" },
  settings: { titleKey: "view.settings.title", subtitleKey: "view.settings.subtitle" },
  help: { titleKey: "view.help.title", subtitleKey: "view.help.subtitle" },
}

export function AppShell({ children, activeView, user, logoutPending = false, onNavigate, onLogout }: AppShellProps) {
  const { language, setLanguage, t } = useI18n()
  const title = viewTitles[activeView]
  const visibleNavigation = navigation.filter((item) => !item.adminOnly || user.role === "admin")

  return (
    <div className="min-h-screen bg-background text-foreground">
      <aside className="fixed inset-y-0 left-0 hidden w-64 border-r bg-sidebar px-3 py-4 lg:block">
        <div className="mb-6 flex items-center gap-2 px-2">
          <div className="flex size-9 items-center justify-center rounded-md bg-primary text-primary-foreground">
            <Boxes className="size-5" aria-hidden="true" />
          </div>
          <div>
            <div className="text-sm font-semibold">{t("shell.product")}</div>
            <div className="text-xs text-muted-foreground">{t("shell.console")}</div>
          </div>
        </div>

        <nav className="space-y-1">
          {visibleNavigation.map((item) => (
            <button
              key={item.id}
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
              <span>{t(item.labelKey)}</span>
            </button>
          ))}
        </nav>
      </aside>

      <div className="lg:pl-64">
        <header className="sticky top-0 z-10 flex h-14 items-center justify-between border-b bg-background/95 px-4 backdrop-blur lg:px-6">
          <div>
            <h1 className="text-base font-semibold">{t(title.titleKey)}</h1>
            <p className="text-xs text-muted-foreground">{t(title.subtitleKey)}</p>
          </div>
          <div className="flex items-center gap-2">
            <select className="h-9 rounded-md border bg-background px-2 text-sm outline-none focus:border-ring focus:ring-2 focus:ring-ring/20" aria-label={t("shell.language")} value={language} onChange={(event) => setLanguage(event.target.value as "en" | "zh-CN")}>
              <option value="zh-CN">中文</option>
              <option value="en">EN</option>
            </select>
            <div className="ml-2 hidden items-center gap-2 border-l pl-3 sm:flex">
              <UserCircle className="size-4 text-muted-foreground" aria-hidden="true" />
              <span className="max-w-40 truncate text-sm">{user.displayName || user.username}</span>
            </div>
            <Button variant="ghost" size="icon" onClick={onLogout} disabled={logoutPending} aria-label={t("shell.logout")}>
              <LogOut aria-hidden="true" />
            </Button>
          </div>
        </header>

        <main className="p-4 lg:p-6">{children}</main>
      </div>
    </div>
  )
}
