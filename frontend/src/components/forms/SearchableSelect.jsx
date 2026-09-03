import { useEffect, useMemo, useRef, useState } from 'react'

export default function SearchableSelect({ label, name, options, value, onChange, placeholder = 'Выберите значение' }) {
  const rootRef = useRef(null)
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')
  const selected = options.find((option) => String(option.value) === String(value))
  const filtered = useMemo(() => {
    const normalized = query.trim().toLocaleLowerCase('ru')
    return options.filter((option) => !normalized || option.label.toLocaleLowerCase('ru').includes(normalized))
  }, [options, query])

  useEffect(() => {
    function closeOnOutsideClick(event) {
      if (!rootRef.current?.contains(event.target)) setOpen(false)
    }
    document.addEventListener('click', closeOnOutsideClick)
    return () => document.removeEventListener('click', closeOnOutsideClick)
  }, [])

  function choose(option) {
    onChange(String(option.value))
    setQuery('')
    setOpen(false)
  }

  return (
    <label>
      <small>{label}</small>
      <select className="searchable-native" name={name} value={value} onChange={(event) => onChange(event.target.value)} tabIndex="-1" aria-hidden="true">
        {options.map((option) => <option value={option.value} key={option.value || '_empty'}>{option.label}</option>)}
      </select>
      <span className="searchable-select" ref={rootRef}>
        <input
          type="text"
          autoComplete="off"
          role="combobox"
          aria-expanded={open}
          value={open ? query : (selected?.value ? selected.label : '')}
          placeholder={placeholder}
          onFocus={(event) => { setQuery(''); setOpen(true); event.currentTarget.select() }}
          onChange={(event) => { setQuery(event.target.value); setOpen(true) }}
          onKeyDown={(event) => {
            if (event.key === 'Escape') setOpen(false)
            if (event.key === 'Enter' && filtered[0]) {
              event.preventDefault()
              choose(filtered[0])
            }
          }}
        />
        <span className={`searchable-options${open ? ' open' : ''}`}>
          {filtered.length ? filtered.map((option) => (
            <button type="button" className={String(option.value) === String(value) ? 'active' : ''} onClick={() => choose(option)} key={option.value || '_empty'}>{option.label}</button>
          )) : <em>Ничего не найдено</em>}
        </span>
      </span>
    </label>
  )
}
