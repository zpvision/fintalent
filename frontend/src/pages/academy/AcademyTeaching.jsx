import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import UserLayout from '../../layouts/UserLayout'
import usePageStyles from '../../hooks/usePageStyles'
import { profileStyles } from '../courses/shared'
import { cohortAPI } from '../../api/cohorts'
import { cohortStyles, CohortEmpty } from './CohortShared'
export default function AcademyTeaching() {
 const [items, setItems] = useState([]), [error, setError] = useState(''), [loading, setLoading] = useState(true)
 usePageStyles([...profileStyles, ...cohortStyles])
 useEffect(() => { cohortAPI.workspaces().then(setItems).catch(e => setError(e.message)).finally(() => setLoading(false)) }, [])
 return <UserLayout active="academy-teaching"><main className="course-page cohort-page"><header className="cohort-page-heading"><div><small>FINTALENT ACADEMY</small><h1>Преподавание</h1><p>Назначенные вам потоки и ученики.</p></div></header>{error && <p className="course-error" role="alert">{error}</p>}{loading ? <p role="status">Открываем ваши курсы…</p> : items.length ? <div className="cohort-card-grid">{items.map(c => <article className="cohort-card" key={c.id}><small>КАБИНЕТ ПРЕПОДАВАТЕЛЯ</small><h3>{c.title}</h3><p>Прогресс учеников и проверка работ в пределах назначенных прав.</p><Link className="course-primary" to={`/profile/courses/${c.id}/academy/cohorts`}>Открыть потоки →</Link><Link className="course-secondary" to={`/profile/courses/${c.id}/academy/reviews`}>Задания и проверки</Link></article>)}</div> : <CohortEmpty title="Назначенных курсов пока нет">Владелец курса может пригласить вас преподавателем или куратором потока.</CohortEmpty>}</main></UserLayout>
}
