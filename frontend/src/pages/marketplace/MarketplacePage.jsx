import { useEffect, useMemo, useState } from 'react'
import { apiClient } from '../../api/client'
import PublicLayout from '../../layouts/PublicLayout'
import { useDocumentPage } from '../../hooks/useDocumentPage'
import usePageStyles from '../../hooks/usePageStyles'

const difficulties = [
  ['all', 'Любой'],
  ['easy', 'Начальный'],
  ['medium', 'Средний'],
  ['hard', 'Продвинутый'],
  ['expert', 'Эксперт'],
]
const difficultyNames = Object.fromEntries(difficulties.slice(1))
const icons = ['%', '1C', '▥', '◕', '♟', '▤', '✓', '₽', 'X']

function pluralTests(count) {
  if (count % 100 >= 11 && count % 100 <= 14) return 'тестов'
  if (count % 10 === 1) return 'тест'
  if (count % 10 >= 2 && count % 10 <= 4) return 'теста'
  return 'тестов'
}

function TestCard({ test, index }) {
  return (
    <article className="test-card">
      <div className="test-card-top"><span className={`test-icon icon-${index % 5}`}>{icons[index % icons.length]}</span><span className={`free-label${test.is_free ? '' : ' paid-label'}`}>{test.is_free ? 'Бесплатно' : `${Number(test.price).toLocaleString('ru-RU')} ₽`}</span></div>
      <h3>{test.title}</h3>
      <p>{test.description}</p>
      <div className="tag-row"><span>{test.category || test.position}</span><span>{difficultyNames[test.difficulty] || test.difficulty || 'Средний'}</span></div>
      <div className="card-bottom"><span>◷ {Number(test.question_count) + 6} мин</span><span>♧ {Number(test.review_count) * 21 + 103}</span><span className="rating">★ {Number(test.rating).toFixed(1)} ({test.review_count})</span></div>
      <span className="take-button">Пройти тест <b>→</b></span>
      <a className="card-link" href={`/tests/take?id=${test.id}`} aria-label={`Пройти тест ${test.title}`} />
    </article>
  )
}

function Filters({ price, setPrice, categories, selectedCategories, toggleCategory, difficulty, setDifficulty }) {
  return (
    <aside className="left-sidebar">
      <section className="side-card filters-card">
        <h2>Подбор тестов</h2>
        <div className="filter-tabs" role="group" aria-label="Стоимость теста">
          {[['all', 'Все тесты'], ['free', 'Бесплатные'], ['paid', 'Платные']].map(([value, label]) => <button className={price === value ? 'active' : ''} type="button" onClick={() => setPrice(value)} key={value}>{label}</button>)}
        </div>
        <div className="filter-group"><h3>Категории <span>⌃</span></h3><div id="category-filters">{categories === null ? <small>Загрузка…</small> : categories.map((item) => <label key={item.id || item.name}><input type="checkbox" value={item.name} checked={selectedCategories.has(item.name)} onChange={() => toggleCategory(item.name)} /><i />{item.name}</label>)}</div></div>
        <div className="filter-group"><h3>Уровень сложности</h3>{difficulties.map(([value, label]) => <label key={value}><input type="radio" name="difficulty" value={value} checked={difficulty === value} onChange={() => setDifficulty(value)} /><i />{label}</label>)}</div>
        <div className="filter-group"><h3>Формат</h3><label><input type="checkbox" disabled /><i />Тест с выбором ответа</label><label><input type="checkbox" disabled /><i />Ситуационные задачи</label><label><input type="checkbox" disabled /><i />Кейс-тесты</label></div>
        <div className="filter-group"><h3>Длительность</h3><label><input type="radio" name="duration" defaultChecked disabled /><i />Любая</label><label><input type="radio" name="duration" disabled /><i />До 15 минут</label></div>
      </section>
    </aside>
  )
}

function MarketplaceSidebar() {
  return (
    <aside className="right-sidebar">
      <section className="right-card create-promo"><div><small className="promo-eyebrow">ДЕЛИТЕСЬ ЭКСПЕРТИЗОЙ</small><h3>Создавайте свои тесты</h3><p>Публикуйте профессиональные тесты, развивайте личный бренд и зарабатывайте на своих знаниях.</p><a href="/marketplace/create-test">Создать тест <b>→</b></a></div><span>▣</span></section>
      <section className="right-card categories-card"><h3>Популярные категории</h3><ul><li><span className="category-icon blue">▤</span><b>Бухгалтерский учёт</b><small>45 тестов</small></li><li><span className="category-icon orange">▥</span><b>Налоговый учёт</b><small>32 теста</small></li><li><span className="category-icon violet">♟</span><b>ТМЦ и зарплата</b><small>18 тестов</small></li><li><span className="category-icon coral">⌘</span><b>1С и программы</b><small>16 тестов</small></li><li><span className="category-icon purple">◕</span><b>Финансовый анализ</b><small>12 тестов</small></li></ul><a className="more-link" href="#catalog">Смотреть все категории <span>→</span></a></section>
      <section className="right-card premium-promo"><div><h3>Премиум доступ</h3><p>Неограниченный доступ ко всем тестам и подробная аналитика результатов.</p><a href="#premium">Подробнее</a></div><span>◆</span></section>
      <section className="right-card authors-card"><h3>Топ авторов</h3><ol><li><span>1</span><i>ФЭ</i><b>ФинЭксперт</b><small>24 теста　⭐ 4.9</small></li><li><span>2</span><i>БП</i><b>Бухгалтер-практик</b><small>18 тестов　⭐ 4.8</small></li><li><span>3</span><i>1С</i><b>1С:Профи</b><small>12 тестов　⭐ 4.7</small></li></ol><a className="more-link" href="#authors">Смотреть всех авторов <span>→</span></a></section>
    </aside>
  )
}

export default function MarketplacePage() {
  const [tests, setTests] = useState(null)
  const [categories, setCategories] = useState(null)
  const [error, setError] = useState('')
  const [price, setPrice] = useState('all')
  const [difficulty, setDifficulty] = useState('all')
  const [selectedCategories, setSelectedCategories] = useState(new Set())
  const [query, setQuery] = useState('')
  const [sort, setSort] = useState('popular')
  usePageStyles([
    '/static/marketplace.css?v=2',
    '/static/marketplace-filters.css?v=1',
    '/static/marketplace-home-background.css?v=1',
    '/static/marketplace-create-promo.css?v=1',
  ])
  useDocumentPage({ title: 'Маркетплейс тестов — FinTalent' })

  useEffect(() => {
    const controller = new AbortController()
    Promise.all([
      apiClient.get('/api/marketplace/tests', { signal: controller.signal, redirectOnUnauthorized: false }),
      apiClient.get('/api/test-categories', { signal: controller.signal, redirectOnUnauthorized: false }),
    ]).then(([testItems, categoryItems]) => {
      setTests(Array.isArray(testItems) ? testItems : [])
      setCategories(Array.isArray(categoryItems) ? categoryItems : [])
    }).catch((requestError) => { if (requestError.name !== 'AbortError') setError(requestError.message || 'Не удалось загрузить тесты') })
    return () => controller.abort()
  }, [])

  const visibleTests = useMemo(() => {
    const normalized = query.trim().toLocaleLowerCase('ru')
    const values = (tests || []).filter((test) => `${test.title} ${test.position} ${test.description}`.toLocaleLowerCase('ru').includes(normalized)
      && (price === 'all' || (price === 'free' ? test.is_free : !test.is_free))
      && (difficulty === 'all' || test.difficulty === difficulty)
      && (!selectedCategories.size || selectedCategories.has(test.category)))
    if (sort === 'rating') values.sort((left, right) => Number(right.rating) - Number(left.rating))
    if (sort === 'name') values.sort((left, right) => left.title.localeCompare(right.title, 'ru'))
    if (sort === 'popular') values.sort((left, right) => Number(right.review_count) - Number(left.review_count))
    return values
  }, [difficulty, price, query, selectedCategories, sort, tests])

  function toggleCategory(name) {
    setSelectedCategories((current) => {
      const next = new Set(current)
      if (next.has(name)) next.delete(name)
      else next.add(name)
      return next
    })
  }

  return (
    <PublicLayout>
      <main className="marketplace-page">
        <Filters price={price} setPrice={setPrice} categories={categories} selectedCategories={selectedCategories} toggleCategory={toggleCategory} difficulty={difficulty} setDifficulty={setDifficulty} />
        <section className="catalog-column"><div className="page-heading"><h1>Маркетплейс тестов</h1><p>Выбирайте и проходите профессиональные тесты, подтверждайте свои знания<br />и повышайте конкурентоспособность.</p></div><div className="catalog-tools"><label className="search-box"><span>⌕</span><input id="search" type="search" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Поиск тестов по названию, навыкам или ключевым словам" /></label><select id="sort" value={sort} onChange={(event) => setSort(event.target.value)} aria-label="Сортировка"><option value="popular">Сначала популярные</option><option value="rating">По рейтингу</option><option value="name">По названию</option></select></div><div className="catalog-title"><div><h2>Все тесты</h2><p id="count">{tests ? `Найдено ${visibleTests.length} ${pluralTests(visibleTests.length)}` : 'Загрузка…'}</p></div></div><section id="catalog" className="catalog">{error ? <div className="loading">{error}</div> : null}{!error && tests === null ? <div className="loading">Загружаем тесты…</div> : null}{tests && !visibleTests.length ? <div className="loading">По выбранным параметрам тестов не найдено</div> : null}{visibleTests.map((test, index) => <TestCard test={test} index={index} key={test.id} />)}</section></section>
        <MarketplaceSidebar />
      </main>
    </PublicLayout>
  )
}
