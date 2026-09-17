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
    const entries = key.split('\u0000').filter(Boolean).map((href) => {
      const link = document.createElement('link')
      link.rel = 'stylesheet'
      link.dataset.reactPageStyle = 'true'
      link.dataset.reactPageStyleOwner = String(owner)
      const ready = new Promise((resolve) => {
        let settled = false
        const finish = () => {
          if (settled) return
          settled = true
          resolve()
        }
        link.addEventListener('load', finish, { once: true })
        link.addEventListener('error', finish, { once: true })
        window.setTimeout(finish, 3000)
      })
      link.href = href
      document.head.insertBefore(link, sharedStylesStart)
      return { link, ready }
    })

    Promise.all(entries.map((entry) => entry.ready)).then(() => {
      pendingStyleOwners.delete(owner)
      if (activeStyleOwners.has(owner)) removeInactiveStyles()
      else entries.forEach((entry) => entry.link.remove())
      finishLoadingWhenReady()
    })

    return () => {
      activeStyleOwners.delete(owner)
      pendingStyleOwners.delete(owner)
      // Keep the previous page styled until the replacement styles have loaded.
      queueMicrotask(() => {
        if (!pendingStyleOwners.size) removeInactiveStyles()
        finishLoadingWhenReady()
      })
    }
  }, [key])
}
