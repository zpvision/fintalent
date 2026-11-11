import { apiClient } from './client'
const base = id => `/api/academy/courses/${id}`
export const academyAPI = {
 workspaceSummary: id => apiClient.get(`${base(id)}?summary=1`),
 workspace: id => apiClient.get(base(id)),
 create: (id, source) => apiClient.post(`${base(id)}/program`, { source }),
 save: (id, data) => apiClient.put(`${base(id)}/program`, data),
 action: (id, data) => apiClient.post(`${base(id)}/actions`, data),
 snapshot: (id, revision) => apiClient.post(`${base(id)}/versions`, { revision }),
 versions: id => apiClient.get(`${base(id)}/versions`),
 versionDraft: (id, version) => apiClient.get(`${base(id)}/versions/${version}/draft`),
 createVersion: (id, source) => apiClient.post(`${base(id)}/versions`, { source_version_id: Number(source) }),
 publication: (id, version) => apiClient.get(`${base(id)}/versions/${version}/publication`),
 versionPreview: (id, version) => apiClient.get(`${base(id)}/versions/${version}/preview`),
 revisionPreview: (id, version, revision) => apiClient.get(`${base(id)}/versions/${version}/preview?revision=${encodeURIComponent(revision)}`),
 publishChanges: (id, version, revision, key) => apiClient.post(`${base(id)}/versions/${version}/publish`, { revision, acknowledge_warnings: true, request_key: key }),
 refreshTests: (id, version, revision) => apiClient.post(`${base(id)}/versions/${version}/refresh-tests`, { revision }),
 publicationHistory: (id, version) => apiClient.get(`${base(id)}/versions/${version}/history`),
 assets: id => apiClient.get(`${base(id)}/assets`),
 upload: (id, key, file) => { const data = new FormData(); data.append('entity_key', key); data.append('file', file); return apiClient.post(`${base(id)}/assets`, data) },
 assetURL: (id, asset, download = false) => `${base(id)}/assets/${asset}${download ? '?download=1' : ''}`,
}
