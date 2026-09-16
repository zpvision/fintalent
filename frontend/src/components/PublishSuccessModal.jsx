import { useEffect, useRef } from 'react'

export default function PublishSuccessModal({ eyebrow, title, description, wishTitle, wishText, primaryHref, primaryText, onClose }) {
  const closeButton = useRef(null)
  useEffect(() => {
    closeButton.current?.focus()
    const onKey = event => event.key === 'Escape' && onClose()
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [onClose])
  return <div className="publish-success-modal" onMouseDown={event => event.target === event.currentTarget && onClose()}>
    <div className="publish-confetti" aria-hidden="true">{Array.from({ length: 18 }, (_, index) => <i key={index} style={{ '--i': index }} />)}</div>
    <section role="dialog" aria-modal="true" aria-labelledby="publish-success-title">
      <button ref={closeButton} type="button" className="publish-success-close" aria-label="Закрыть" onClick={onClose}>×</button>
      <div className="publish-success-mark" aria-hidden="true"><span>✓</span></div>
      <small>{eyebrow}</small><h2 id="publish-success-title">{title}</h2><p>{description}</p>
      <div className="publish-success-wish"><i>✦</i><span><b>{wishTitle}</b><small>{wishText}</small></span></div>
      <div className="publish-success-actions"><a href={primaryHref} target="_self">{primaryText} <span>→</span></a><button type="button" onClick={onClose}>Остаться здесь</button></div>
    </section>
  </div>
}
