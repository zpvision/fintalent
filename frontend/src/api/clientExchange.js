import { apiClient, apiRequest } from './client'

const base = '/api/client-exchange'
export const getClientExchangeMeta = options => apiClient.get(`${base}/meta`, options)
export const getClientExchangeListings = (params, options) => apiClient.get(`${base}/listings?${params}`, options)
export const getClientExchangeListing = (id, options) => apiClient.get(`${base}/listings/${encodeURIComponent(id)}`, options)
export const getClientExchangeNotifications = options => apiClient.get(`${base}/notifications`, options)
export const setClientExchangeFavorite = (id, active, options) => apiRequest(`${base}/listings/${encodeURIComponent(id)}/favorite`, { ...options, method: active ? 'POST' : 'DELETE' })
export const sendClientExchangeProposal = (id, body, options) => apiClient.post(`${base}/listings/${encodeURIComponent(id)}/responses`, body, options)
export const createClientExchangeListing = (body, options) => apiClient.post(`${base}/listings`, body, options)
export const updateClientExchangeListing = (id, body, options) => apiClient.put(`${base}/listings/${encodeURIComponent(id)}`, body, options)
export const publishClientExchangeListing = (id, options) => apiClient.post(`${base}/listings/${encodeURIComponent(id)}/publish`, null, options)
export const findCities = (query, options) => apiClient.get(`/api/public/cities?country=RU&q=${encodeURIComponent(query)}`, { redirectOnUnauthorized: false, ...options })
