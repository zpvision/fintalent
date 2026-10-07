import { apiClient } from './client'

export const courseAPI = {
  meta: () => apiClient.get('/api/courses/meta'),
  catalog: () => apiClient.get('/api/courses'),
  mine: () => apiClient.get('/api/courses/my'),
  create: () => apiClient.post('/api/courses/my', {}),
  get: id => apiClient.get(`/api/courses/my/${id}`),
  public: slug => apiClient.get(`/api/courses/view/${encodeURIComponent(slug)}`),
  save: (id, value) => apiClient.put(`/api/courses/my/${id}`, value),
  action: (id, action) => apiClient.post(`/api/courses/my/${id}/${action}`, {}),
  upload: file => { const body = new FormData(); body.append('image', file); return apiClient.post('/api/courses/upload', body) },
}
