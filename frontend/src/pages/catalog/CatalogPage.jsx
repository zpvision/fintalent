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

function formatExperience(months) {
  const value = Number(months) || 0
  if (!value) return 'Без опыта'
  if (value < 12) return `${value} мес.`
  const years = Math.floor(value / 12)
  const tail = value % 12
  return tail ? `${years} г. ${tail} мес.` : `${years} ${years % 10 === 1 && years % 100 !== 11 ? 'год' : years % 10 >= 2 && years % 10 <= 4 && (years % 100 < 10 || years % 100 >= 20) ? 'года' : 'лет'}`
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
  if (type === 'vacancies') {
    return <a className="catalog-card" href={href} ref={triggerRef}>
      <CatalogAvatar item={item} type={type} />
      <div><h2>{item.title}</h2><span className="company">{item.name} · {item.city || 'Россия'}</span>{item.description ? <p>{item.description}</p> : null}<div className="catalog-tags">{(item.tags || []).map((tag, index) => <span key={`${tag}-${index}`}>{tag}</span>)}</div></div>
      <div className="catalog-side"><strong>{formatMoney(item.salary)} ₽</strong><small>{incomeLabel}</small></div>
    </a>
  }
  return (
    <article className={`catalog-card${type === 'resumes' ? ' profile-catalog-card' : ''}`} ref={triggerRef}>
      <CatalogAvatar item={item} type={type} />
      <div className="catalog-card-content">
        <h2>{item.title}</h2>
        <span className="company">{item.name} · {item.city || 'Россия'}</span>
        {type === 'resumes' ? <div className="profile-card-facts"><span>◷ {formatExperience(item.experience_months)}</span>{item.tests_count > 0 ? <span>✓ Тестов в профиле: {item.tests_count}</span> : null}<span className={professional ? 'is-professional' : 'is-searching'}>{professional ? 'Открыт к профессиональному общению' : 'Ищет работу'}</span></div> : null}
        {!professional && item.description ? <p>{item.description}</p> : null}
        <div className="catalog-tags">{(item.tags || []).map((tag, index) => <span key={`${tag}-${index}`}>{tag}</span>)}</div>
      </div>
      <div className={`catalog-side${professional ? ' professional' : ''}`}>{professional ? <><strong>Помогаю коллегам</strong><small>Делюсь опытом и знаниями</small></> : <><strong>{formatMoney(item.salary)} ₽</strong><small>{incomeLabel}</small></>}<a className="catalog-open-profile" href={href}>Открыть профиль</a></div>
    </article>
  )
}

export default function CatalogPage({ type }) {
  const copy = catalogCopy[type]
  const [searchParams, setSearchParams] = useSearchParams()
  const [query, setQuery] = useState(searchParams.get('q') || '')
  const [city, setCity] = useState(searchParams.get('city') || '')
  const [position, setPosition] = useState(type === 'vacancies' ? searchParams.get('position') || '' : '')
  const [workFormat, setWorkFormat] = useState(type === 'vacancies' ? searchParams.get('work_format') || '' : '')
  const [salaryFrom, setSalaryFrom] = useState(type === 'vacancies' ? searchParams.get('salary_from') || '' : '')
  const [vacancyFilters, setVacancyFilters] = useState({ positions: [], workFormats: [] })
  const [helpTopic, setHelpTopic] = useState(type === 'resumes' ? searchParams.get('help_topic') || '' : '')
  const [helpTopics, setHelpTopics] = useState([])
  const [accountingArea, setAccountingArea] = useState(type === 'resumes' ? searchParams.get('accounting_area') || '' : '')
  const [accountingAreas, setAccountingAreas] = useState([])
  const [experience, setExperience] = useState(type === 'resumes' ? searchParams.get('experience') || '' : '')
  const [profileMode, setProfileMode] = useState(type === 'resumes' ? searchParams.get('profile_mode') || '' : '')
  const [items, setItems] = useState([])
  const [total, setTotal] = useState(0)
  const [status, setStatus] = useState('loading')
  const [loadingMore, setLoadingMore] = useState(false)
  const [hasMore, setHasMore] = useState(false)
  const firstRequest = useRef(true)
  const activeRequest = useRef(null)
  const loadTriggerRef = useRef(null)
  usePageStyles(['/static/catalog.css?v=6', '/static/catalog-help.css?v=2'])
  useDocumentPage({ title: copy.title, bodyData: { catalog: type } })

  const loadCatalog = useCallback(async (signal, offset = 0, append = false) => {
    if (append) setLoadingMore(true)
    else setStatus('loading')
    const params = new URLSearchParams({ kind: type, q: query.trim(), city: city.trim(), limit: '30', offset: String(offset) })
    if (type === 'resumes' && helpTopic) params.set('help_topic', helpTopic)
    if (type === 'resumes' && accountingArea) params.set('accounting_area', accountingArea)
    if (type === 'resumes' && experience) params.set('experience', experience)
    if (type === 'resumes' && profileMode) params.set('profile_mode', profileMode)
    if (type === 'vacancies' && position) params.set('position', position)
    if (type === 'vacancies' && workFormat) params.set('work_format', workFormat)
    if (type === 'vacancies' && salaryFrom) params.set('salary_from', salaryFrom)
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
  }, [accountingArea, city, experience, helpTopic, position, profileMode, query, salaryFrom, type, workFormat])

  const loadMore = useCallback(() => {
    if (loadingMore || !hasMore || status !== 'ready') return
    const controller = new AbortController()
    activeRequest.current = controller
    loadCatalog(controller.signal, items.length, true)
  }, [hasMore, items.length, loadCatalog, loadingMore, status])

  useEffect(() => {
    if (type !== 'resumes') return
    const controller = new AbortController()
    Promise.all([
      apiClient.get('/api/public/help-topics', { signal: controller.signal, redirectOnUnauthorized: false }),
      apiClient.get('/api/public/dictionaries/accounting_areas', { signal: controller.signal, redirectOnUnauthorized: false }),
    ]).then(([topics, areas]) => {
      setHelpTopics(Array.isArray(topics) ? topics : [])
      setAccountingAreas(Array.isArray(areas?.items) ? areas.items : [])
    }).catch(() => {})
    return () => controller.abort()
  }, [type])

  useEffect(() => {
    if (type !== 'vacancies') return
    const controller = new AbortController()
    Promise.all([apiClient.get('/api/public/dictionaries/position', { signal: controller.signal, redirectOnUnauthorized: false }),apiClient.get('/api/public/dictionaries/work_format', { signal: controller.signal, redirectOnUnauthorized: false })]).then(([positions,formats]) => setVacancyFilters({ positions: positions?.items || [], workFormats: formats?.items || [] })).catch(() => {})
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

  function updateProfileFilter(setter, name, value) {
    setter(value)
    setSearchParams((current) => {
      const next = new URLSearchParams(current)
      if (value) next.set(name, value)
      else next.delete(name)
      return next
    }, { replace: true })
  }

  function resetProfileFilters() {
    setCity('')
    setHelpTopic('')
    setAccountingArea('')
    setExperience('')
    setProfileMode('')
    setSearchParams((current) => {
      const next = new URLSearchParams(current)
      ;['city', 'help_topic', 'accounting_area', 'experience', 'profile_mode'].forEach((name) => next.delete(name))
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
    const next = new URLSearchParams()
    if(query.trim())next.set('q',query.trim());if(city.trim())next.set('city',city.trim());if(position)next.set('position',position);if(workFormat)next.set('work_format',workFormat);if(salaryFrom)next.set('salary_from',salaryFrom);if(helpTopic)next.set('help_topic',helpTopic);if(accountingArea)next.set('accounting_area',accountingArea);if(experience)next.set('experience',experience);if(profileMode)next.set('profile_mode',profileMode)
    setSearchParams(next,{replace:true})
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
          <form className={`catalog-search${type === 'resumes' ? ' catalog-search-resumes' : ' catalog-search-vacancies'}`} onSubmit={handleSubmit}>
            <label>⌕<input name="q" value={query} onChange={(event) => setQuery(event.target.value)} placeholder={copy.queryPlaceholder} /></label>
            {type === 'vacancies' ? <label>⌖<input name="city" value={city} onChange={(event) => setCity(event.target.value)} placeholder="Город" /></label> : null}
            {type === 'vacancies' ? <><label className="catalog-select-filter"><span>Специализация</span><select value={position} onChange={(event) => setPosition(event.target.value)}><option value="">Любая</option>{vacancyFilters.positions.map(item=><option value={item.id} key={item.id}>{item.value}</option>)}</select></label><label className="catalog-select-filter"><span>Формат работы</span><select value={workFormat} onChange={(event) => setWorkFormat(event.target.value)}><option value="">Любой</option>{vacancyFilters.workFormats.map(item=><option value={item.id} key={item.id}>{item.value}</option>)}</select></label><label className="catalog-select-filter"><span>Зарплата от</span><select value={salaryFrom} onChange={(event) => setSalaryFrom(event.target.value)}><option value="">Любая</option><option value="50000">50 000 ₽</option><option value="80000">80 000 ₽</option><option value="100000">100 000 ₽</option><option value="150000">150 000 ₽</option><option value="200000">200 000 ₽</option></select></label></> : null}
            <button>Найти</button>
          </form>
          <div className={type === 'resumes' ? 'profile-catalog-layout' : undefined}>
            {type === 'resumes' ? <aside className="profile-filters">
              <div className="profile-filters-title"><div><small>ТОЧНЫЙ ПОИСК</small><h2>Фильтры</h2></div><button type="button" onClick={resetProfileFilters}>Сбросить</button></div>
              <label className="profile-filter-field"><span>Город</span><input value={city} onChange={(event) => updateProfileFilter(setCity, 'city', event.target.value)} placeholder="Например, Москва" /></label>
              <label className="profile-filter-field"><span>Специализация</span><select value={helpTopic} onChange={(event) => selectHelpTopic(event.target.value)}><option value="">Все направления</option>{helpTopics.map((topic) => <option value={topic.id} key={topic.id}>{topic.name}</option>)}</select></label>
              <label className="profile-filter-field"><span>Участки</span><select value={accountingArea} onChange={(event) => updateProfileFilter(setAccountingArea, 'accounting_area', event.target.value)}><option value="">Все участки</option>{accountingAreas.map((area) => <option value={area.id} key={area.id}>{area.value}</option>)}</select></label>
              <fieldset className="profile-filter-options"><legend>Опыт работы</legend>{[['', 'Любой опыт'], ['none', 'Без опыта'], ['under_1', 'До 1 года'], ['1_3', '1–3 года'], ['3_5', '3–5 лет'], ['5_10', '5–10 лет'], ['10_plus', 'Более 10 лет']].map(([value, label]) => <label key={value || 'all'}><input type="radio" name="profile-experience" checked={experience === value} onChange={() => updateProfileFilter(setExperience, 'experience', value)} /><span>{label}</span></label>)}</fieldset>
              <fieldset className="profile-filter-options"><legend>Статус</legend>{[['', 'Все специалисты'], ['job_search', 'Ищет работу'], ['professional', 'Профессиональный профиль']].map(([value, label]) => <label key={value || 'all'}><input type="radio" name="profile-mode" checked={profileMode === value} onChange={() => updateProfileFilter(setProfileMode, 'profile_mode', value)} /><span>{label}</span></label>)}</fieldset>
            </aside> : null}
            <div className="catalog-results">
          <p className="catalog-meta">{status === 'ready' ? `${type === 'resumes' ? 'Найдено специалистов' : 'Найдено'}: ${total}` : ''}</p>
          <section className="catalog-list">
            {status === 'loading' ? <div className="catalog-empty">Загружаем предложения…</div> : null}
            {status === 'error' ? <div className="catalog-empty">Не удалось загрузить каталог</div> : null}
            {status === 'ready' && !items.length ? <div className="catalog-empty">По вашему запросу ничего не найдено</div> : null}
            {status === 'ready' ? items.map((item, index) => <CatalogCard item={item} type={type} incomeLabel={copy.incomeLabel} triggerRef={hasMore && index === Math.max(0, items.length - 10) ? loadTriggerRef : undefined} key={item.id} />) : null}
            {loadingMore ? <div className="catalog-loading-more">Загружаем ещё…</div> : null}
          </section>
            </div>
          </div>
        </div>
      </main>
    </PublicLayout>
  )
}
