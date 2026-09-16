import { useLayoutEffect } from 'react'

let activeStyleGeneration = 0

export default function usePageStyles(stylesheets) {
  const key = stylesheets.join('\u0000')

  useLayoutEffect(() => {
    const generation = ++activeStyleGeneration
    const sharedStylesStart = document.querySelector('link[href="/static/layout-safety.css"]')
    const links = key.split('\u0000').filter(Boolean).map((href) => {
      const link = document.createElement('link')
      link.rel = 'stylesheet'
      link.href = href
      link.dataset.reactPageStyle = 'true'
      link.dataset.reactPageStyleGeneration = String(generation)
      document.head.insertBefore(link, sharedStylesStart)
      return link
    })

    const ready = links.map((link) => new Promise((resolve) => {
      if (link.sheet) {
        resolve()
        return
      }
      link.addEventListener('load', resolve, { once: true })
      link.addEventListener('error', resolve, { once: true })
    }))

    Promise.all(ready).then(() => {
      if (generation !== activeStyleGeneration) return
      document.querySelectorAll('link[data-react-page-style]').forEach((link) => {
        if (link.dataset.reactPageStyleGeneration !== String(generation)) link.remove()
      })
    })

    return () => {
      // The next page removes this set only after its own styles are ready.
    }
  }, [key])
}
