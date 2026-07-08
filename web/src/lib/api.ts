const apiBaseURL = import.meta.env.VITE_API_BASE_URL ?? ""

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

export async function apiFetch<T>(path: string, options: ApiFetchOptions = {}): Promise<T> {
  const { json, headers, ...requestOptions } = options
  const requestHeaders = new Headers(headers)

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
