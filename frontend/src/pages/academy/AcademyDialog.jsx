import { useEffect, useRef } from 'react'

// Native modal provides focus trapping, Escape and focus restoration for drawers and preview.
export default function AcademyDialog({ title, children, close, wide = false, drawer = false }) {
  const ref = useRef(null), callback = useRef(close)
  callback.current = close
  useEffect(() => { const dialog = ref.current; dialog.showModal(); return () => dialog.close() }, [])
  return <dialog ref={ref} className={`academy-dialog${wide ? ' wide' : ''}${drawer ? ' drawer' : ''}`} onCancel={e => { e.preventDefault(); callback.current() }} onClick={e => { if (e.target === e.currentTarget) callback.current() }} aria-label={title}><div className="academy-dialog-inner"><header><h2>{title}</h2><button type="button" className="academy-icon-button" onClick={close} aria-label="Закрыть">×</button></header>{children}</div></dialog>
}
