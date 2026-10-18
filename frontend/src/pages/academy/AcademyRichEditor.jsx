import { useEffect, useRef, useState } from 'react'
let bundle
function loadEditor() {
  if (window.FinTalentEditorJS?.supportsAcademy) return Promise.resolve()
  bundle ||= new Promise((resolve, reject) => { const script = document.createElement('script'); script.src = '/static/vendor/publication-editor-editorjs.js?v=3'; script.onload = resolve; script.onerror = () => { bundle = null; reject(Error('Не удалось загрузить текстовый редактор')) }; document.head.append(script) })
  return bundle
}
export default function AcademyRichEditor({ block, onChange, register }) {
  const holder = useRef(null), callback = useRef(onChange), [error, setError] = useState('')
  callback.current = onChange
  useEffect(() => {
    let active = true, mounted, sequence = 0, last = JSON.stringify(block.data.rich || [])
    const save = async () => {
      if (!mounted || !active) return
      const seq = ++sequence, data = await mounted.save()
      if (!active || seq !== sequence) return
      const next = JSON.stringify(data)
      if (next !== last) { last = next; callback.current(data) }
    }
    loadEditor().then(async () => {
      if (!active) return
      mounted = await window.FinTalentEditorJS.mount({ holder: holder.current, data: block.data.rich || [], academy: true, onChange: () => save().catch(e => setError(e.message)) })
      if (!active) { mounted.editor.destroy(); return }
      register(block.key, save)
    }).catch(e => { if (active) setError(e.message) })
    return () => { active = false; register(block.key, null); mounted?.editor.destroy?.() }
  }, [block.key, register])
  return <div className="academy-rich">{error && <p role="alert" className="course-error">{error}</p>}<div ref={holder} /></div>
}
