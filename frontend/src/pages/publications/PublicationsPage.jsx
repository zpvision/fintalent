import { useEffect, useMemo, useRef, useState } from 'react'
import { getPublications, getPublicationsMeta, togglePublicationAuthor, togglePublicationBookmark } from '../../api/publications'
import useDebouncedValue from '../../hooks/useDebouncedValue'
import { useDocumentPage } from '../../hooks/useDocumentPage'
import usePageStyles from '../../hooks/usePageStyles'
import PublicLayout from '../../layouts/PublicLayout'

const initialFilters = {
  sort: 'new',
  category: '',
  topic: '',
  skill: '',
  difficulty: '',
  author: '',
  date: '',
  q: '',
}

const scopes = [
  ['', 'Все публикации'],
  ['subscriptions', 'Подписки'],
  ['popular', 'Популярные'],
  ['new', 'Новые'],
  ['recommended', 'Рекомендуемые'],
  ['saved', 'Сохранённые'],
  ['mine', 'Мои публикации'],
  ['drafts', 'Черновики'],
]

function formatDate(value) {
  return value ? new Date(value).toLocaleDateString('ru-RU', { day: 'numeric', month: 'long', year: 'numeric' }) : 'Черновик'
}

function word(count, one, few, many) {
  const mod = count % 100
  if (mod >= 11 && mod <= 19) return many
  return count % 10 === 1 ? one : count % 10 >= 2 && count % 10 <= 4 ? few : many
}

function AuthorAvatar({ item }) {
  return item.author_avatar ? <img src={item.author_avatar} alt="" /> : (item.author_name || 'F').charAt(0)
}

function PublicationCard({ item, onBookmark }) {
  const tags = item.tags || []
  const labels = [item.category, ...tags.slice(0, 2)].filter(Boolean)
  const href = `/publications/${item.slug}`
  return (
    <article className="publication-card">
      <a className="publication-card-open" href={href} aria-label="Открыть публикацию" />
      <a className="publication-cover" href={href} style={{ backgroundImage: `url('${item.cover_image || '/static/publication-usn-cover.png'}')` }} />
      <div className="publication-content">
        <div className="publication-labels">{labels.map((label, index) => <span key={`${label}-${index}`}>{label}</span>)}{tags.length > 2 ? <span>+{tags.length - 2}</span> : null}</div>
        <h2><a href={href}>{item.title}</a></h2>
        <p className="publication-description">{item.excerpt || item.subtitle}</p>
        <div className="publication-author-line"><i className="publication-author-avatar"><AuthorAvatar item={item} /></i><span className="publication-author-copy"><b>{item.author_name} <em>✓</em></b><small>Эксперт FinTalent</small></span><span className="publication-date">{formatDate(item.published_at || item.updated_at)}</span><span className="publication-reading">◷ {item.reading_time} мин</span></div>
        <div className="publication-tags">{[...tags, ...(item.skills || [])].slice(0, 5).map((tag, index) => <span key={`${tag}-${index}`}>{tag}</span>)}</div>
      </div>
      <aside className="publication-metrics"><span className="publication-state">{item.relevance_status === 'current' ? 'Актуально' : 'Требует проверки'}</span><button className={`publication-bookmark${item.is_saved ? ' active' : ''}`} type="button" onClick={() => onBookmark(item.id)} title={item.is_saved ? 'Удалить из сохранённых' : 'Сохранить'}>{item.is_saved ? '★' : '☆'}</button><div className="publication-stat-list"><span className="publication-stat"><i>◉</i><span>Просмотров</span><b>{Number(item.views).toLocaleString('ru-RU')}</b></span><span className="publication-stat"><i>☆</i><span>Полезно</span><b>{item.useful}</b></span><span className="publication-stat"><i>▱</i><span>Сохранений</span><b>{item.saves}</b></span><span className="publication-stat"><i>☵</i><span>Обсуждений</span><b>{item.discussions}</b></span></div><span className="publication-usefulness"><b>{Math.min(100, Math.max(item.usefulness, item.useful ? 70 : 0))}%</b> полезности</span></aside>
    </article>
  )
}

function PublicationsSidebar({ items, onFollow, focusSearch }) {
  const topics = useMemo(() => {
    const counts = new Map()
    items.forEach((item) => [item.category, ...(item.tags || [])].filter(Boolean).forEach((name) => counts.set(name, (counts.get(name) || 0) + 1)))
    const values = [...counts.entries()]
    for (const name of ['УСН', 'НДС', 'Зарплата', 'Отчётность', 'Проверки']) {
      if (!counts.has(name)) values.push([name, 0])
    }
    return values.slice(0, 5)
  }, [items])

  const authors = useMemo(() => {
    const ids = new Set()
    return items.filter((item) => {
      if (ids.has(item.author_id)) return false
      ids.add(item.author_id)
      return true
    }).slice(0, 5)
  }, [items])

  const icons = ['◎', '♧', '▣', '▤', '♨']
  return (
    <aside className="publications-sidebar">
      <section className="publication-side-card topics-card"><header><h2>Популярные темы</h2><a href="#publication-search">Все темы</a></header><div id="popular-topics">{topics.map(([name, count], index) => <a className="topic-row" href={`?q=${encodeURIComponent(name)}`} key={name}><i>{icons[index % icons.length]}</i><span><b>{name}</b><small>{count ? `${count} ${word(count, 'публикация', 'публикации', 'публикаций')}` : 'Материалы по теме'}</small></span></a>)}</div><button id="show-all-topics" type="button" onClick={focusSearch}>Ещё темы⌄</button></section>
      <section className="publication-side-card subscriptions-card"><header><h2>Авторы публикаций</h2><a href="?scope=subscriptions">Мои подписки</a></header><div id="publication-authors">{authors.map((item) => <div className="author-row" key={item.author_id}><i><AuthorAvatar item={item} /></i><span><b>{item.author_name}</b><small>Эксперт FinTalent</small></span><button className={`sidebar-follow${item.is_following ? ' active' : ''}`} type="button" onClick={() => onFollow(item.author_id)} title={item.is_following ? 'Вы подписаны' : 'Подписаться'} /></div>)}{!authors.length ? <p className="sidebar-empty">Авторы появятся вместе с публикациями.</p> : null}</div></section>
      <section className="publication-side-card publication-promo"><div className="promo-illustration">✎</div><h2>Публикуйте и развивайте<br />свою экспертность</h2><p>Делитесь опытом, получайте признание коллег и повышайте свой профессиональный индекс.</p><a href="/publications/create">Создать публикацию</a></section>
    </aside>
  )
}

function FilterSelect({ id, value, onChange, placeholder, items, valueKey = 'slug' }) {
  return <select id={id} value={value} onChange={onChange}><option value="">{placeholder}</option>{items.map((item) => <option value={item[valueKey] || item.name} key={item.id}>{item.name}</option>)}</select>
}

export default function PublicationsPage({ saved = false }) {
  const urlState = useMemo(() => new URLSearchParams(window.location.search), [])
  const initialQuery = urlState.get('q') || ''
  const initialScope = saved ? 'saved' : urlState.get('scope') || ''
  const [scope, setScope] = useState(initialScope)
  const [filters, setFilters] = useState({ ...initialFilters, q: initialQuery })
  const [search, setSearch] = useState(initialQuery)
  const [author, setAuthor] = useState('')
  const debouncedAuthor = useDebouncedValue(author, 350)
  const [page, setPage] = useState(1)
  const [items, setItems] = useState([])
  const [hasMore, setHasMore] = useState(false)
  const [meta, setMeta] = useState({ categories: [], topics: [], skills: [] })
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [extraVisible, setExtraVisible] = useState(false)
  const [grid, setGrid] = useState(false)
  const searchRef = useRef(null)
  const filtersRef = useRef(null)

  usePageStyles(['/static/publications.css?v=2', '/static/publications-fixes.css?v=1', '/static/publications-readable.css?v=1', '/static/publications-polish.css?v=1'])
  useDocumentPage({ title: 'Публикации — FinTalent', description: 'Экспертные материалы и профессиональные разборы специалистов финансовой отрасли' })

  useEffect(() => {
    const controller = new AbortController()
    getPublicationsMeta({ signal: controller.signal }).then(setMeta).catch(() => {})
    return () => controller.abort()
  }, [])

  useEffect(() => {
    setFilters((current) => ({ ...current, author: debouncedAuthor }))
    setPage(1)
  }, [debouncedAuthor])

  useEffect(() => {
    const controller = new AbortController()
    setLoading(true)
    setError('')
    getPublications({ ...filters, scope }, page, { signal: controller.signal })
      .then((value) => {
        const nextItems = value?.items || []
        setItems((current) => page === 1 ? nextItems : [...current, ...nextItems])
        setHasMore(Boolean(value?.has_more))
      })
      .catch((requestError) => { if (requestError.name !== 'AbortError') setError(requestError.message || 'Ошибка запроса') })
      .finally(() => { if (!controller.signal.aborted) setLoading(false) })
    return () => controller.abort()
  }, [filters, page, scope])

  function updateFilter(name, value) {
    setFilters((current) => ({ ...current, [name]: value }))
    setPage(1)
  }

  function selectScope(nextScope) {
    setScope(nextScope)
    setPage(1)
    if (nextScope === 'popular') updateFilter('sort', 'popular')
    if (nextScope === 'new') updateFilter('sort', 'new')
  }

  function submitSearch(event) {
    event.preventDefault()
    updateFilter('q', search.trim())
  }

  async function bookmark(id) {
    try {
      const result = await togglePublicationBookmark(id)
      setItems((current) => current.map((item) => item.id === id ? { ...item, is_saved: result.active } : item))
    } catch {}
  }

  async function follow(authorId) {
    try {
      const result = await togglePublicationAuthor(authorId)
      setItems((current) => current.map((item) => item.author_id === authorId ? { ...item, is_following: result.active } : item))
    } catch {}
  }

  function focusSearch() {
    searchRef.current?.focus()
    window.scrollTo({ top: (filtersRef.current?.offsetTop || 90) - 90, behavior: 'smooth' })
  }

  return (
    <PublicLayout>
      <main className="publications-page">
        <section className="publications-heading"><div><h1>Публикации</h1><p>Экспертные материалы, профессиональные разборы<br />и практический опыт специалистов финансовой отрасли.</p></div><a className="publication-create" href="/publications/create"><span>＋</span> Создать публикацию</a></section>
        <div className="publications-layout">
          <section className="publications-workspace">
            <nav id="scope-filter" className="publication-tabs" aria-label="Разделы публикаций">{scopes.map(([value, label]) => <button type="button" data-scope={value} className={scope === value ? 'active' : ''} onClick={() => selectScope(value)} key={label}>{label}</button>)}</nav>
            <section ref={filtersRef} className="publication-filters">
              <form id="publication-search" className="publication-search" onSubmit={submitSearch}><span>⌕</span><input ref={searchRef} name="q" value={search} onChange={(event) => setSearch(event.target.value)} placeholder="Поиск по заголовку, теме, автору, навыку…" /><button aria-label="Найти">Найти</button></form>
              <FilterSelect id="category-filter" value={filters.category} onChange={(event) => updateFilter('category', event.target.value)} placeholder="Категория" items={meta.categories || []} />
              <FilterSelect id="topic-filter" value={filters.topic} onChange={(event) => updateFilter('topic', event.target.value)} placeholder="Тема" items={meta.topics || []} />
              <FilterSelect id="skill-filter" value={filters.skill} onChange={(event) => updateFilter('skill', event.target.value)} placeholder="Навык" items={meta.skills || []} valueKey="name" />
              <select id="difficulty-filter" value={filters.difficulty} onChange={(event) => updateFilter('difficulty', event.target.value)}><option value="">Уровень</option><option value="beginner">Начальный</option><option value="medium">Средний</option><option value="advanced">Продвинутый</option><option value="expert">Экспертный</option></select>
              <button id="more-filters" className="more-filters" type="button" onClick={() => setExtraVisible((current) => !current)}>⚙ Ещё фильтры</button>
              <div id="extra-filters" className={`extra-filters${extraVisible ? '' : ' hidden'}`}><input id="author-filter" value={author} onChange={(event) => setAuthor(event.target.value)} placeholder="Автор" /><input type="date" id="date-filter" value={filters.date} onChange={(event) => updateFilter('date', event.target.value)} /></div>
            </section>
            <div className="publication-list-tools"><label>Сортировка: <select id="sort-filter" value={filters.sort} onChange={(event) => updateFilter('sort', event.target.value)}><option value="new">Сначала новые</option><option value="popular">Сначала популярные</option><option value="useful">По полезности</option><option value="saved">По сохранениям</option><option value="discussed">По обсуждениям</option></select></label><div className="view-buttons"><button id="list-view" className={grid ? '' : 'active'} type="button" onClick={() => setGrid(false)} title="Список">☷</button><button id="grid-view" className={grid ? 'active' : ''} type="button" onClick={() => setGrid(true)} title="Плитка">▦</button></div></div>
            <div id="publication-list" className={`publication-list${grid ? ' publication-grid' : ''}`}>{loading && page === 1 ? <div className="publication-loading">Загружаем экспертные материалы…</div> : null}{error ? <div className="publication-empty">{error}</div> : null}{!error && !(loading && page === 1) ? items.map((item) => <PublicationCard item={item} onBookmark={bookmark} key={item.id} />) : null}{!error && !loading && !items.length ? <div className="publication-empty">Публикаций по выбранным условиям пока нет</div> : null}</div>
            <button id="load-more" className={`load-more${hasMore ? '' : ' hidden'}`} type="button" disabled={loading} onClick={() => setPage((current) => current + 1)}>Показать ещё</button>
          </section>
          <PublicationsSidebar items={items} onFollow={follow} focusSearch={focusSearch} />
        </div>
      </main>
    </PublicLayout>
  )
}
