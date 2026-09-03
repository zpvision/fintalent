import { useEffect, useMemo, useRef, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { getPublicVacancy } from '../../api/vacancies'
import { useDocumentPage } from '../../hooks/useDocumentPage'
import usePageStyles from '../../hooks/usePageStyles'
import PublicLayout from '../../layouts/PublicLayout'

const colorThemes = {
  blue: ['#1559f6', '#edf3ff'],
  green: ['#08a874', '#e9faf4'],
  violet: ['#7857d7', '#f2edff'],
  orange: ['#e47c17', '#fff3e6'],
  rose: ['#d94c7d', '#ffedf4'],
  teal: ['#078f91', '#e8f8f8'],
}

const importanceLabels = {
  required: 'Важно',
  preferred: 'Желательно',
  bonus: 'Будет плюсом',
}

function allDictionaries(vacancy) {
  return (vacancy?.blocks || []).flatMap((block) => block.dictionaries || [])
}

function findDictionary(vacancy, alias) {
  return allDictionaries(vacancy).find((dictionary) => dictionary.alias === alias)
}

function positionNames(vacancy) {
  const dictionary = findDictionary(vacancy, 'position')
  if (dictionary) return (dictionary.items || []).map((item) => item.value)
  if (vacancy?.blocks?.[0]) {
    return (vacancy.blocks[0].dictionaries || []).flatMap((item) => (item.items || []).map((option) => option.value))
  }
  return ['Вакансия']
}

function firstValue(vacancy, alias, fallback = 'Не указано') {
  return findDictionary(vacancy, alias)?.items?.[0]?.value || fallback
}

function formatMoney(value) {
  return Number(value).toLocaleString('ru-RU')
}

function salary(vacancy) {
  if (vacancy.salary_from != null && vacancy.salary_to != null) {
    return `${formatMoney(vacancy.salary_from)} — ${formatMoney(vacancy.salary_to)} ₽`
  }
  const value = vacancy.salary_from ?? vacancy.salary_to
  if (value == null) return 'Зарплата обсуждается'
  return `${vacancy.salary_to != null ? 'До' : 'От'} ${formatMoney(value)} ₽`
}

function minutes(seconds) {
  return seconds ? `${Math.ceil(Number(seconds) / 60)} мин` : 'Без ограничения'
}

function ItemImage({ path, alt = '' }) {
  if (!path) return '◇'
  return /^(\/|https?:|data:)/.test(path) ? <img src={path} alt={alt} /> : path
}

function ConditionIcon({ type }) {
  if (type === 'city') {
    return <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 21s6-5.2 6-11a6 6 0 1 0-12 0c0 5.8 6 11 6 11Z" /><circle cx="12" cy="10" r="2.3" /></svg>
  }
  if (type === 'format') {
    return <svg viewBox="0 0 24 24" aria-hidden="true"><rect x="3" y="4" width="18" height="12" rx="2" /><path d="M8 20h8M12 16v4" /></svg>
  }
  if (type === 'employment') {
    return <svg viewBox="0 0 24 24" aria-hidden="true"><rect x="3" y="7" width="18" height="13" rx="2" /><path d="M9 7V5.5A1.5 1.5 0 0 1 10.5 4h3A1.5 1.5 0 0 1 15 5.5V7M3 12h18M10 12v2h4v-2" /></svg>
  }
  return <svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="12" cy="12" r="9" /><path d="M12 7v5l3.5 2" /></svg>
}

function KeyConditions({ vacancy, onOpenMap }) {
  const items = [
    { key: 'city', label: 'Город', value: vacancy.city || 'Не указан', address: vacancy.address || '' },
    { key: 'format', label: 'Формат работы', value: firstValue(vacancy, 'work_format') },
    { key: 'employment', label: 'Условия работы', value: firstValue(vacancy, 'type_of_employment') },
    { key: 'experience', label: 'Опыт работы', value: firstValue(vacancy, 'experience') },
  ]

  return (
    <div className="vacancy-key-conditions">
      {items.map((item) => {
        const hasMap = item.key === 'city' && item.address
        const mapAddress = [item.value, item.address].filter(Boolean).join(', ')
        const openMap = () => hasMap && onOpenMap(mapAddress)
        return (
          <div
            className={`condition-${item.key}${hasMap ? ' condition-city--map' : ''}`}
            role={hasMap ? 'button' : undefined}
            tabIndex={hasMap ? 0 : undefined}
            aria-label={hasMap ? 'Показать адрес на карте' : undefined}
            onClick={openMap}
            onKeyDown={(event) => {
              if (hasMap && (event.key === 'Enter' || event.key === ' ')) {
                event.preventDefault()
                openMap()
              }
            }}
            key={item.key}
          >
            <i><ConditionIcon type={item.key} /></i>
            <span>
              <small>{item.label}</small>
              <b>{item.value}</b>
              {hasMap ? <em className="condition-address">{item.address}</em> : null}
            </span>
            {hasMap ? <u className="condition-map-hint">На карте</u> : null}
          </div>
        )
      })}
    </div>
  )
}

function Cooperation({ vacancy }) {
  if (!vacancy.accepts_individual_entrepreneur && !vacancy.accepts_self_employed) return null
  return (
    <div className="vacancy-cooperation">
      {vacancy.accepts_individual_entrepreneur ? <span><i>ИП</i>Можно работать как ИП</span> : null}
      {vacancy.accepts_self_employed ? <span><i>СЗ</i>Можно работать как самозанятый</span> : null}
    </div>
  )
}

function DictionaryView({ dictionary }) {
  const [color, background] = colorThemes[dictionary.selection_color] || colorThemes.blue
  return (
    <section className="dictionary-view" style={{ '--dict-color': color, '--dict-bg': background }}>
      <header><i><ItemImage path={dictionary.icon} alt={dictionary.name} /></i><b>{dictionary.name}</b></header>
      <div className="dictionary-options">
        {(dictionary.items || []).map((item) => (
          <div className="dictionary-option" key={item.id ?? item.value}>
            <i><ItemImage path={item.icon} alt={item.value} /></i>
            <span><b>{item.value}</b>{item.comment ? <small>{item.comment}</small> : null}</span>
            {item.importance !== 'required' ? <em className="importance-label">{importanceLabels[item.importance] || ''}</em> : null}
          </div>
        ))}
      </div>
    </section>
  )
}

function BlocksView({ vacancy }) {
  const hidden = new Set(['position', 'accounting_areas', 'software', 'crm', 'work_format', 'type_of_employment', 'experience'])
  const blocks = (vacancy.blocks || [])
    .map((block) => ({ ...block, dictionaries: (block.dictionaries || []).filter((dictionary) => !hidden.has(dictionary.alias)) }))
    .filter((block) => block.dictionaries.length)

  return blocks.map((block) => (
    <section className="vacancy-block" key={block.id ?? block.name}>
      <header><h3>{block.name}</h3><span>{block.dictionaries.reduce((sum, item) => sum + (item.items || []).length, 0)} параметров</span></header>
      {block.dictionaries.map((dictionary) => <DictionaryView dictionary={dictionary} key={dictionary.id ?? dictionary.alias} />)}
    </section>
  ))
}

function SpecialOption({ item, color, background }) {
  return (
    <div className="special-option" style={{ '--special-color': color, '--special-bg': background }}>
      <i><ItemImage path={item.icon} alt={item.value} /></i><span>{item.value}</span>
    </div>
  )
}

function SpecialDictionary({ vacancy, alias, title, sectionIcon, grouped = false }) {
  const dictionary = findDictionary(vacancy, alias)
  if (!dictionary?.items?.length) return null
  const [color, background] = colorThemes[dictionary.selection_color] || colorThemes.blue
  const groups = [
    { key: 'required', label: 'Обязательно', symbol: '✓' },
    { key: 'preferred', label: 'Желательно', symbol: '★' },
    { key: 'bonus', label: 'Преимущество', symbol: '+' },
  ].map((group) => ({ ...group, items: dictionary.items.filter((item) => item.importance === group.key) })).filter((group) => group.items.length)

  return (
    <section className="vacancy-section vacancy-special">
      <div className="vacancy-section-heading"><i>{sectionIcon}</i><div><h2>{title}</h2><p>{grouped ? 'Участки распределены по важности для работодателя' : `${dictionary.items.length} выбранных вариантов`}</p></div></div>
      {grouped ? (
        <div className="importance-groups">
          {groups.map((group) => (
            <section className={`importance-group ${group.key}`} key={group.key}>
              <header><span>{group.symbol}</span><b>{group.label}</b><small>{group.items.length}</small></header>
              <div className="special-options">{group.items.map((item) => <SpecialOption item={item} color={color} background={background} key={item.id ?? item.value} />)}</div>
            </section>
          ))}
        </div>
      ) : (
        <div className="special-options">{dictionary.items.map((item) => <SpecialOption item={item} color={color} background={background} key={item.id ?? item.value} />)}</div>
      )}
    </section>
  )
}

function DutiesView({ duties }) {
  if (!duties.length) return null
  return (
    <section className="vacancy-section">
      <div className="vacancy-section-heading"><i>✓</i><div><h2>Обязанности</h2><p>Задачи, которые предстоит решать на этой позиции</p></div></div>
      <div className="vacancy-duties-grid">
        {duties.map((group) => (
          <section className="duty-group" key={group.id ?? group.name}>
            <header><i><ItemImage path={group.icon} alt={group.name} /></i><b>{group.name}</b></header>
            <ul>{(group.duties || []).map((item, index) => <li key={`${item}-${index}`}>{item}</li>)}</ul>
          </section>
        ))}
      </div>
    </section>
  )
}

function TestHero({ test, count, vacancyId }) {
  if (!test) {
    return <section className="vacancy-test-hero"><h2>Расскажите о своём опыте</h2><p>После отклика работодатель сможет познакомиться с вашим профилем.</p></section>
  }
  return (
    <section className="vacancy-test-hero">
      <h2>Чтобы отклик попал к работодателю, нужно пройти тестирование</h2>
      <p>{count > 1 ? `Для вакансии назначено ${count} теста. Они проходят последовательно.` : 'Работодатель предлагает пройти короткий профессиональный тест.'}</p>
      <button className="fintalent-test-button" type="button" onClick={() => { window.location.href = `/tests/take?id=${test.id}&vacancy_id=${vacancyId}` }}>
        <span className="fintalent-test-button__arcs" aria-hidden="true"><span className="fintalent-test-button__arc fintalent-test-button__arc--one" /><span className="fintalent-test-button__arc fintalent-test-button__arc--two" /><span className="fintalent-test-button__arc fintalent-test-button__arc--three" /></span>
        <span className="fintalent-test-button__text">Пройти тестирование</span>
        <span className="fintalent-test-button__action" aria-hidden="true"><span className="fintalent-test-button__ring fintalent-test-button__ring--outer" /><span className="fintalent-test-button__ring fintalent-test-button__ring--middle" /><span className="fintalent-test-button__circle"><svg className="fintalent-test-button__arrow" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg"><path d="M5 12H19M19 12L13.5 6.5M19 12L13.5 17.5" stroke="currentColor" strokeWidth="2.3" strokeLinecap="round" strokeLinejoin="round" /></svg></span></span>
      </button>
      <div className="hero-test-metrics"><span><b>{minutes(test.time_limit_seconds)}</b><small>время</small></span><span><b>{test.question_count}</b><small>вопросов</small></span><span><b>★ {Number(test.rating).toFixed(1)}</b><small>{test.review_count} отзывов</small></span></div>
      <div className="hero-test-flow"><b>Как это работает</b><div><span><i>1</i><small>Пройдите<br />тестирование</small></span><em>→</em><span><i>2</i><small>Отклик уйдёт<br />работодателю</small></span><em>→</em><span><i>3</i><small>Работодатель увидит<br />ваш результат</small></span></div></div>
    </section>
  )
}

function Applications({ vacancy }) {
  const stats = vacancy.applications || { total: 0, passed: 0, not_passed: 0 }
  return (
    <section className="vacancy-side-card vacancy-applications sticky">
      <h2>Отклики и результаты</h2><p>Прозрачная статистика по вакансии</p>
      <div className="application-stats"><span><b>{Number(stats.total) || 0}</b><small>всего откликов</small></span><span><b>{Number(stats.passed) || 0}</b><small>прошли тест</small></span><span><b>{Number(stats.not_passed) || 0}</b><small>не прошли тест</small></span></div>
    </section>
  )
}

function MapModal({ address, onClose }) {
  const closeButton = useRef(null)

  useEffect(() => {
    document.body.classList.add('map-modal-open')
    closeButton.current?.focus()
    const handleKeyDown = (event) => {
      if (event.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', handleKeyDown)
    return () => {
      document.body.classList.remove('map-modal-open')
      document.removeEventListener('keydown', handleKeyDown)
    }
  }, [onClose])

  const encodedAddress = encodeURIComponent(address)
  return (
    <div className="vacancy-map-modal" onClick={(event) => { if (event.target === event.currentTarget) onClose() }}>
      <div className="vacancy-map-dialog" role="dialog" aria-modal="true" aria-label="Адрес вакансии">
        <header><div><small>МЕСТО РАБОТЫ</small><h2>{address}</h2></div><button ref={closeButton} type="button" className="vacancy-map-close" aria-label="Закрыть" onClick={onClose}>×</button></header>
        <div className="vacancy-map-frame"><iframe title="Адрес вакансии на Яндекс Картах" src={`https://yandex.ru/map-widget/v1/?text=${encodedAddress}&z=16`} loading="lazy" allowFullScreen /></div>
        <a href={`https://yandex.ru/maps/?text=${encodedAddress}`} target="_blank" rel="noopener noreferrer">Открыть маршрут в Яндекс Картах →</a>
      </div>
    </div>
  )
}

function Loading() {
  return <div className="vacancy-view-loading"><i /><b>Собираем вакансию…</b><span>Загружаем условия, обязанности и тестирование</span></div>
}

function ErrorView({ missingId, message }) {
  return (
    <section className="vacancy-view-error">
      <h1>{missingId ? 'Вакансия не выбрана' : 'Вакансия недоступна'}</h1>
      <p>{missingId ? 'Откройте вакансию из каталога или личного кабинета.' : message}</p>
      <a href="/">Вернуться на главную</a>
    </section>
  )
}

function VacancyContent({ vacancy, onOpenMap }) {
  const names = positionNames(vacancy)
  const tests = vacancy.tests || []
  const ownerName = vacancy.owner_name || 'Работодатель FinTalent'
  const published = new Date(vacancy.published_at).toLocaleDateString('ru-RU', { day: 'numeric', month: 'long', year: 'numeric' })

  return (
    <div className="vacancy-view-shell">
      <nav className="vacancy-breadcrumb"><a href="/">Главная</a><i>›</i><a href="/#jobs">Вакансии</a><i>›</i><span>{names.join(', ')}</span></nav>
      <div className="vacancy-top-layout">
        <section className="vacancy-hero">
          <div className="vacancy-identity">
            <span className="vacancy-eyebrow">Вакансия активна</span>
            <h1>{names.join(', ')}</h1>
            <p className="vacancy-salary">{salary(vacancy)} <small>{vacancy.salary_tax_mode === 'gross' ? 'до вычета налогов' : 'на руки'}</small></p>
            <Cooperation vacancy={vacancy} />
            <KeyConditions vacancy={vacancy} onOpenMap={onOpenMap} />
          </div>
          <TestHero test={tests[0]} count={tests.length} vacancyId={vacancy.id} />
        </section>
        <aside className="vacancy-top-owner">
          <span className="top-owner-label">РАБОТОДАТЕЛЬ</span>
          <div className="top-owner-person"><i>{ownerName.charAt(0).toUpperCase()}</i><div><b>{ownerName}</b><small>Проверенный пользователь</small></div><em>✓</em></div>
          <div className="top-owner-published"><i>◷</i><span><small>Вакансия опубликована</small><b>{published}</b></span></div>
          <p>Работодатель подробно заполнил условия вакансии и подготовил профессиональное тестирование.</p>
        </aside>
      </div>
      <div className="vacancy-body">
        <div className="vacancy-content">
          <SpecialDictionary vacancy={vacancy} alias="accounting_areas" title="Участки" sectionIcon="▦" grouped />
          <SpecialDictionary vacancy={vacancy} alias="software" title="Программы" sectionIcon="◫" />
          <SpecialDictionary vacancy={vacancy} alias="crm" title="CRM" sectionIcon="◎" />
          {vacancy.description ? <section className="vacancy-section"><div className="vacancy-section-heading"><i>✦</i><div><h2>О вакансии</h2><p>Дополнительная информация от работодателя</p></div></div><p className="vacancy-about">{vacancy.description}</p></section> : null}
          <DutiesView duties={vacancy.duties || []} />
          <section className="vacancy-section vacancy-requirements">
            <div className="vacancy-section-heading"><i>▦</i><div><h2>Условия и требования</h2><p>Ключевые параметры объёма и характера предстоящей работы</p></div><span className="requirements-badge">Важно знать до отклика</span></div>
            <BlocksView vacancy={vacancy} />
          </section>
        </div>
        <aside className="vacancy-aside">
          <Applications vacancy={vacancy} />
          <section className="vacancy-side-card"><h2>Почему стоит откликнуться</h2><div className="side-confidence"><i>{tests.length ? '✓' : '→'}</i><span><b>Подробные условия</b><small>Работодатель заполнил требования, задачи и формат сотрудничества.</small></span></div></section>
          <section className="vacancy-side-card"><h2>О работодателе</h2><p><b>{ownerName}</b><br /><br />Контакты откроются после отклика и прохождения назначенного тестирования.</p></section>
        </aside>
      </div>
    </div>
  )
}

export default function VacancyViewPage() {
  const [searchParams] = useSearchParams()
  const vacancyId = Number(searchParams.get('id'))
  const [vacancy, setVacancy] = useState(null)
  const [status, setStatus] = useState(vacancyId ? 'loading' : 'missing')
  const [error, setError] = useState('')
  const [mapAddress, setMapAddress] = useState('')
  const names = useMemo(() => vacancy ? positionNames(vacancy) : [], [vacancy])

  usePageStyles(['/static/vacancy-view.css?v=23', '/static/vacancy-contractors.css?v=1'])
  useDocumentPage({ title: vacancy ? `${names.join(', ')} — FinTalent` : 'Вакансия — FinTalent' })

  useEffect(() => {
    if (!vacancyId) {
      setStatus('missing')
      setVacancy(null)
      return undefined
    }
    const controller = new AbortController()
    setStatus('loading')
    setError('')
    getPublicVacancy(vacancyId, { signal: controller.signal })
      .then((value) => {
        setVacancy(value)
        setStatus('ready')
      })
      .catch((requestError) => {
        if (requestError.name !== 'AbortError') {
          setError(requestError.message || 'Не удалось загрузить вакансию')
          setStatus('error')
        }
      })
    return () => controller.abort()
  }, [vacancyId])

  return (
    <PublicLayout>
      <main id="vacancy-view">
        {status === 'loading' ? <Loading /> : null}
        {status === 'missing' ? <ErrorView missingId /> : null}
        {status === 'error' ? <ErrorView message={error} /> : null}
        {status === 'ready' && vacancy ? <VacancyContent vacancy={vacancy} onOpenMap={setMapAddress} /> : null}
      </main>
      {mapAddress ? <MapModal address={mapAddress} onClose={() => setMapAddress('')} /> : null}
    </PublicLayout>
  )
}
