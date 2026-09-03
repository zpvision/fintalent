import { apiClient } from './client'

export function getPublicVacancy(id, options) {
  return apiClient.get(`/api/public/vacancies/${encodeURIComponent(id)}`, {
    redirectOnUnauthorized: false,
    ...options,
  })
}
