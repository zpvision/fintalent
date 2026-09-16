import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { apiClient } from '../../api/client'
import AuthLayout from '../../layouts/AuthLayout'

export default function ForgotPasswordPage() {
  const [step, setStep] = useState(1)
  const [email, setEmail] = useState('')
  const [code, setCode] = useState('')
  const [resetToken, setResetToken] = useState('')
  const [password, setPassword] = useState('')
  const [confirmation, setConfirmation] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [resendIn, setResendIn] = useState(0)

  useEffect(() => {
    if (!resendIn) return undefined
    const timer = window.setInterval(() => setResendIn((value) => Math.max(0, value - 1)), 1000)
    return () => window.clearInterval(timer)
  }, [resendIn])

  async function requestCode(event) {
    event?.preventDefault()
    setError(''); setBusy(true)
    try {
      await apiClient.post('/api/password-reset/request', { email }, { redirectOnUnauthorized: false })
      setStep(2); setResendIn(60)
    } catch (requestError) { setError(requestError.message) } finally { setBusy(false) }
  }

  async function verifyCode(event) {
    event.preventDefault(); setError(''); setBusy(true)
    try {
      const result = await apiClient.post('/api/password-reset/verify', { email, code }, { redirectOnUnauthorized: false })
      setResetToken(result.reset_token); setStep(3)
    } catch (requestError) { setError(requestError.message) } finally { setBusy(false) }
  }

  async function changePassword(event) {
    event.preventDefault(); setError('')
    if (password !== confirmation) { setError('Пароли не совпадают'); return }
    setBusy(true)
    try {
      await apiClient.post('/api/password-reset/complete', { reset_token: resetToken, password, password_confirmation: confirmation }, { redirectOnUnauthorized: false })
      setStep(4)
    } catch (requestError) { setError(requestError.message) } finally { setBusy(false) }
  }

  return (
    <AuthLayout title="Восстановление пароля — FinTalent" login aside={<aside className="register-aside login-aside"><div className="aside-glow" /><h2>Верните доступ<br /><span>за несколько минут</span></h2><p className="reset-aside-text">Мы отправим одноразовый код на почту. Код действует 10 минут и нужен только для смены пароля.</p></aside>}>
      <section className="register-card reset-card">
        <div className="register-intro"><span className="register-icon">🔐</span><h1>{step === 4 ? 'Пароль изменён' : 'Восстановление пароля'}</h1><p>{step === 1 && 'Введите email, указанный при регистрации'}{step === 2 && <>Введите код, отправленный на <strong>{email}</strong></>}{step === 3 && 'Придумайте новый пароль'}{step === 4 && 'Теперь можно войти с новым паролем'}</p></div>
        {step === 1 && <form className="register-form" onSubmit={requestCode}><label>Email<span className="input-wrap"><i>✉</i><input type="email" value={email} onChange={(e) => setEmail(e.target.value)} autoComplete="email" placeholder="name@example.ru" required /></span></label><div className={`form-message${error ? ' error' : ''}`} role="alert">{error}</div><button className="submit-register" disabled={busy}>{busy ? 'Отправляем…' : <>Получить код <span>→</span></>}</button></form>}
        {step === 2 && <form className="register-form" onSubmit={verifyCode}><label>Код из письма<span className="input-wrap"><i>#</i><input className="reset-code-input" inputMode="numeric" pattern="[0-9]{6}" maxLength="6" value={code} onChange={(e) => setCode(e.target.value.replace(/\D/g, '').slice(0, 6))} autoComplete="one-time-code" placeholder="000000" required /></span></label><div className={`form-message${error ? ' error' : ''}`} role="alert">{error}</div><button className="submit-register" disabled={busy || code.length !== 6}>{busy ? 'Проверяем…' : <>Продолжить <span>→</span></>}</button><button type="button" className="reset-resend" disabled={busy || resendIn > 0} onClick={() => requestCode()}>{resendIn ? `Отправить повторно через ${resendIn} сек.` : 'Отправить код повторно'}</button></form>}
        {step === 3 && <form className="register-form" onSubmit={changePassword}><label>Новый пароль<span className="input-wrap"><i>♦</i><input type={showPassword ? 'text' : 'password'} value={password} onChange={(e) => setPassword(e.target.value)} minLength="8" maxLength="72" autoComplete="new-password" required /><button type="button" className="show-password" onClick={() => setShowPassword((value) => !value)}>{showPassword ? '⊘' : '◉'}</button></span></label><label>Повторите пароль<span className="input-wrap"><i>♦</i><input type={showPassword ? 'text' : 'password'} value={confirmation} onChange={(e) => setConfirmation(e.target.value)} minLength="8" maxLength="72" autoComplete="new-password" required /></span></label><div className={`form-message${error ? ' error' : ''}`} role="alert">{error}</div><button className="submit-register" disabled={busy}>{busy ? 'Сохраняем…' : <>Изменить пароль <span>→</span></>}</button></form>}
        {step === 4 && <Link className="submit-register reset-login-link" to="/login">Войти в аккаунт <span>→</span></Link>}
        {step !== 4 && <p className="login-prompt"><Link to="/login">← Вернуться ко входу</Link></p>}
      </section>
    </AuthLayout>
  )
}
