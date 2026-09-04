import { useState } from 'react'
import { Link, useLocation } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'

const navigation = [
  { href: '/vacancies', label: 'Вакансии', paths: ['/vacancies'] },
  { href: '/resumes', label: 'Резюме', paths: ['/resumes', '/resume'] },
  { href: '/marketplace', label: 'Тесты', paths: ['/marketplace', '/tests'] },
  { href: '/profimarket', label: 'ПрофиМаркет', paths: ['/profimarket'] },
  { href: '/publications', label: 'Публикации', paths: ['/publications'] },
  { href: '/client-exchange', label: 'Клиентская биржа', paths: ['/client-exchange'] },
  { href: '/accounting-companies', label: 'Сообщества', paths: ['/accounting-companies'] },
]

function pathIsActive(pathname, paths) {
  return paths.some((path) => pathname === path || (path !== '/' && pathname.startsWith(path)))
}

export default function SiteHeader() {
  const { pathname } = useLocation()
  const { user, loading } = useAuth()
  const [menuOpen, setMenuOpen] = useState(false)
  const name = user?.full_name || user?.email || 'Профиль'
  const initial = name.trim().charAt(0).toUpperCase()

  return (
    <header className={`ft-site-header${menuOpen ? ' menu-open' : ''}`}>
      <Link className="ft-brand" to="/">
        <span className="ft-logo-crop"><img src="/static/logo.png" alt="" /></span>
        <span className="ft-brand-copy">
          <b>Fin<span>Talent</span></b>
          <small>Биржа вакансий для бухгалтеров</small>
        </span>
      </Link>
      <button
        className="ft-menu-toggle"
        type="button"
        aria-label="Открыть меню"
        aria-expanded={menuOpen}
        onClick={() => setMenuOpen((open) => !open)}
      >
        ☰
      </button>
      <nav className="ft-main-nav">
        {navigation.map((item) => {
          const className = pathIsActive(pathname, item.paths) ? 'active' : ''
          return item.legacy
            ? <a className={className} href={item.href} key={item.href}>{item.label}</a>
            : <Link className={className} to={item.href} key={item.href}>{item.label}</Link>
        })}
      </nav>
      <div className="ft-account">
        {user ? (
          <Link className="ft-profile" to="/profile">
            <i>{initial}</i>
            <span><small>Личный кабинет</small><b>{name}</b></span>
          </Link>
        ) : loading ? (
          <span className="ft-auth-loading" aria-label="Проверяем авторизацию" />
        ) : (
          <>
            <Link className="ft-login" to="/login">Войти</Link>
            <Link className="ft-register" to="/register">Регистрация</Link>
          </>
        )}
      </div>
    </header>
  )
}
