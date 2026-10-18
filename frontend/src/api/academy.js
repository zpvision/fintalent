import { apiClient } from './client'
const base = id => `/api/academy/courses/${id}`
export const academyAPI = {
 workspace: id => apiClient.get(base(id)),
 create: (id, source) => apiClient.post(`${base(id)}/program`, { source }),
 save: (id, data) => apiClient.put(`${base(id)}/program`, data),
 action: (id, data) => apiClient.post(`${base(id)}/actions`, data),
 snapshot: (id, revision) => apiClient.post(`${base(id)}/versions`, { revision }),
 assets: id => apiClient.get(`${base(id)}/assets`),
 upload: (id, key, file) => { const data = new FormData(); data.append('entity_key', key); data.append('file', file); return apiClient.post(`${base(id)}/assets`, data) },
 assetURL: (id, asset, download = false) => `${base(id)}/assets/${asset}${download ? '?download=1' : ''}`,
}
