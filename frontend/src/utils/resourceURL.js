// React resource properties still need scheme validation for stored legacy data.
export function resourceURL(value) {
  const text = String(value ?? '').trim()
  if (!text || /[\x00-\x20\x7f<>"'\\]/.test(text)) return ''
  try {
    const url = new URL(text, window.location.origin)
    return ['http:', 'https:'].includes(url.protocol) && !url.username && !url.password ? text : ''
  } catch { return '' }
}
