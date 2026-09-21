import { useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { apiClient } from '../../api/client'
import { useAuth } from '../../context/AuthContext'
import AuthLayout, { AuthSwitchLink } from '../../layouts/AuthLayout'
import { navigateInApp } from '../../navigation'

export default function LoginPage() {
  const { refresh } = useAuth()
  const [searchParams] = useSearchParams()
  const [showPassword, setShowPassword] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')

  async function handleSubmit(event) {
    event.preventDefault()
    const form = event.currentTarget
    setError('')
    if (!form.reportValidity()) return
    setSubmitting(true)
    try {
      await apiClient.post('/api/login', new FormData(form), { redirectOnUnauthorized: false })
      await refresh()
      const next = searchParams.get('next')
      navigateInApp(next?.startsWith('/') && !next.startsWith('//') ? next : '/')
    } catch (requestError) {
      setError(requestError.message || 'Не удалось войти')
      setSubmitting(false)
    }
  }

  return (
    <AuthLayout
      title="Вход — FinTalent"
      login
      aside={(
        <aside className="register-aside login-aside login-marketing">
          <div className="aside-glow" />
          <div className="register-aside-orbit orbit-one" />
          <div className="register-aside-orbit orbit-two" />
          <span className="register-aside-kicker"><i>✦</i> С ВОЗВРАЩЕНИЕМ В FINTALENT</span>
          <h2>Ваши возможности<br />уже <span>ждут вас</span></h2>
          <p className="register-aside-lead">Продолжайте развивать профессиональную страницу, делиться решениями и укреплять свою репутацию.</p>
          <div className="register-value-grid">
            <article><i>◎</i><div><b>Ваш профиль</b><p>Опыт и компетенции всегда под рукой</p></div></article>
            <article><i>✓</i><div><b>Ваши результаты</b><p>Тесты и подтверждённые знания сохранены</p></div></article>
            <article><i>◇</i><div><b>Ваши решения</b><p>Управляйте продуктами и заказами</p></div></article>
            <article><i>↗</i><div><b>Новые возможности</b><p>Возвращайтесь к предложениям и контактам</p></div></article>
          </div>
          <div className="register-aside-summary"><span><i>FT</i><b>Продолжайте с того места, где остановились</b></span><small>Всё важное сохранено в вашем аккаунте</small></div>
        </aside>
      )}
    >
      <section className="register-card">
        <div className="register-intro">
          <span className="register-icon login-icon" aria-hidden="true"><svg viewBox="0 0 24 24"><rect x="5" y="10" width="14" height="10" rx="2" /><path d="M8 10V7a4 4 0 0 1 8 0v3M12 14v2" /></svg></span>
          <h1>Добро пожаловать</h1>
          <p>Войдите в аккаунт, чтобы продолжить<br />работу с FinTalent</p>
        </div>
        <form className="register-form" method="post" action="/api/login" noValidate onSubmit={handleSubmit}>
          <label>
            Email
            <span className="input-wrap"><i>✉</i><input type="email" name="email" autoComplete="email" placeholder="name@example.ru" required /></span>
          </label>
          <a className="forgot-password-link" href="/forgot-password">Забыли пароль?</a>
          <label>
            Пароль
            <span className="input-wrap">
              <i>♢</i>
              <input type={showPassword ? 'text' : 'password'} name="password" autoComplete="current-password" placeholder="Введите пароль" required />
              <button type="button" className="show-password" aria-label={showPassword ? 'Скрыть пароль' : 'Показать пароль'} onClick={() => setShowPassword((shown) => !shown)}>{showPassword ? '⊘' : '◉'}</button>
            </span>
          </label>
          <div className={`form-message${error ? ' error' : ''}`} role="alert">{error}</div>
          <button className="submit-register" type="submit" disabled={submitting}>{submitting ? 'Входим…' : <>Войти <span>→</span></>}</button>
        </form>
        <AuthSwitchLink prompt="Нет аккаунта?" to={searchParams.get('next')?`/register?next=${encodeURIComponent(searchParams.get('next'))}`:'/register'}>Зарегистрироваться <span>→</span></AuthSwitchLink>
      </section>
    </AuthLayout>
  )
}
