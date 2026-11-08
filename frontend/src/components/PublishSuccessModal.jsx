import { useEffect, useRef } from 'react'

export default function PublishSuccessModal({ eyebrow, title, description, wishTitle, wishText, primaryHref, primaryText, onClose, className = '', primaryOnClick, secondaryText = 'Остаться здесь', footerNote, containFocus = false }) {
  const closeButton = useRef(null)
  useEffect(() => {
    const previousFocus = document.activeElement, previousOverflow = document.body.style.overflow, previousRootOverflow = document.documentElement.style.overflow
    if (containFocus) { document.body.style.overflow = 'hidden'; document.documentElement.style.overflow = 'hidden' }
    closeButton.current?.focus()
    const onKey = event => {
      if (event.key === 'Escape') { event.preventDefault(); onClose(); return }
      if (containFocus && event.key === 'Tab') {
        const buttons = [...closeButton.current.closest('[role="dialog"]').querySelectorAll('button:not(:disabled),a[href]')]
        const first = buttons[0], last = buttons.at(-1)
        if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last?.focus() }
        else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first?.focus() }
      }
    }
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('keydown', onKey)
      if (containFocus) { document.body.style.overflow = previousOverflow; document.documentElement.style.overflow = previousRootOverflow; if (previousFocus?.isConnected) previousFocus.focus() }
    }
  }, [onClose, containFocus])
  return <div className={`publish-success-modal ${className}`} onMouseDown={event => event.target === event.currentTarget && onClose()}>
    <div className="publish-confetti" aria-hidden="true">{Array.from({ length: 18 }, (_, index) => <i key={index} style={{ '--i': index }} />)}</div>
    <section role="dialog" aria-modal="true" aria-labelledby="publish-success-title" aria-describedby="publish-success-description">
      <button ref={closeButton} type="button" className="publish-success-close" aria-label="Закрыть" onClick={onClose}>×</button>
      <div className="publish-success-mark" aria-hidden="true"><span>✓</span></div>
      <small>{eyebrow}</small><h2 id="publish-success-title">{title}</h2><p id="publish-success-description">{description}</p>
      <div className="publish-success-wish"><i>✦</i><span><b>{wishTitle}</b><small>{wishText}</small></span></div>
      <div className="publish-success-actions">{primaryOnClick ? <button type="button" className="publish-success-primary" onClick={primaryOnClick}>{primaryText}</button> : <a href={primaryHref} target="_self">{primaryText} <span>→</span></a>}{secondaryText && <button type="button" onClick={onClose}>{secondaryText}</button>}</div>
      {footerNote && <p className="publish-success-footer">{footerNote}</p>}
    </section>
  </div>
}
