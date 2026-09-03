import { apiClient } from './client'

export function getProfiMarketSolutions({ query = '', type = '' }, options) {
  const params = new URLSearchParams()
  if (query.trim()) params.set('q', query.trim())
  if (type) params.set('type', type)
  return apiClient.get(`/api/profimarket?${params}`, {
    redirectOnUnauthorized: false,
    ...options,
  })
}

export function getProfiMarketMeta(options) {
  return apiClient.get('/api/profimarket/meta', {
    redirectOnUnauthorized: false,
    ...options,
  })
}
