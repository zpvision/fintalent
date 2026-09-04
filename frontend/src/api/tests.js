import { apiClient, apiRequest } from './client'

export const getMyTestResults = (options) => apiClient.get('/api/me/test-results', options)
export const getMyTests = (options) => apiClient.get('/api/tests?scope=mine', options)
export const setTestResultVisibility = (id, visible, options) =>
  apiRequest(`/api/me/test-results/${encodeURIComponent(id)}/resume-visibility`, {
    ...options,
    method: 'PATCH',
    body: { visible },
  })
export const getEmployees = (options) => apiClient.get('/api/employee-testing/employees', options)
export const getEmployeeTests = (options) => apiClient.get('/api/employee-testing/tests', options)
export const getEmployeeResults = (options) => apiClient.get('/api/employee-testing/results', options)
export const addEmployees = (employees, options) => apiClient.post('/api/employee-testing/employees', { employees }, options)
export const deleteEmployee = (id, options) => apiClient.delete(`/api/employee-testing/employees/${encodeURIComponent(id)}`, options)
export const importEmployeesFromFinKoper = (email, password, options) =>
  apiClient.post('/api/employee-testing/import/finkoper', { email, password }, { redirectOnUnauthorized: false, ...options })
export const createInvitations = (testId, employeeIds, options) =>
  apiClient.post('/api/employee-testing/invitations', { test_id: testId, employee_ids: employeeIds }, options)
export const getTest = (id, options) => apiClient.get(`/api/tests/${encodeURIComponent(id)}`, options)
export const getTestReviews = (id, options) => apiClient.get(`/api/marketplace/test-reviews?test_id=${encodeURIComponent(id)}`, { redirectOnUnauthorized: false, ...options })
export const startTestAttempt = (id, options) => apiClient.post(`/api/tests/${encodeURIComponent(id)}/attempts`, null, options)
export const getTestAttempt = (id, options) => apiClient.get(`/api/attempts/${encodeURIComponent(id)}`, options)
export const saveTestAnswer = (id, body, options) => apiClient.post(`/api/attempts/${encodeURIComponent(id)}/answers`, body, options)
export const finishTestAttempt = (id, options) => apiClient.post(`/api/attempts/${encodeURIComponent(id)}/finish`, null, options)
const employeePath = (token, action='') => `/api/employee-test/${encodeURIComponent(token)}${action?`/${action}`:''}`
export const getEmployeeTest = (token, options) => apiClient.get(employeePath(token), { redirectOnUnauthorized:false, ...options })
export const startEmployeeTest = (token, options) => apiClient.post(employeePath(token,'start'), {}, { redirectOnUnauthorized:false, ...options })
export const saveEmployeeTestAnswer = (token, body, options) => apiClient.post(employeePath(token,'answer'), body, { redirectOnUnauthorized:false, ...options })
export const finishEmployeeTest = (token, options) => apiClient.post(employeePath(token,'finish'), {}, { redirectOnUnauthorized:false, ...options })
export const getTestCategories = options => apiClient.get('/api/test-categories', { redirectOnUnauthorized:false, ...options })
export const createTest = (body, options) => apiClient.post('/api/tests', body, options)
export const updateTest = (id, body, options) => apiClient.put(`/api/tests/${encodeURIComponent(id)}`, body, options)
export const forkTestDraft = (id, options) => apiClient.post(`/api/tests/${encodeURIComponent(id)}/draft-version`, null, options)
export const publishTest = (id, options) => apiClient.post(`/api/tests/${encodeURIComponent(id)}/publish`, null, options)
export const createQuestion = (id, body, options) => apiClient.post(`/api/tests/${encodeURIComponent(id)}/questions`, body, options)
export const updateQuestion = (id, body, options) => apiClient.put(`/api/questions/${encodeURIComponent(id)}`, body, options)
export const deleteQuestion = (id, options) => apiClient.delete(`/api/questions/${encodeURIComponent(id)}`, options)
