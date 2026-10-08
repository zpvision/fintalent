import { useEffect, useId, useRef, useState } from 'react'
import Icon from './Icon'

export default function IconSelect({ value, options, onChange, label = 'Выбрать иконку' }) {
  const [open, setOpen] = useState(false), root = useRef(null), trigger = useRef(null), id = useId()
  useEffect(() => {
    const close = event => { if (!root.current?.contains(event.target)) setOpen(false) }
    document.addEventListener('pointerdown', close)
    return () => document.removeEventListener('pointerdown', close)
  }, [])
  useEffect(() => {
    if (open) root.current?.querySelector('[aria-selected="true"]')?.focus()
  }, [open])
  function keyboard(event) {
    if (event.key === 'Escape') { event.preventDefault(); setOpen(false); trigger.current?.focus() }
    const buttons = [...root.current.querySelectorAll('[role="option"]')]
    const index = buttons.indexOf(document.activeElement)
    const delta = { ArrowRight: 1, ArrowLeft: -1, ArrowDown: 5, ArrowUp: -5 }[event.key]
    if (index >= 0 && delta != null) { event.preventDefault(); buttons[(index + delta + buttons.length) % buttons.length]?.focus() }
    if (event.key === 'Home' || event.key === 'End') { event.preventDefault(); buttons[event.key === 'Home' ? 0 : buttons.length - 1]?.focus() }
  }
  return <div className="course-icon-select" ref={root} onKeyDown={keyboard} onBlur={event => { if (!event.currentTarget.contains(event.relatedTarget)) setOpen(false) }}>
    <button type="button" className="course-icon-current" ref={trigger} aria-label={`${label}: ${options[value] || value}`} aria-haspopup="listbox" aria-expanded={open} aria-controls={id} onClick={() => setOpen(v => !v)}><Icon name={value} /><span>Выбрать</span><b aria-hidden="true">⌄</b></button>
    {open && <div className="course-icon-options" id={id} role="listbox" aria-label={label}>{Object.entries(options).map(([key, name]) => <button type="button" role="option" key={key} aria-label={name} title={name} aria-selected={value === key} onClick={() => { onChange(key); setOpen(false); trigger.current?.focus() }}><Icon name={key} /></button>)}</div>}
  </div>
}
