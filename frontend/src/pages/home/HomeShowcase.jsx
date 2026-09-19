import { useEffect, useState } from 'react'
import { apiClient } from '../../api/client'
import { getClientExchangeListings } from '../../api/clientExchange'
import { getProfiMarketSolutions } from '../../api/profimarket'

const fallbackVacancies = [
  { id: 0, title: 'Главный бухгалтер', name: 'ООО «Финанс Групп»', city: 'Москва', salary: 150000 },
  { id: 0, title: 'Бухгалтер на участок (ОС и ТМЦ)', name: 'АО «Технопром»', city: 'Санкт-Петербург', salary: 90000 },
  { id: 0, title: 'Бухгалтер по расчёту заработной платы', name: 'ООО «Альфа-Бизнес»', city: 'Казань', salary: 80000 },
]
const fallbackSolutions = [
  { slug: '', type: 'AI-ассистент', title: 'Помощник бухгалтера по первичным документам', short_description: 'Разбирает документы и помогает быстро подготовить проводки.', tone: 'ai' },
  { slug: '', type: 'Регламент', title: 'Закрытие месяца без авралов', short_description: 'Пошаговый регламент, контрольные точки и готовые чек-листы.', tone: 'regulation' },
  { slug: '', type: 'Шаблон', title: 'Финансовая модель для малого бизнеса', short_description: 'Планирование выручки, расходов и движения денежных средств.', tone: 'template' },
]
const fallbackClients = [
  { id: '', title: 'Интернет-магазин', city: 'Москва', tax_system: { name: 'УСН' }, revenue: { name: 'до 30 млн ₽' }, current_monthly_fee: 38000, transferLabel: '15% · 2 месяца' },
  { id: '', title: 'Сеть кофеен', city: 'Казань', tax_system: { name: 'УСН + патент' }, revenue: { name: 'до 60 млн ₽' }, current_monthly_fee: 52000, transferLabel: '120 000 ₽' },
  { id: '', title: 'Оптовая торговля', city: 'Екатеринбург', tax_system: { name: 'ОСНО' }, revenue: { name: 'до 120 млн ₽' }, current_monthly_fee: 74000, transferLabel: 'По договорённости' },
  { id: '', title: 'Студия дизайна', city: 'Санкт-Петербург', tax_system: { name: 'УСН' }, revenue: { name: 'до 15 млн ₽' }, current_monthly_fee: 29000, transferLabel: '60 000 ₽' },
]
const fallbackTests = [
  { id: 0, title: 'Бухгалтерский учёт и отчётность', difficulty: 'medium', question_count: 24 },
  { id: 0, title: 'Налоги и расчёты с бюджетом', difficulty: 'hard', question_count: 20 },
  { id: 0, title: '1С:Бухгалтерия 8.3', difficulty: 'medium', question_count: 18 },
  { id: 0, title: 'Excel для финансового специалиста', difficulty: 'easy', question_count: 16 },
  { id: 0, title: 'Управленческий учёт', difficulty: 'medium', question_count: 20 },
  { id: 0, title: 'Расчёт заработной платы', difficulty: 'hard', question_count: 22 },
  { id: 0, title: 'Финансовый анализ', difficulty: 'expert', question_count: 18 },
  { id: 0, title: 'Первичная документация', difficulty: 'easy', question_count: 16 },
]
const testIcons = ['1C', '%', '▥', 'X', '₽', '✓']
const difficulty = { easy: 'Начальный', medium: 'Средний', hard: 'Продвинутый', expert: 'Эксперт' }
const productLabels = { AI_ASSISTANT: 'ИИ-ассистент', REGULATION: 'Регламент', AUTOMATION: 'Автоматизация', INSTRUCTION: 'Инструкция', ONEC_INTEGRATION: '1С Интеграция', TEMPLATE: 'Шаблон', CHECKLIST: 'Чек-лист' }

function money(value) {
  return new Intl.NumberFormat('ru-RU', { maximumFractionDigits: 0 }).format(value || 0)
}

function solutionTone(type) {
  return { AI_ASSISTANT: 'ai', REGULATION: 'regulation', AUTOMATION: 'automation', TEMPLATE: 'template', CHECKLIST: 'checklist' }[type] || type || 'instruction'
}

function pickHomepageSolutions(items, limit = 3) {
  const shuffled = [...items]
  for (let index = shuffled.length - 1; index > 0; index -= 1) {
    const swapIndex = Math.floor(Math.random() * (index + 1))
    const current = shuffled[index]
    shuffled[index] = shuffled[swapIndex]
    shuffled[swapIndex] = current
  }
  const owners = new Set()
  return shuffled.filter((item, index) => {
    const owner = item.author_user_id ? `owner:${item.author_user_id}` : `solution:${item.id || item.slug || index}`
    if (owners.has(owner)) return false
    owners.add(owner)
    return true
  }).slice(0, limit)
}

function clientDeal(item) {
  if (item.transferLabel) return item.transferLabel
  if (item.transfer_type?.code === 'fixed') return `${money(item.transfer_price)} ₽`
  if (item.transfer_type?.code === 'monthly_commission') return `${item.monthly_commission_percent || 0}% ежемесячно`
  if (item.transfer_type?.code === 'term_commission') return `${item.monthly_commission_percent || 0}% · ${item.commission_months || 0} мес.`
  return item.transfer_type?.name || 'По договорённости'
}

export default function HomeShowcase() {
  const [vacancies, setVacancies] = useState(fallbackVacancies)
  const [tests, setTests] = useState(fallbackTests)
  const [solutions, setSolutions] = useState(fallbackSolutions)
  const [clients, setClients] = useState(fallbackClients)
  const [helpTopics, setHelpTopics] = useState([])

  useEffect(() => {
    const controller = new AbortController()
    apiClient.get('/api/public/home-showcase', { cache: 'no-store', signal: controller.signal, redirectOnUnauthorized: false }).then((data) => {
      if (data?.vacancies?.length) setVacancies(data.vacancies.slice(0, 4).reverse())
    }).catch(() => {})
    apiClient.get('/api/marketplace/tests', { cache: 'no-store', signal: controller.signal, redirectOnUnauthorized: false }).then((data) => {
      const values = Array.isArray(data) ? [...data] : []
      setTests(values.sort(() => Math.random() - 0.5).slice(0, 8))
    }).catch(() => {})
    getProfiMarketSolutions({}, { signal: controller.signal }).then((data) => {
      if (data?.items?.length) setSolutions(pickHomepageSolutions(data.items))
    }).catch(() => {})
    getClientExchangeListings('page=1&limit=4&sort=new', { signal: controller.signal, redirectOnUnauthorized: false }).then((data) => {
      if (data?.items?.length) setClients(data.items.slice(0, 4))
    }).catch(() => {})
    apiClient.get('/api/public/help-topics', { signal: controller.signal, redirectOnUnauthorized: false }).then((data) => {
      if (Array.isArray(data)) setHelpTopics([...data].sort(() => Math.random() - 0.5).slice(0, 8))
    }).catch(() => {})
    return () => controller.abort()
  }, [])

  return (
    <div className="home-showcase">
      <section className="cards container home-primary-grid">
        <article className="panel" id="jobs">
          <header><h2>Актуальные вакансии</h2><a href="/vacancies">Смотреть все</a></header>
          {vacancies.map((item, index) => <a className="job" href={item.id ? `/vacancies/view?id=${item.id}` : '/vacancies'} key={`${item.id}-${item.title}-${index}`}><div className={`company c${index % 3 + 1}`}>{(item.name || '').slice(0, 2).toUpperCase()}</div><div><b>{item.title}</b><small>{item.name}</small><span>⌖ {item.city}　 ◇ Опубликована</span></div><strong>от {money(item.salary)} ₽</strong></a>)}
          <a className="more" href="/vacancies">Смотреть все вакансии　→</a>
        </article>

        <article className="panel tests home-tests">
          <header><div><small>ОЦЕНКА КОМПЕТЕНЦИЙ</small><h2>Проверка навыков и тесты</h2></div><a href="/marketplace">Все тесты</a></header>
          <p>Подтвердите знания и покажите работодателям свой реальный уровень.</p>
          <div className="test-list" aria-live="polite">
            {tests?.map((test, index) => <a href={test.id ? `/tests/take?id=${test.id}` : '/marketplace'} key={`${test.id}-${test.title}`}><i>{testIcons[index % testIcons.length]}</i><span><b>{test.title}</b><small>{difficulty[test.difficulty] || 'Тест'} · {test.question_count} вопр.</small></span></a>)}
          </div>
          <a className="more" href="/marketplace">Выбрать тест　→</a>
        </article>

        <article className="panel home-profimarket">
          <header><div><small>ГОТОВЫЕ РЕШЕНИЯ ЭКСПЕРТОВ</small><h2>ПрофиМаркет</h2></div><a href="/profimarket">Все решения</a></header>
          <div className="home-product-list">
            {solutions.map((item, index) => <a href={item.slug ? `/profimarket/solution/${encodeURIComponent(item.slug)}` : '/profimarket'} className="home-product" key={`${item.slug}-${item.title}-${index}`}>
              <div className={`home-product-cover ${solutionTone(item.type || item.tone)}`}>{item.cover_image ? <img src={item.cover_image} alt={item.title} loading="lazy" /> : <><span>{index === 0 ? 'AI' : index === 1 ? 'PRO' : 'DOC'}</span><i>✦</i></>}</div>
              <div><small>{productLabels[item.type] || item.type}</small><b>{item.title}</b><p>{item.short_description}</p></div>
            </a>)}
          </div>
          <a className="more" href="/profimarket">Перейти в раздел →</a>
        </article>
      </section>

      <section className="container home-promo-row">
        <article className="home-consultation">
          <header><span>ПОМОЩЬ КОЛЛЕГ</span><h2>Нужна консультация?</h2></header>
          <div className="home-consultation-grid">{helpTopics.map((topic) => <a href={`/profiles?help_topic=${topic.id}`} key={topic.id}><i>{/^\/|^https?:\/\//i.test(topic.icon || '') ? <img src={topic.icon} alt="" /> : (topic.icon || '◇')}</i><b>{topic.name}</b><small>{topic.category || 'Консультация'}</small></a>)}</div>
          <a className="home-consultation-more" href="/profiles">Найдите специалиста по нужному направлению <span>→</span></a>
        </article>
        <a className="skills-promo home-skills-banner" href="/marketplace"><span className="promo-kicker">ПРОФЕССИОНАЛЬНОЕ РАЗВИТИЕ</span><h2>Подтвердите навыки —<br />получайте больше приглашений</h2><p>Пройдите профессиональный тест и покажите работодателям свой реальный уровень.</p><strong>Выбрать тест <i>→</i></strong></a>
      </section>

      <section className="home-client-exchange">
        <div className="container">
          <header><div><small>НОВЫЕ ВОЗМОЖНОСТИ ДЛЯ БУХГАЛТЕРСКИХ КОМПАНИЙ</small><h2>Клиентская биржа</h2><p>Выберите клиента с понятными условиями передачи и заранее оцените объём работы.</p></div><a href="/client-exchange">Перейти на биржу <span>→</span></a></header>
          <div className="home-client-grid">
            {clients.map((item, index) => <a className="home-client-card" href={item.id ? `/client-exchange?listing=${item.id}` : '/client-exchange'} key={`${item.id}-${item.title}-${index}`}>
              <div className="home-client-top"><i>{['🛒', '☕', '▦', '✦'][index % 4]}</i><span><small>{item.industry?.name || 'Клиент на сопровождение'}</small><b>{item.title || item.industry?.name || 'Новый клиент'}</b><em>{item.city || 'Россия'} · {item.tax_system?.name || 'Система уточняется'}</em></span></div>
              <div className="home-client-metrics"><span><small>Выручка</small><b>{item.revenue?.name || 'уточняется'}</b></span><span><small>Абонплата</small><b>{money(item.current_monthly_fee)} ₽</b></span></div>
              <footer><div className="home-client-deal"><i>₽</i><span><small>Передача за</small><b>{clientDeal(item)}</b></span></div><strong>Открыть <i>→</i></strong></footer>
            </a>)}
          </div>
        </div>
      </section>
    </div>
  )
}
