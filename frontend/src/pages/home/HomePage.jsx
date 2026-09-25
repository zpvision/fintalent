import { useEffect, useState } from 'react'
import { apiClient } from '../../api/client'
import SearchableSelect from '../../components/forms/SearchableSelect'
import CityPicker from '../../features/geography/CityPicker'
import PublicLayout from '../../layouts/PublicLayout'
import { useDocumentPage } from '../../hooks/useDocumentPage'
import HeroRotator from './HeroRotator'
import HomeShowcase from './HomeShowcase'
import usePageStyles from '../../hooks/usePageStyles'

const popularQueries = ['Главный бухгалтер', 'Бухгалтер на участок', 'Бухгалтер по зарплате', 'Финансовый аналитик', 'Налоговый консультант']
const salaryOptions = [{ value: '', label: 'Любая' }, { value: '50000', label: '50 000 ₽' }, { value: '80000', label: '80 000 ₽' }, { value: '100000', label: '100 000 ₽' }, { value: '150000', label: '150 000 ₽' }, { value: '200000', label: '200 000 ₽' }]

function HomeSearch() {
  const [positions, setPositions] = useState([{ value: '', label: 'Любая' }])
  const [workFormats, setWorkFormats] = useState([{ value: '', label: 'Любой' }])
  const [query, setQuery] = useState('')
  const [position, setPosition] = useState('')
  const [city, setCity] = useState('')
  const [cityId, setCityId] = useState('')
  const [workFormat, setWorkFormat] = useState('')
  const [salary, setSalary] = useState('')

  useEffect(() => {
    const controller = new AbortController()
    Promise.all([apiClient.get('/api/public/dictionaries/position', { signal: controller.signal, redirectOnUnauthorized: false }),apiClient.get('/api/public/dictionaries/work_format', { signal: controller.signal, redirectOnUnauthorized: false })]).then(([positionDictionary,formatDictionary]) => {
      setPositions([{ value: '', label: 'Любая' }, ...(positionDictionary?.items || []).map((item) => ({ value: String(item.id), label: item.value }))])
      setWorkFormats([{ value: '', label: 'Любой' }, ...(formatDictionary?.items || []).map((item) => ({ value: String(item.id), label: item.value }))])
    }).catch((error) => { if (error.name !== 'AbortError') { setPositions([{ value: '', label: 'Не удалось загрузить' }]); setWorkFormats([{ value: '', label: 'Не удалось загрузить' }]) } })
    return () => controller.abort()
  }, [])

  function search(event) {
    event?.preventDefault()
    const params = new URLSearchParams()
    if (query.trim()) params.set('q', query.trim())
    if (position) params.set('position', position)
    if (city.trim()) params.set('city', city.trim())
    if (cityId) params.set('city_id', cityId)
    if (workFormat) params.set('work_format', workFormat)
    if (salary) params.set('salary_from', salary)
    window.location.assign(`/vacancies${params.size ? `?${params}` : ''}`)
  }

  function selectPopular(value) { setQuery(value) }

  return (
    <form className="search-panel container" onSubmit={search}>
      <div className="search-grid">
        <label className="keyword">⌕ <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Должность или ключевые навыки" /></label>
        <SearchableSelect label="Специализация" name="position" options={positions} value={position} onChange={setPosition} placeholder="Любая" />
        <CityPicker value={city} cityId={cityId} onChange={(name, id) => { setCity(name); setCityId(id) }} />
        <SearchableSelect label="Формат работы" options={workFormats} value={workFormat} onChange={setWorkFormat} placeholder="Любой" />
        <SearchableSelect label="Зарплата от" options={salaryOptions} value={salary} onChange={setSalary} placeholder="Любая" />
        <button className="btn primary find">Найти вакансии</button>
      </div>
      <div className="popular"><span>Популярные запросы:</span>{popularQueries.map((value) => <button type="button" className={query === value ? 'active' : ''} onClick={() => selectPopular(value)} key={value}>{value}</button>)}</div>
    </form>
  )
}

function HomeFooter() {
  const year = new Date().getFullYear()
  return (
    <footer className="home-footer">
      <div className="container home-footer-main">
        <div className="home-footer-about"><a className="home-footer-brand" href="/" aria-label="FinTalent — главная"><span className="home-footer-logo"><img src="/static/logo.png" alt="" /></span><span className="home-footer-brand-copy"><b>Fin<span>Talent</span></b><small>Платформа профессиональных возможностей</small></span></a><p>Платформа для бухгалтеров и финансовых специалистов</p></div>
        <nav className="home-footer-nav" aria-label="Разделы сайта"><b>Разделы</b><div><a href="/vacancies">Вакансии</a><a href="/profiles">Профили</a><a href="/marketplace">Тесты</a><a href="/profimarket">ПрофиМаркет</a><a href="/client-exchange">Клиентская биржа</a><a href="/publications">Публикации</a></div></nav>
        <div className="home-footer-help"><b>Помощь</b><a href="mailto:info@fintalent.ru">info@fintalent.ru</a><a href="/profile?section=help">Обратная связь</a><span>Частые вопросы</span></div>
        <div className="home-footer-rights"><b>Для правообладателей</b><a href="/static/docs/fintalent-rightsholders.pdf" target="_blank" rel="noopener noreferrer">Открыть документ <span aria-hidden="true">↗</span></a></div>
      </div>
      <div className="container home-footer-legal"><a href="/static/docs/fintalent-user-agreement.pdf" target="_blank" rel="noopener noreferrer">Пользовательское соглашение</a><a href="/static/docs/fintalent-privacy-policy.pdf" target="_blank" rel="noopener noreferrer">Политика конфиденциальности</a><a href="/static/docs/fintalent-personal-data-consent.pdf" target="_blank" rel="noopener noreferrer">Согласие на обработку персональных данных</a></div>
      <div className="container home-footer-bottom">
        <div><b>© {year}</b><span>Все торговые марки являются собственностью их правообладателей</span></div>
        <address>ООО «Финансово-Инновационное Партнерство», ИНН: 7717583711.<br />107564, г. Москва, ул. Краснобогатырская, д. 38, стр. 2, эт. 2, комн. 17, оф. 8</address>
      </div>
    </footer>
  )
}

export default function HomePage() {
  usePageStyles([
    '/static/hero-typing.css?v=1',
    '/static/client-exchange-hero-slide.css?v=5',
    '/static/profimarket-hero.css?v=1',
    '/static/geography.css',
    '/static/searchable-select.css',
    '/static/home-showcase-react.css?v=15',
    '/static/home-footer-rights.css?v=2',
    '/static/home-search-enhancements.css?v=1',
  ])
  useDocumentPage({
    title: 'FinTalent — биржа вакансий для бухгалтеров',
    description: 'FinTalent — вакансии и профили для бухгалтеров, финансистов, руководителей и директоров',
  })

  return (
    <PublicLayout>
      <main className="home-page">
        <HeroRotator />
        <HomeSearch />
        <HomeShowcase />
      </main>
      <HomeFooter />
    </PublicLayout>
  )
}
