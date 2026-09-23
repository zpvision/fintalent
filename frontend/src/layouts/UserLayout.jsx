import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { apiClient } from '../api/client'
import { useAuth } from '../context/AuthContext'
import { navigateInApp } from '../navigation'
import SiteHeader from '../components/SiteHeader'
import { getMyProfiMarketOrderSummary } from '../api/profimarket'

const Icon = ({ children }) => <i aria-hidden="true">{children}</i>
const MenuFire = () => <span className="menu-fire" aria-hidden="true"><svg viewBox="0 0 24 24"><path className="flame-outer" d="M13.6 2.4c.5 3.1-1 4.4-2.3 5.7-1.1 1.1-2 2.1-1.5 4 .7-.4 1.4-1.1 1.8-2.1 2.5 1.7 4 3.9 4 6.3 0 3-2 5.2-5.1 5.2-3.6 0-6.1-2.5-6.1-6.1 0-4.5 3.4-6.8 6.2-9.3 1.1-1 2.2-2 3-3.7Z"/><path className="flame-inner" d="M11.2 13.2c.2 1.5-.6 2.1-1.1 2.8-.5.6-.7 1.3-.2 2.1.3-.3.6-.7.8-1.2 1.2.7 1.8 1.7 1.8 2.6 0 1.2-.8 2-2.1 2-1.5 0-2.5-1-2.5-2.4 0-2.1 1.8-3.6 3.3-5.9Z"/></svg></span>

export default function UserLayout({ children, active = '' }) {
  const { user, loading, refresh } = useAuth()
  const [resume, setResume] = useState(null)
  const [marketOrders,setMarketOrders]=useState({total_count:0,new_count:0})
  const [helpRequests,setHelpRequests]=useState(0)
  const name = String(user?.full_name || 'Пользователь').trim() || 'Пользователь'
  const initial = name.charAt(0).toUpperCase()

  useEffect(() => {
    apiClient.get('/api/v1/resumes/status', { redirectOnUnauthorized: false }).then(setResume).catch(() => {})
    getMyProfiMarketOrderSummary({redirectOnUnauthorized:false}).then(setMarketOrders).catch(()=>{})
  }, [])

  useEffect(() => {
    let active=true
    const update=()=>apiClient.get('/api/v1/help/notifications',{redirectOnUnauthorized:false}).then(data=>{if(active)setHelpRequests(data.incoming_new||0)}).catch(()=>{})
    update()
    const timer=window.setInterval(update,30000)
    window.addEventListener('focus',update)
    window.addEventListener('help:notifications-changed',update)
    return()=>{active=false;window.clearInterval(timer);window.removeEventListener('focus',update);window.removeEventListener('help:notifications-changed',update)}
  },[])

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
  const resumeHref = resume?.published ? `/profiles/view/${resume.id}` : '/profiles/create'

  return <>
    <SiteHeader />
    <div className="dashboard-layout"><aside className="profile-sidebar"><div className="sidebar-actions"><Link className="create-action vacancy-action" to="/vacancies/create"><strong>＋</strong><span><b>Создать вакансию</b><small>Найдите специалиста</small></span></Link><Link className="create-action resume-action" to={resumeHref}><strong>▣</strong><span><b>{resume?.published ? 'Мой профиль (просмотр)' : 'Создать профиль'}</b><small>{resume?.published ? 'Посмотреть опубликованный профиль' : 'Расскажите о своём опыте'}</small></span></Link></div>
      <nav className="profile-menu">
        <div className="menu-group"><a className="home-link" href="/"><Icon>⌂</Icon><span>На главную</span></a></div>
        <div className="menu-group"><small>Для компаний</small><a className={active === 'vacancies' ? 'active' : ''} href="/profile?section=vacancies" target="_self"><Icon>▣</Icon><span>Мои вакансии</span></a><a className={active === 'tests' ? 'active' : ''} href="/tests?tab=mine" target="_self"><Icon>♧</Icon><span>Мои тесты</span><b className="menu-new"><MenuFire/>Новое</b></a><a className={active === 'employee-tests' ? 'active' : ''} href="/tests?tab=employees" target="_self"><Icon>♙</Icon><span>Тестирование сотрудников</span></a><a className={active === 'my-company' ? 'active' : ''} href="/profile?section=my-company" target="_self"><Icon>▦</Icon><span>Моя компания</span></a></div>
        <div className="menu-group"><small>О компании</small><a className={`profile-market-link${active === 'profimarket' ? ' active' : ''}`} href="/profile?section=profimarket" target="_self"><Icon>◇</Icon><span>ПрофиМаркет</span><b className="menu-new"><MenuFire/>Новое</b>{marketOrders.new_count > 0 && <b className="market-order-count" aria-label={`${marketOrders.new_count} новых обращений`}>{marketOrders.new_count}</b>}</a><a className={active === 'profimarket-purchases' ? 'active' : ''} href="/profile?section=profimarket-purchases" target="_self"><Icon>▣</Icon><span>Мои покупки</span></a><a className={active === 'client-exchange' ? 'active' : ''} href="/profile?section=client-exchange" target="_self"><Icon>▤</Icon><span>Клиентская биржа</span></a><a className={active === 'help' ? 'active' : ''} href="/profile?section=help" target="_self"><Icon>♧</Icon><span>Помощь коллегам</span>{helpRequests>0&&<b className="help-request-count" aria-label={`${helpRequests} новых запросов помощи`}>{helpRequests>99?'99+':helpRequests}</b>}</a><a className={active === 'profile' ? 'active' : ''} href="/profile" target="_self"><Icon>▤</Icon><span>Профиль</span></a><a className={active === 'settings' ? 'active' : ''} href="/profile?section=settings" target="_self"><Icon>⚙</Icon><span>Настройки</span></a></div>
      </nav><div className="assistant-card"><div><i>✦</i><b>ИИ-помощник</b></div><p>Подберёт кандидатов под ваши задачи и сэкономит время</p><button>Попробовать</button></div><div className="sidebar-user"><i>{initial}</i><span><b>{name}</b><small>{user.email || ''}</small></span><button onClick={logout} title="Выйти">↪</button></div></aside>{children}</div>
  </>
}
