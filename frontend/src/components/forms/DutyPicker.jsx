import { useEffect, useRef, useState } from 'react'

let dutyPickerPromise

function loadDutyPicker() {
  if (window.DutyPicker) return Promise.resolve(window.DutyPicker)
  if (!dutyPickerPromise) {
    dutyPickerPromise = new Promise((resolve, reject) => {
      const script = document.createElement('script')
      script.src = '/static/duty-picker.js?v=4'
      script.dataset.reactDutyPicker = 'true'
      script.onload = () => resolve(window.DutyPicker)
      script.onerror = () => reject(new Error('Не удалось загрузить выбор обязанностей'))
      document.head.append(script)
    })
  }
  return dutyPickerPromise
}

export default function DutyPicker({ categories, selected, onChange, title, subtitle }) {
  const container = useRef(null)
  const changeHandler = useRef(onChange)
  const [error, setError] = useState('')
  changeHandler.current = onChange

  useEffect(() => {
    let cancelled = false
    loadDutyPicker().then(picker => {
      if (cancelled || !container.current) return
      picker.mount(container.current, {
        categories,
        selected: new Set(selected || []),
        title,
        subtitle,
        onChange: ids => changeHandler.current?.(ids),
      })
    }).catch(loadError => { if (!cancelled) setError(loadError.message) })
    return () => { cancelled = true; if (container.current) container.current.replaceChildren() }
  }, [categories, title, subtitle])

  return <div ref={container}>{error && <p className="duty-empty">{error}</p>}</div>
}
