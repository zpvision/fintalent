// Recover shared styles (including the Vite bundle) after transient failures.
// Page-specific styles have their own loader and retry controls.
(() => {
  if (window.__finTalentStyleRecovery) return
  window.__finTalentStyleRecovery = true
  const failures = new Map()
  function retry(link, state) {
    clearTimeout(state.timer)
    state.timer = null
    if (!link.isConnected) { failures.delete(link); return }
    if (state.attempts >= 2) return
    state.attempts++
    const url = new URL(state.href)
    url.searchParams.set('_style_retry', String(state.attempts))
    link.href = url.href
  }
  window.addEventListener('error', event => {
    const link = event.target
    if (!(link instanceof HTMLLinkElement) || !link.relList.contains('stylesheet') || link.dataset.reactPageStyle) return
    const url = new URL(link.href, location.href)
    if (url.origin !== location.origin || !url.pathname.startsWith('/static/')) return
    let state = failures.get(link)
    if (!state) {
      state = { href: link.href, attempts: 0, timer: null }
      failures.set(link, state)
      link.addEventListener('load', () => { clearTimeout(state.timer); failures.delete(link) }, { once: true })
    }
    if (state.timer === null && state.attempts < 2) state.timer = setTimeout(() => retry(link, state), 500)
  }, true)
  window.addEventListener('online', () => {
    for (const [link, state] of failures) retry(link, state)
  })
})()
