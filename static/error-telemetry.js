/* Metadata only: never send exception messages, form values, query strings or bodies. */
(() => {
  if (window.reportFinTalentError) return
  const send = window.fetch.bind(window), seen = new Map()
  let windowStart = Date.now(), sent = 0
  const release = document.querySelector('meta[name="fintalent-release"]')?.content || 'development'
  function routeOf(value) {
    try { const url = new URL(value || location.pathname, location.origin); return url.origin === location.origin ? url.pathname : '/external' }
    catch { return '/unknown' }
  }
  window.reportFinTalentError = (event = {}) => {
    const now = Date.now()
    if (now - windowStart >= 60000) { windowStart = now; sent = 0; seen.clear() }
    if (sent >= 10 || navigator.onLine === false) return
    const payload = {
      code: event.code || 'javascript_error', route: routeOf(event.route),
      method: String(event.method || '').slice(0, 12), status: Number(event.status) || 0,
      request_id: String(event.request_id || '').slice(0, 64),
      // Locations only; discard the first line containing the exception message.
      stack: (String(event.stack || '').match(/\/(?:static|src)\/[a-zA-Z0-9_./-]+\.(?:js|jsx|mjs|tsx|ts):[0-9]+(?::[0-9]+)?/g) || []).slice(0, 12).join('\n'),
      release,
    }
    const key = JSON.stringify([payload.code, payload.route, payload.status, payload.stack])
    if (seen.has(key)) return
    seen.set(key, now); sent++
    // Use the captured transport: telemetry failures must never report themselves.
    void send('/api/diagnostics/events', { method: 'POST', credentials: 'same-origin', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload), keepalive: true }).catch(() => {})
  }
  window.addEventListener('error', event => {
    if (event.error) window.reportFinTalentError({ code: 'javascript_error', stack: event.error.stack })
    else if (event.target instanceof HTMLScriptElement || event.target instanceof HTMLLinkElement) window.reportFinTalentError({ code: 'resource_error', route: event.target.src || event.target.href })
  }, true)
  window.addEventListener('unhandledrejection', event => {
    if (event.reason?.name !== 'AbortError') window.reportFinTalentError({ code: 'unhandled_rejection', stack: event.reason?.stack })
  })
})()
