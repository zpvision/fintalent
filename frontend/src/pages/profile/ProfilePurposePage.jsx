import { useEffect, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { apiClient } from '../../api/client'
import { useAuth } from '../../context/AuthContext'
import { useDocumentPage } from '../../hooks/useDocumentPage'
import usePageStyles from '../../hooks/usePageStyles'
import { navigateInApp } from '../../navigation'

const choices = [
  {
    mode: 'job_search',
    eyebrow: 'КАРЬЕРА И ВАКАНСИИ',
    title: 'Ищу работу',
    description: 'Хочу откликаться на вакансии и получать предложения работодателей.',
    details: ['Желаемая должность и обязанности', 'Условия работы и ожидаемый доход', 'Видимость для работодателей'],
    icon: '↗',
  },
  {
    mode: 'professional',
    eyebrow: 'ПРОФЕССИОНАЛЬНОЕ СООБЩЕСТВО',
    title: 'Создаю профессиональный профиль',
    description: 'Хочу представить свой опыт и компетенции, проходить тесты, размещать материалы и общаться с коллегами.',
    details: ['Представьте свой профессиональный опыт', 'Покажите сильные стороны и экспертизу', 'Делитесь знаниями и проводите тестирование сотрудников'],
    icon: '✦',
  },
]

export default function ProfilePurposePage() {
  useDocumentPage({ title: 'Назначение профиля — FinTalent' })
  usePageStyles(['/static/profile-purpose.css?v=1'])
  const { user, loading: authLoading } = useAuth()
  const [searchParams] = useSearchParams()
  const [currentMode, setCurrentMode] = useState('')
  const [loading, setLoading] = useState(true)
  const [submitting, setSubmitting] = useState('')
  const [error, setError] = useState('')

  useEffect(() => {
    if (authLoading) return
    if (!user) {
      navigateInApp(`/login?next=${encodeURIComponent('/profile-purpose')}`, { replace: true })
      return
    }
    apiClient.get('/api/profile-purpose')
      .then((data) => setCurrentMode(data.mode || ''))
      .catch((requestError) => setError(requestError.message || 'Не удалось загрузить настройки профиля'))
      .finally(() => setLoading(false))
  }, [authLoading, user])

  async function choose(mode) {
    setError('')
    setSubmitting(mode)
    try {
      await apiClient.put('/api/profile-purpose', { mode })
      navigateInApp('/profiles/create')
    } catch (requestError) {
      setError(requestError.message || 'Не удалось сохранить выбор')
      setSubmitting('')
    }
  }

  function skip() {
    const next = searchParams.get('next')
    navigateInApp(next?.startsWith('/') && !next.startsWith('//') ? next : '/profile')
  }

  return (
    <main className="profile-purpose-page">
      <section className="profile-purpose-shell">
        <header className="profile-purpose-heading">
          <span className="profile-purpose-mark">FT</span>
          <small>НАСТРОЙТЕ СВОЙ ПРОФИЛЬ</small>
          <h1>Как вы планируете использовать FinTalent?</h1>
          <p>Выберите подходящий сценарий. Его можно изменить позже в настройках профиля.</p>
        </header>

        {loading ? <div className="profile-purpose-loading"><i /><span>Загружаем настройки…</span></div> : (
          <div className="profile-purpose-choices">
            {choices.map((choice) => (
              <article className={`profile-purpose-choice ${choice.mode}${currentMode === choice.mode ? ' current' : ''}`} key={choice.mode}>
                {currentMode === choice.mode && <span className="profile-purpose-current">Текущий вариант</span>}
                <div className="profile-purpose-icon" aria-hidden="true">{choice.icon}</div>
                <small>{choice.eyebrow}</small>
                <h2>{choice.title}</h2>
                <p>{choice.description}</p>
                <ul>{choice.details.map((detail) => <li key={detail}><span>✓</span>{detail}</li>)}</ul>
                <button type="button" disabled={Boolean(submitting)} onClick={() => choose(choice.mode)}>
                  {submitting === choice.mode ? 'Сохраняем…' : currentMode === choice.mode ? 'Продолжить' : 'Выбрать'} <span>→</span>
                </button>
              </article>
            ))}
          </div>
        )}
        {error && <div className="profile-purpose-error" role="alert">{error}</div>}
        <button className="profile-purpose-skip" type="button" onClick={skip} disabled={Boolean(submitting)}>Решу позже</button>
        <p className="profile-purpose-note"><span>i</span> В профессиональном профиле не будет вопросов о желаемой должности, общей информации, обязанностях и доходе.</p>
      </section>
    </main>
  )
}
