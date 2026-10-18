import { apiClient } from './client'
const makeAPI = prefix => { const base = id => `${prefix}/courses/${id}/analytics`; return {
 get: (id, section, query, options) => apiClient.get(`${base(id)}/${section}?${query}`, options),
 settings: (id, value) => apiClient.put(`${base(id)}/settings`, value),
 reports: id => apiClient.get(`${base(id)}/reports`),
 create: (id, kind, format, query, key) => { const filters = Object.fromEntries(query); for (const k of ['cohort','version','curator','days','page','size','enrollment','progress_min']) if (filters[k] != null) filters[k] = Number(filters[k]); return apiClient.post(`${base(id)}/reports`, { kind, format, filters, request_key: key }) },
 remove: (id, report) => apiClient.delete(`${base(id)}/reports/${report}`),
 download: (id, report) => apiClient.get(`${base(id)}/reports/${report}/file`, { responseType: 'blob' }),
}}
export const analyticsAPI = makeAPI('/api/academy')
export const adminAnalyticsAPI = makeAPI('/api/admin/academy')
