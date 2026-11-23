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
  function resourceURL(value) {
    if (!value) return ''
    try {
      const url = new URL(value, location.origin), path = url.pathname
      if (!['http:', 'https:'].includes(url.protocol)) return '/:redacted'
      const asset = /^\/[a-zA-Z0-9_./-]+\.(?:js|mjs|css|ico|woff|woff2|ttf|otf)$/.test(path) && !path.includes('..')
      if (url.origin === location.origin) {
        return asset && ((path.startsWith('/static/') && !path.startsWith('/static/uploads/')) || path === '/favicon.ico') ? path : '/:redacted'
      }
      if (url.hostname === 'fonts.googleapis.com') return url.origin + (['/css', '/css2'].includes(path) ? path : '/:redacted')
      if (['fonts.gstatic.com', 'cdn.jsdelivr.net', 'unpkg.com'].includes(url.hostname)) return url.origin + (asset ? path : '/:redacted')
      return '/external/:redacted'
    } catch { return '/:redacted' }
  }
  window.reportFinTalentError = (event = {}) => {
    const now = Date.now()
    if (now - windowStart >= 60000) { windowStart = now; sent = 0; seen.clear() }
    if (sent >= 10 || navigator.onLine === false) return
    const payload = {
      code: event.code || 'javascript_error', route: routeOf(event.route),
      page_route: location.pathname,
      resource_url: resourceURL(event.resource_url),
      resource_type: ['script', 'stylesheet', 'icon', 'preload', 'modulepreload', 'preconnect', 'dns-prefetch', 'link'].includes(event.resource_type) ? event.resource_type : '',
      method: String(event.method || '').slice(0, 12), status: Number(event.status) || 0,
      request_id: String(event.request_id || '').slice(0, 64),
      // Locations only; discard the first line containing the exception message.
      stack: (String(event.stack || '').match(/\/(?:static|src)\/[a-zA-Z0-9_./-]+\.(?:js|jsx|mjs|tsx|ts):[0-9]+(?::[0-9]+)?/g) || []).slice(0, 12).join('\n'),
      release,
    }
    const key = JSON.stringify([payload.code, payload.route, payload.page_route, payload.resource_url, payload.resource_type, payload.status, payload.stack])
    if (seen.has(key)) return
    seen.set(key, now); sent++
    // Use the captured transport: telemetry failures must never report themselves.
    void send('/api/diagnostics/events', { method: 'POST', credentials: 'same-origin', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload), keepalive: true }).catch(() => {})
  }
  window.addEventListener('error', event => {
    if (event.error) window.reportFinTalentError({ code: 'javascript_error', stack: event.error.stack })
    else if (event.target instanceof HTMLScriptElement || event.target instanceof HTMLLinkElement) {
      const element = event.target, url = element.src || element.href
      const type = element instanceof HTMLScriptElement ? 'script' : ['stylesheet', 'icon', 'preload', 'modulepreload', 'preconnect', 'dns-prefetch'].find(rel => element.relList.contains(rel)) || 'link'
      // Browsers often hide the status (CORS, DNS, blocked requests). Never guess it or retry the failed resource.
      const timing = performance.getEntriesByName(url, 'resource').at(-1)
      const status = timing?.responseStatus
      window.reportFinTalentError({ code: 'resource_error', resource_url: url, resource_type: type, status: Number.isInteger(status) && status >= 100 && status <= 599 ? status : 0 })
    }
  }, true)
  window.addEventListener('unhandledrejection', event => {
    if (event.reason?.name !== 'AbortError') window.reportFinTalentError({ code: 'unhandled_rejection', stack: event.reason?.stack })
  })
})()
