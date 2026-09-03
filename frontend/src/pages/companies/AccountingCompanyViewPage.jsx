import { useEffect, useMemo, useRef, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { createAccountingCompanyReview, getAccountingCompany, getAccountingCompanyPassport } from '../../api/companies'
import { useDocumentPage } from '../../hooks/useDocumentPage'
import usePageStyles from '../../hooks/usePageStyles'
import PublicLayout from '../../layouts/PublicLayout'

const priceTypes = {
  from_month: 'от {p} / мес.',
  month: '{p} / мес.',
  from_hour: 'от {p} / час',
  hour: '{p} / час',
  from_once: 'от {p}',
  request: 'По запросу',
}

const iconMap = {
  laptop: '▣',
  briefcase: '◆',
  globe: '◎',
  store: '▤',
  factory: '▥',
  video: '▶',
  basket: 'WB',
  package: 'OZ',
  building: '▦',
  users: '♟',
  home: '⌂',
  market: '▦',
  monitor: '▣',
  gears: '⚙',
}

function money(value) {
  return value == null ? 'По запросу' : `${new Intl.NumberFormat('ru-RU').format(value)} ₽`
}

function servicePrice(service) {
  return (priceTypes[service.price_type] || '{p}').replace('{p}', money(service.price_from))
}

function websiteHref(value) {
  if (!value) return ''
  return /^https?:\/\//i.test(value) ? value : `https://${value}`
}

function ContactRow({ kind, label, value, href = '' }) {
  if (!value) return null
  const content = <><i>{kind}</i><span><b>{value}</b>{label ? <small>{label}</small> : null}</span></>
  return href
    ? <a className="ac-profile-contact-row" href={href} target="_blank" rel="noopener">{content}</a>
    : <div className="ac-profile-contact-row">{content}</div>
}

function SocialLink({ type, value, children }) {
  if (!value) return null
  const href = type === 'email' ? `mailto:${value}` : websiteHref(value)
  return <a className="ac-profile-social" href={href} target="_blank" rel="noopener">{children}</a>
}

function CompanyLogo({ company }) {
  return company.logo ? <img src={company.logo} alt="" /> : (company.name || 'Б').trim().charAt(0).toUpperCase()
}

function BrandCard({ company }) {
  const managerName = company.manager_name || 'Руководитель'
  return (
    <aside className="ac-profile-brand-card">
      <div className="ac-profile-logo"><CompanyLogo company={company} /></div>
      <h2>{company.name}</h2>
      <p>{company.short_description || 'Бухгалтерские услуги'}</p>
      <div className="ac-profile-manager">
        {company.manager_photo ? <img src={company.manager_photo} alt="" /> : <i>{managerName.trim().charAt(0).toUpperCase()}</i>}
        <span><b>{managerName}</b><small>{company.manager_position || 'Основатель и главный бухгалтер'}</small>{company.manager_description ? <a href="#about">Подробнее о руководителе →</a> : null}</span>
      </div>
    </aside>
  )
}

function ProfileIntro({ company, onContact }) {
  const taxes = (company.tax_systems || []).map((item) => item.name).join(', ') || 'Все системы налогообложения'
  const image = company.header_image || '/static/accounting-company-headers/header-01.jpg'
  return (
    <section className="ac-profile-intro">
      <div className="ac-profile-intro-copy">
        {company.verified ? <span className="ac-profile-verified">✓ Проверенная компания FinTalent</span> : null}
        <h1>{company.name} <button type="button" aria-label="Добавить в избранное">♡</button></h1>
        <p>{company.short_description || company.full_description || 'Комплексное бухгалтерское сопровождение бизнеса.'}</p>
        <div className="ac-profile-facts"><span>⌖ {company.city || 'Онлайн'}</span>{company.founded_year ? <span>▣ Работаем с {company.founded_year} года</span> : null}{company.employee_count ? <span>♟ {company.employee_count} специалистов</span> : null}{company.remote_all_russia ? <span>◎ Онлайн по всей России</span> : null}</div>
        <div className="ac-profile-tags">{company.tax_systems?.length ? company.tax_systems.slice(0, 3).map((item) => <span key={item.id ?? item.name}>{item.name}</span>) : <span>{taxes}</span>}{company.remote_all_russia ? <span>Онлайн</span> : null}</div>
        <div className="ac-profile-actions"><button className="ac-profile-primary" id="ac-contact" type="button" onClick={onContact}>✈ Связаться с компанией</button>{company.services?.length ? <a className="ac-profile-secondary" href="#services">Смотреть услуги</a> : null}<button className="ac-profile-more" type="button" aria-label="Больше действий">…</button></div>
      </div>
      <div className="ac-profile-visual" style={{ backgroundImage: `url("${image}")` }}><div><h3>Порядок в учёте —<br />уверенность в бизнесе</h3><p>Берём на себя рутину, чтобы вы могли развивать бизнес.</p></div></div>
    </section>
  )
}

function ContactsCard({ company }) {
  const hasContacts = Boolean(company.phone || company.email || company.website || company.address || company.city)
  return (
    <aside className="ac-profile-contacts">
      <h2>Контакты</h2>
      <div className="ac-profile-contact-list">{hasContacts ? <><ContactRow kind="☎" label={company.work_hours || 'Пн-Пт 9:00-18:00'} value={company.phone} href={company.phone ? `tel:${company.phone}` : ''} /><ContactRow kind="✉" value={company.email} href={company.email ? `mailto:${company.email}` : ''} /><ContactRow kind="◎" value={company.website} href={websiteHref(company.website)} /><ContactRow kind="⌖" value={company.address || company.city} /></> : <p>Компания пока не добавила контакты.</p>}</div>
      <div className="ac-profile-socials"><SocialLink type="whatsapp" value={company.whatsapp}>WA</SocialLink><SocialLink type="telegram" value={company.telegram}>TG</SocialLink><SocialLink type="vk" value={company.vk}>VK</SocialLink><SocialLink type="email" value={company.email}>✉</SocialLink></div>
    </aside>
  )
}

function Directions({ company }) {
  const [showAll, setShowAll] = useState(false)
  if (!company.directions?.length) return null
  return (
    <section className="ac-profile-section ac-profile-directions">
      <div className="ac-profile-section-head"><h2>Наши направления</h2>{company.directions.length > 8 && !showAll ? <button id="ac-all-directions" type="button" onClick={() => setShowAll(true)}>Смотреть все направления →</button> : null}</div>
      <div className="ac-profile-direction-list">{company.directions.map((item, index) => <article data-extra={index >= 8 ? '' : undefined} hidden={index >= 8 && !showAll} style={index >= 8 ? { display: showAll ? 'block' : 'none' } : undefined} key={item.id ?? item.name}><i>{iconMap[item.icon] || '◇'}</i><b>{item.name}</b></article>)}</div>
    </section>
  )
}

function Services({ company }) {
  const items = (company.services || []).slice(0, 7)
  return (
    <section className="ac-profile-panel" id="services">
      <div className="ac-profile-section-head"><h2>Услуги и цены</h2></div>
      <div className="ac-profile-service-list">{items.length ? items.map((item) => <div key={item.id ?? item.name}><span><i>{iconMap[item.icon] || '▧'}</i>{item.name}</span><b>{servicePrice(item)}</b></div>) : <p className="ac-profile-muted">Услуги и цены уточняются.</p>}</div>
      {company.services?.length > 7 ? <a className="ac-profile-link" href="#services">Смотреть все услуги →</a> : null}
    </section>
  )
}

function Tariffs({ company, onContact }) {
  const items = (company.tariffs || []).slice(0, 3)
  return (
    <section className="ac-profile-panel ac-profile-tariffs">
      <div className="ac-profile-section-head"><h2>Тарифы на бухгалтерское сопровождение</h2></div>
      <div className="ac-profile-tariff-grid">
        {items.length ? items.map((item) => <article className={`ac-profile-tariff ${item.popular ? 'popular' : ''}`} key={item.id ?? item.name}>{item.popular ? <em>Популярный</em> : null}<h3>{item.name || 'Тариф'}</h3><p>{item.subtitle || 'Для бизнеса'}</p><strong>{item.price == null ? 'По запросу' : money(item.price)} {item.price == null ? null : <small>{item.period || ''}</small>}</strong><ul>{(item.benefits || []).slice(0, 5).map((benefit, index) => <li key={`${benefit}-${index}`}>{benefit}</li>)}</ul><button type="button" data-calculate="" onClick={onContact}>Выбрать тариф</button></article>) : <p className="ac-profile-muted">Тарифы появятся после публикации компанией.</p>}
      </div>
      <p>Точная стоимость зависит от системы налогообложения и количества операций.</p>
      <a className="ac-profile-link" href="#contacts">Получить индивидуальный расчёт →</a>
    </section>
  )
}

function drawRadar(canvas, scores, color) {
  if (!canvas || scores.length < 3) return
  const context = canvas.getContext('2d')
  const width = canvas.width
  const height = canvas.height
  const centerX = width / 2
  const centerY = height / 2
  const radius = Math.min(width, height) * 0.31
  const count = scores.length
  context.clearRect(0, 0, width, height)
  context.font = '600 17px Inter'
  context.textAlign = 'center'
  context.textBaseline = 'middle'
  for (let level = 1; level <= 5; level += 1) {
    context.beginPath()
    for (let index = 0; index < count; index += 1) {
      const angle = -Math.PI / 2 + index * 2 * Math.PI / count
      const x = centerX + Math.cos(angle) * radius * level / 5
      const y = centerY + Math.sin(angle) * radius * level / 5
      if (index) context.lineTo(x, y)
      else context.moveTo(x, y)
    }
    context.closePath()
    context.strokeStyle = '#e4e9f4'
    context.stroke()
  }
  scores.forEach((score, index) => {
    const angle = -Math.PI / 2 + index * 2 * Math.PI / count
    context.beginPath()
    context.moveTo(centerX, centerY)
    context.lineTo(centerX + Math.cos(angle) * radius, centerY + Math.sin(angle) * radius)
    context.strokeStyle = '#edf1f7'
    context.stroke()
    const label = score.name.length > 16 ? `${score.name.slice(0, 15)}…` : score.name
    context.fillStyle = '#25306f'
    context.fillText(label, centerX + Math.cos(angle) * (radius + 54), centerY + Math.sin(angle) * (radius + 45))
  })
  context.beginPath()
  scores.forEach((score, index) => {
    const angle = -Math.PI / 2 + index * 2 * Math.PI / count
    const valueRadius = radius * score.percent / 100
    const x = centerX + Math.cos(angle) * valueRadius
    const y = centerY + Math.sin(angle) * valueRadius
    if (index) context.lineTo(x, y)
    else context.moveTo(x, y)
  })
  context.closePath()
  context.fillStyle = `${color}22`
  context.fill()
  context.strokeStyle = color
  context.lineWidth = 4
  context.stroke()
}

function PassportGraphic({ company, passport, color }) {
  const canvas = useRef(null)
  const scores = passport?.scores || []
  useEffect(() => {
    drawRadar(canvas.current, scores.slice(0, 12), color)
  }, [color, scores])
  if (!scores.length) {
    return (
      <div className="ac-profile-passport-empty">
        <div className="ac-passport-empty-copy"><span>Паспорт компетенций</span><b>Пока формируется</b><p>После тестирования сотрудников здесь появится общий индекс и карта компетенций компании.</p></div>
        <svg viewBox="0 0 340 260" role="img" aria-label="Пример графика компетенций"><g className="grid"><circle cx="170" cy="130" r="32" /><circle cx="170" cy="130" r="64" /><circle cx="170" cy="130" r="96" /><path d="M170 34v192M74 130h192M102 62l136 136M238 62L102 198" /></g><path className="area" d="M170 58 223 76 250 130 214 176 170 207 115 185 88 130 124 80Z" /><path className="line" d="M170 58 223 76 250 130 214 176 170 207 115 185 88 130 124 80Z" /><g className="points"><circle cx="170" cy="58" r="5" /><circle cx="223" cy="76" r="5" /><circle cx="250" cy="130" r="5" /><circle cx="214" cy="176" r="5" /><circle cx="170" cy="207" r="5" /><circle cx="115" cy="185" r="5" /><circle cx="88" cy="130" r="5" /><circle cx="124" cy="80" r="5" /></g></svg>
      </div>
    )
  }
  return <div className="ac-profile-passport-ready"><div className="ac-profile-passport-index"><span>Общий индекс</span><b>{Math.round(passport.overall_index)}%</b><p>На основе независимого тестирования сотрудников на FinTalent</p></div><canvas ref={canvas} className="ac-radar" id="ac-public-radar" width="620" height="620" /><div className="ac-profile-passport-legend"><span className="green">90% и выше</span><span className="orange">70-89%</span><span className="red">ниже 70%</span></div><a href={`/accounting-companies/passport?id=${company.id}`}>Подробнее →</a></div>
}

function PassportCard({ company, passport, color }) {
  const ready = Boolean(passport?.scores?.length)
  return <aside className="ac-profile-panel ac-profile-passport"><div className="ac-profile-section-head"><h2>Паспорт компетенций</h2>{ready ? <a href={`/accounting-companies/passport?id=${company.id}`}>Подробнее →</a> : null}</div><PassportGraphic company={company} passport={passport} color={color} /></aside>
}

function Reviews({ company }) {
  const items = (company.reviews || []).slice(0, 2)
  return <section className="ac-profile-panel ac-profile-reviews"><div className="ac-profile-section-head"><h2>Отзывы клиентов</h2>{company.reviews?.length ? <a href="#reviews">Смотреть все ({company.reviews.length}) →</a> : null}</div>{items.length ? items.map((review) => <article key={review.id ?? review.created_at}><i>“</i><p>{review.text}</p><footer><b>{review.author_name}</b><span>{review.author_company || ''}</span><small>{new Date(review.created_at).toLocaleDateString('ru-RU')}</small></footer></article>) : <p className="ac-profile-muted">Отзывы появятся после модерации.</p>}</section>
}

function About({ company }) {
  const advantages = company.advantages?.length ? company.advantages.slice(0, 5) : ['Индивидуальный подход', 'Всегда на связи', 'Профессиональная ответственность', 'Работаем с ЭДО и банками']
  return <section className="ac-profile-panel ac-profile-about" id="about"><h2>О компании</h2><p>{company.full_description || company.short_description || 'Компания оказывает бухгалтерские услуги для бизнеса.'}</p><div>{advantages.map((item, index) => <span key={`${item}-${index}`}>✓ {item}</span>)}</div></section>
}

function ContactModal({ company, mode, onClose }) {
  const [rating, setRating] = useState('5')
  const [authorCompany, setAuthorCompany] = useState('')
  const [text, setText] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')

  async function submitReview(event) {
    event.preventDefault()
    setSubmitting(true)
    setError('')
    try {
      const response = await createAccountingCompanyReview(company.id, { rating: Number(rating), author_company: authorCompany, text })
      setSuccess(response?.message || 'Отзыв отправлен на модерацию')
    } catch (requestError) {
      setError(requestError.message || 'Не удалось отправить отзыв')
      setSubmitting(false)
    }
  }

  return (
    <div id="ac-contact-modal" className="ac-modal" hidden={!mode} onClick={(event) => { if (event.target === event.currentTarget) onClose() }}>
      {mode === 'contact' ? <section className="ac-modal-box"><div className="ac-modal-head"><h2>Связаться с «{company.name}»</h2><button className="ac-modal-close" type="button" onClick={onClose}>×</button></div><p style={{ fontSize: '12px', color: '#7180a4' }}>Выберите удобный способ связи.</p><div className="ac-profile-contact-list" style={{ marginTop: '18px' }}><ContactRow kind="☎" value={company.phone} href={company.phone ? `tel:${company.phone}` : ''} /><ContactRow kind="✉" value={company.email} href={company.email ? `mailto:${company.email}` : ''} /><ContactRow kind="TG" value={company.telegram} href={websiteHref(company.telegram)} /><ContactRow kind="WA" value={company.whatsapp} href={websiteHref(company.whatsapp)} /></div></section> : null}
      {mode === 'review' ? <section className="ac-modal-box">{success ? <form id="ac-review-form"><div className="ac-success" style={{ padding: '25px' }}><i>✓</i><h2>Спасибо за отзыв</h2><p>{success}</p><button type="button" className="ac-button" onClick={onClose}>Закрыть</button></div></form> : <form id="ac-review-form" onSubmit={submitReview}><div className="ac-modal-head"><h2>Отзыв о «{company.name}»</h2><button type="button" className="ac-modal-close" onClick={onClose}>×</button></div><label className="ac-field" style={{ marginTop: '18px' }}><span>Оценка</span><select name="rating" required value={rating} onChange={(event) => setRating(event.target.value)}><option value="5">5 — Отлично</option><option value="4">4 — Хорошо</option><option value="3">3 — Нормально</option><option value="2">2 — Есть проблемы</option><option value="1">1 — Плохо</option></select></label><label className="ac-field" style={{ marginTop: '12px' }}><span>Ваша компания (необязательно)</span><input name="author_company" value={authorCompany} onChange={(event) => setAuthorCompany(event.target.value)} /></label><label className="ac-field" style={{ marginTop: '12px' }}><span>Отзыв</span><textarea name="text" minLength="10" required placeholder="Расскажите о качестве работы" value={text} onChange={(event) => setText(event.target.value)} /></label><button className="ac-button" style={{ width: '100%', marginTop: '14px' }} disabled={submitting}>Отправить отзыв</button><p id="ac-review-error" style={{ fontSize: '10px', color: '#b83246' }}>{error}</p></form>}</section> : null}
    </div>
  )
}

function CompanyContent({ company, passport }) {
  const [modal, setModal] = useState('')
  const color = company.accent?.color_value || '#6d35ff'
  return (
    <>
      <div className="ac-profile-shell" style={{ '--company-accent': color }}>
        <nav className="ac-profile-breadcrumbs"><a href="/">Главная</a><span>›</span><a href="/accounting-companies">Компании</a><span>›</span><span>Бухгалтерские компании</span><span>›</span><b>{company.name}</b></nav>
        <section className="ac-profile-hero"><BrandCard company={company} /><ProfileIntro company={company} onContact={() => setModal('contact')} /><ContactsCard company={company} /></section>
        <Directions company={company} />
        <section className="ac-profile-main-grid"><Services company={company} /><Tariffs company={company} onContact={() => setModal('contact')} /><PassportCard company={company} passport={passport} color={color} /><Reviews company={company} /><About company={company} /></section>
        <section className="ac-profile-panel ac-profile-review-cta"><div><h2>Работали с этой компанией?</h2><p>Поделитесь опытом — отзыв появится после модерации.</p></div><button className="ac-profile-secondary" id="ac-review-open" type="button" onClick={() => setModal('review')}>Оставить отзыв</button></section>
      </div>
      <ContactModal company={company} mode={modal} onClose={() => setModal('')} />
    </>
  )
}

export default function AccountingCompanyViewPage() {
  const [searchParams] = useSearchParams()
  const id = searchParams.get('id')
  const slug = searchParams.get('slug') || ''
  const key = useMemo(() => id ? encodeURIComponent(id) : `slug/${encodeURIComponent(slug)}`, [id, slug])
  const [company, setCompany] = useState(null)
  const [passport, setPassport] = useState({ scores: [] })
  const [status, setStatus] = useState('loading')
  const [error, setError] = useState('')

  usePageStyles(['/static/accounting-company.css?v=1', '/static/accounting-company-profile.css?v=2'])
  useDocumentPage({ title: company ? `${company.name} — FinTalent` : 'Бухгалтерская компания — FinTalent' })

  useEffect(() => {
    const controller = new AbortController()
    setStatus('loading')
    setError('')
    getAccountingCompany(key, { signal: controller.signal })
      .then(async (response) => {
        const value = response.company
        const passportValue = await getAccountingCompanyPassport(value.id, { signal: controller.signal }).catch(() => ({ scores: [] }))
        setCompany(value)
        setPassport(passportValue || { scores: [] })
        setStatus('ready')
      })
      .catch((requestError) => {
        if (requestError.name !== 'AbortError') {
          setError(requestError.message || 'Ошибка загрузки')
          setStatus('error')
        }
      })
    return () => controller.abort()
  }, [key])

  return (
    <PublicLayout>
      <main id="ac-company-view" className="ac-page">
        {status === 'loading' ? <div className="ac-loading-page"><div className="ac-skeleton" /></div> : null}
        {status === 'error' ? <div className="ac-empty"><i>!</i><h3>Страница компании не найдена</h3><p>{error}</p><a className="ac-button" href="/accounting-companies">Вернуться в каталог</a></div> : null}
        {status === 'ready' && company ? <CompanyContent company={company} passport={passport} /> : null}
      </main>
    </PublicLayout>
  )
}
