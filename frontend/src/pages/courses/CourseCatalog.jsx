import { useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { useAuth } from '../../context/AuthContext'
import PublicLayout from '../../layouts/PublicLayout'
import Icon from '../../components/Icon'
import { levels, money, catalogPrice } from './shared'

function CatalogCard({ course }) {
  const href = `/courses/${course.slug}`
  const discounted = course.pricing_mode !== 'tariffs' && course.old_price > course.price
  return <article className="cc-card">
    <Link to={href} className={`cc-cover${course.cover_image ? '' : ' cc-cover-placeholder'}`} aria-label={`Подробнее о курсе «${course.title}»`}>
      {course.cover_image ? <img src={course.cover_image} alt="" loading="lazy" /> : <div className="cc-cover-art" aria-hidden="true"><Icon name="book" /><span>ЗНАНИЯ В ДЕЛЕ</span><i>✦</i></div>}
      <span className="cc-cover-category">{course.category || 'Профессиональное развитие'}</span>
      {discounted && <span className="cc-discount">−{Math.round((1 - course.price / course.old_price) * 100)}%</span>}
      <div className="cc-card-labels"><span>{levels[course.level] || 'Для специалистов'}</span>{course.certificate && <span><Icon name="award" />Сертификат</span>}</div>
    </Link>
    <div className="cc-card-body">
      <h3><Link to={href}>{course.title}</Link></h3>
      <p className="cc-description">{course.description}</p>
      <div className="cc-card-byline"><div className="cc-card-meta">{course.hours > 0 && <span><Icon name="clock" />{new Intl.NumberFormat('ru-RU').format(course.hours)} ч обучения</span>}{course.access_months > 0 && <span><Icon name="folder" />Доступ: {course.access_months} мес.</span>}</div>
      <div className="cc-author"><i aria-hidden="true">{(course.author_name || 'А').trim().charAt(0).toUpperCase()}</i><div><small>Автор курса</small><b>{course.author_name || 'Специалист FinTalent'}</b></div></div>
      </div><div className="cc-card-bottom"><div className="cc-card-price"><small>Стоимость курса</small><div><strong>{catalogPrice(course)}</strong>{discounted && <del>{money(course.old_price)}</del>}</div></div><Link to={href} className="cc-details">Посмотреть подробнее <span aria-hidden="true">↗</span></Link></div>
    </div>
  </article>
}

export default function CourseCatalog({ items, loading, error, retry }) {
  const { user } = useAuth()
  const [params, setParams] = useSearchParams()
  const [search, setSearch] = useState('')
  const category = params.get('category') || ''
  const categories = [...new Set(items.map(c => c.category).filter(Boolean))].sort((a, b) => a.localeCompare(b, 'ru'))
  const query = search.trim().toLocaleLowerCase('ru-RU')
  const filtered = items.filter(c => (!category || c.category === category) && (!query || [c.title, c.description, c.author_name].some(v => (v || '').toLocaleLowerCase('ru-RU').includes(query))))
  function chooseCategory(value) { const next = new URLSearchParams(params); value ? next.set('category', value) : next.delete('category'); setParams(next, { replace: true }) }
  function reset() { setSearch(''); chooseCategory('') }

  return <PublicLayout><main className="course-page course-catalog">
    <section className="cc-hero">
      <div className="cc-hero-copy"><span className="cc-eyebrow"><i aria-hidden="true" /> FINTALENT · КУРСЫ</span><h1>Новые знания.<br /><span>Новые возможности.</span></h1><p>Курсы для бухгалтеров и финансовых специалистов: новые навыки и идеи для ежедневной работы.</p><div className="cc-hero-actions"><a href="#course-catalog" className="cc-primary">Выбрать курс <span aria-hidden="true">→</span></a>{user && <Link to="/profile/courses" className="cc-my-courses">Мои курсы <span aria-hidden="true">↗</span></Link>}</div><div className="cc-hero-tags"><span><Icon name="briefcase" />Для вашей профессии</span><span><Icon name="lightbulb" />От знаний к практике</span><span><Icon name="globe" />Обучение онлайн</span></div></div>
      <div className="cc-hero-art" aria-hidden="true"><div className="cc-art-orbit" /><div className="cc-art-card"><div className="cc-art-top"><Icon name="book" /><span>ВАШ СЛЕДУЮЩИЙ ШАГ</span><i>✦</i></div><b>Знания,<br />которые<br /><em>двигают вперёд</em></b><div className="cc-art-lines"><span /><span /></div><div className="cc-art-footer"><span>FinTalent</span><i>↗</i></div></div><div className="cc-art-tag cc-art-finance"><Icon name="chart" /><span>Финансы и учёт</span></div><div className="cc-art-tag cc-art-growth"><Icon name="sparkles" /><span>Новые навыки</span></div></div>
    </section>
    <section id="course-catalog" className="cc-catalog-section" aria-labelledby="cc-title" aria-busy={loading}>
      <div className="cc-section-heading"><div><small>ВЫБЕРИТЕ СВОЁ НАПРАВЛЕНИЕ</small><h2 id="cc-title">Курсы для роста в профессии</h2></div>{!loading && !error && <span className="cc-count" role="status">Найдено курсов: {filtered.length}</span>}</div>
      <div className="cc-filters"><label className="cc-search"><span>Поиск курса</span><div><Icon name="search" /><input type="search" value={search} onChange={e => setSearch(e.target.value)} placeholder="Название, тема или автор" /></div></label><label className="cc-category"><span>Направление</span><select value={category} onChange={e => chooseCategory(e.target.value)}><option value="">Все направления</option>{category && !categories.includes(category) && <option value={category}>{category}</option>}{categories.map(name => <option key={name} value={name}>{name}</option>)}</select></label>{(search || category) && <button type="button" className="cc-reset" onClick={reset}>Сбросить</button>}</div>
      {error ? <div className="cc-empty" role="alert"><Icon name="message" /><h3>Не удалось загрузить курсы</h3><p>{error}</p><button className="cc-primary" type="button" onClick={retry}>Попробовать снова</button></div> : loading ? <><p className="cc-loading" role="status">Подбираем курсы…</p><div className="cc-grid" aria-hidden="true">{Array.from({ length: 6 }, (_, i) => <div className="cc-skeleton" key={i}><div /><span /><span /><span /></div>)}</div></> : filtered.length ? <div className="cc-grid">{filtered.map(c => <CatalogCard key={c.id} course={c} />)}</div> : <div className="cc-empty"><Icon name="book" /><h3>{items.length ? 'Попробуйте другое направление' : 'Первые курсы уже на подходе'}</h3><p>{items.length ? 'По вашему запросу курсов пока нет. Измените поиск или посмотрите все направления.' : 'Здесь появятся курсы от авторов FinTalent. Загляните сюда чуть позже — новые возможности для обучения впереди.'}</p>{items.length ? <button type="button" className="cc-primary" onClick={reset}>Показать все курсы</button> : user && <Link to="/profile/courses/create" className="cc-primary">Стать автором курса <span aria-hidden="true">↗</span></Link>}</div>}
    </section>
  </main></PublicLayout>
}
