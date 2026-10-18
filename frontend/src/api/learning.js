import { apiClient } from './client'
const owner = id => `/api/academy/courses/${id}`
const enrollment = id => `/api/academy/enrollments/${id}`
async function collection(path) {
  const items = []
  let before = ''
  for (;;) {
    const page = await apiClient.get(path + (before ? `?before=${before}` : ''))
    items.push(...page)
    if (page.length < 200) return items
    before = page.at(-1).id
  }
}
export const learningAPI = {
  mine: () => collection('/api/academy/learning'),
  room: id => apiClient.get(enrollment(id)),
  open: (id, key) => apiClient.post(`${enrollment(id)}/lessons/${key}/open`, { event_key: crypto.randomUUID() }),
  complete: (id, key, acknowledged) => apiClient.post(`${enrollment(id)}/lessons/${key}/complete`, { materials_acknowledged: acknowledged }),
  assetURL: (id, key, asset, download = false) => `${enrollment(id)}/assets/${asset}?lesson=${encodeURIComponent(key)}${download ? '&download=1' : ''}`,
  releases: id => apiClient.get(`${owner(id)}/releases`),
  publish: (id, version, acknowledge) => apiClient.post(`${owner(id)}/releases`, { version_id: version, acknowledge_warnings: acknowledge }),
  preview: (id, version) => apiClient.get(`${owner(id)}/releases/${version}/preview`),
  students: id => collection(`${owner(id)}/students`),
  search: (id, query) => apiClient.get(`${owner(id)}/user-search?q=${encodeURIComponent(query)}`),
  grant: (id, body) => apiClient.post(`${owner(id)}/students`, body),
  student: (id, student) => apiClient.get(`${owner(id)}/students/${student}`),
  access: (id, student, body) => apiClient.put(`${owner(id)}/students/${student}`, body),
}
