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

export default function HomePage() {
  usePageStyles([
    '/static/hero-typing.css?v=1',
    '/static/hero-rotator.css?v=2',
    '/static/client-exchange-hero-slide.css?v=5',
    '/static/profimarket-hero.css?v=1',
    '/static/geography.css',
    '/static/searchable-select.css',
  ])
  useDocumentPage({
    title: 'FinTalent — биржа вакансий для бухгалтеров',
    description: 'FinTalent — вакансии и резюме для бухгалтеров, финансистов, руководителей и директоров',
  })

  return (
    <PublicLayout>
      <main>
        <HeroRotator />
        <HomeSearch />
        <HomeShowcase />
      </main>
    </PublicLayout>
  )
}
