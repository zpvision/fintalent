export class ApiError extends Error {
  constructor(message, status, payload) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.payload = payload
  }
}

function responseMessage(payload, fallback) {
  if (typeof payload === 'string' && payload.trim()) return payload.trim()
  if (payload && typeof payload.error === 'string') return payload.error
  if (payload && typeof payload.message === 'string') return payload.message
  return fallback
}

async function parseResponse(response) {
  if (response.status === 204) return null
  const contentType = response.headers.get('content-type') || ''
  if (contentType.includes('application/json')) {
    return response.json().catch(() => null)
  }
  return response.text().catch(() => '')
}

export async function apiRequest(path, options = {}) {
  const {
    redirectOnUnauthorized = true,
    headers,
    body,
    ...requestOptions
  } = options
  const requestHeaders = new Headers(headers)
  let requestBody = body

  if (body != null && !(body instanceof FormData) && typeof body !== 'string') {
    requestHeaders.set('Content-Type', 'application/json')
    requestBody = JSON.stringify(body)
  }

  const response = await fetch(path, {
    credentials: 'include',
    ...requestOptions,
    headers: requestHeaders,
    body: requestBody,
  })
  const payload = await parseResponse(response)

  if (!response.ok) {
    if (response.status === 401 && redirectOnUnauthorized && window.location.pathname !== '/login') {
      const next = `${window.location.pathname}${window.location.search}`
      window.location.assign(`/login?next=${encodeURIComponent(next)}`)
    }
    throw new ApiError(
      responseMessage(payload, `Ошибка запроса (${response.status})`),
      response.status,
      payload,
    )
  }

  return payload
}

export const apiClient = {
  get(path, options) {
    return apiRequest(path, { cache: 'no-store', ...options, method: 'GET' })
  },
  post(path, body, options) {
    return apiRequest(path, { ...options, method: 'POST', body })
  },
  put(path, body, options) {
    return apiRequest(path, { ...options, method: 'PUT', body })
  },
  delete(path, options) {
    return apiRequest(path, { ...options, method: 'DELETE' })
  },
}
