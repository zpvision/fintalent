import { useEffect, useRef, useState } from 'react'
import { apiClient } from '../../api/client'

export default function CityPicker({ value, cityId, onChange }) {
  const rootRef = useRef(null)
  const requestRef = useRef(null)
  const timerRef = useRef(null)
  const [cities, setCities] = useState([])
  const [open, setOpen] = useState(false)
  const [status, setStatus] = useState('idle')

  useEffect(() => {
    function closeOnOutsideClick(event) {
      if (!rootRef.current?.contains(event.target)) setOpen(false)
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
      setCities(Array.isArray(result) ? result : [])
      setStatus('ready')
      setOpen(true)
    } catch (error) {
      if (error.name !== 'AbortError') {
        setStatus('error')
        setOpen(true)
      }
    }
  }

  function handleInput(event) {
    const nextValue = event.target.value
    onChange(nextValue, '')
    window.clearTimeout(timerRef.current)
    timerRef.current = window.setTimeout(() => findCities(nextValue), 220)
  }

  return (
    <label className="city-picker" ref={rootRef}>
      <small>Город</small>
      <input value={value} autoComplete="off" placeholder="Любой город" onFocus={() => findCities(value)} onChange={handleInput} />
      <input name="city" type="hidden" value={cityId} readOnly />
      <span className={`city-suggestions${open ? ' open' : ''}`}>
        {status === 'error' ? <em>Не удалось загрузить города</em> : null}
        {status === 'ready' && !cities.length ? <em>Города не найдены</em> : null}
        {status === 'ready' ? cities.map((city) => (
          <button type="button" onClick={() => { onChange(city.name, String(city.id)); setOpen(false) }} key={city.id}>
            <b>{city.name}</b>{city.region ? <small>{city.region}</small> : null}
          </button>
        )) : null}
      </span>
    </label>
  )
}
