import {
  Boxes,
  GitBranch,
  HardDrive,
  HelpCircle,
  Home,
  Image,
  ListChecks,
  Settings,
  Warehouse,
} from "lucide-react"
import { type PropsWithChildren } from "react"

import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

const navigation = [
  { label: "Dashboard", icon: Home, active: true },
  { label: "Build Hosts", icon: HardDrive },
  { label: "Registries", icon: Warehouse },
  { label: "Image Projects", icon: GitBranch },
  { label: "Build Tasks", icon: ListChecks },
  { label: "Artifacts", icon: Image },
  { label: "Settings", icon: Settings },
  { label: "Help", icon: HelpCircle },
]

export function AppShell({ children }: PropsWithChildren) {
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
              className={cn(
                "flex h-9 w-full items-center gap-2 rounded-md px-2 text-sm text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground",
                item.active && "bg-accent text-accent-foreground",
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
            <h1 className="text-base font-semibold">Dashboard</h1>
            <p className="text-xs text-muted-foreground">M1 project scaffold</p>
          </div>
          <div className="flex items-center gap-2">
            <Button variant="outline" size="sm">
              Add Host
            </Button>
            <Button size="sm">Create Project</Button>
          </div>
        </header>

        <main className="p-4 lg:p-6">{children}</main>
      </div>
    </div>
  )
}
