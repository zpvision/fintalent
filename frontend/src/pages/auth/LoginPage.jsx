import { useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { apiClient } from '../../api/client'
import AuthLayout, { AuthFeature, AuthSwitchLink } from '../../layouts/AuthLayout'

export default function LoginPage() {
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
      const next = searchParams.get('next')
      window.location.assign(next?.startsWith('/') && !next.startsWith('//') ? next : '/')
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
        <aside className="register-aside login-aside">
          <div className="aside-glow" />
          <h2>Все возможности<br />FinTalent <span>в одном месте</span></h2>
          <AuthFeature title="Персональные рекомендации">Подходящие вакансии на основе вашего профиля</AuthFeature>
          <AuthFeature title="История откликов">Следите за статусами в личном кабинете</AuthFeature>
          <AuthFeature title="Результаты тестирования">Подтверждайте свои профессиональные навыки</AuthFeature>
        </aside>
      )}
    >
      <section className="register-card">
        <div className="register-intro">
          <span className="register-icon">♙</span>
          <h1>Добро пожаловать</h1>
          <p>Войдите в аккаунт, чтобы продолжить<br />работу с FinTalent</p>
        </div>
        <form className="register-form" method="post" action="/api/login" noValidate onSubmit={handleSubmit}>
          <label>
            Email
            <span className="input-wrap"><i>✉</i><input type="email" name="email" autoComplete="email" placeholder="name@example.ru" required /></span>
          </label>
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
        <AuthSwitchLink prompt="Нет аккаунта?" to="/register">Зарегистрироваться</AuthSwitchLink>
      </section>
    </AuthLayout>
  )
}
