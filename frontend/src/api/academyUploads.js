import { apiClient } from './client'

const pause = (ms, signal) => new Promise((resolve, reject) => {
  if (signal.aborted) return reject(new DOMException('Отменено', 'AbortError'))
  const abort = () => { clearTimeout(timer); reject(new DOMException('Отменено', 'AbortError')) }
  const timer = setTimeout(() => { signal.removeEventListener('abort', abort); resolve() }, ms)
  signal.addEventListener('abort', abort, { once: true })
})
function putPart(signed, blob, signal, progress) {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest(), abort = () => xhr.abort()
    if (signal.aborted) return reject(new DOMException('Отменено', 'AbortError'))
    xhr.open('PUT', signed.url); xhr.timeout = 900000; xhr.withCredentials = false
    for (const [key, value] of Object.entries(signed.headers || {})) xhr.setRequestHeader(key, value)
    xhr.upload.onprogress = e => progress(e.loaded)
    const finish = error => { signal.removeEventListener('abort', abort); error ? reject(error) : resolve() }
    xhr.onload = () => finish(xhr.status >= 200 && xhr.status < 300 ? null : Error('Не удалось загрузить часть файла'))
    xhr.onerror = xhr.ontimeout = () => finish(Error('Соединение с хранилищем прервано'))
    xhr.onabort = () => finish(new DOMException('Отменено', 'AbortError'))
    signal.addEventListener('abort', abort, { once: true }); xhr.send(blob)
  })
}
export async function uploadAcademyFile(course, entity, file, { signal, progress = () => {}, video = false } = {}) {
  const base = `/api/academy/courses/${course}/assets`, options = { signal }
  const caps = await apiClient.get(`${base}/uploads`, options)
  if (video && !caps.enabled) throw Error('Для загрузки MP4 подключите S3-хранилище')
  const limit = video ? caps.video_max_bytes : caps.document_max_bytes
  if (file.size <= 0 || file.size > limit) throw Error(`Допустимый размер: до ${Math.floor(limit / 1048576)} МБ`)
  if (!caps.enabled) {
    if (video) throw Error('Для загрузки MP4 подключите S3-хранилище')
    const data = new FormData(); data.append('entity_key', entity); data.append('file', file)
    return apiClient.post(base, data, options)
  }
  const cache = `fintalent.upload.${course}.${entity}.${file.name}.${file.size}.${file.lastModified}`
  let id
  try { id = sessionStorage.getItem(cache) } catch {}
  id ||= crypto.randomUUID()
  try { sessionStorage.setItem(cache, id) } catch {}
  let session
  try { session = await apiClient.post(`${base}/uploads`, { request_key: id, entity_key: entity, name: file.name, size: file.size }, options) }
  catch (error) {
    if ([404, 409, 410].includes(error.status)) { try { sessionStorage.removeItem(cache) } catch {} }
    if (signal.aborted) { try { await apiClient.delete(`${base}/uploads/${id}`); sessionStorage.removeItem(cache) } catch {}; throw Error('Загрузка отменена') }
    throw error
  }
  for (let n = 0; session.status === 'initiating' && n < 60; n++) { await pause(2000, signal); session = await apiClient.get(`${base}/uploads/${id}`, options) }
  if (session.status === 'initiating') throw Error('Сессия ещё подготавливается. Повторите выбор файла позже.')
  if (['failed', 'cancelled'].includes(session.status) || new Date(session.expires_at) <= new Date()) {
    try { sessionStorage.removeItem(cache) } catch {}
    throw Error('Предыдущая загрузка закончилась. Выберите файл ещё раз.')
  }
  try { sessionStorage.setItem(cache, id) } catch {}
  let finalizing = session.status === 'finalizing' || session.status === 'ready'
  try {
    if (!finalizing) {
      const count = Math.ceil(file.size / session.part_size), loaded = new Map()
      const existing = new Map((session.parts || []).map(p => [p.number, p]))
      const report = () => progress(Math.min(99, Math.floor([...loaded.values()].reduce((a, b) => a + b, 0) / file.size * 100)), 'Загрузка напрямую в хранилище')
      const transfer = new AbortController(), abortTransfer = () => transfer.abort()
      signal.addEventListener('abort', abortTransfer, { once: true })
      if (signal.aborted) transfer.abort()
      let next = 1
      const workers = Array.from({ length: Math.min(3, count) }, async () => {
        while (next <= count) {
          const number = next++, start = (number - 1) * session.part_size, blob = file.slice(start, Math.min(file.size, start + session.part_size))
          if (existing.get(number)?.size === blob.size) { loaded.set(number, blob.size); report(); continue }
          for (let retry = 0; ; retry++) {
            try {
              const signed = await apiClient.post(`${base}/uploads/${id}/parts/${number}`, {}, { signal: transfer.signal })
              await putPart(signed, blob, transfer.signal, bytes => { loaded.set(number, bytes); report() }); loaded.set(number, blob.size); report(); break
            } catch (error) { if (transfer.signal.aborted || retry >= 3) throw error; loaded.set(number, 0); await pause(1000 * 2 ** retry, transfer.signal) }
          }
        }
      })
      try { await Promise.all(workers) } catch (error) { transfer.abort(); await Promise.allSettled(workers); throw error }
      finally { signal.removeEventListener('abort', abortTransfer) }
      await apiClient.post(`${base}/uploads/${id}/complete`, {}, options); finalizing = true
    }
    progress(100, 'Проверяем и сохраняем файл…')
    for (let poll = 0; poll < 1800; poll++) {
      session = await apiClient.get(`${base}/uploads/${id}`, options)
      if (session.status === 'finalizing') progress(100, `Проверяем целостность файла · ${Math.floor((session.verified_bytes || 0) / session.size * 100)}%`)
      if (session.status === 'ready' && session.asset) { try { sessionStorage.removeItem(cache) } catch {}; return session.asset }
      if (['failed', 'cancelled'].includes(session.status)) { try { sessionStorage.removeItem(cache) } catch {}; throw Error('Файл не прошёл проверку типа или размера. Загрузите его повторно.') }
      await pause(2000, signal)
    }
    throw Error('Проверка ещё выполняется. Выберите тот же файл позже для получения результата.')
  } catch (error) {
    if ([404, 410].includes(error.status)) { try { sessionStorage.removeItem(cache) } catch {} }
    if (signal.aborted && !finalizing) {
      try { await apiClient.delete(`${base}/uploads/${id}`); sessionStorage.removeItem(cache) } catch {}
      throw Error('Загрузка отменена')
    }
    throw Error(`${error.message}. Для продолжения повторно выберите этот же файл.`)
  }
}
