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
export const getMyProfiMarketSolutions = options => apiClient.get('/api/profimarket/my-solutions', options)
export const getMyProfiMarketPurchases = options => apiClient.get('/api/profimarket/my-purchases', options)
export const getMyProfiMarketOrders = options => apiClient.get('/api/profimarket/my-orders', options)
export const getProfiMarketSolution = (slug, options) => apiClient.get(`/api/profimarket/solution/${encodeURIComponent(slug)}`, options)
export const addProfiMarketFavorite = (id, options) => apiClient.post(`/api/profimarket/solution/${encodeURIComponent(id)}/favorite`, null, options)
export const removeProfiMarketFavorite = (id, options) => apiClient.delete(`/api/profimarket/solution/${encodeURIComponent(id)}/favorite`, options)
export const purchaseProfiMarketSolution = (id, payload = {}, options) => apiClient.post(`/api/profimarket/solution/${encodeURIComponent(id)}/purchase`, payload, options)
export const createProfiMarketSolution = (payload, options) => apiClient.post('/api/profimarket', payload, options)
export const updateProfiMarketSolution = (id, payload, options) => apiClient.put(`/api/profimarket/solution/${encodeURIComponent(id)}`, payload, options)
export const publishProfiMarketSolution = (id, options) => apiClient.post(`/api/profimarket/solution/${encodeURIComponent(id)}/publish`, null, options)
export const uploadProfiMarketImage = (file, options) => { const body = new FormData(); body.append('image', file); return apiClient.post('/api/profimarket/upload', body, options) }
export const unpublishProfiMarketSolution = (id, options) => apiClient.post(`/api/profimarket/solution/${encodeURIComponent(id)}/unpublish`, null, options)
export const deleteProfiMarketSolution = (id, options) => apiClient.delete(`/api/profimarket/solution/${encodeURIComponent(id)}`, options)
