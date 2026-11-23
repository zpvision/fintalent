import { navigateInApp } from '../navigation'

export class ApiError extends Error {
  constructor(message, status, payload, requestID = '') {
    super(status >= 500 && requestID ? `${message} Код ошибки: ${requestID}` : message)
    this.name = 'ApiError'
    this.status = status
    this.payload = payload
    this.requestID = requestID
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
    responseType = 'auto',
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

  let response
  const started = performance.now()
  try { response = await fetch(path, {
    credentials: 'include',
    ...requestOptions,
    headers: requestHeaders,
    body: requestBody,
  }) } catch (error) {
    if (error.name !== 'AbortError') window.reportFinTalentError?.({ code: 'network_error', route: path, method: requestOptions.method || 'GET', duration_ms: Math.round(performance.now() - started) })
    throw error
  }
  const payload = response.ok && responseType === 'blob' ? await response.blob() : await parseResponse(response)

  if (!response.ok) {
    const requestID = response.headers.get('X-Request-ID') || ''
    if (response.status >= 500) window.reportFinTalentError?.({ code: 'api_error', route: path, method: requestOptions.method || 'GET', status: response.status, request_id: requestID, duration_ms: Math.round(performance.now() - started) })
    if (response.status === 401 && redirectOnUnauthorized && window.location.pathname !== '/login') {
      const next = `${window.location.pathname}${window.location.search}`
      navigateInApp(`/login?next=${encodeURIComponent(next)}`)
    }
    throw new ApiError(
      responseMessage(payload, `Ошибка запроса (${response.status})`),
      response.status,
      payload,
      requestID,
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
