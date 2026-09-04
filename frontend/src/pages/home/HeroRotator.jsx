import { useEffect, useRef, useState } from 'react'
import useTypingWords from '../../hooks/useTypingWords'
import '../../../../static/hero-rotator.css'

const roleWords = ['бухгалтеров', 'финансистов', 'руководителей', 'директоров']
const testingWords = ['сотрудников', 'по собственным тестам', 'с понятным результатом']
const profimarketWords = ['готовые решения', 'полезные шаблоны', 'опыт экспертов']
const clientExchangeWords = ['передача клиентов', 'новые клиенты', 'защищённая передача']

function VacancyHero({ active, leaving }) {
  const role = useTypingWords(roleWords, { initialDelay: 1500, deleteDelay: 62, typeDelay: 100, wordDelay: 1750, emptyDelay: 280 })
  return (
    <section className={`hero container hero-slide${active ? ' is-active' : ''}${leaving ? ' is-leaving' : ''}`} data-hero-slide="vacancies" aria-hidden={!active}>
      <div className="hero-copy">
        <div className="eyebrow">♢&nbsp; Проверенные специалисты <i /> Работа в финансах и управлении</div>
        <h1 aria-label="Биржа вакансий для бухгалтеров, финансистов, руководителей и директоров">Биржа вакансий<br />для <span className="typing-role">{role}</span></h1>
        <p>Быстрый подбор персонала и лучшие вакансии<br />в сфере финансов, учёта и управления.</p>
        <div className="hero-actions">
          <a className="action blue resume-action" href="/resume/create"><strong>▣</strong><span><b>Разместить резюме</b><small>Найдите работу мечты</small></span></a>
          <a className="action green resume-action" href="/vacancies/create"><strong>▢</strong><span><b>Разместить вакансию</b><small>Найдите специалиста</small></span></a>
        </div>
      </div>
      <div className="hero-visual">
        <img src="/static/hero-professionals-v3.png" alt="Специалисты в финансах и управлении" />
        <aside className="hero-speed-card" aria-label="Быстрый подбор за три дня">
          <small>БЫСТРЫЙ ПОДБОР</small><b><strong>3</strong> дня</b><span>от публикации до первых подходящих кандидатов</span>
          <svg viewBox="0 0 142 48" role="img" aria-label="График роста"><defs><linearGradient id="speed-fill" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stopColor="#00c689" stopOpacity=".28" /><stop offset="1" stopColor="#00c689" stopOpacity="0" /></linearGradient></defs><path className="speed-area" d="M2 43 C15 41 17 31 29 34 S44 41 54 28 S70 20 79 27 S96 35 106 19 S122 24 140 5 L140 48 L2 48 Z" /><path className="speed-line" d="M2 43 C15 41 17 31 29 34 S44 41 54 28 S70 20 79 27 S96 35 106 19 S122 24 140 5" /><circle cx="140" cy="5" r="3" /></svg>
        </aside>
        <a className="hero-promo-card" href="/resume/create"><i>✦</i><span><small>ВАШ СЛЕДУЮЩИЙ ШАГ</small><b>Создайте сильное резюме</b><em>Расскажите о своём опыте и станьте заметнее для работодателей</em><strong>Начать <u>→</u></strong></span></a>
      </div>
    </section>
  )
}

function TestingHero({ active, leaving }) {
  const text = useTypingWords(testingWords, { enabled: active })
  return (
    <section className={`hero container hero-slide employee-testing-hero${active ? ' is-active' : ''}${leaving ? ' is-leaving' : ''}`} data-hero-slide="testing" aria-hidden={!active}>
      <div className="testing-hero-bg"><img src="/static/employee-testing-hero.png?v=2" alt="Команда проходит профессиональное тестирование" /></div>
      <div className="testing-hero-copy">
        <div className="testing-eyebrow"><i>✓</i> ОЦЕНКА КОМАНДЫ В FINTALENT</div>
        <h1>Тестирование<br /><span>{text}</span></h1>
        <p>Назначайте собственные тесты по персональной ссылке<br />и получайте понятные результаты по каждому сотруднику.</p>
        <div className="testing-hero-actions"><a className="testing-primary" href="/tests?tab=employees"><span>Назначить тест</span><i>→</i></a><a className="testing-secondary" href="/tests?tab=mine">Создать свой тест</a></div>
        <div className="testing-benefits"><span>Уникальная ссылка</span><span>Контроль времени</span><span>Результаты команды</span></div>
      </div>
    </section>
  )
}

function ProfiMarketHero({ active, leaving }) {
  const text = useTypingWords(profimarketWords, { enabled: active })
  return (
    <section className={`hero container hero-slide profimarket-hero${active ? ' is-active' : ''}${leaving ? ' is-leaving' : ''}`} data-hero-slide="profimarket" aria-hidden={!active}>
      <div className="profimarket-hero-bg"><img src="/static/profimarket-hero.png" alt="Эксперт создаёт профессиональные решения для бизнеса" /></div>
      <div className="profimarket-hero-copy">
        <div className="profimarket-eyebrow"><i>✦</i> ПРОФИМАРКЕТ FINTALENT</div>
        <h1>ПрофиМаркет —<br /><span>{text}</span></h1>
        <p>Используйте готовые материалы для работы<br />или делитесь своими решениями с другими.</p>
        <div className="profimarket-hero-actions"><a className="profimarket-primary" href="/profimarket"><span>Найти решение</span><i>→</i></a><a className="profimarket-secondary" href="/profimarket/create">Создать решение</a></div>
        <div className="profimarket-benefits"><span>Практика экспертов</span><span>Готово к внедрению</span><span>Новые возможности</span></div>
      </div>
    </section>
  )
}

function ClientExchangeHero({ active, leaving }) {
  const text = useTypingWords(clientExchangeWords, { enabled: active })
  return (
    <section className={`hero container hero-slide client-exchange-hero${active ? ' is-active' : ''}${leaving ? ' is-leaving' : ''}`} data-hero-slide="client-exchange" aria-hidden={!active}>
      <div className="client-exchange-hero-bg" aria-hidden="true">
        <div className="ce-transfer-scene"><div className="ce-glow ce-glow-one" /><div className="ce-glow ce-glow-two" />
          <article className="ce-listing-preview"><div className="ce-preview-match"><i>✓</i> 91% подходит вашей компании</div><div className="ce-preview-top"><i className="ce-preview-icon">▤</i><div><h3>Интернет магазин</h3><span>Москва · УСН · НДС: нет</span></div></div><div className="ce-preview-metrics"><div><small>Выручка</small><b>до 30 млн ₽</b></div><div><small>Сотрудники</small><b>5-10</b></div><div><small>Абонплата</small><b>38 000 ₽</b></div></div><div className="ce-preview-tags"><span>Озон</span><span>ВБ</span><span>1С</span><span>ЭДО</span></div><div className="ce-preview-reason"><small>Причина передачи</small><b>Клиент слишком маленький для нашей компании</b></div><div className="ce-preview-transfer">Передам за 15% в течение 2 месяцев</div></article>
          <aside className="ce-preview-modal"><span>Просмотр карточки</span><b>Контакты скрыты</b><small>Откроются после выбора компании</small></aside>
        </div>
      </div>
      <div className="client-exchange-copy"><div className="client-exchange-eyebrow"><i>◇</i> НОВЫЙ РАЗДЕЛ FINTALENT</div><h1>Клиентская биржа<br /><span>{text}</span></h1><p>Передавайте клиентов на обслуживание или находите новые проекты для своей бухгалтерской компании: условия видны заранее, контакты открываются только после выбора.</p><div className="client-exchange-actions"><a className="client-exchange-primary" href="/client-exchange"><span>Открыть биржу</span><i>→</i></a><a className="client-exchange-secondary" href="/client-exchange/create">Передать клиента</a></div><div className="client-exchange-benefits"><span>Обезличенные карточки</span><span>Отклики компаний</span><span>Безопасная передача</span></div></div>
    </section>
  )
}

export default function HeroRotator() {
  const [active, setActive] = useState(0)
  const [leaving, setLeaving] = useState(null)
  const [paused, setPaused] = useState(false)
  const leaveTimer = useRef(null)
  const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches
  const slides = [VacancyHero, TestingHero, ProfiMarketHero, ClientExchangeHero]
  const labels = ['Биржа вакансий', 'Тестирование сотрудников', 'ПрофиМаркет', 'Клиентская биржа']

  function show(index) {
    if (index === active) return
    window.clearTimeout(leaveTimer.current)
    setLeaving(active)
    setActive(index)
    leaveTimer.current = window.setTimeout(() => setLeaving(null), 850)
  }

  useEffect(() => {
    if (paused || reducedMotion) return undefined
    const timer = window.setInterval(() => {
      setActive((current) => {
        window.clearTimeout(leaveTimer.current)
        setLeaving(current)
        leaveTimer.current = window.setTimeout(() => setLeaving(null), 850)
        return (current + 1) % slides.length
      })
    }, 10000)
    return () => window.clearInterval(timer)
  }, [paused, reducedMotion, slides.length])

  useEffect(() => () => window.clearTimeout(leaveTimer.current), [])

  return (
    <div className="hero-rotator" onMouseEnter={() => setPaused(true)} onMouseLeave={() => setPaused(false)} onFocus={() => setPaused(true)} onBlur={(event) => { if (!event.currentTarget.contains(event.relatedTarget)) setPaused(false) }}>
      {slides.map((Slide, index) => <Slide active={active === index} leaving={leaving === index} key={labels[index]} />)}
      <div className="hero-rotator-dots" aria-label="Рекламные предложения">{labels.map((label, index) => <button className={active === index ? 'active' : ''} type="button" aria-label={label} onClick={() => show(index)} key={label} />)}</div>
    </div>
  )
}
