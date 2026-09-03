import { useCallback, useEffect, useRef, useState } from 'react'
import { apiClient } from '../../api/client'
import PublicLayout from '../../layouts/PublicLayout'
import { useDocumentPage } from '../../hooks/useDocumentPage'
import usePageStyles from '../../hooks/usePageStyles'

const catalogCopy = {
  vacancies: {
    title: 'Вакансии — FinTalent',
    eyebrow: 'КАТАЛОГ FINTALENT',
    heading: 'Вакансии в финансах и учёте',
    description: 'Найдите работу, которая соответствует вашим навыкам и ожиданиям.',
    createHref: '/resume/create',
    createLabel: 'Разместить резюме',
    queryPlaceholder: 'Должность, навык или ключевое слово',
    incomeLabel: 'зарплата от',
  },
  resumes: {
    title: 'Резюме — FinTalent',
    eyebrow: 'БАЗА СПЕЦИАЛИСТОВ',
    heading: 'Резюме финансовых специалистов',
    description: 'Ищите кандидатов по должности, навыкам и предпочтительному городу.',
    createHref: '/vacancies/create',
    createLabel: 'Разместить вакансию',
    queryPlaceholder: 'Должность, имя или профессиональный навык',
    incomeLabel: 'желаемый доход',
  },
}

function formatMoney(value) {
  return new Intl.NumberFormat('ru-RU', { maximumFractionDigits: 0 }).format(value || 0)
}

function CatalogAvatar({ item, type }) {
  const [imageFailed, setImageFailed] = useState(false)
  const initials = (item.name || '').split(' ').filter(Boolean).map((part) => part[0]).slice(0, 2).join('').toUpperCase()
  return (
    <div className="catalog-card-icon">
      <span>{initials}</span>
      {type === 'resumes' && item.avatar && !imageFailed ? <img src={item.avatar} alt={`Фото ${item.name}`} onError={() => setImageFailed(true)} /> : null}
    </div>
  )
}

function CatalogCard({ item, type, incomeLabel }) {
  const href = type === 'resumes' ? `/resume/view/${item.id}` : `/vacancies/view?id=${item.id}`
  return (
    <a className="catalog-card" href={href}>
      <CatalogAvatar item={item} type={type} />
      <div>
        <h2>{item.title}</h2>
        <span className="company">{item.name} · {item.city || 'Россия'}</span>
        {item.description ? <p>{item.description}</p> : null}
        <div className="catalog-tags">{(item.tags || []).map((tag, index) => <span key={`${tag}-${index}`}>{tag}</span>)}</div>
      </div>
      <div className="catalog-side"><strong>{formatMoney(item.salary)} ₽</strong><small>{incomeLabel}</small></div>
    </a>
  )
}

export default function CatalogPage({ type }) {
  const copy = catalogCopy[type]
  const [query, setQuery] = useState('')
  const [city, setCity] = useState('')
  const [items, setItems] = useState([])
  const [total, setTotal] = useState(0)
  const [status, setStatus] = useState('loading')
  const firstRequest = useRef(true)
  const activeRequest = useRef(null)
  usePageStyles(['/static/catalog.css'])
  useDocumentPage({ title: copy.title, bodyData: { catalog: type } })

  const loadCatalog = useCallback(async (signal) => {
    setStatus('loading')
    const params = new URLSearchParams({ kind: type, q: query.trim(), city: city.trim() })
    try {
      const data = await apiClient.get(`/api/public/catalog?${params}`, { cache: 'no-store', signal, redirectOnUnauthorized: false })
      setItems(Array.isArray(data?.items) ? data.items : [])
      setTotal(Number(data?.total) || 0)
      setStatus('ready')
    } catch (error) {
      if (error.name !== 'AbortError') setStatus('error')
    }
  }, [city, query, type])

  useEffect(() => {
    activeRequest.current?.abort()
    const controller = new AbortController()
    activeRequest.current = controller
    const delay = firstRequest.current ? 0 : 300
    firstRequest.current = false
    const timer = window.setTimeout(() => loadCatalog(controller.signal), delay)
    return () => {
      window.clearTimeout(timer)
      controller.abort()
    }
  }, [loadCatalog])

  function handleSubmit(event) {
    event.preventDefault()
    activeRequest.current?.abort()
    const controller = new AbortController()
    activeRequest.current = controller
    loadCatalog(controller.signal)
  }

  return (
    <PublicLayout>
      <main className="catalog-page">
        <div className="catalog-wrap">
          <section className="catalog-hero">
            <div><small>{copy.eyebrow}</small><h1>{copy.heading}</h1><p>{copy.description}</p></div>
            <a className="catalog-create" href={copy.createHref}>{copy.createLabel}</a>
          </section>
          <form className="catalog-search" onSubmit={handleSubmit}>
            <label>⌕<input name="q" value={query} onChange={(event) => setQuery(event.target.value)} placeholder={copy.queryPlaceholder} /></label>
            <label>⌖<input name="city" value={city} onChange={(event) => setCity(event.target.value)} placeholder="Город" /></label>
            <button>Найти</button>
          </form>
          <p className="catalog-meta">{status === 'ready' ? `Найдено: ${total}` : ''}</p>
          <section className="catalog-list">
            {status === 'loading' ? <div className="catalog-empty">Загружаем предложения…</div> : null}
            {status === 'error' ? <div className="catalog-empty">Не удалось загрузить каталог</div> : null}
            {status === 'ready' && !items.length ? <div className="catalog-empty">По вашему запросу ничего не найдено</div> : null}
            {status === 'ready' ? items.map((item) => <CatalogCard item={item} type={type} incomeLabel={copy.incomeLabel} key={item.id} />) : null}
          </section>
        </div>
      </main>
    </PublicLayout>
  )
}
