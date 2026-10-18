import { useCallback, useEffect, useRef, useState } from 'react'
import { academyAPI } from '../../api/academy'

// Only server IDs/revision are merged into later edits; late responses never replace their content.
export function mergeServerIDs(local, server) {
  const copy = structuredClone(local), entities = new Map()
  server.modules.forEach(m => { entities.set(m.key, m.id); m.lessons.forEach(l => { entities.set(l.key, l.id); l.blocks.forEach(b => entities.set(b.key, b.id)) }) })
  copy.modules.forEach(m => { m.id = entities.get(m.key) || m.id; m.lessons.forEach(l => { l.id = entities.get(l.key) || l.id; l.blocks.forEach(b => { b.id = entities.get(b.key) || b.id }) }) })
  copy.revision = server.revision
  return copy
}
export default function useAcademyDraft(id) {
  const [program, setProgram] = useState(null), [state, setState] = useState('loading'), [error, setError] = useState('')
  const current = useRef(null), sequence = useRef(0), saved = useRef(0), flight = useRef(null), timer = useRef(null), storage = useRef(''), rich = useRef(new Map()), alive = useRef(true)
  useEffect(() => { alive.current = true; return () => { alive.current = false; clearTimeout(timer.current) } }, [])
  function backup() { if (storage.current) try { sessionStorage.setItem(storage.current, JSON.stringify(current.current)) } catch {} }
  function accept(p) { current.current = p; if (alive.current) setProgram(p) }
  function load(p, owner) {
    storage.current = `fintalent.academy.${owner}.${id}`
    sequence.current = 0; saved.current = 0; setError('')
    if (p) { try { const recovered = JSON.parse(sessionStorage.getItem(storage.current) || 'null'); if (recovered && recovered.id === p.id && confirm('Восстановить несохранённые изменения учебной программы из этой вкладки?')) { accept(recovered); sequence.current = 1; setState('error'); setError(recovered.revision !== p.revision ? 'На сервере другая ревизия. Скачайте свои правки, затем загрузите серверную версию.' : 'Изменения восстановлены. Нажмите «Сохранить».'); return } } catch {} }
    accept(p); setState('saved')
  }
  function change(fn) { if (!current.current) return; accept(fn(structuredClone(current.current))); sequence.current++; backup(); setState('dirty'); setError(''); clearTimeout(timer.current); timer.current = setTimeout(() => flush().catch(() => {}), 900) }
  async function flush() {
    clearTimeout(timer.current)
    await Promise.all([...rich.current.values()].map(save => save()))
    if (flight.current) return flight.current
    if (saved.current >= sequence.current) return current.current
    flight.current = (async () => {
      try {
        while (saved.current < sequence.current) {
          const seq = sequence.current, snapshot = structuredClone(current.current)
          if (alive.current) setState('saving')
          const result = await academyAPI.save(id, snapshot)
          accept(seq === sequence.current ? result : mergeServerIDs(current.current, result))
          saved.current = seq
          if (seq === sequence.current) { try { sessionStorage.removeItem(storage.current) } catch {} } else backup()
        }
        if (alive.current) { setState('saved'); setError('') }
        return current.current
      } catch (e) { backup(); if (alive.current) { setState('error'); setError(e.message) }; throw e }
      finally { flight.current = null }
    })()
    return flight.current
  }
  async function action(data) {
    await flush()
    // The UI locks structural commands until their result has been applied.
    setState('saving')
    try { const p = await academyAPI.action(id, { ...data, revision: current.current.revision }); accept(p); setState('saved'); return p }
    catch (e) { setError(e.message); setState('error'); throw e }
  }
  useEffect(() => {
    const unload = e => { if (saved.current < sequence.current || flight.current) { backup(); e.preventDefault(); e.returnValue = '' } }
    const leave = e => { const link = e.target.closest?.('a[href]'); if (link && !link.target && saved.current < sequence.current && !confirm('Есть несохранённые изменения. Покинуть Академию?')) { e.preventDefault(); e.stopPropagation() } }
    window.addEventListener('beforeunload', unload); document.addEventListener('click', leave, true)
    return () => { window.removeEventListener('beforeunload', unload); document.removeEventListener('click', leave, true) }
  }, [])
  const registerRich = useCallback((key, fn) => { if (fn) rich.current.set(key, fn); else rich.current.delete(key) }, [])
  function discardBackup() { try { sessionStorage.removeItem(storage.current) } catch {} }
  return { program, state, error, load, change, flush, action, registerRich, current, discardBackup }
}
