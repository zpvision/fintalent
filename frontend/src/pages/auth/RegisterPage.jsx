import { useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { apiClient } from '../../api/client'
import AuthLayout, { AuthSwitchLink } from '../../layouts/AuthLayout'
import { useAuth } from '../../context/AuthContext'
import { navigateInApp } from '../../navigation'

export default function RegisterPage() {
  const { refresh } = useAuth()
  const [searchParams] = useSearchParams()
  const [showPassword, setShowPassword] = useState(false)
  const [agreementAccepted, setAgreementAccepted] = useState(false)
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
          <div className="register-aside-orbit orbit-one" />
          <div className="register-aside-orbit orbit-two" />
          <span className="register-aside-kicker"><i>✦</i> ПРОФЕССИОНАЛЬНАЯ ЭКОСИСТЕМА</span>
          <h2>Развивайте карьеру<br />и профессиональную <span>репутацию</span></h2>
          <p className="register-aside-lead">Создайте пространство, где ваш опыт работает на вас — помогает находить проекты, клиентов и новые профессиональные возможности.</p>
          <div className="register-value-grid">
            <article><i>◎</i><div><b>Профессиональный профиль</b><p>Покажите опыт, навыки и направления экспертизы</p></div></article>
            <article><i>✓</i><div><b>Проверка компетенций</b><p>Подтверждайте знания тестами и результатами</p></div></article>
            <article><i>◇</i><div><b>ПрофиМаркет</b><p>Размещайте и продавайте собственные решения</p></div></article>
            <article><i>↗</i><div><b>Новые связи</b><p>Находите коллег, клиентов и предложения</p></div></article>
          </div>
          <div className="register-aside-summary"><span><i>FT</i><b>Ваш профессиональный капитал</b></span><small>Всё для профессионального роста — в одном месте</small></div>
        </aside>
      )}
    >
      <section className="register-card">
        <div className="register-intro">
          <span className="register-icon" aria-hidden="true"><svg viewBox="0 0 24 24"><circle cx="9" cy="7" r="3" /><path d="M3.5 19c.5-4 2.3-6 5.5-6s5 2 5.5 6M18 8v6M15 11h6" /></svg></span>
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
          <label className="agree"><input type="checkbox" name="agreement" required checked={agreementAccepted} onChange={(event) => setAgreementAccepted(event.target.checked)} /><span /><span className="agree-copy">Я принимаю условия <a href="/static/docs/fintalent-user-agreement.pdf" target="_blank" rel="noopener noreferrer">Пользовательского соглашения</a>, ознакомлен(а) с <a href="/static/docs/fintalent-privacy-policy.pdf" target="_blank" rel="noopener noreferrer">Политикой конфиденциальности</a> и даю <a href="/static/docs/fintalent-personal-data-consent.pdf" target="_blank" rel="noopener noreferrer">Согласие на обработку персональных данных</a>.</span></label>
          <div className={`form-message${error ? ' error' : ''}`} role="alert">{error}</div>
          <button className="submit-register" type="submit" disabled={submitting || !agreementAccepted}>{submitting ? 'Создаём аккаунт…' : <>Зарегистрироваться <span>→</span></>}</button>
        </form>
        <AuthSwitchLink prompt="Уже есть аккаунт?" to={searchParams.get('next')?`/login?next=${encodeURIComponent(searchParams.get('next'))}`:'/login'}>Войти</AuthSwitchLink>
      </section>
    </AuthLayout>
  )
}
