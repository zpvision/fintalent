import { useEffect, useMemo, useState } from 'react'
import { getAccountingCompanies, getAccountingCompaniesMeta } from '../../api/companies'
import useDebouncedValue from '../../hooks/useDebouncedValue'
import { useDocumentPage } from '../../hooks/useDocumentPage'
import usePageStyles from '../../hooks/usePageStyles'
import PublicLayout from '../../layouts/PublicLayout'

const emptyFilters = {
  city: '',
  direction_id: '',
  service_id: '',
  tax_system_id: '',
  price_from: '',
  price_to: '',
}

const chipIcons = ['▤', '⌑', '▣', '◉', '▧', '◫', '◆', '◇', '✦']
const priceTypes = {
  from_month: 'от {p} / мес.',
  month: '{p} / мес.',
  from_hour: 'от {p} / час',
  hour: '{p} / час',
  from_once: 'от {p}',
  request: 'по запросу',
}

function formatMoney(value) {
  return value == null ? 'по запросу' : `${new Intl.NumberFormat('ru-RU').format(value)} ₽`
}

function formatPrice(service) {
  return (priceTypes[service.price_type] || '{p}').replace('{p}', formatMoney(service.price_from))
}

function CompanyLogo({ company }) {
  return company.logo
    ? <img src={company.logo} alt="" />
    : <span>{(company.name || 'Б').trim().charAt(0).toLocaleUpperCase('ru')}</span>
}

function CompanyPassport({ company }) {
  const value = company.passport_summary ? Math.round(company.passport_summary.overall_index) : 0
  if (!value) {
    return (
      <div className="ac-directory-passport ghost">
        <div className="ac-directory-ring" style={{ '--score': 62, '--ring': '#b9c3de' }}><b>?</b></div>
        <span><small>Паспорт компетенций</small><b>Формируется</b><em>Примерный график появится после тестов</em></span>
      </div>
    )
  }

  const tone = value >= 90 ? 'green' : value >= 80 ? 'blue' : value >= 70 ? 'orange' : 'red'
  const label = value >= 90 ? 'Отличный уровень' : value >= 80 ? 'Хороший уровень' : value >= 70 ? 'Средний уровень' : 'Требует проверки'
  const color = { green: '#10a970', blue: '#2a7bf6', orange: '#ff8a00', red: '#ef496f' }[tone]
  return (
    <div className={`ac-directory-passport ${tone}`}>
      <div className={`ac-directory-ring ${tone}`} style={{ '--score': value, '--ring': color }}><b>{value}%</b></div>
      <span><small>Паспорт компетенций</small><b>{label}</b><em>{company.passport_summary.tests_count || 0} направлений<br />{company.passport_summary.specialists_count || 0} специалистов</em></span>
    </div>
  )
}

function CompanyServices({ company }) {
  const services = (company.services || []).slice(0, 3)
  if (!services.length) return <div><small>Услуги</small><b>по запросу</b></div>
  return services.map((service) => <div key={service.id || service.name}><small>{service.name}</small><b>{formatPrice(service)}</b></div>)
}

function CompanyRow({ company }) {
  const href = `/accounting-companies/view?slug=${encodeURIComponent(company.slug)}`
  const directions = (company.directions || []).slice(0, 3)
  const more = Math.max(0, (company.directions || []).length - 3)
  return (
    <article className="ac-directory-row">
      <a className="ac-directory-logo" href={href}><CompanyLogo company={company} /></a>
      <section className="ac-directory-company">
        <h3><a href={href}>{company.name}</a>{company.verified ? <i>✓</i> : null}</h3>
        <p>{company.short_description || 'Комплексное бухгалтерское сопровождение бизнеса'}</p>
        <div><span>⌖ {company.city || 'Онлайн'}</span><span>{company.founded_year ? `Работает с ${company.founded_year} года` : 'Опыт работы указан в профиле'}</span>{company.employee_count ? <span>{company.employee_count} сотрудников</span> : null}</div>
      </section>
      <section className="ac-directory-tags">{directions.map((item) => <span key={item.id || item.name}>{item.name}</span>)}{more ? <span>+{more}</span> : null}</section>
      <section className="ac-directory-services"><CompanyServices company={company} /></section>
      <CompanyPassport company={company} />
      <section className="ac-directory-actions"><a href={href}>Посмотреть</a><button type="button" aria-label="Сохранить компанию">♡ <span>Сохранить</span></button></section>
    </article>
  )
}

function SelectFilter({ label, name, value, items, placeholder, onChange }) {
  return (
    <label><span>{label}</span><select name={name} value={value} onChange={onChange}><option value="">{placeholder}</option>{items.map((item) => <option value={item.id} key={item.id}>{item.name}</option>)}</select></label>
  )
}

export default function AccountingCompaniesPage() {
  const [filters, setFilters] = useState(emptyFilters)
  const debouncedFilters = useDebouncedValue(filters)
  const [meta, setMeta] = useState({ directions: [], services: [], tax_systems: [] })
  const [result, setResult] = useState({ items: [], total: 0, page: 1, pages: 0 })
  const [page, setPage] = useState(1)
  const [limit, setLimit] = useState('10')
  const [sort, setSort] = useState('passport')
  const [reload, setReload] = useState(0)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  usePageStyles(['/static/accounting-company.css?v=5'])
  useDocumentPage({ title: 'Бухгалтерские компании — FinTalent' })

  useEffect(() => {
    const controller = new AbortController()
    getAccountingCompaniesMeta({ signal: controller.signal })
      .then((value) => setMeta(value || { directions: [], services: [], tax_systems: [] }))
      .catch(() => {})
    return () => controller.abort()
  }, [])

  useEffect(() => {
    const controller = new AbortController()
    setLoading(true)
    setError('')
    getAccountingCompanies({ ...debouncedFilters, page, limit }, { signal: controller.signal })
      .then((value) => setResult(value || { items: [], total: 0, page: 1, pages: 0 }))
      .catch((requestError) => {
        if (requestError.name !== 'AbortError') setError(requestError.message || 'Ошибка загрузки')
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false)
      })
    return () => controller.abort()
  }, [debouncedFilters, limit, page, reload, sort])

  const companies = useMemo(() => {
    const items = [...(result.items || [])]
    if (sort === 'passport') items.sort((left, right) => (right.passport_summary?.overall_index || 0) - (left.passport_summary?.overall_index || 0))
    return items
  }, [result.items, sort])

  const quickFilters = useMemo(() => [
    ...(meta.services || []).slice(0, 6).map((item) => ({ ...item, kind: 'service_id' })),
    ...(meta.directions || []).slice(0, 3).map((item) => ({ ...item, kind: 'direction_id' })),
  ], [meta])

  function updateFilter(event) {
    const { name, value } = event.target
    setFilters((current) => ({ ...current, [name]: value }))
    setPage(1)
  }

  function selectQuickFilter(kind, value) {
    setFilters((current) => ({ ...current, [kind]: String(value) }))
    setPage(1)
  }

  function selectPage(nextPage) {
    setPage(nextPage)
    window.scrollTo({ top: 260, behavior: 'smooth' })
  }

  return (
    <PublicLayout>
      <main className="ac-page ac-directory-page">
        <nav className="ac-directory-breadcrumbs"><a href="/">Главная</a><span>›</span><b>Компании</b></nav>
        <section className="ac-directory-top">
          <div className="ac-directory-title"><h1>Бухгалтерские компании</h1><p>Найдите надёжного партнёра для ведения бухгалтерии вашего бизнеса</p></div>
          <a className="ac-directory-promo" href="/accounting-companies/create"><span><b>Представьте свою компанию тысячам клиентов</b><small>Создайте страницу компании на FinTalent. Покажите услуги, тарифы и подтвердите экспертизу команды.</small><strong>Создать компанию <i>→</i></strong></span><i className="ac-promo-art" aria-hidden="true"><img src="/static/images/accounting-directory-promo.png" alt="" /></i></a>
        </section>
        <form id="ac-filters" className="ac-directory-filters" onSubmit={(event) => event.preventDefault()}>
          <label><span>Город</span><input name="city" value={filters.city} onChange={updateFilter} placeholder="Все города" /></label>
          <SelectFilter label="Направления" name="direction_id" value={filters.direction_id} items={meta.directions || []} placeholder="Все направления" onChange={updateFilter} />
          <SelectFilter label="Услуги" name="service_id" value={filters.service_id} items={meta.services || []} placeholder="Выберите услугу" onChange={updateFilter} />
          <SelectFilter label="Система налогообложения" name="tax_system_id" value={filters.tax_system_id} items={meta.tax_systems || []} placeholder="Все системы" onChange={updateFilter} />
          <fieldset><legend>Цена</legend><input name="price_from" value={filters.price_from} onChange={updateFilter} aria-label="Цена от" type="number" min="0" placeholder="от" /><input name="price_to" value={filters.price_to} onChange={updateFilter} aria-label="Цена до" type="number" min="0" placeholder="до" /><b>₽</b></fieldset>
          <button id="ac-reset" type="button" onClick={() => { setFilters(emptyFilters); setPage(1) }}>↻ Сбросить</button>
          <div id="ac-quick-filters" className="ac-directory-chips">{quickFilters.map((item, index) => <button type="button" data-kind={item.kind} data-value={item.id} onClick={() => selectQuickFilter(item.kind, item.id)} key={`${item.kind}-${item.id}`}><i>{chipIcons[index % chipIcons.length]}</i>{item.name}</button>)}<button type="button" data-more="">Ещё⌄</button></div>
        </form>
        <section className="ac-directory-toolbar"><p id="ac-found">{loading ? 'Загрузка…' : error ? 'Не удалось загрузить' : `Найдено компаний: ${result.total}`}</p><div><label>Сортировать:<select id="ac-sort" value={sort} onChange={(event) => setSort(event.target.value)}><option value="passport">По индексу компетенций</option><option value="new">Сначала новые</option></select></label><button type="button" className="active" aria-label="Компактный список">☷</button><button type="button" aria-label="Плитка">▦</button></div></section>
        <section id="ac-company-list" className="ac-directory-list">
          {loading ? <><div className="ac-skeleton" /><div className="ac-skeleton" /><div className="ac-skeleton" /></> : null}
          {!loading && error ? <div className="ac-empty"><i>!</i><h3>Ошибка загрузки</h3><p>{error}</p><button className="ac-button" type="button" onClick={() => setReload((current) => current + 1)}>Повторить</button></div> : null}
          {!loading && !error && !companies.length ? <div className="ac-empty"><i>⌕</i><h3>Компании не найдены</h3><p>Измените параметры или сбросьте фильтры.</p></div> : null}
          {!loading && !error ? companies.map((company) => <CompanyRow company={company} key={company.id} />) : null}
        </section>
        <footer className="ac-directory-footer"><nav id="ac-pagination" className="ac-pagination">{Array.from({ length: result.pages || 0 }, (_, index) => index + 1).filter((number) => number === 1 || number === result.pages || Math.abs(number - result.page) < 3).map((number) => <button type="button" className={number === result.page ? 'active' : ''} onClick={() => selectPage(number)} key={number}>{number}</button>)}</nav><label>Показывать по <select id="ac-limit" value={limit} onChange={(event) => { setLimit(event.target.value); setPage(1) }}><option>10</option><option>20</option></select></label></footer>
      </main>
    </PublicLayout>
  )
}
