import { useCallback, useEffect, useRef, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
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
    createHref: '/profiles/create',
    createLabel: 'Разместить профиль',
    queryPlaceholder: 'Должность, навык или ключевое слово',
    incomeLabel: 'зарплата от',
  },
  resumes: {
    title: 'Профиль — FinTalent',
    eyebrow: 'БАЗА СПЕЦИАЛИСТОВ',
    heading: 'Профили финансовых специалистов',
    description: 'Ищите специалистов по опыту, навыкам и направлениям экспертизы.',
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

function CatalogCard({ item, type, incomeLabel, triggerRef }) {
  const href = type === 'resumes' ? `/profiles/view/${item.id}` : `/vacancies/view?id=${item.id}`
  const professional = type === 'resumes' && item.profile_mode === 'professional'
  return (
    <a className="catalog-card" href={href} ref={triggerRef}>
      <CatalogAvatar item={item} type={type} />
      <div>
        <h2>{item.title}</h2>
        <span className="company">{item.name} · {item.city || 'Россия'}</span>
        {!professional && item.description ? <p>{item.description}</p> : null}
        <div className="catalog-tags">{(item.tags || []).map((tag, index) => <span key={`${tag}-${index}`}>{tag}</span>)}</div>
      </div>
      <div className={`catalog-side${professional ? ' professional' : ''}`}>{professional ? <><strong>Профиль</strong><small>опыт и компетенции</small></> : <><strong>{formatMoney(item.salary)} ₽</strong><small>{incomeLabel}</small></>}</div>
    </a>
  )
}

export default function CatalogPage({ type }) {
  const copy = catalogCopy[type]
  const [searchParams, setSearchParams] = useSearchParams()
  const [query, setQuery] = useState('')
  const [city, setCity] = useState('')
  const [helpTopic, setHelpTopic] = useState(type === 'resumes' ? searchParams.get('help_topic') || '' : '')
  const [helpTopics, setHelpTopics] = useState([])
  const [items, setItems] = useState([])
  const [total, setTotal] = useState(0)
  const [status, setStatus] = useState('loading')
  const [loadingMore, setLoadingMore] = useState(false)
  const [hasMore, setHasMore] = useState(false)
  const firstRequest = useRef(true)
  const activeRequest = useRef(null)
  const loadTriggerRef = useRef(null)
  usePageStyles(['/static/catalog.css?v=5', '/static/catalog-help.css?v=1'])
  useDocumentPage({ title: copy.title, bodyData: { catalog: type } })

  const loadCatalog = useCallback(async (signal, offset = 0, append = false) => {
    if (append) setLoadingMore(true)
    else setStatus('loading')
    const params = new URLSearchParams({ kind: type, q: query.trim(), city: city.trim(), limit: '30', offset: String(offset) })
    if (type === 'resumes' && helpTopic) params.set('help_topic', helpTopic)
    try {
      const data = await apiClient.get(`/api/public/catalog?${params}`, { cache: 'no-store', signal, redirectOnUnauthorized: false })
      const nextItems = Array.isArray(data?.items) ? data.items : []
      setItems((current) => append ? [...current, ...nextItems] : nextItems)
      setTotal(Number(data?.total) || 0)
      setHasMore(Boolean(data?.has_more))
      setStatus('ready')
    } catch (error) {
      if (error.name !== 'AbortError' && !append) setStatus('error')
    } finally {
      if (append && !signal.aborted) setLoadingMore(false)
    }
  }, [city, helpTopic, query, type])

  const loadMore = useCallback(() => {
    if (loadingMore || !hasMore || status !== 'ready') return
    const controller = new AbortController()
    activeRequest.current = controller
    loadCatalog(controller.signal, items.length, true)
  }, [hasMore, items.length, loadCatalog, loadingMore, status])

  useEffect(() => {
    if (type !== 'resumes') return
    const controller = new AbortController()
    apiClient.get('/api/public/help-topics', { signal: controller.signal, redirectOnUnauthorized: false }).then((data) => setHelpTopics(Array.isArray(data) ? data : [])).catch(() => {})
    return () => controller.abort()
  }, [type])

  function selectHelpTopic(value) {
    setHelpTopic(value)
    setSearchParams((current) => {
      const next = new URLSearchParams(current)
      if (value) next.set('help_topic', value)
      else next.delete('help_topic')
      return next
    }, { replace: true })
  }

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

  useEffect(() => {
    const target = loadTriggerRef.current
    if (!target || !hasMore || loadingMore || status !== 'ready') return undefined
    const observer = new IntersectionObserver((entries) => {
      if (entries.some((entry) => entry.isIntersecting)) loadMore()
    }, { rootMargin: '0px 0px 240px' })
    observer.observe(target)
    return () => observer.disconnect()
  }, [hasMore, items.length, loadMore, loadingMore, status])

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
          <form className={`catalog-search${type === 'resumes' ? ' catalog-search-resumes' : ''}`} onSubmit={handleSubmit}>
            <label>⌕<input name="q" value={query} onChange={(event) => setQuery(event.target.value)} placeholder={copy.queryPlaceholder} /></label>
            <label>⌖<input name="city" value={city} onChange={(event) => setCity(event.target.value)} placeholder="Город" /></label>
            {type === 'resumes' ? <label className="catalog-help-filter"><span>Найти специалиста по профилю</span><select value={helpTopic} onChange={(event) => selectHelpTopic(event.target.value)}><option value="">Любое направление</option>{helpTopics.map((topic) => <option value={topic.id} key={topic.id}>{topic.name}</option>)}</select></label> : null}
            <button>Найти</button>
          </form>
          <p className="catalog-meta">{status === 'ready' ? `Найдено: ${total}` : ''}</p>
          <section className="catalog-list">
            {status === 'loading' ? <div className="catalog-empty">Загружаем предложения…</div> : null}
            {status === 'error' ? <div className="catalog-empty">Не удалось загрузить каталог</div> : null}
            {status === 'ready' && !items.length ? <div className="catalog-empty">По вашему запросу ничего не найдено</div> : null}
            {status === 'ready' ? items.map((item, index) => <CatalogCard item={item} type={type} incomeLabel={copy.incomeLabel} triggerRef={hasMore && index === Math.max(0, items.length - 10) ? loadTriggerRef : undefined} key={item.id} />) : null}
            {loadingMore ? <div className="catalog-loading-more">Загружаем ещё…</div> : null}
          </section>
        </div>
      </main>
    </PublicLayout>
  )
}
