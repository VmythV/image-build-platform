const apiBaseURL = import.meta.env.VITE_API_BASE_URL ?? ""
const csrfCookieName = "ibp_csrf_token"
const csrfHeaderName = "X-CSRF-Token"
const csrfTokenPath = "/api/v1/csrf"

type ApiFetchOptions = Omit<RequestInit, "body"> & {
  body?: BodyInit | null
  json?: unknown
}

type NormalizedApiError = {
  code: string
  message: string
  details: unknown
}

export class ApiRequestError extends Error {
  status: number
  code: string
  details: unknown

  constructor(message: string, status: number, code = "REQUEST_FAILED", details?: unknown) {
    super(message)
    this.name = "ApiRequestError"
    this.status = status
    this.code = code
    this.details = details
  }
}

export function apiURL(path: string): string {
  return `${apiBaseURL}${path}`
}

export async function apiFetch<T>(path: string, options: ApiFetchOptions = {}): Promise<T> {
  const { json, headers, ...requestOptions } = options
  const requestHeaders = await prepareHeaders(path, requestOptions.method, headers)

  if (json !== undefined) {
    requestHeaders.set("Content-Type", "application/json")
    requestOptions.body = JSON.stringify(json)
  }

  requestHeaders.set("Accept", "application/json")

  const response = await fetch(`${apiBaseURL}${path}`, {
    credentials: "include",
    ...requestOptions,
    headers: requestHeaders,
  })
  const payload = await readResponse(response)

  if (!response.ok) {
    const error = parseAPIError(payload)
    throw new ApiRequestError(error.message, response.status, error.code, error.details)
  }

  if (isRecord(payload) && "data" in payload) {
    return payload.data as T
  }

  return payload as T
}

export async function apiFetchText(path: string, options: ApiFetchOptions = {}): Promise<string> {
  const { json, headers, ...requestOptions } = options
  const requestHeaders = await prepareHeaders(path, requestOptions.method, headers)

  if (json !== undefined) {
    requestHeaders.set("Content-Type", "application/json")
    requestOptions.body = JSON.stringify(json)
  }

  const response = await fetch(`${apiBaseURL}${path}`, {
    credentials: "include",
    ...requestOptions,
    headers: requestHeaders,
  })

  if (!response.ok) {
    const payload = await readResponse(response)
    const error = parseAPIError(payload)
    throw new ApiRequestError(error.message, response.status, error.code, error.details)
  }

  return response.text()
}

let cachedCSRFToken: string | null = null

async function prepareHeaders(path: string, method: string | undefined, headers: HeadersInit | undefined): Promise<Headers> {
  const requestHeaders = new Headers(headers)
  if (requiresCSRFToken(path, method)) {
    requestHeaders.set(csrfHeaderName, await ensureCSRFToken())
  }
  return requestHeaders
}

function requiresCSRFToken(path: string, method: string | undefined): boolean {
  const requestMethod = (method ?? "GET").toUpperCase()
  if (requestMethod === "GET" || requestMethod === "HEAD" || requestMethod === "OPTIONS") {
    return false
  }

  const requestPath = path.split("?")[0]
  return requestPath !== "/api/v1/setup/admin" && requestPath !== "/api/v1/auth/login"
}

async function ensureCSRFToken(): Promise<string> {
  const cookieToken = readCookie(csrfCookieName)
  if (cookieToken !== null) {
    cachedCSRFToken = cookieToken
    return cookieToken
  }
  if (cachedCSRFToken !== null) {
    return cachedCSRFToken
  }

  const response = await fetch(`${apiBaseURL}${csrfTokenPath}`, {
    credentials: "include",
    headers: {
      Accept: "application/json",
    },
  })
  const payload = await readResponse(response)
  if (!response.ok) {
    const error = parseAPIError(payload)
    throw new ApiRequestError(error.message, response.status, error.code, error.details)
  }
  if (!isRecord(payload) || !isRecord(payload.data) || typeof payload.data.token !== "string") {
    throw new ApiRequestError("CSRF token response is invalid.", response.status, "INVALID_CSRF_RESPONSE", payload)
  }

  cachedCSRFToken = payload.data.token
  return cachedCSRFToken
}

function readCookie(name: string): string | null {
  if (typeof document === "undefined") {
    return null
  }

  const prefix = `${name}=`
  const cookie = document.cookie
    .split(";")
    .map((item) => item.trim())
    .find((item) => item.startsWith(prefix))

  if (cookie === undefined) {
    return null
  }

  return decodeURIComponent(cookie.slice(prefix.length))
}

async function readResponse(response: Response): Promise<unknown> {
  const contentType = response.headers.get("Content-Type") ?? ""
  if (!contentType.includes("application/json")) {
    return response.text()
  }
  return response.json()
}

function parseAPIError(payload: unknown): NormalizedApiError {
  if (isRecord(payload) && isRecord(payload.error)) {
    return {
      code: typeof payload.error.code === "string" ? payload.error.code : "REQUEST_FAILED",
      message: typeof payload.error.message === "string" ? payload.error.message : "Request failed.",
      details: payload.error.details,
    }
  }

  return {
    code: "REQUEST_FAILED",
    message: "Request failed.",
    details: payload,
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null
}
