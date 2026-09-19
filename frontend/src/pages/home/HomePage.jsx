import { useEffect, useState } from 'react'
import { apiClient } from '../../api/client'
import SearchableSelect from '../../components/forms/SearchableSelect'
import CityPicker from '../../features/geography/CityPicker'
import PublicLayout from '../../layouts/PublicLayout'
import { useDocumentPage } from '../../hooks/useDocumentPage'
import HeroRotator from './HeroRotator'
import HomeShowcase from './HomeShowcase'
import usePageStyles from '../../hooks/usePageStyles'

const simpleOptions = [{ value: '', label: 'Любой' }]
const popularQueries = ['Главный бухгалтер', 'Бухгалтер на участок', 'Бухгалтер по зарплате', 'Финансовый аналитик', 'Налоговый консультант']

function HomeSearch() {
  const [positions, setPositions] = useState([{ value: '', label: 'Любая' }])
  const [position, setPosition] = useState('')
  const [city, setCity] = useState('')
  const [cityId, setCityId] = useState('')
  const [workFormat, setWorkFormat] = useState('')
  const [salary, setSalary] = useState('')

  useEffect(() => {
    const controller = new AbortController()
    apiClient.get('/api/public/dictionaries/position', { signal: controller.signal, redirectOnUnauthorized: false }).then((dictionary) => {
      setPositions([{ value: '', label: 'Любая' }, ...(dictionary?.items || []).map((item) => ({ value: String(item.id), label: item.value }))])
    }).catch((error) => {
      if (error.name !== 'AbortError') setPositions([{ value: '', label: 'Не удалось загрузить должности' }])
    })
    return () => controller.abort()
  }, [])

  return (
    <section className="search-panel container">
      <div className="search-grid">
        <label className="keyword">⌕ <input placeholder="Должность или ключевые навыки" /></label>
        <SearchableSelect label="Специализация" name="position" options={positions} value={position} onChange={setPosition} placeholder="Любая" />
        <CityPicker value={city} cityId={cityId} onChange={(name, id) => { setCity(name); setCityId(id) }} />
        <SearchableSelect label="Формат работы" options={simpleOptions} value={workFormat} onChange={setWorkFormat} placeholder="Любой" />
        <SearchableSelect label="Зарплата от" options={[{ value: '', label: 'Любая' }]} value={salary} onChange={setSalary} placeholder="Любая" />
        <button className="btn primary find" type="button">Найти вакансии</button>
      </div>
      <div className="popular"><span>Популярные запросы:</span>{popularQueries.map((query) => <a key={query}>{query}</a>)}<b>Расширенный поиск ⚙</b></div>
    </section>
  )
}

function HomeFooter() {
  const year = new Date().getFullYear()
  return (
    <footer className="home-footer">
      <div className="container home-footer-main">
        <div className="home-footer-about"><a className="home-footer-brand" href="/" aria-label="FinTalent — главная"><span className="home-footer-logo"><img src="/static/logo.png" alt="" /></span><span className="home-footer-brand-copy"><b>Fin<span>Talent</span></b><small>Биржа вакансий для бухгалтеров</small></span></a><p>Платформа для бухгалтеров и финансовых специалистов</p></div>
        <nav className="home-footer-nav" aria-label="Разделы сайта"><b>Разделы</b><div><a href="/vacancies">Вакансии</a><a href="/resumes">Профили</a><a href="/marketplace">Тесты</a><a href="/profimarket">ПрофиМаркет</a><a href="/client-exchange">Клиентская биржа</a><a href="/publications">Публикации</a></div></nav>
        <div className="home-footer-help"><b>Помощь</b><a href="mailto:info@fintalent.ru">info@fintalent.ru</a><a href="/profile?section=help">Обратная связь</a><span>Частые вопросы</span></div>
      </div>
      <div className="container home-footer-legal"><span>Пользовательское соглашение</span><span>Политика конфиденциальности</span><span>Обработка персональных данных</span></div>
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
    '/static/home-showcase-react.css?v=13',
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
