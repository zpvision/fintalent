import { useLayoutEffect } from 'react'

export default function usePageStyles(stylesheets) {
  const key = stylesheets.join('\u0000')

  useLayoutEffect(() => {
    const sharedStylesStart = document.querySelector('link[href="/static/layout-safety.css"]')
    const links = key.split('\u0000').filter(Boolean).map((href) => {
      const link = document.createElement('link')
      link.rel = 'stylesheet'
      link.href = href
      link.dataset.reactPageStyle = 'true'
      document.head.insertBefore(link, sharedStylesStart)
      return link
    })
    return () => links.forEach((link) => link.remove())
  }, [key])
}
