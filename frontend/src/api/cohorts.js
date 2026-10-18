import { apiClient } from './client'
const base = id => `/api/academy/courses/${id}`
export const cohortAPI = {
 preview:(id,cohort)=>apiClient.get(`${base(id)}/cohorts/${cohort}/preview`),
 assetURL:(id,cohort,lesson,asset,download=false)=>`${base(id)}/cohorts/${cohort}/assets/${asset}?lesson=${encodeURIComponent(lesson)}${download?'&download=1':''}`,
 list: id => apiClient.get(`${base(id)}/cohorts`),
 get: (id, cohort) => apiClient.get(`${base(id)}/cohorts/${cohort}`),
 create: (id, body) => apiClient.post(`${base(id)}/cohorts`, body),
 save: (id, cohort, body) => apiClient.put(`${base(id)}/cohorts/${cohort}`, body),
 enroll: (id, cohort, body) => apiClient.post(`${base(id)}/cohorts/${cohort}/enroll`, body),
 staff: (id, cohort, body, remove = false) => remove ? apiClient.delete(`${base(id)}/cohorts/${cohort}/staff`, { body }) : apiClient.put(`${base(id)}/cohorts/${cohort}/staff`, body),
 students: (id, cohort = '') => apiClient.get(`${base(id)}/crm?cohort=${cohort}`),
 card: (id, enrollment) => apiClient.get(`${base(id)}/crm/${enrollment}`),
 curator: (id, enrollment, user) => apiClient.put(`${base(id)}/crm/${enrollment}/curator`, { user_id: user }),
 note: (id, enrollment, body, note, remove = false) => { const path = `${base(id)}/crm/${enrollment}/notes${note ? '/' + note : ''}`; return remove ? apiClient.delete(path, { body }) : note ? apiClient.put(path, body) : apiClient.post(path, body) },
 tags: id => apiClient.get(`${base(id)}/crm/tags`),
 tag: (id, body, tag, remove = false) => { const path = `${base(id)}/crm/tags${tag ? '/' + tag : ''}`; return remove ? apiClient.delete(path, { body }) : tag ? apiClient.put(path, body) : apiClient.post(path, body) },
 assignTags: (id, enrollment, tags) => apiClient.put(`${base(id)}/crm/${enrollment}/tags`, { tag_ids: tags }),
 workspaces: () => apiClient.get('/api/academy/workspaces'),
}
