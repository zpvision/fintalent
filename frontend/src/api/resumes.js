import { apiClient } from './client'

export function getPublicResume(id, options) {
  return apiClient.get(`/api/public/resumes/${encodeURIComponent(id)}`, {
    redirectOnUnauthorized: false,
    ...options,
  })
}

export function getResumeKnowledge(id, options) {
  return apiClient.get(`/api/resumes/${encodeURIComponent(id)}/test-knowledge`, {
    redirectOnUnauthorized: false,
    ...options,
  })
}

export function setResumeKnowledgeConfirmation(id, testId, confirmed, options) {
  const path = `/api/resumes/${encodeURIComponent(id)}/test-knowledge/confirmations`
  if (confirmed) {
    return apiClient.delete(path, { ...options, body: { test_id: Number(testId) } })
  }
  return apiClient.post(path, { test_id: Number(testId) }, options)
}

export function createHelpRequest(payload, options) {
  return apiClient.post('/api/v1/help/requests', payload, {
    redirectOnUnauthorized: false,
    ...options,
  })
}
