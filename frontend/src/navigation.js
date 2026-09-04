export function navigateInApp(to, { replace = false } = {}) {
  const event = new CustomEvent('fintalent:navigate', {
    cancelable: true,
    detail: { to, replace },
  })
  window.dispatchEvent(event)
  if (!event.defaultPrevented) {
    if (replace) window.location.replace(to)
    else window.location.assign(to)
  }
}
