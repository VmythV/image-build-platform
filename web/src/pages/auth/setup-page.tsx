import { FormEvent, useState } from "react"
import { Boxes, KeyRound, Loader2 } from "lucide-react"

import { Button } from "@/components/ui/button"
import { ApiRequestError } from "@/lib/api"
import { login, setupAdmin, type User } from "@/lib/auth-api"

type SetupPageProps = {
  onCompleted: (user: User) => void
}

export function SetupPage({ onCompleted }: SetupPageProps) {
  const [username, setUsername] = useState("admin")
  const [displayName, setDisplayName] = useState("Administrator")
  const [password, setPassword] = useState("")
  const [confirmPassword, setConfirmPassword] = useState("")
  const [errorMessage, setErrorMessage] = useState("")
  const [isSubmitting, setIsSubmitting] = useState(false)

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setErrorMessage("")

    if (password !== confirmPassword) {
      setErrorMessage("两次输入的密码不一致。")
      return
    }

    setIsSubmitting(true)
    try {
      await setupAdmin({ username, password, displayName })
      const user = await login({ username, password })
      onCompleted(user)
    } catch (error) {
      if (error instanceof ApiRequestError && error.status === 409) {
        setErrorMessage("平台已经完成初始化，请直接登录。")
      } else {
        setErrorMessage(error instanceof Error ? error.message : "初始化管理员失败，请稍后重试。")
      }
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <main className="flex min-h-screen items-center justify-center bg-background px-4 py-8 text-foreground">
      <section className="w-full max-w-[460px] rounded-lg border bg-card p-6 text-card-foreground shadow-sm">
        <div className="mb-6 flex items-center gap-3">
          <div className="flex size-10 items-center justify-center rounded-md bg-primary text-primary-foreground">
            <Boxes className="size-5" aria-hidden="true" />
          </div>
          <div>
            <h1 className="text-lg font-semibold">初始化平台</h1>
            <p className="text-sm text-muted-foreground">创建第一个管理员账号后即可进入管理后台。</p>
          </div>
        </div>

        <form className="space-y-4" onSubmit={handleSubmit}>
          <label className="block space-y-2 text-sm font-medium">
            <span>用户名</span>
            <input
              className="h-10 w-full rounded-md border bg-background px-3 text-sm outline-none transition-colors focus:border-ring focus:ring-2 focus:ring-ring/20"
              autoComplete="username"
              value={username}
              onChange={(event) => setUsername(event.target.value)}
              required
            />
          </label>

          <label className="block space-y-2 text-sm font-medium">
            <span>显示名称</span>
            <input
              className="h-10 w-full rounded-md border bg-background px-3 text-sm outline-none transition-colors focus:border-ring focus:ring-2 focus:ring-ring/20"
              autoComplete="name"
              value={displayName}
              onChange={(event) => setDisplayName(event.target.value)}
              required
            />
          </label>

          <label className="block space-y-2 text-sm font-medium">
            <span>密码</span>
            <input
              className="h-10 w-full rounded-md border bg-background px-3 text-sm outline-none transition-colors focus:border-ring focus:ring-2 focus:ring-ring/20"
              autoComplete="new-password"
              minLength={10}
              type="password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              required
            />
          </label>

          <label className="block space-y-2 text-sm font-medium">
            <span>确认密码</span>
            <input
              className="h-10 w-full rounded-md border bg-background px-3 text-sm outline-none transition-colors focus:border-ring focus:ring-2 focus:ring-ring/20"
              autoComplete="new-password"
              minLength={10}
              type="password"
              value={confirmPassword}
              onChange={(event) => setConfirmPassword(event.target.value)}
              required
            />
          </label>

          {errorMessage ? (
            <div className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
              {errorMessage}
            </div>
          ) : null}

          <Button className="w-full" type="submit" disabled={isSubmitting}>
            {isSubmitting ? <Loader2 className="animate-spin" aria-hidden="true" /> : <KeyRound aria-hidden="true" />}
            创建管理员
          </Button>
        </form>
      </section>
    </main>
  )
}
