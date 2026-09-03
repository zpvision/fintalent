import { apiClient } from './client'

export function getPublications(criteria, page, options) {
  const params = new URLSearchParams({ page: String(page), ...criteria })
  return apiClient.get(`/api/publications?${params}`, {
    redirectOnUnauthorized: false,
    ...options,
  })
}

export function getPublicationsMeta(options) {
  return apiClient.get('/api/publications/meta', {
    redirectOnUnauthorized: false,
    ...options,
  })
}

export function togglePublicationBookmark(id, options) {
  return apiClient.post(`/api/publications/${id}/bookmark`, null, options)
}

export function togglePublicationAuthor(authorId, options) {
  return apiClient.post(`/api/publication-authors/${authorId}/subscribe`, null, options)
}
