import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { apiClient } from '../api/client'
import { useAuth } from '../context/AuthContext'
import { navigateInApp } from '../navigation'
import SiteHeader from '../components/SiteHeader'

const Icon = ({ children }) => <i aria-hidden="true">{children}</i>

export default function UserLayout({ children, active = '' }) {
  const { user, loading, refresh } = useAuth()
  const [resume, setResume] = useState(null)
  const name = String(user?.full_name || 'Пользователь').trim() || 'Пользователь'
  const initial = name.charAt(0).toUpperCase()

  useEffect(() => {
    apiClient.get('/api/v1/resumes/status', { redirectOnUnauthorized: false }).then(setResume).catch(() => {})
  }, [])

  useEffect(() => {
    if (!loading && !user) navigateInApp(`/login?next=${encodeURIComponent(window.location.pathname + window.location.search)}`)
  }, [loading, user])

  async function logout() {
    await apiClient.post('/api/logout', null, { redirectOnUnauthorized: false }).catch(() => {})
    await refresh()
    navigateInApp('/')
  }

  if (loading || !user) return <><SiteHeader /><div className="loading">Загрузка…</div></>
  const resumeHref = resume?.published ? `/resume/view/${resume.id}` : '/resume/create'

  return <>
    <SiteHeader />
    <div className="dashboard-layout"><aside className="profile-sidebar"><div className="sidebar-actions"><Link className="create-action vacancy-action" to="/vacancies/create"><strong>＋</strong><span><b>Создать вакансию</b><small>Найдите специалиста</small></span></Link><Link className="create-action resume-action" to={resumeHref}><strong>▣</strong><span><b>{resume?.published ? 'Моё резюме (просмотр)' : 'Создать резюме'}</b><small>{resume?.published ? 'Посмотреть опубликованное резюме' : 'Найдите работу мечты'}</small></span></Link></div>
      <nav className="profile-menu">
        <div className="menu-group"><a className="home-link" href="/"><Icon>⌂</Icon><span>На главную</span></a><a href="#"><Icon>◇</Icon><span>Сообщения</span><b className="menu-badge">8</b></a></div>
        <div className="menu-group"><small>Для компаний</small><Link className={active === 'vacancies' ? 'active' : ''} to="/profile?section=vacancies"><Icon>▣</Icon><span>Мои вакансии</span></Link><Link className={active === 'tests' ? 'active' : ''} to="/tests"><Icon>♧</Icon><span>Тестирование</span><b className="menu-new"><span className="menu-fire" aria-hidden="true">🔥</span>Новое</b></Link><Link className={active === 'my-company' ? 'active' : ''} to="/profile?section=my-company"><Icon>▦</Icon><span>Моя компания</span></Link></div>
        <div className="menu-group"><small>О компании</small><Link className={`profile-market-link${active === 'profimarket' ? ' active' : ''}`} to="/profile?section=profimarket"><Icon>◇</Icon><span>ПрофиМаркет</span><b className="menu-new"><span className="menu-fire" aria-hidden="true">🔥</span>Новое</b></Link><Link className={active === 'client-exchange' ? 'active' : ''} to="/profile?section=client-exchange"><Icon>▤</Icon><span>Клиентская биржа</span></Link><Link className={active === 'help' ? 'active' : ''} to="/profile?section=help"><Icon>♧</Icon><span>Помощь коллегам</span></Link><Link className={active === 'profile' ? 'active' : ''} to="/profile"><Icon>▤</Icon><span>Профиль</span></Link><Link className={active === 'settings' ? 'active' : ''} to="/profile?section=settings"><Icon>⚙</Icon><span>Настройки</span></Link></div>
      </nav><div className="assistant-card"><div><i>✦</i><b>ИИ-помощник</b></div><p>Подберёт кандидатов под ваши задачи и сэкономит время</p><button>Попробовать</button></div><div className="sidebar-user"><i>{initial}</i><span><b>{name}</b><small>{user.email || ''}</small></span><button onClick={logout} title="Выйти">↪</button></div></aside>{children}</div>
  </>
}
