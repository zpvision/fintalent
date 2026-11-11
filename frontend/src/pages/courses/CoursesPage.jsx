import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { courseAPI } from '../../api/courses'
import UserLayout from '../../layouts/UserLayout'
import CourseCatalog from './CourseCatalog'
import usePageStyles from '../../hooks/usePageStyles'
import { useDocumentPage } from '../../hooks/useDocumentPage'
import { statuses, styles, profileStyles, EmptyArt } from './shared'
import Icon from '../../components/Icon'

export default function CoursesPage({ mine = false }) {
  usePageStyles(mine ? profileStyles : [...styles, '/static/course-catalog.css?v=2'])
  useDocumentPage({ title: `${mine ? 'Мои курсы' : 'Курсы'} — FinTalent` })
  const [items, setItems] = useState([]), [error, setError] = useState(''), [loading, setLoading] = useState(true), [busy, setBusy] = useState(0)
  const [category, setCategory] = useState(() => new URLSearchParams(window.location.search).get('category') || '')
  function load() { setLoading(true); setError(''); return (mine ? courseAPI.mine() : courseAPI.catalog()).then(setItems).catch(e => setError(e.message)).finally(() => setLoading(false)) }
  useEffect(() => { setLoading(true); load() }, [mine])
  async function act(c) { setBusy(c.id); setError(''); try { await courseAPI.action(c.id, ['published', 'moderation'].includes(c.status) ? 'hide' : 'submit'); await load() } catch (e) { setError(e.message) } finally { setBusy(0) } }
  if (!mine) return <CourseCatalog items={items} loading={loading} error={error} retry={load} />
  return <UserLayout active="courses"><main className="course-page course-dashboard">
    <header className="course-page-head"><div><small>FINTALENT · ОБУЧЕНИЕ</small><h1>Мои курсы</h1><p>Превратите свой опыт в знания, которыми хочется делиться.</p></div>{items.length > 0 && <Link className="course-primary" to="/profile/courses/create">＋ Создать курс</Link>}</header>
    {new URLSearchParams(window.location.search).get('academy') === 'moderation' && <p className="course-notice" role="status">Академия станет доступна после одобрения лендинга курса модератором.</p>}
    {error && <p className="course-error" role="alert">{error}</p>}
    {loading ? <p role="status">Загружаем курсы…</p> : items.length === 0 ? <section className="course-empty"><EmptyArt /><small>ЗНАНИЯ, КОТОРЫЕ МЕНЯЮТ КАРЬЕРУ</small><h2>Создайте свой первый курс</h2><p>Делитесь профессиональными знаниями, обучайте других специалистов и в дальнейшем продавайте свои курсы. Начните с красивой страницы — мы поможем с каждым шагом.</p><Link className="course-primary" to="/profile/courses/create">Создать мой первый курс <span>→</span></Link><div className="course-empty-steps"><span>01 · Расскажите о курсе</span><span>02 · Добавьте программу</span><span>03 · Поделитесь опытом</span></div></section> : <div className="course-grid">{items.map(c => { const academyAvailable = c.status === 'published' || Boolean(c.published_at); return <article className="course-list-card" key={c.id}>
      <Link to={`/profile/courses/${c.id}/preview`} className="course-card-cover" aria-label={`Предпросмотр лендинга: ${c.title || 'Новый курс'}`}>{c.cover_image ? <img src={c.cover_image} alt="" /> : <EmptyArt />}</Link>
      <div className="course-card-body">
        <h2>{c.title || 'Новый курс'}</h2>
        <section className="course-card-landing" aria-label="Лендинг курса">
          <div className="course-card-section-heading"><span className="course-card-section-icon"><Icon name="globe" /></span><h3>Лендинг курса</h3><span className={`course-badge ${c.status}`}>{statuses[c.status]}</span></div>
          <p>Страница с описанием и предложением курса</p>
          <div className="course-card-landing-actions">
            <Link className="course-secondary" to={`/profile/courses/${c.id}/edit`}>Редактировать</Link>
            <Link className="course-secondary" to={`/profile/courses/${c.id}/preview`}>Предпросмотр</Link>
            <button type="button" className="course-secondary course-card-publish" disabled={busy === c.id} onClick={() => act(c)}>{c.status === 'published' ? 'Снять с публикации' : c.status === 'moderation' ? 'Отозвать с модерации' : c.published_at ? 'Опубликовать снова' : 'Отправить на публикацию'}</button>
          </div>
        </section>
        <section className={`course-card-academy${academyAvailable ? '' : ' is-locked'}`} aria-label="Академия">
          <div className="course-card-section-heading"><span className="course-card-section-icon"><Icon name="book" /></span><div><h3>Академия</h3><p>Уроки, материалы и ученики</p></div></div>
          {academyAvailable ? <Link className="course-primary" to={`/profile/courses/${c.id}/academy`}>Открыть Академию <span aria-hidden="true">→</span></Link> : <>
            <button type="button" className="course-primary" disabled aria-describedby={`academy-access-note-${c.id}`}>Открыть Академию <Icon name="shield" /></button>
            <p className="course-card-access-note" id={`academy-access-note-${c.id}`}>{c.status === 'moderation' ? 'Лендинг на проверке. После одобрения здесь откроется управление обучением.' : 'Отправьте лендинг на модерацию — после одобрения здесь откроется управление обучением.'}</p>
          </>}
        </section>
      </div>
    </article>})}</div>}
  </main></UserLayout>
}
