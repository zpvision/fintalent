import { useEffect, useMemo, useRef, useState } from 'react'
import { getProfiMarketMeta, getProfiMarketSolutions } from '../../api/profimarket'
import Icon from '../../components/Icon'
import { useDocumentPage } from '../../hooks/useDocumentPage'
import usePageStyles from '../../hooks/usePageStyles'
import PublicLayout from '../../layouts/PublicLayout'

const categories = [
  { type: 'AI_ASSISTANT', name: 'ИИ-ассистенты и боты', icon: 'bot' },
  { name: 'Автоматизации', icon: 'sparkles', soon: true },
  { name: '1С и интеграции', icon: 'calculator', soon: true },
  { type: 'REGULATION', name: 'Регламенты', icon: 'workflow' },
  { name: 'Инструкции', icon: 'list', soon: true },
  { name: 'Шаблоны', icon: 'folder', soon: true },
]

function priceText(solution) {
  if (solution.pricing_type === 'FREE' || !Number(solution.price)) return 'Бесплатно'
  const suffix = { MONTHLY: ' / месяц', YEARLY: ' / год' }[solution.pricing_type] || ''
  return `${new Intl.NumberFormat('ru-RU', { maximumFractionDigits: 0 }).format(Number(solution.price))} ₽${suffix}`
}

function Sidebar({ selectType }) {
  return (
    <aside className="pmh-sidebar">
      <nav>
        <a className="active" href="/profimarket"><Icon name="bag" /><b>ПрофиМаркет</b></a>
        {categories.map((category, index) => (
          <a
            href={category.type ? '#popular' : '#'}
            className={category.soon ? 'disabled' : undefined}
            data-type={category.type}
            onClick={category.type ? (event) => { event.preventDefault(); selectType(category.type) } : (event) => event.preventDefault()}
            key={category.name}
          >
            <Icon name={category.icon} /><span>{category.name}</span>{index === 0 ? <em>NEW</em> : category.soon ? <small>Скоро</small> : null}
          </a>
        ))}
      </nav>
      <div className="pmh-side-links">
        <a href="/profimarket/my?tab=favorites"><Icon name="heart" /> Избранное</a>
        <a href="/profimarket/my?tab=purchases"><Icon name="bag" /> Мои покупки</a>
        <a href="/profimarket/my"><Icon name="folder" /> Мои решения</a>
        <a href="/profimarket/create"><Icon name="users" /> Стать автором</a>
      </div>
      <section className="pmh-author-cta"><i><Icon name="sparkles" /></i><h3>Есть свои наработки?</h3><p>Размещайте решения на ПрофиМаркете и зарабатывайте на своём опыте</p><a href="/profimarket/create">Стать автором</a></section>
    </aside>
  )
}

function Hero({ query, setQuery, submitQuery }) {
  function submit(event) {
    event.preventDefault()
    submitQuery(query)
  }

  return (
    <section className="pmh-hero">
      <div className="pmh-hero-copy">
        <h1>ПрофиМаркет</h1><p>Готовые решения и наработки<br />от профессионалов для профессионалов</p>
        <form id="pmh-search" onSubmit={submit}><span><Icon name="search" /></span><input name="q" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Что вы ищете?" /><button>Найти</button></form>
        <div className="pmh-queries"><span>Популярные запросы:</span>{['НДС', '1С', 'Закрытие месяца', 'ФНС'].map((value) => <button type="button" onClick={() => { setQuery(value); submitQuery(value) }} key={value}>{value}</button>)}</div>
      </div>
      <div className="pmh-visual"><div className="pmh-laptop"><div><i><Icon name="bag" /></i><b>FinTalent</b></div></div><i className="float bot"><Icon name="bot" /></i><i className="float onec">1С</i><i className="float excel">X</i><i className="float doc"><Icon name="list" /></i><i className="float flow"><Icon name="workflow" /></i><div className="pmh-plant"><i /><i /><i /><b /></div></div>
    </section>
  )
}

function CategoryRow({ counts, selectType }) {
  return (
    <section id="pmh-category-row" className="pmh-category-row">
      {categories.map((category) => <button className={category.soon ? 'soon' : undefined} disabled={!category.type} data-type={category.type} onClick={() => selectType(category.type)} key={category.name}><i><Icon name={category.icon} /></i><span><b>{category.name}</b><small>{category.type ? `${counts[category.type] || 0} решений` : 'Скоро'}</small></span></button>)}
      <b>›</b>
    </section>
  )
}

function SolutionCard({ solution }) {
  const ai = solution.type === 'AI_ASSISTANT'
  return (
    <article className="pmh-card">
      <div className={`pmh-card-cover ${ai ? 'ai' : 'reg'}`}><em>{solution.is_new ? 'НОВИНКА' : 'ХИТ'}</em><button type="button"><Icon name="folder" /></button><i><Icon name={ai ? 'bot' : 'workflow'} /></i><b>{ai ? 'AI' : 'PRO'}</b></div>
      <div className="pmh-card-body"><h3>{solution.title}</h3><p>{solution.short_description}</p><div className="pmh-author"><i>{(solution.author_name || 'А').charAt(0)}</i><span>{solution.author_name || 'Автор FinTalent'}</span></div><footer><span><b>★ {Number(solution.rating || 4.9).toFixed(1)}</b> ({solution.review_count || 0})</span><strong>{priceText(solution)}</strong></footer></div>
      <a href={`/profimarket/solution/${encodeURIComponent(solution.slug)}`} aria-label={`Открыть ${solution.title}`} />
    </article>
  )
}

export default function ProfiMarketPage() {
  const [query, setQuery] = useState('')
  const [submittedQuery, setSubmittedQuery] = useState('')
  const [selectedType, setSelectedType] = useState('')
  const [solutions, setSolutions] = useState(null)
  const [meta, setMeta] = useState({ categories: [] })
  const [error, setError] = useState('')
  const popularRef = useRef(null)

  usePageStyles(['/static/profimarket.css?v=1', '/static/profimarket-regulation.css?v=12', '/static/profimarket-home.css?v=2'])
  useDocumentPage({ title: 'ПрофиМаркет — FinTalent' })

  useEffect(() => {
    const controller = new AbortController()
    getProfiMarketMeta({ signal: controller.signal }).then(setMeta).catch(() => {})
    return () => controller.abort()
  }, [])

  useEffect(() => {
    const controller = new AbortController()
    setSolutions(null)
    setError('')
    getProfiMarketSolutions({ query: submittedQuery, type: selectedType }, { signal: controller.signal })
      .then((value) => setSolutions(value?.items || []))
      .catch((requestError) => { if (requestError.name !== 'AbortError') setError(requestError.message || 'Не удалось загрузить решения') })
    return () => controller.abort()
  }, [selectedType, submittedQuery])

  const counts = useMemo(() => Object.fromEntries((meta.categories || []).map((item) => [item.type, item.count])), [meta])

  function selectType(type) {
    setSelectedType(type || '')
    window.setTimeout(() => popularRef.current?.scrollIntoView({ behavior: 'smooth' }), 0)
  }

  function submitQuery(value) {
    setSelectedType('')
    setSubmittedQuery(value.trim())
  }

  return (
    <PublicLayout>
      <main className="pm-page">
        <div className="pmh-layout">
          <Sidebar selectType={selectType} />
          <main className="pmh-main">
            <Hero query={query} setQuery={setQuery} submitQuery={submitQuery} />
            <CategoryRow counts={counts} selectType={selectType} />
            <section id="popular" ref={popularRef} className="pmh-popular"><header><h2>Популярное сейчас <span>●</span></h2><a href="#popular">Смотреть все ›</a></header><div id="pmh-cards" className="pmh-cards">{solutions === null && !error ? <div className="pm-loading">Загружаем решения…</div> : null}{error ? <div className="pm-loading">{error}</div> : null}{solutions && !solutions.length ? <div className="pm-loading">По вашему запросу решений пока нет</div> : null}{(solutions || []).map((solution) => <SolutionCard solution={solution} key={solution.id} />)}</div></section>
            <section className="pmh-trust"><article><Icon name="users" /><span><b>Проверенные авторы</b><small>Все авторы проходят проверку командой FinTalent</small></span></article><article><Icon name="bag" /><span><b>Безопасная покупка</b><small>Защищённое оформление профессиональных решений</small></span></article><article><Icon name="clock" /><span><b>Обновления</b><small>Получайте обновления купленных решений от авторов</small></span></article><article><Icon name="message" /><span><b>Поддержка</b><small>Команда FinTalent всегда на связи</small></span></article></section>
          </main>
        </div>
      </main>
    </PublicLayout>
  )
}
