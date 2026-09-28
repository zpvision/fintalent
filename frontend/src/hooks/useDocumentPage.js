import { useLayoutEffect } from 'react'

export function useDocumentPage({ title, description, bodyClass = '', bodyData = {}, referrerPolicy }) {
  useLayoutEffect(() => {
    const previousTitle = document.title
    const previousClassName = document.body.className
    const descriptionElement = document.querySelector('meta[name="description"]')
    const previousDescription = descriptionElement?.getAttribute('content')
    const previousData = {}
    const previousReferrerElement = document.querySelector('meta[name="referrer"]')
    const previousReferrer = previousReferrerElement?.getAttribute('content')
    let referrerElement
    if (referrerPolicy) {
      referrerElement = previousReferrerElement || document.createElement('meta')
      referrerElement.name = 'referrer'
      referrerElement.content = referrerPolicy
      if (!previousReferrerElement) document.head.appendChild(referrerElement)
    }

    document.title = title
    if (description && descriptionElement) descriptionElement.setAttribute('content', description)
    document.body.className = bodyClass
    for (const [key, value] of Object.entries(bodyData)) {
      previousData[key] = document.body.dataset[key]
      document.body.dataset[key] = value
    }

    return () => {
      document.title = previousTitle
      if (referrerElement) {
        if (!previousReferrerElement) referrerElement.remove()
        else if (previousReferrer == null) referrerElement.removeAttribute('content')
        else referrerElement.setAttribute('content', previousReferrer)
      }
      if (descriptionElement && previousDescription != null) descriptionElement.setAttribute('content', previousDescription)
      document.body.className = previousClassName
      for (const key of Object.keys(bodyData)) {
        if (previousData[key] == null) delete document.body.dataset[key]
        else document.body.dataset[key] = previousData[key]
      }
    }
  }, [title, description, bodyClass, JSON.stringify(bodyData), referrerPolicy])
}
