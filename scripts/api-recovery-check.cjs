// Transport unit checks: no real server, credentials or database.
const assert = require('node:assert/strict')
const vm = require('node:vm')
const { buildSync } = require('esbuild')
const path = require('node:path')
const code = buildSync({ entryPoints: [path.join(__dirname, '../frontend/src/api/client.js')], bundle: true, write: false, platform: 'browser', format: 'iife', globalName: 'client' }).outputFiles[0].text

function setup(transport) {
  const events = []
  let calls = 0
  const context = { Headers, FormData, performance, setTimeout, window: { reportFinTalentError: e => events.push(e) }, fetch: (...args) => { calls++; return transport(...args) } }
  vm.createContext(context)
  vm.runInContext(code, context)
  return { client: context.client, events, calls: () => calls }
}
const ok = () => new Response('{"allowed":true}', { headers: { 'Content-Type': 'application/json' } })
;(async () => {
  let first = true
  const recovered = setup(() => { if (first) { first = false; throw new TypeError('Failed to fetch') } return ok() })
  assert.equal((await recovered.client.apiClient.get('/api/courses/access')).allowed, true)
  assert.equal(recovered.calls(), 2)
  assert.equal(recovered.events.length, 0)

  const failure = setup(() => { throw new TypeError('Failed to fetch') })
  await assert.rejects(failure.client.apiClient.get('/api/academy/learning/access'))
  assert.equal(failure.calls(), 2)
  assert.equal(failure.events.length, 1)
  assert.equal(failure.events[0].code, 'network_error')

  for (const method of ['post', 'put', 'delete']) {
    const write = setup(() => { throw new TypeError('Failed to fetch') })
    await assert.rejects(write.client.apiClient[method]('/api/academy/enrollments/1/lessons/lesson/complete', {}))
    assert.equal(write.calls(), 1, `${method} must never be replayed`)
  }
  const serverError = setup(() => new Response('{}', { status: 500, headers: { 'Content-Type': 'application/json' } }))
  await assert.rejects(serverError.client.apiClient.get('/api/courses/access'))
  assert.equal(serverError.calls(), 1)
  assert.equal(serverError.events[0].code, 'api_error')

  const controller = new AbortController()
  const cancelled = setup(() => { controller.abort(); throw new DOMException('Cancelled', 'AbortError') })
  await assert.rejects(cancelled.client.apiClient.get('/api/courses/access', { signal: controller.signal }))
  assert.equal(cancelled.calls(), 1)
  assert.equal(cancelled.events.length, 0)

  const abortedSignal = new AbortController()
  const browserCancellation = setup(() => { abortedSignal.abort(); throw new TypeError('Failed to fetch') })
  await assert.rejects(browserCancellation.client.apiClient.get('/api/courses/access', { signal: abortedSignal.signal }))
  assert.equal(browserCancellation.calls(), 1)
  assert.equal(browserCancellation.events.length, 0)

  const pending = new AbortController()
  const betweenAttempts = setup(() => { setTimeout(() => pending.abort(), 20); throw new TypeError('Failed to fetch') })
  await assert.rejects(betweenAttempts.client.apiClient.get('/api/courses/access', { signal: pending.signal }))
  assert.equal(betweenAttempts.calls(), 1)
  assert.equal(betweenAttempts.events.length, 0)
  console.log('PASS: GET recovery; bounded retries; writes and HTTP failures never replayed; permanent failures reported; cancellation respected')
})().catch(error => { console.error(error); process.exitCode = 1 })
