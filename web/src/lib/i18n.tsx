import { createContext, type PropsWithChildren, useContext, useMemo, useState } from "react"

type Language = "en" | "zh-CN"

type Dictionary = Record<string, string>

const dictionaries: Record<Language, Dictionary> = {
  en: {
    "app.connecting.title": "Connecting to platform",
    "app.connecting.detail": "Checking initialization status.",
    "app.backend_error.title": "Cannot connect to backend",
    "app.backend_error.detail": "Confirm the service is running and the frontend API URL is configured.",
    "app.retry": "Retry",
    "app.session.title": "Loading session",
    "app.session.detail": "Reading the current signed-in user.",
    "nav.dashboard": "Dashboard",
    "nav.build-hosts": "Build Hosts",
    "nav.registries": "Registries",
    "nav.image-projects": "Image Projects",
    "nav.build-tasks": "Build Tasks",
    "nav.artifacts": "Artifacts",
    "nav.users": "Users",
    "nav.settings": "Settings",
    "nav.help": "Help",
    "view.dashboard.title": "Dashboard",
    "view.dashboard.subtitle": "Platform overview",
    "view.build-hosts.title": "Build Hosts",
    "view.build-hosts.subtitle": "Local and SSH builder capacity",
    "view.registries.title": "Registries",
    "view.registries.subtitle": "Pull and push destinations",
    "view.image-projects.title": "Image Projects",
    "view.image-projects.subtitle": "Root images, versions, and branches",
    "view.build-tasks.title": "Build Tasks",
    "view.build-tasks.subtitle": "Queue, scheduling, cancel, and retry",
    "view.artifacts.title": "Artifacts",
    "view.artifacts.subtitle": "Pushed image records and pull commands",
    "view.users.title": "Users",
    "view.users.subtitle": "Accounts, roles, and access state",
    "view.settings.title": "Settings",
    "view.settings.subtitle": "System defaults and audit logs",
    "view.help.title": "Help",
    "view.help.subtitle": "Operations guide and failure troubleshooting",
    "shell.product": "Image Build",
    "shell.console": "Platform Console",
    "shell.language": "Language",
    "shell.logout": "Logout",
  },
  "zh-CN": {
    "app.connecting.title": "正在连接平台",
    "app.connecting.detail": "正在检查初始化状态。",
    "app.backend_error.title": "无法连接后端服务",
    "app.backend_error.detail": "请确认服务已经启动，并且前端 API 地址配置正确。",
    "app.retry": "重试",
    "app.session.title": "正在加载会话",
    "app.session.detail": "正在读取当前登录用户。",
    "nav.dashboard": "总览",
    "nav.build-hosts": "构建主机",
    "nav.registries": "镜像仓库",
    "nav.image-projects": "镜像项目",
    "nav.build-tasks": "构建任务",
    "nav.artifacts": "镜像产物",
    "nav.users": "用户",
    "nav.settings": "设置",
    "nav.help": "帮助",
    "view.dashboard.title": "总览",
    "view.dashboard.subtitle": "平台运行概览",
    "view.build-hosts.title": "构建主机",
    "view.build-hosts.subtitle": "本机和 SSH 构建容量",
    "view.registries.title": "镜像仓库",
    "view.registries.subtitle": "拉取来源和推送目标",
    "view.image-projects.title": "镜像项目",
    "view.image-projects.subtitle": "基础镜像、版本和分支",
    "view.build-tasks.title": "构建任务",
    "view.build-tasks.subtitle": "队列、调度、取消和重试",
    "view.artifacts.title": "镜像产物",
    "view.artifacts.subtitle": "已推送镜像记录和拉取命令",
    "view.users.title": "用户",
    "view.users.subtitle": "账号、角色和访问状态",
    "view.settings.title": "设置",
    "view.settings.subtitle": "系统默认值和审计日志",
    "view.help.title": "帮助",
    "view.help.subtitle": "操作指南和故障排查",
    "shell.product": "镜像构建",
    "shell.console": "平台控制台",
    "shell.language": "语言",
    "shell.logout": "退出登录",
  },
}

type I18nContextValue = {
  language: Language
  setLanguage: (language: Language) => void
  t: (key: string) => string
}

const I18nContext = createContext<I18nContextValue | null>(null)

export function I18nProvider({ children }: PropsWithChildren) {
  const [language, setLanguageState] = useState<Language>(() => {
    const stored = localStorage.getItem("ibp.language")
    return stored === "en" || stored === "zh-CN" ? stored : "zh-CN"
  })

  const value = useMemo<I18nContextValue>(() => {
    return {
      language,
      setLanguage: (nextLanguage) => {
        localStorage.setItem("ibp.language", nextLanguage)
        setLanguageState(nextLanguage)
      },
      t: (key) => dictionaries[language][key] ?? dictionaries.en[key] ?? key,
    }
  }, [language])

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>
}

export function useI18n() {
  const value = useContext(I18nContext)
  if (value === null) {
    throw new Error("useI18n must be used inside I18nProvider")
  }
  return value
}
