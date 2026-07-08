import { FormEvent, useState } from "react"
import { Boxes, Loader2, LogIn } from "lucide-react"

import { Button } from "@/components/ui/button"
import { ApiRequestError } from "@/lib/api"
import { login, type User } from "@/lib/auth-api"

type LoginPageProps = {
  onLoggedIn: (user: User) => void
}

export function LoginPage({ onLoggedIn }: LoginPageProps) {
  const [username, setUsername] = useState("")
  const [password, setPassword] = useState("")
  const [errorMessage, setErrorMessage] = useState("")
  const [isSubmitting, setIsSubmitting] = useState(false)

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setErrorMessage("")
    setIsSubmitting(true)

    try {
      const user = await login({ username, password })
      onLoggedIn(user)
    } catch (error) {
      if (error instanceof ApiRequestError && error.status === 401) {
        setErrorMessage("用户名或密码不正确。")
      } else {
        setErrorMessage(error instanceof Error ? error.message : "登录失败，请稍后重试。")
      }
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <main className="flex min-h-screen items-center justify-center bg-background px-4 py-8 text-foreground">
      <section className="w-full max-w-[420px] rounded-lg border bg-card p-6 text-card-foreground shadow-sm">
        <div className="mb-6 flex items-center gap-3">
          <div className="flex size-10 items-center justify-center rounded-md bg-primary text-primary-foreground">
            <Boxes className="size-5" aria-hidden="true" />
          </div>
          <div>
            <h1 className="text-lg font-semibold">登录 Image Build</h1>
            <p className="text-sm text-muted-foreground">使用管理员账号进入镜像构建平台。</p>
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
            <span>密码</span>
            <input
              className="h-10 w-full rounded-md border bg-background px-3 text-sm outline-none transition-colors focus:border-ring focus:ring-2 focus:ring-ring/20"
              autoComplete="current-password"
              type="password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              required
            />
          </label>

          {errorMessage ? (
            <div className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
              {errorMessage}
            </div>
          ) : null}

          <Button className="w-full" type="submit" disabled={isSubmitting}>
            {isSubmitting ? <Loader2 className="animate-spin" aria-hidden="true" /> : <LogIn aria-hidden="true" />}
            登录
          </Button>
        </form>
      </section>
    </main>
  )
}
