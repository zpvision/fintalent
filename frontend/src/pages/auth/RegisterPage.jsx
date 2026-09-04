import { useState } from 'react'
import { apiClient } from '../../api/client'
import AuthLayout, { AuthFeature, AuthSwitchLink } from '../../layouts/AuthLayout'
import { useAuth } from '../../context/AuthContext'
import { navigateInApp } from '../../navigation'

export default function RegisterPage() {
  const { refresh } = useAuth()
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
      navigateInApp('/')
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
          <h2>Найдите работу,<br />которая подходит <span>именно вам</span></h2>
          <AuthFeature title="Точный подбор вакансий">Рекомендации на основе навыков и опыта</AuthFeature>
          <AuthFeature title="Подтверждение квалификации">Тесты помогут выделиться среди кандидатов</AuthFeature>
          <AuthFeature title="Проверенные работодатели">Только надежные компании и честные условия</AuthFeature>
          <div className="match-card"><small>Совпадение с вакансией</small><strong>94%</strong><div><span /></div><p>Ваши навыки отлично подходят</p></div>
        </aside>
      )}
    >
      <section className="register-card">
        <div className="register-intro">
          <span className="register-icon">♙</span>
          <h1>Создайте аккаунт</h1>
          <p>Откройте доступ к проверенным вакансиям<br />и специалистам в сфере финансов</p>
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
        <AuthSwitchLink prompt="Уже есть аккаунт?" to="/login">Войти</AuthSwitchLink>
      </section>
    </AuthLayout>
  )
}
