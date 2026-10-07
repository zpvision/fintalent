import { useLayoutEffect } from 'react'

let nextStyleOwner = 0
const activeStyleOwners = new Set()
const pendingStyleOwners = new Set()

function removeInactiveStyles() {
  document.querySelectorAll('link[data-react-page-style]').forEach((link) => {
    if (!activeStyleOwners.has(Number(link.dataset.reactPageStyleOwner))) link.remove()
  })
}

function finishLoadingWhenReady() {
  if (!pendingStyleOwners.size) document.documentElement.classList.remove('react-page-styles-loading')
}

export default function usePageStyles(stylesheets) {
  const key = stylesheets.join('\u0000')

  useLayoutEffect(() => {
    const owner = ++nextStyleOwner
    activeStyleOwners.add(owner)
    pendingStyleOwners.add(owner)
    document.documentElement.classList.add('react-page-styles-loading')
    const sharedStylesStart = document.querySelector('link[href="/static/layout-safety.css"]')
    let notice
    const clearNotice = () => { notice?.remove(); notice = undefined }
    const showFailure = () => {
      if (notice || !activeStyleOwners.has(owner)) return
      notice = document.createElement('div')
      notice.setAttribute('role', 'alert')
      notice.dataset.pageStylesError = String(owner)
      // Outside #root: the page stays hidden until its styles really load.
      Object.assign(notice.style, { position: 'fixed', top: '90px', left: '16px', right: '16px', zIndex: '10000', padding: '20px', background: '#fff', color: '#20243c', border: '1px solid #ddd', borderRadius: '12px', textAlign: 'center' })
      const message = document.createElement('p')
      message.textContent = 'Не удалось загрузить оформление страницы. Проверьте соединение и повторите.'
      const retry = document.createElement('button')
      retry.type = 'button'; retry.textContent = 'Повторить загрузку'
      retry.addEventListener('click', () => { clearNotice(); entries.forEach(entry => entry.retry()) })
      notice.append(message, retry)
      document.body.append(notice)
    }
    const entries = key.split('\u0000').filter(Boolean).map((href) => {
      let link, timer, retryTimer, detach, resolveReady, loaded = false, attempts = 0, cancelled = false
      const ready = new Promise(resolve => { resolveReady = resolve })
      const stop = () => { clearTimeout(timer); clearTimeout(retryTimer); detach?.() }
      const start = () => {
        if (loaded || cancelled || !activeStyleOwners.has(owner)) return
        stop(); link?.remove(); attempts++
        link = document.createElement('link')
        link.rel = 'stylesheet'
        link.dataset.reactPageStyle = 'true'
        link.dataset.reactPageStyleOwner = String(owner)
        const success = () => {
          if (cancelled || loaded) return
          stop(); loaded = true; resolveReady(true)
        }
        const failure = () => {
          stop()
          if (cancelled) return
          showFailure()
          if (attempts < 3) retryTimer = window.setTimeout(start, 500)
        }
        link.addEventListener('load', success, { once: true })
        link.addEventListener('error', failure, { once: true })
        detach = () => { link.removeEventListener('load', success); link.removeEventListener('error', failure) }
        timer = window.setTimeout(failure, 10000)
        const url = new URL(href, location.href)
        if (attempts > 1) url.searchParams.set('_style_retry', `${owner}-${attempts}`)
        link.href = url.href
        document.head.insertBefore(link, sharedStylesStart)
      }
      start()
      return { ready, retry: start, cancel: () => { cancelled = true; stop(); resolveReady(false) } }
    })

    Promise.all(entries.map((entry) => entry.ready)).then((loaded) => {
      if (!activeStyleOwners.has(owner) || loaded.some(value => !value)) return
      clearNotice()
      pendingStyleOwners.delete(owner)
      removeInactiveStyles()
      finishLoadingWhenReady()
    })

    return () => {
      activeStyleOwners.delete(owner)
      pendingStyleOwners.delete(owner)
      clearNotice()
      entries.forEach(entry => entry.cancel())
      // Keep the previous page styled until the replacement styles have loaded.
      queueMicrotask(() => {
        if (!pendingStyleOwners.size) removeInactiveStyles()
        finishLoadingWhenReady()
      })
    }
  }, [key])
}
