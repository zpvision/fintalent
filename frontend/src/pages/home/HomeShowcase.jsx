import { useEffect, useState } from 'react'
import { apiClient } from '../../api/client'

const fallbackVacancies = [
  { id: 0, title: 'Главный бухгалтер', name: 'ООО «Финанс Групп»', city: 'Москва', salary: 150000 },
  { id: 0, title: 'Бухгалтер на участок (ОС и ТМЦ)', name: 'АО «Технопром»', city: 'Санкт-Петербург', salary: 90000 },
  { id: 0, title: 'Бухгалтер по расчету заработной платы', name: 'ООО «Альфа-Бизнес»', city: 'Казань', salary: 80000 },
]
const fallbackResumes = [
  { id: 0, name: 'Ирина Смирнова', title: 'Главный бухгалтер', city: 'Москва', salary: 150000, tags: ['1С:БУ', 'Налоговый учет', 'МСФО'] },
  { id: 0, name: 'Алексей Кузнецов', title: 'Бухгалтер на участок', city: 'Санкт-Петербург', salary: 95000, tags: ['1С:ЗУП', 'Excel', 'Отчетность'] },
  { id: 0, name: 'Мария Лебедева', title: 'Бухгалтер', city: 'Новосибирск', salary: 75000, tags: ['1С:БУ', 'Учет ТМЦ', 'Excel'] },
]
const testIcons = ['1C', '%', '▥', 'X', '₽', '✓']
const difficulty = { easy: 'Начальный', medium: 'Средний', hard: 'Продвинутый', expert: 'Эксперт' }

function money(value) {
  return new Intl.NumberFormat('ru-RU', { maximumFractionDigits: 0 }).format(value || 0)
}

function initials(name) {
  return (name || '').split(' ').filter(Boolean).map((part) => part[0]).slice(0, 2).join('').toUpperCase()
}

export default function HomeShowcase() {
  const [vacancies, setVacancies] = useState(fallbackVacancies)
  const [resumes, setResumes] = useState(fallbackResumes)
  const [tests, setTests] = useState(null)
  const [testsError, setTestsError] = useState(false)

  useEffect(() => {
    const controller = new AbortController()
    apiClient.get('/api/public/home-showcase', { cache: 'no-store', signal: controller.signal, redirectOnUnauthorized: false }).then((data) => {
      if (data?.vacancies?.length) setVacancies(data.vacancies.slice(0, 4).reverse())
      if (data?.resumes?.length) setResumes(data.resumes.slice(0, 4).reverse())
    }).catch(() => {})
    apiClient.get('/api/marketplace/tests', { cache: 'no-store', signal: controller.signal, redirectOnUnauthorized: false }).then((data) => {
      const values = Array.isArray(data) ? [...data] : []
      setTests(values.sort(() => Math.random() - 0.5).slice(0, 4))
    }).catch((error) => { if (error.name !== 'AbortError') setTestsError(true) })
    return () => controller.abort()
  }, [])

  return (
    <>
      <section className="cards container">
        <article className="panel" id="jobs"><header><h2>Актуальные вакансии</h2><a href="/vacancies">Смотреть все</a></header>{vacancies.map((item, index) => <a className="job" href={item.id ? `/vacancies/view?id=${item.id}` : '/vacancies'} key={`${item.id}-${item.title}-${index}`}><div className={`company c${index % 3 + 1}`}>{(item.name || '').slice(0, 2).toUpperCase()}</div><div><b>{item.title}</b><small>{item.name}</small><span>⌖ {item.city}　 ◇ Опубликована</span></div><strong>от {money(item.salary)} ₽</strong></a>)}<a className="more" href="/vacancies">Смотреть все вакансии　→</a></article>
        <article className="panel" id="candidates"><header><h2>Проверенные кандидаты</h2><a href="/resumes">Смотреть всех</a></header>{resumes.map((item, index) => <a className="candidate" href={item.id ? `/resume/view/${item.id}` : '/resumes'} key={`${item.id}-${item.name}-${index}`}><div className={`avatar av${index % 3 + 1}`}>{initials(item.name)}</div><div><b>{item.name}</b><small>{item.title}</small><span>⌖ {item.city || 'Россия'}</span><i>{(item.tags || []).slice(0, 4).join('　')}</i></div><strong>{money(item.salary)} ₽</strong></a>)}<a className="more" href="/resumes">Смотреть всех кандидатов　→</a></article>
        <article className="panel benefits"><header><h2>Преимущества Fin Talent</h2></header><div><i>♙</i><span><b>Целевые специалисты</b><small>Бухгалтеры, финансисты и руководители</small></span></div><div><i>♢</i><span><b>Проверенные навыки</b><small>Тестирование и верификация</small></span></div><div><i>ϟ</i><span><b>Быстрый подбор</b><small>Среднее время закрытия вакансии — 3 дня</small></span></div><div><i>◉</i><span><b>Прозрачные условия</b><small>Реальные зарплаты и честные компании</small></span></div><div><i>♧</i><span><b>Безопасность данных</b><small>Конфиденциальность и защита информации</small></span></div></article>
      </section>
      <section className="bottom container">
        <article className="panel tests"><h2>Проверка навыков и тесты</h2><p>Подтвердите свои знания и выделитесь среди других кандидатов</p><div className="test-list" aria-live="polite">{testsError ? <span className="test-loading">Не удалось загрузить тесты</span> : null}{tests === null && !testsError ? <span className="test-loading">Загружаем тесты…</span> : null}{tests?.length === 0 ? <span className="test-loading">В Маркетплейсе пока нет опубликованных тестов</span> : null}{tests?.map((test, index) => <a href={`/tests/take?id=${test.id}`} key={test.id}><i>{testIcons[index % testIcons.length]}</i><b>{test.title}</b><small>{difficulty[test.difficulty] || 'Тест'} · {test.question_count} вопр.</small></a>)}</div><a className="more" href="/marketplace">Пройти тесты　→</a></article>
        <a className="skills-promo" href="/marketplace"><span className="promo-kicker">ПРОФЕССИОНАЛЬНОЕ РАЗВИТИЕ</span><h2>Подтвердите навыки —<br />получайте больше приглашений</h2><p>Пройдите профессиональный тест и покажите работодателям свой реальный уровень.</p><strong>Выбрать тест <i>→</i></strong></a>
      </section>
    </>
  )
}
