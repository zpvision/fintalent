import { apiClient } from './client'
const base = id => `/api/academy/courses/${id}/communications`
export const communicationAPI = {
 campaigns: (id, before, status = "") => apiClient.get(`${base(id)}/campaigns?${new URLSearchParams({ ...(before ? { before } : {}), ...(status ? { status } : {}) })}`),
 campaign: (id, campaign) => apiClient.get(`${base(id)}/campaigns/${campaign}`),
 save: (id, value) => value.id ? apiClient.put(`${base(id)}/campaigns/${value.id}`, value) : apiClient.post(`${base(id)}/campaigns`, value),
 action: (id, campaign, action, body = {}) => apiClient.post(`${base(id)}/campaigns/${campaign}/${action}`, body),
 remove: (id, campaign) => apiClient.delete(`${base(id)}/campaigns/${campaign}`),
 audience: (id, body) => apiClient.post(`${base(id)}/audience`, body),
 preview: (id, message) => apiClient.post(`${base(id)}/preview`, { message }),
 test: (id, message, email) => apiClient.post(`${base(id)}/test`, { message, email }),
 templates: id => apiClient.get(`${base(id)}/templates`),
 template: (id, value, remove = false) => remove ? apiClient.delete(`${base(id)}/templates/${value.id}`, { body: { revision: value.revision } }) : value.id ? apiClient.put(`${base(id)}/templates/${value.id}`, value) : apiClient.post(`${base(id)}/templates`, value),
 automations: (id, before, status = "") => apiClient.get(`${base(id)}/automations?${new URLSearchParams({ ...(before ? { before } : {}), ...(status ? { status } : {}) })}`),
 automation: (id, rule) => apiClient.get(`${base(id)}/automations/${rule}`),
 saveAutomation: (id, value) => value.id ? apiClient.put(`${base(id)}/automations/${value.id}`, value) : apiClient.post(`${base(id)}/automations`, value),
 history: (id, query = {}) => apiClient.get(`${base(id)}/deliveries?${new URLSearchParams(query)}`),
 runs: (id, rule, before) => apiClient.get(`${base(id)}/automations/${rule}/runs${before ? `?before=${before}` : ""}`),
 retry: (id, delivery, confirm = false) => apiClient.post(`${base(id)}/deliveries/${delivery}/retry`, { confirm_uncertain: confirm }),
 upload: file => { const body = new FormData(); body.append('image', file); return apiClient.post('/api/profimarket/upload', body) },
 preferences: () => apiClient.get('/api/academy/communication-preferences'),
 preference: value => apiClient.put('/api/academy/communication-preferences', value),
 unsubscribe: (token, confirm = false) => confirm ? apiClient.post(`/api/academy/unsubscribe/${encodeURIComponent(token)}`, {}) : apiClient.get(`/api/academy/unsubscribe/${encodeURIComponent(token)}`),
}
