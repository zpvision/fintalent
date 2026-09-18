import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { apiClient } from '../api/client'
import { useAuth } from '../context/AuthContext'
import { navigateInApp } from '../navigation'
import SiteHeader from '../components/SiteHeader'
import { getMyProfiMarketOrderSummary } from '../api/profimarket'

const Icon = ({ children }) => <i aria-hidden="true">{children}</i>

export default function UserLayout({ children, active = '' }) {
  const { user, loading, refresh } = useAuth()
  const [resume, setResume] = useState(null)
  const [marketOrders,setMarketOrders]=useState({total_count:0,new_count:0})
  const name = String(user?.full_name || 'Пользователь').trim() || 'Пользователь'
  const initial = name.charAt(0).toUpperCase()

  useEffect(() => {
    apiClient.get('/api/v1/resumes/status', { redirectOnUnauthorized: false }).then(setResume).catch(() => {})
    getMyProfiMarketOrderSummary({redirectOnUnauthorized:false}).then(setMarketOrders).catch(()=>{})
  }, [])

  useEffect(()=>{const update=event=>setMarketOrders(event.detail);window.addEventListener('profimarket:orders-read',update);return()=>window.removeEventListener('profimarket:orders-read',update)},[])

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
    <div className="dashboard-layout"><aside className="profile-sidebar"><div className="sidebar-actions"><Link className="create-action vacancy-action" to="/vacancies/create"><strong>＋</strong><span><b>Создать вакансию</b><small>Найдите специалиста</small></span></Link><Link className="create-action resume-action" to={resumeHref}><strong>▣</strong><span><b>{resume?.published ? 'Мой профиль (просмотр)' : 'Создать профиль'}</b><small>{resume?.published ? 'Посмотреть опубликованный профиль' : 'Найдите работу мечты'}</small></span></Link></div>
      <nav className="profile-menu">
        <div className="menu-group"><a className="home-link" href="/"><Icon>⌂</Icon><span>На главную</span></a><a href="#"><Icon>◇</Icon><span>Сообщения</span><b className="menu-badge">8</b></a></div>
        <div className="menu-group"><small>Для компаний</small><a className={active === 'vacancies' ? 'active' : ''} href="/profile?section=vacancies" target="_self"><Icon>▣</Icon><span>Мои вакансии</span></a><a className={active === 'tests' ? 'active' : ''} href="/tests?tab=mine" target="_self"><Icon>♧</Icon><span>Мои тесты</span><b className="menu-new"><span className="menu-fire" aria-hidden="true">🔥</span>Новое</b></a><a className={active === 'employee-tests' ? 'active' : ''} href="/tests?tab=employees" target="_self"><Icon>♙</Icon><span>Тестирование сотрудников</span></a><a className={active === 'my-company' ? 'active' : ''} href="/profile?section=my-company" target="_self"><Icon>▦</Icon><span>Моя компания</span></a></div>
        <div className="menu-group"><small>О компании</small><a className={`profile-market-link${active === 'profimarket' ? ' active' : ''}`} href="/profile?section=profimarket" target="_self"><Icon>◇</Icon><span>ПрофиМаркет</span><b className="menu-new"><span className="menu-fire" aria-hidden="true">🔥</span>Новое</b>{marketOrders.new_count > 0 && <b className="market-order-count" aria-label={`${marketOrders.new_count} новых обращений`}>{marketOrders.new_count}</b>}</a><a className={active === 'profimarket-purchases' ? 'active' : ''} href="/profile?section=profimarket-purchases" target="_self"><Icon>▣</Icon><span>Мои покупки</span></a><a className={active === 'client-exchange' ? 'active' : ''} href="/profile?section=client-exchange" target="_self"><Icon>▤</Icon><span>Клиентская биржа</span></a><a className={active === 'help' ? 'active' : ''} href="/profile?section=help" target="_self"><Icon>♧</Icon><span>Помощь коллегам</span></a><a className={active === 'profile' ? 'active' : ''} href="/profile" target="_self"><Icon>▤</Icon><span>Профиль</span></a><a className={active === 'settings' ? 'active' : ''} href="/profile?section=settings" target="_self"><Icon>⚙</Icon><span>Настройки</span></a></div>
      </nav><div className="assistant-card"><div><i>✦</i><b>ИИ-помощник</b></div><p>Подберёт кандидатов под ваши задачи и сэкономит время</p><button>Попробовать</button></div><div className="sidebar-user"><i>{initial}</i><span><b>{name}</b><small>{user.email || ''}</small></span><button onClick={logout} title="Выйти">↪</button></div></aside>{children}</div>
  </>
}
