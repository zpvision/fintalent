import { apiClient } from './client'
const base = c => `/api/academy/enrollments/${c.enrollmentId}/lessons/${c.lessonKey}/assessments/${c.blockKey}`
const owner = id => `/api/academy/courses/${id}`
export const assessmentAPI = {
  get: (c, attempt) => apiClient.get(base(c) + (attempt ? `?attempt=${attempt}` : '')),
  start: (c, key) => apiClient.post(`${base(c)}/start`, { request_key: key }),
  answer: (c, id, body) => apiClient.put(`${base(c)}/attempts/${id}/answer`, body),
  finish: (c, id) => apiClient.post(`${base(c)}/attempts/${id}/finish`, {}),
  draft: (c, key) => apiClient.post(`${base(c)}/draft`, { request_key: key }),
  save: (c, id, body) => apiClient.put(`${base(c)}/submissions/${id}`, body),
  submit: (c, id, revision) => apiClient.post(`${base(c)}/submissions/${id}/submit`, { revision }),
  upload: (c, id, file) => { const data = new FormData(); data.append('file', file); return apiClient.post(`${base(c)}/submissions/${id}/files`, data) },
  file: (c, id, file) => `${base(c)}/submissions/${id}/files/${file}`,
  queue: (id, filters) => apiClient.get(`${owner(id)}/reviews?${new URLSearchParams(filters)}`),
  review: (id, submission) => apiClient.get(`${owner(id)}/reviews/${submission}`),
  decide: (id, submission, body) => apiClient.post(`${owner(id)}/reviews/${submission}`, body),
  reviewFile: (id, submission, file) => `${owner(id)}/reviews/${submission}/files/${file}`,
  results: (id, before) => apiClient.get(`${owner(id)}/assessment-results${before ? `?before=${before}` : ''}`),
  activate: (id, version) => apiClient.post(`${owner(id)}/releases/${version}/assessments`, {}),
  notifications: () => apiClient.get('/api/academy/notifications'),
  read: ids => apiClient.put('/api/academy/notifications', { ids }),
}
