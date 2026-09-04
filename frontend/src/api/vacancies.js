import { apiClient } from './client'

export function getPublicVacancy(id, options) {
  return apiClient.get(`/api/public/vacancies/${encodeURIComponent(id)}`, {
    redirectOnUnauthorized: false,
    ...options,
  })
}
export const getVacancyBuilder = options => apiClient.get('/api/v1/vacancy-builder', options)
export const getDutyCatalog = options => apiClient.get('/api/v1/duty-categories?include_duties=true', options)
export const getVacancyDraft = (id, options) => apiClient.get(`/api/v1/vacancies/${encodeURIComponent(id)}`, options)
export const createVacancyDraft = options => apiClient.post('/api/v1/vacancies', {}, options)
export const updateVacancyDraft = (id, body, options) => apiClient.put(`/api/v1/vacancies/${encodeURIComponent(id)}`, body, options)
export const updateVacancyDuties = (id, dutyIds, options) => apiClient.put(`/api/v1/vacancies/${encodeURIComponent(id)}/duties`, { duty_ids:dutyIds }, options)
export const publishVacancy = (id, options) => apiClient.post(`/api/v1/vacancies/${encodeURIComponent(id)}/publish`, null, options)
export const previewVacancyMatch = (body, options) => apiClient.post('/api/v1/vacancies/match-preview', body, options)
export const getVacancyTests = options => apiClient.get('/api/marketplace/tests', options)
