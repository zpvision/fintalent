import { apiClient } from './client'

export function getAccountingCompanies(params, options) {
  const query = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value !== '' && value != null) query.set(key, value)
  }
  return apiClient.get(`/api/accounting-companies?${query}`, {
    redirectOnUnauthorized: false,
    ...options,
  })
}

export function getAccountingCompaniesMeta(options) {
  return apiClient.get('/api/accounting-companies/meta', {
    redirectOnUnauthorized: false,
    ...options,
  })
}

export function getAccountingCompany(key, options) {
  return apiClient.get(`/api/accounting-companies/${key}`, {
    redirectOnUnauthorized: false,
    ...options,
  })
}

export function getAccountingCompanyPassport(id, options) {
  return apiClient.get(`/api/accounting-companies/${encodeURIComponent(id)}/passport`, {
    redirectOnUnauthorized: false,
    ...options,
  })
}

export function createAccountingCompanyReview(id, payload, options) {
  return apiClient.post(`/api/accounting-companies/${encodeURIComponent(id)}/reviews`, payload, {
    redirectOnUnauthorized: false,
    ...options,
  })
}
