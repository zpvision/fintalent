import { useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { apiClient } from '../../api/client'
import AuthLayout, { AuthFeature, AuthSwitchLink } from '../../layouts/AuthLayout'
import { useAuth } from '../../context/AuthContext'
import { navigateInApp } from '../../navigation'

export default function RegisterPage() {
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
      await apiClient.post('/api/register', new FormData(form), { redirectOnUnauthorized: false })
      await refresh()
      const next = searchParams.get('next')
      const purposeURL = next?.startsWith('/') && !next.startsWith('//')
        ? `/profile-purpose?next=${encodeURIComponent(next)}`
        : '/profile-purpose'
      navigateInApp(purposeURL)
    } catch (requestError) {
      setError(requestError.message || 'Не удалось зарегистрироваться')
      setSubmitting(false)
    }
  }

  return (
    <AuthLayout
      title="Регистрация — FinTalent"
      aside={(
        <aside className="register-aside">
          <div className="aside-glow" />
          <h2>Развивайте карьеру<br />и профессиональную <span>репутацию</span></h2>
          <AuthFeature title="Профессиональный профиль">Представьте опыт, навыки и направления экспертизы</AuthFeature>
          <AuthFeature title="Материалы и тесты">Делитесь наработками и подтверждайте компетенции</AuthFeature>
          <AuthFeature title="Вакансии и коллеги">Находите новые возможности и полезные контакты</AuthFeature>
          <div className="match-card"><small>Ваш профиль в FinTalent</small><strong>Вместе</strong><div><span /></div><p>Опыт, знания и возможности в одном месте</p></div>
        </aside>
      )}
    >
      <section className="register-card">
        <div className="register-intro">
          <span className="register-icon">♙</span>
          <h1>Создайте аккаунт</h1>
          <p>Присоединяйтесь к сообществу специалистов<br />в сфере финансов и учёта</p>
        </div>
        <form className="register-form" method="post" action="/api/register" noValidate onSubmit={handleSubmit}>
          <label>
            ФИО
            <span className="input-wrap"><i>♙</i><input name="full_name" autoComplete="name" placeholder="Иванов Иван Иванович" required maxLength="200" /></span>
          </label>
          <label>
            Email
            <span className="input-wrap"><i>✉</i><input type="email" name="email" autoComplete="email" placeholder="name@example.ru" required maxLength="254" /></span>
          </label>
          <label>
            Пароль
            <span className="input-wrap">
              <i>♢</i>
              <input type={showPassword ? 'text' : 'password'} name="password" autoComplete="new-password" placeholder="Не менее 8 символов" required minLength="8" maxLength="72" />
              <button type="button" className="show-password" aria-label={showPassword ? 'Скрыть пароль' : 'Показать пароль'} onClick={() => setShowPassword((shown) => !shown)}>{showPassword ? '⊘' : '◉'}</button>
            </span>
          </label>
          <label className="agree"><input type="checkbox" name="agreement" required /><span />Я принимаю условия использования и обработки персональных данных</label>
          <div className={`form-message${error ? ' error' : ''}`} role="alert">{error}</div>
          <button className="submit-register" type="submit" disabled={submitting}>{submitting ? 'Создаём аккаунт…' : <>Зарегистрироваться <span>→</span></>}</button>
        </form>
        <AuthSwitchLink prompt="Уже есть аккаунт?" to={searchParams.get('next')?`/login?next=${encodeURIComponent(searchParams.get('next'))}`:'/login'}>Войти</AuthSwitchLink>
      </section>
    </AuthLayout>
  )
}
