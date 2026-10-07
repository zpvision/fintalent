import { useEffect, useId, useRef, useState } from 'react'
import { apiClient } from '../../api/client'

export default function CityPicker({ value, cityId, onChange }) {
  const rootRef = useRef(null)
  const requestRef = useRef(null)
  const timerRef = useRef(null)
  const listID = useId()
  const [activeIndex, setActiveIndex] = useState(-1)
  const [cities, setCities] = useState([])
  const [open, setOpen] = useState(false)
  const [status, setStatus] = useState('idle')

  function close() {
    window.clearTimeout(timerRef.current)
    requestRef.current?.abort()
    setOpen(false)
    setActiveIndex(-1)
  }

  function selectCity(city) {
    close()
    onChange(city.name, String(city.id))
  }

  function handleKeyDown(event) {
    if (event.key === 'Escape') { event.preventDefault(); close(); return }
    if (event.key === 'Tab') { close(); return }
    if (!open || status !== 'ready' || !cities.length) return
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault()
      const next = event.key === 'ArrowDown' ? (activeIndex + 1) % cities.length : (activeIndex <= 0 ? cities.length - 1 : activeIndex - 1)
      setActiveIndex(next)
      document.getElementById(`${listID}-${next}`)?.scrollIntoView({ block: 'nearest' })
    } else if (event.key === 'Enter' && activeIndex >= 0) {
      event.preventDefault()
      selectCity(cities[activeIndex])
    }
  }

  useEffect(() => {
    function closeOnOutsideClick(event) {
      if (!rootRef.current?.contains(event.target)) close()
    }
    document.addEventListener('click', closeOnOutsideClick)
    return () => {
      document.removeEventListener('click', closeOnOutsideClick)
      window.clearTimeout(timerRef.current)
      requestRef.current?.abort()
    }
  }, [])

  async function findCities(query) {
    requestRef.current?.abort()
    const controller = new AbortController()
    requestRef.current = controller
    setStatus('loading')
    try {
      const result = await apiClient.get(`/api/public/cities?country=RU&q=${encodeURIComponent(query.trim())}`, { signal: controller.signal, redirectOnUnauthorized: false })
	  if (controller.signal.aborted || requestRef.current !== controller) return
      setCities(Array.isArray(result) ? result : [])
      setActiveIndex(-1)
      setStatus('ready')
      setOpen(true)
    } catch (error) {
      if (!controller.signal.aborted && requestRef.current === controller && error.name !== 'AbortError') {
        setStatus('error')
        setOpen(true)
      }
    }
  }

  function handleInput(event) {
    const nextValue = event.target.value
    setActiveIndex(-1)
    onChange(nextValue, '')
    window.clearTimeout(timerRef.current)
    requestRef.current?.abort()
    timerRef.current = window.setTimeout(() => findCities(nextValue), 220)
  }

  return (
    <label className="city-picker" ref={rootRef}>
      <small>Город</small>
      <input value={value} autoComplete="off" placeholder="Любой город" role="combobox" aria-autocomplete="list" aria-expanded={open} aria-controls={listID} aria-activedescendant={open && activeIndex >= 0 ? `${listID}-${activeIndex}` : undefined} onKeyDown={handleKeyDown} onBlur={event => { if (!rootRef.current?.contains(event.relatedTarget)) close() }} onFocus={() => findCities(value)} onChange={handleInput} />
      <input name="city" type="hidden" value={cityId} readOnly />
      <span id={listID} role="listbox" aria-label="Города" className={`city-suggestions${open ? ' open' : ''}`}>
        {status === 'error' ? <em>Не удалось загрузить города</em> : null}
        {status === 'ready' && !cities.length ? <em>Города не найдены</em> : null}
        {status === 'ready' ? cities.map((city, index) => (
          <button id={`${listID}-${index}`} role="option" aria-selected={activeIndex === index} tabIndex={-1} type="button" onMouseDown={event => event.preventDefault()} onClick={() => selectCity(city)} key={city.id}>
            <b>{city.name}</b>{city.region ? <small>{city.region}</small> : null}
          </button>
        )) : null}
      </span>
    </label>
  )
}
