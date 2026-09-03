import { Fragment, useCallback, useEffect, useMemo, useState } from 'react'
import { createPortal } from 'react-dom'
import { useParams } from 'react-router-dom'
import { ApiError } from '../../api/client'
import { createHelpRequest, getPublicResume, getResumeKnowledge, setResumeKnowledgeConfirmation } from '../../api/resumes'
import { useDocumentPage } from '../../hooks/useDocumentPage'
import usePageStyles from '../../hooks/usePageStyles'
import PublicLayout from '../../layouts/PublicLayout'

const monthNames = ['января', 'февраля', 'марта', 'апреля', 'мая', 'июня', 'июля', 'августа', 'сентября', 'октября', 'ноября', 'декабря']
const educationTypes = {
  higher: 'Высшее образование',
  incomplete_higher: 'Неоконченное высшее',
  secondary_vocational: 'Среднее профессиональное',
  secondary: 'Среднее образование',
  professional_retraining: 'Профессиональная переподготовка',
  course: 'Курсы',
  certificate: 'Сертификат',
  other: 'Другое',
}

function allDictionaries(data) {
  return (data?.blocks || []).flatMap((block) => block.dictionaries || [])
}

function positionNames(data) {
  const dictionary = allDictionaries(data).find((item) => /position|должност/i.test(`${item.alias} ${item.name}`))
  return dictionary?.items?.map((item) => item.value) || []
}

function russianCount(value, one, few, many) {
  const number = Math.abs(Number(value)) % 100
  const last = number % 10
  if (number > 10 && number < 20) return many
  if (last === 1) return one
  return last >= 2 && last <= 4 ? few : many
}

function totalExperience(items) {
  let months = 0
  for (const item of items || []) {
    const end = item.is_current ? new Date() : new Date(item.end_year, item.end_month - 1)
    months += Math.max(0, (end.getFullYear() - item.start_year) * 12 + end.getMonth() - (item.start_month - 1))
  }
  if (!months) return 'Не указан'
  const years = Math.floor(months / 12)
  const rest = months % 12
  const parts = []
  if (years) parts.push(`${years} ${russianCount(years, 'год', 'года', 'лет')}`)
  if (rest) parts.push(`${rest} ${russianCount(rest, 'месяц', 'месяца', 'месяцев')}`)
  return parts.join(' ')
}

function period(item) {
  const start = `${monthNames[item.start_month - 1]} ${item.start_year}`
  const end = item.is_current ? 'по настоящее время' : `${monthNames[(item.end_month || 1) - 1]} ${item.end_year}`
  return `${start} — ${end}`
}

function formatMoney(value) {
  return new Intl.NumberFormat('ru-RU', { maximumFractionDigits: 0 }).format(Number(value) || 0)
}

function findResumeDictionary(data, pattern) {
  return allDictionaries(data).find((dictionary) => pattern.test(`${dictionary.alias} ${dictionary.name}`))
}

function maximumNumber(values) {
  const numbers = (values || [])
    .flatMap((value) => (String(value).match(/\d[\d\s]*/g) || []).map((number) => Number(number.replace(/\s/g, ''))))
    .filter(Number.isFinite)
  return numbers.length ? String(Math.max(...numbers)) : '—'
}

function OptionIcon({ item }) {
  return item.icon ? <img src={item.icon} alt="" /> : <span className="fallback-icon">◇</span>
}

function Sidebar({ data, knowledgeAvailable }) {
  const [active, setActive] = useState('#overview')
  const hasHelp = Boolean(data.help?.topics?.length)
  const links = [
    { href: '#overview', icon: '▤', label: 'Обзор резюме' },
    ...(hasHelp ? [{ href: '#help', icon: '🤝', label: 'Могу помочь' }] : []),
    { href: '#skills', icon: '◇', label: 'Навыки и компетенции' },
    { href: '#experience', icon: '▣', label: 'Опыт работы' },
    { href: '#education', icon: '⌂', label: 'Образование' },
    { href: '#languages', icon: '文', label: 'Языки' },
    knowledgeAvailable
      ? { href: '#knowledge', icon: '◉', label: 'Тестирование' }
      : { href: '#', icon: '◉', label: 'Тестирование', disabled: true, soon: true },
    { href: '#', icon: '✎', label: 'Публикации', disabled: true, soon: true },
  ]

  return (
    <aside className="resume-view-sidebar">
      <nav>
        {links.map((link, index) => (
          <a
            className={`${active === link.href && !link.disabled ? 'active' : ''}${link.disabled ? ' disabled' : ''}`}
            href={link.href}
            onClick={() => { if (!link.disabled) setActive(link.href) }}
            key={`${link.label}-${index}`}
          >
            <i>{link.icon}</i><span>{link.label}</span>{link.soon ? <small>скоро</small> : null}
          </a>
        ))}
      </nav>
      <div className="resume-contact-card"><span>✦</span><b>Заинтересовал кандидат?</b><p>Свяжитесь и предложите обсудить профессиональные возможности.</p><button type="button">Связаться</button></div>
    </aside>
  )
}

function ResumePreferences({ data }) {
  const cities = data.cities || []
  const formats = data.work_formats || []
  if (!cities.length && !formats.length) return null
  return (
    <div className="resume-preferences-compact">
      {cities.length ? <div><img src="/static/icons/resume-profile/location.svg" alt="" /><span>{cities.map((city, index) => <Fragment key={city.id ?? city.name}>{index ? <i>·</i> : null}<b>{index === 0 ? '★ ' : ''}{city.name}</b></Fragment>)}</span></div> : null}
      {formats.length ? <div><img src="/static/icons/resume-profile/work-format.svg" alt="" /><span>{formats.map((format, index) => <Fragment key={format.id ?? format.value}>{index ? <i>·</i> : null}<b>{format.icon ? <img src={format.icon} alt="" /> : null}{format.value}</b></Fragment>)}</span></div> : null}
    </div>
  )
}

function WorkPreferences({ text }) {
  const value = String(text || '').trim()
  if (!value) return null
  return <div className="resume-work-note"><span><img src="/static/icons/resume-profile/work-preferences.svg" alt="" /></span><div><small>ПОЖЕЛАНИЯ К БУДУЩЕЙ РАБОТЕ</small><p>{value}</p></div></div>
}

function ZodiacBadge({ zodiac }) {
  if (!zodiac) return null
  return <span className="resume-zodiac-badge" title={zodiac.name} aria-label={`Знак зодиака: ${zodiac.name}`}><img src={`${zodiac.icon}?v=2`} alt="" /><span>{zodiac.name}</span></span>
}

function HeroMetric({ dictionary, value, fallback, defaultIcon }) {
  return (
    <div className="resume-kpi">
      <span className="resume-kpi-icon">{dictionary?.icon ? <img src={dictionary.icon} alt="" /> : <i>{defaultIcon}</i>}</span>
      <span><small>{dictionary?.name || fallback}</small><b>{value || '—'}</b></span>
    </div>
  )
}

function ResumeHero({ data, title }) {
  const experience = findResumeDictionary(data, /^experience |опыт работы/i)
  const legal = findResumeDictionary(data, /legal_entities_managed_total|юридических лиц/i)
  const turnover = findResumeDictionary(data, /maximum_company_turnover|максимальн.*оборот/i)
  const audits = findResumeDictionary(data, /tax_audits|налогов.*проверк/i)
  const languages = { name: 'Знание языков', icon: '/static/icons/dictionaries/kpi-languages.svg' }
  return (
    <section className="resume-hero" id="overview">
      <div className="resume-hero-main">
        {data.available_immediately || data.is_owner ? (
          <div className="resume-hero-controls">
            {data.available_immediately ? <span className="resume-ready-control"><i>✓</i><span><small>Готовность к работе</small><b>Может выйти сразу</b></span></span> : null}
            {data.is_owner ? <a className="resume-owner-edit" href="/resume/create" title="Редактировать резюме" aria-label="Редактировать резюме"><span>✎</span></a> : null}
          </div>
        ) : null}
        <div className="resume-avatar-wrap"><img className="resume-avatar" src={data.avatar || '/static/profile-3-avatar.png'} alt={data.name} /><i className="resume-online" /></div>
        <div className="resume-identity">
          <small className="resume-status-kicker">{data.search_status || 'Готов(а) к предложениям'}</small>
          <div className="resume-name-line"><h1>{data.name}</h1><ZodiacBadge zodiac={data.zodiac} /></div>
          <p className="resume-title">{title}</p>
          <ResumePreferences data={data} />
          <WorkPreferences text={data.work_preferences} />
        </div>
        <div className="resume-hero-stats">
          <h2>Ключевые показатели</h2>
          <HeroMetric dictionary={experience} value={totalExperience(data.experiences)} fallback="Опыт работы" defaultIcon="◷" />
          <HeroMetric dictionary={legal} value={maximumNumber(legal?.items?.map((item) => item.value))} fallback="На обслуживании" defaultIcon="▦" />
          <HeroMetric dictionary={turnover} value={turnover?.items?.map((item) => item.value).join(', ')} fallback="Максимальный оборот" defaultIcon="↗" />
          <HeroMetric dictionary={audits} value={audits?.items?.map((item) => item.value).join(', ')} fallback="Налоговые проверки" defaultIcon="✓" />
          <HeroMetric dictionary={languages} value={String(data.languages?.length || 0)} fallback="Знание языков" defaultIcon="文" />
        </div>
      </div>
      <div className="resume-hero-rail">
        <aside className="resume-hero-side resume-salary-redesign"><small>ЖЕЛАЕМАЯ ЗАРПЛАТА</small><strong>{formatMoney(data.desired_salary)} ₽</strong><p>на руки в месяц</p></aside>
        <section className="resume-hero-rail-empty resume-match-preview"><h2>Почему кандидат вам подходит</h2><div className="resume-match-summary"><div className="resume-match-ring"><span>86%</span></div><p>Ваше соответствие<br />на основе навыков<br />и опыта работы</p></div><div className="resume-match-scale"><i /></div><button type="button">Подробнее о соответствии <span>→</span></button></section>
      </div>
    </section>
  )
}

function SkillColumn({ title, kind, dictionaries }) {
  const items = dictionaries.flatMap((dictionary) => dictionary.items || [])
  const icon = kind === 'professional' ? '◇' : kind === 'software' ? '▦' : '◎'
  return (
    <section className={`skill-column skill-column--${kind}`}>
      <header><i>{icon}</i><h3>{title}</h3></header>
      <div className="skill-options">{items.length ? items.map((item) => <div className="skill-option" key={item.id ?? item.value}><OptionIcon item={item} /><span>{item.value}</span></div>) : <p className="resume-empty">Не указано</p>}</div>
    </section>
  )
}

function Skills({ data }) {
  const dictionaries = (data.blocks || [])
    .filter((block) => !/общая информация/i.test(block.name))
    .flatMap((block) => (block.dictionaries || []).map((dictionary) => ({ ...dictionary, blockName: block.name })))
    .filter((dictionary) => !/position|желаемая должность|должност|work_format|type_of_employment/i.test(`${dictionary.alias} ${dictionary.name}`))
  const software = dictionaries.filter((dictionary) => /software|программ/i.test(`${dictionary.alias} ${dictionary.name} ${dictionary.blockName}`))
  const crm = dictionaries.filter((dictionary) => /crm/i.test(`${dictionary.alias} ${dictionary.name} ${dictionary.blockName}`))
  const professional = dictionaries.filter((dictionary) => !software.includes(dictionary) && !crm.includes(dictionary))
  return (
    <section className="resume-card" id="skills">
      <div className="resume-card-head"><i>◇</i><div><h2>Навыки и профессиональные инструменты</h2><small>Компетенции, программы и направления работы</small></div></div>
      <div className="skill-groups"><SkillColumn title="Профессиональные навыки" kind="professional" dictionaries={professional} /><SkillColumn title="Программы" kind="software" dictionaries={software} /><SkillColumn title="CRM" kind="crm" dictionaries={crm} /></div>
    </section>
  )
}

function Experience({ data }) {
  return (
    <section className="resume-card" id="experience">
      <div className="resume-card-head"><i>▣</i><div><h2>Опыт работы</h2><small>Карьерный путь и профессиональные достижения</small></div></div>
      <div className="timeline">
        {data.experiences?.length ? data.experiences.map((item) => (
          <article className="timeline-item" key={item.id ?? `${item.company}-${item.start_year}`}>
            <header><h3>{item.position}</h3><span>{period(item)}</span></header>
            <div className="timeline-company">{item.company}{item.city ? ` · ${item.city}` : ''}{item.industry ? ` · ${item.industry}` : ''}</div>
            {item.responsibilities ? <p>{item.responsibilities}</p> : null}
            {item.achievements ? <p><b>Достижения:</b> {item.achievements}</p> : null}
            {item.duties?.length ? <div className="timeline-duties">{item.duties.map((duty, index) => <span key={`${duty}-${index}`}>{duty}</span>)}</div> : null}
          </article>
        )) : <p className="resume-empty">Опыт работы не указан</p>}
      </div>
    </section>
  )
}

function Languages({ data }) {
  return <section className="resume-card" id="languages"><div className="resume-card-head"><i>文</i><div><h2>Языки</h2><small>Выбранные кандидатом</small></div></div><div className="language-list">{data.languages?.length ? data.languages.map((item) => <span key={item.id ?? item.name}>{item.name}</span>) : <p className="resume-empty">Не указаны</p>}</div></section>
}

function Education({ data }) {
  return (
    <section className="resume-card" id="education">
      <div className="resume-card-head"><i>⌂</i><div><h2>Образование</h2><small>Основное и дополнительное</small></div></div>
      <div className="education-list">
        {data.education?.length ? data.education.map((item) => <article className="education-item" key={item.id ?? `${item.institution}-${item.end_year}`}><h3>{item.institution}</h3><p>{item.specialization || educationTypes[item.type] || 'Образование'}</p><small>{item.is_current ? 'Учится сейчас' : item.end_year ? `Год окончания: ${item.end_year}` : ''}{item.certificate ? ' · Документ загружен' : ''}</small></article>) : <p className="resume-empty">Образование не указано</p>}
      </div>
    </section>
  )
}

function HelpIcon({ item }) {
  const icon = String(item.icon || '').trim()
  return /^\/|^https?:\/\//i.test(icon) ? <img src={icon} alt="" /> : (icon || '◇')
}

function ReviewList({ reviews }) {
  if (!reviews.length) return <div className="resume-empty">Отзывов пока нет</div>
  return reviews.map((item) => (
    <article className="resume-help-review" key={item.id ?? `${item.created_at}-${item.author?.full_name}`}>
      <img src={item.author?.avatar || '/static/avatar-placeholder.svg'} alt="" />
      <div><b>{item.author?.full_name || 'Пользователь'}</b><small>{item.topic?.name || 'Направление'} · {'★'.repeat(Math.max(0, Math.min(5, Number(item.rating) || 0)))} · {new Date(item.created_at).toLocaleDateString('ru-RU')}</small></div>
      <p>{item.text || 'Без текста'}</p>
    </article>
  ))
}

function HelpModal({ data, mode, onClose }) {
  const topics = data.help?.topics || []
  const reviews = data.help?.reviews || []
  const [selected, setSelected] = useState(Number(topics[0]?.id) || 0)
  const [text, setText] = useState('')
  const [status, setStatus] = useState('form')
  const [error, setError] = useState('')

  async function submit() {
    setError('')
    try {
      setStatus('sending')
      await createHelpRequest({ resume_id: data.id, topic_id: selected, text: text.trim() })
      setStatus('sent')
    } catch (requestError) {
      if (requestError instanceof ApiError && requestError.status === 401) {
        window.location.href = '/login'
        return
      }
      setError(requestError.message || 'Не удалось отправить запрос')
      setStatus('form')
    }
  }

  const dialog = mode === 'reviews' ? (
    <div className="resume-help-dialog" role="dialog" aria-modal="true">
      <header><div><h2>Отзывы о помощи</h2><p>{reviews.length ? 'Реальные отзывы по завершенным обращениям' : 'Отзывы появятся после завершенных обращений.'}</p></div><button className="close" type="button" aria-label="Закрыть" onClick={onClose}>×</button></header>
      <div className="resume-help-review-list"><ReviewList reviews={reviews} /></div>
    </div>
  ) : status === 'sent' ? (
    <div className="resume-help-dialog" role="dialog" aria-modal="true"><header><div><h2>Запрос отправлен</h2><p>Статус обращения появится в личном кабинете в разделе “Помощь коллегам”.</p></div><button className="close" type="button" onClick={onClose}>×</button></header><footer><a className="primary" href="/profile?section=help">Открыть обращения</a></footer></div>
  ) : (
    <div className="resume-help-dialog" role="dialog" aria-modal="true">
      <header><div><h2>Попросить помощи</h2><p>Выберите направление и коротко опишите вопрос или ситуацию.</p></div><button className="close" type="button" aria-label="Закрыть" onClick={onClose}>×</button></header>
      <div className="resume-help-choice">{topics.map((item) => <button type="button" className={Number(item.id) === selected ? 'selected' : ''} onClick={() => setSelected(Number(item.id))} key={item.id}><HelpIcon item={item} /> {item.name}</button>)}</div>
      <textarea maxLength="4000" value={text} onChange={(event) => setText(event.target.value)} placeholder="Например: нужна подсказка по валютному контролю, документам или ответу на требование" />
      {error ? <p>{error}</p> : null}
      <footer><button type="button" className="cancel" onClick={onClose}>Отмена</button><button type="button" className="primary" disabled={status === 'sending'} onClick={submit}>Отправить запрос</button></footer>
    </div>
  )

  return createPortal(<div className="resume-help-modal" onClick={(event) => { if (event.target === event.currentTarget) onClose() }}>{dialog}</div>, document.body)
}

function ResumeHelp({ data }) {
  const topics = data.help?.topics || []
  const [modal, setModal] = useState('')
  if (!topics.length) return null
  const completed = Number(data.help?.stats?.completed) || 0
  const reviews = Number(data.help?.stats?.review_count) || 0
  const average = Number(data.help?.stats?.average) || 0
  return (
    <>
      <section className="resume-card resume-help-public" id="help">
        <div className="resume-help-top"><div className="resume-card-head"><i>🤝</i><div><h2>Могу помочь</h2><small>Темы, по которым специалист готов подсказать коллегам</small></div></div><div className="resume-help-stats"><button className="resume-help-stat" type="button" onClick={() => setModal('reviews')}><small>Помог(ла)</small><b>{completed} коллегам</b></button><button className="resume-help-stat" type="button" onClick={() => setModal('reviews')}><small>Отзывы</small><b>{reviews}{reviews ? ` · ${average.toFixed(1)}` : ''}</b></button></div></div>
        <div className="resume-help-cards">{topics.map((item) => <article className="resume-help-card" key={item.id}><i><HelpIcon item={item} /></i><span><b>{item.name}</b><small>{item.short_description || item.category || 'Готов(а) помочь по этому направлению'}</small></span></article>)}</div>
        <div className="resume-help-cta"><span>Опишите ситуацию, а специалист сможет принять запрос и продолжить общение внутри обращения.</span>{data.is_owner ? <a href="/resume/create">Редактировать направления</a> : <button type="button" onClick={() => setModal('request')}>Попросить помощи</button>}</div>
      </section>
      {modal ? <HelpModal data={data} mode={modal} onClose={() => setModal('')} /> : null}
    </>
  )
}

function knowledgeLevel(percent) {
  if (percent >= 90) return 'Экспертный'
  if (percent >= 80) return 'Продвинутый'
  if (percent >= 65) return 'Хороший'
  return 'Базовый'
}

function initials(name) {
  return String(name || '?').trim().split(/\s+/).slice(0, 2).map((part) => part[0]).join('').toUpperCase()
}

function KnowledgeResult({ item, data, onToggle, busy }) {
  const percent = Math.round(item.percent)
  const people = item.confirmers || []
  return (
    <article className={`knowledge-result ${item.passed ? 'knowledge-passed' : 'knowledge-completed'}`}>
      <header><span>{item.category || 'Профессиональные знания'}</span><i>{item.passed ? 'Тест пройден' : 'Результат сохранён'}</i></header>
      <h3>{item.title}</h3>
      <div className="knowledge-score"><div className="knowledge-ring" style={{ '--score': percent }}><strong>{percent}%</strong></div><div><b>{knowledgeLevel(percent)}</b><small>{item.passed ? 'Проходной балл набран' : 'Тест завершён, результат отображается в резюме'}</small></div></div>
      <div className="knowledge-confirmers"><span className="knowledge-avatars">{people.slice(0, 3).map((person, index) => person.avatar ? <img style={{ '--i': index }} src={person.avatar} alt={person.name} title={person.name} key={person.id ?? person.name} /> : <i style={{ '--i': index }} title={person.name} key={person.id ?? person.name}>{initials(person.name)}</i>)}{item.confirmations > 3 ? <b>+{item.confirmations - 3}</b> : null}</span><small>{item.confirmations ? `${item.confirmations} ${item.confirmations === 1 ? 'подтверждение' : 'подтверждений'}` : 'Пока без подтверждений'}</small></div>
      <footer><span><i>✓</i> {new Date(item.finished_at).toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric' })}</span>{data.can_confirm ? <button className={item.confirmed_by_me ? 'confirmed' : ''} disabled={busy} onClick={() => onToggle(item)}>{item.confirmed_by_me ? '✓ Знания подтверждены' : 'Подтвердить знания'}</button> : null}</footer>
    </article>
  )
}

function PreviewKnowledgeResult({ title, percent }) {
  return <article className="knowledge-result"><header><span>ПРОФЕССИОНАЛЬНЫЕ ЗНАНИЯ</span><i>Тест пройден</i></header><h3>{title}</h3><div className="knowledge-score"><div className="knowledge-ring" style={{ '--score': percent }}><strong>{percent}%</strong></div><div><b>{knowledgeLevel(percent)}</b><small>Проходной балл набран</small></div></div><div className="knowledge-confirmers"><span className="knowledge-avatars"><i style={{ '--i': 0 }}>АК</i><i style={{ '--i': 1 }}>МС</i></span><small>2 подтверждения</small></div><footer><span><i>✓</i> 01.08.2026</span></footer></article>
}

function EmptyKnowledge({ owner }) {
  return (
    <section className="resume-card resume-knowledge knowledge-empty-preview" id="knowledge">
      <div className="knowledge-title"><div><span>ТЕСТЫ И ЗНАНИЯ</span><h2>Подтверждённые профессиональные знания</h2><p>Результаты тестов укрепляют профессиональную репутацию</p></div></div>
      <div className="knowledge-preview-stage">
        <div className="knowledge-results knowledge-preview-results" aria-hidden="true"><PreviewKnowledgeResult title="Налоги и отчётность" percent={92} /><PreviewKnowledgeResult title="1С: Бухгалтерия" percent={87} /><PreviewKnowledgeResult title="Первичная документация" percent={84} /></div>
        <div className="knowledge-empty-overlay"><div className="knowledge-motivation-art" aria-hidden="true"><i className="knowledge-art-spark">✦</i><div className="knowledge-medal"><svg viewBox="0 0 100 100"><path d="M35 60 27 91l23-10 23 10-8-31" /><circle cx="50" cy="42" r="31" /></svg><strong>80%<small>и выше</small></strong></div><span className="knowledge-art-proof">✓ В профиле</span></div><div className="knowledge-motivation-copy"><span>{owner ? 'УСИЛЬТЕ СВОЙ ПРОФИЛЬ' : 'ПРОФЕССИОНАЛЬНАЯ РЕПУТАЦИЯ'}</span><h3>{owner ? 'Сделайте свою экспертизу заметной' : 'Подтверждённые знания скоро появятся'}</h3><p>{owner ? 'Пройдите короткий тест по своей специализации. Высокий результат подчеркнёт вашу квалификацию и повысит доверие к профилю.' : 'Владелец профиля ещё не проходил профессиональные тесты, но результаты будут выглядеть именно так.'}</p><div className="knowledge-motivation-benefits"><b>✓ Повышает профессиональный рейтинг</b><b>✓ Добавляется автоматически</b></div><a href="/marketplace">{owner ? 'Пройти первый тест' : 'Посмотреть тесты'} <b>→</b></a><small>{owner ? 'Выберите направление — результат от 80% появится здесь' : 'Тесты доступны по разным направлениям бухгалтерии'}</small></div></div>
      </div>
    </section>
  )
}

function Knowledge({ data, resumeId, onReload }) {
  const [busyTest, setBusyTest] = useState(0)
  const results = data.results || []

  async function toggle(item) {
    setBusyTest(item.test_id)
    try {
      await setResumeKnowledgeConfirmation(resumeId, item.test_id, item.confirmed_by_me)
      await onReload()
    } finally {
      setBusyTest(0)
    }
  }

  if (!results.length) return <EmptyKnowledge owner={data.is_owner} />
  return <section className="resume-card resume-knowledge knowledge-complete" id="knowledge"><div className="knowledge-title"><div><span>ТЕСТЫ И ЗНАНИЯ</span><h2>Подтверждённые профессиональные знания</h2><p>Результаты тестирования и рекомендации профессионального сообщества</p></div>{data.is_owner ? <a href="/marketplace">Пройти новый тест <span>→</span></a> : null}</div><div className="knowledge-results">{results.map((item) => <KnowledgeResult item={item} data={data} onToggle={toggle} busy={busyTest === item.test_id} key={item.test_id} />)}</div></section>
}

function ResumePageContent({ data, knowledge, resumeId, reloadKnowledge }) {
  const positions = positionNames(data)
  const title = positions.join(', ') || data.experiences?.[0]?.position || 'Финансовый специалист'
  return (
    <div className="resume-view-shell">
      <Sidebar data={data} knowledgeAvailable={Boolean(knowledge)} />
      <main className="resume-view-main">
        <div id="resume-view-content" className="resume-view-content">
          <ResumeHero data={data} title={title} />
          <ResumeHelp data={data} />
          <div className="resume-view-grid resume-view-grid-wide"><div className="resume-column"><Skills data={data} />{knowledge ? <Knowledge data={knowledge} resumeId={resumeId} onReload={reloadKnowledge} /> : null}</div></div>
          <div className="resume-career-grid"><div className="resume-career-primary"><Experience data={data} /><Languages data={data} /></div><Education data={data} /><section className="resume-card resume-career-placeholder" aria-hidden="true" /></div>
        </div>
      </main>
    </div>
  )
}

export default function ResumeViewPage() {
  const params = useParams()
  const resumeId = Number(params.id)
  const [resume, setResume] = useState(null)
  const [knowledge, setKnowledge] = useState(null)
  const [status, setStatus] = useState('loading')
  const [error, setError] = useState('')
  const positions = useMemo(() => resume ? positionNames(resume) : [], [resume])
  const title = resume ? (positions.join(', ') || resume.experiences?.[0]?.position || 'Финансовый специалист') : ''

  usePageStyles([
    '/static/resume-view.css?v=2',
    '/static/resume-view-header.css?v=1',
    '/static/resume-knowledge.css?v=3',
    '/static/resume-knowledge-empty.css?v=2',
    '/static/resume-zodiac.css?v=2',
    '/static/resume-help-public.css?v=1',
  ])
  useDocumentPage({ title: resume ? `${resume.name} — ${title} | FinTalent` : 'Резюме — FinTalent' })

  const reloadKnowledge = useCallback(async () => {
    const value = await getResumeKnowledge(resumeId)
    setKnowledge({ ...value, results: value?.results || [] })
  }, [resumeId])

  useEffect(() => {
    const controller = new AbortController()
    if (!resumeId) {
      setStatus('error')
      setError('Некорректная ссылка на резюме')
      return () => controller.abort()
    }
    setStatus('loading')
    setError('')
    Promise.all([
      getPublicResume(resumeId, { signal: controller.signal }),
      getResumeKnowledge(resumeId, { signal: controller.signal }).catch(() => null),
    ]).then(([resumeValue, knowledgeValue]) => {
      setResume(resumeValue)
      setKnowledge(knowledgeValue ? { ...knowledgeValue, results: knowledgeValue.results || [] } : null)
      setStatus('ready')
    }).catch((requestError) => {
      if (requestError.name !== 'AbortError') {
        setError(requestError.message || 'Не удалось загрузить резюме')
        setStatus('error')
      }
    })
    return () => controller.abort()
  }, [resumeId])

  return (
    <PublicLayout>
      {status === 'loading' ? <div className="resume-view-shell"><aside className="resume-view-sidebar" /><main className="resume-view-main"><div id="resume-view-content" className="resume-view-content"><div className="resume-view-loading"><i /><b>Собираем профессиональный профиль…</b></div></div></main></div> : null}
      {status === 'error' ? <div className="resume-view-shell"><aside className="resume-view-sidebar" /><main className="resume-view-main"><div id="resume-view-content" className="resume-view-content"><div className="resume-view-error"><h2>Резюме недоступно</h2><p>{error}</p><a href="/">Вернуться на главную</a></div></div></main></div> : null}
      {status === 'ready' && resume ? <ResumePageContent data={resume} knowledge={knowledge} resumeId={resumeId} reloadKnowledge={reloadKnowledge} /> : null}
    </PublicLayout>
  )
}
