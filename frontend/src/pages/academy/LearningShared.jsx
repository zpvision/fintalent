import Icon from '../../components/Icon'
export const accessNames = { active: 'Доступ открыт', suspended: 'Доступ приостановлен', expired: 'Срок доступа закончился', revoked: 'Доступ отозван' }
export const learningNames = { not_started: 'Не начато', in_progress: 'В процессе', completed: 'Завершено' }
export const learningDate = value => value ? new Date(value).toLocaleDateString('ru-RU', { day: 'numeric', month: 'short', year: 'numeric' }) : 'Ещё не заходил'
export function ProgressBar({ summary = {}, compact = false }) {
  const percent = Math.max(0, Math.min(100, summary.percent || 0))
  return <div className={`learning-progress${compact ? ' compact' : ''}`}><div><span>Пройдено {summary.completed || 0} из {summary.required || 0} обязательных уроков</span><b>{percent}%</b></div><progress max="100" value={percent} aria-label="Прогресс курса">{percent}%</progress>{!summary.required && <small>В этой версии нет обязательных уроков. Прогресс не рассчитывается автоматически.</small>}</div>
}
export function AccessNotice({ enrollment, children }) {
  return <section className="learning-empty learning-access-notice"><div className="learning-access-art"><img src="/static/academy/program-scratch.png" alt="" /><span aria-hidden="true"><Icon name="shield" /></span></div><small>ВАШЕ ОБУЧЕНИЕ</small><h1>{accessNames[enrollment.effective_status] || 'Доступ недоступен'}</h1><p>{enrollment.title}</p><p>Материалы курса сейчас закрыты. Обратитесь к преподавателю, чтобы уточнить условия восстановления доступа.</p>{children}</section>
}
