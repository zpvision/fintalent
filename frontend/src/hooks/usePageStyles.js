import { useLayoutEffect } from 'react'

let activeStyleGeneration = 0

export default function usePageStyles(stylesheets) {
  const key = stylesheets.join('\u0000')

  useLayoutEffect(() => {
    const generation = ++activeStyleGeneration
    document.documentElement.classList.add('react-page-styles-loading')
    const sharedStylesStart = document.querySelector('link[href="/static/layout-safety.css"]')
    const entries = key.split('\u0000').filter(Boolean).map((href) => {
      const link = document.createElement('link')
      link.rel = 'stylesheet'
      link.dataset.reactPageStyle = 'true'
      link.dataset.reactPageStyleGeneration = String(generation)
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
      if (generation !== activeStyleGeneration) return
      document.querySelectorAll('link[data-react-page-style]').forEach((link) => {
        if (link.dataset.reactPageStyleGeneration !== String(generation)) link.remove()
      })
      document.documentElement.classList.remove('react-page-styles-loading')
    })

    return () => {
      // The next page removes this set only after its own styles are ready.
      queueMicrotask(() => {
        if (generation === activeStyleGeneration) {
          document.documentElement.classList.remove('react-page-styles-loading')
        }
      })
    }
  }, [key])
}
