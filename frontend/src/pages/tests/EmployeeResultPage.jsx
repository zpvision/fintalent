import { useEffect, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { getEmployeeResultReview } from '../../api/tests'
import usePageStyles from '../../hooks/usePageStyles'
import { useDocumentPage } from '../../hooks/useDocumentPage'
import PublicLayout from '../../layouts/PublicLayout'
import { Review } from './TestTakePage'

const styles = ['/static/test-chat-history.css', '/static/test-answer-review.css', '/static/employee-test.css?v=1']

export default function EmployeeResultPage() {
  usePageStyles(styles)
  useDocumentPage({ title: 'Разбор результатов — FinTalent' })
  const [params] = useSearchParams()
  const invitation = params.get('invitation') || ''
  const [state, setState] = useState({ loading: true })
  useEffect(() => {
    if (!/^\d+$/.test(invitation)) { setState({ error: 'Ссылка недоступна' }); return }
    getEmployeeResultReview(invitation).then(result => setState({ result })).catch(error => setState({ error: error.status === 401 ? 'auth' : 'Ссылка недоступна для этого аккаунта' }))
  }, [invitation])
  const next = `/employee-result?invitation=${encodeURIComponent(invitation)}`
  return <PublicLayout><main className="et-public-result" style={{ maxWidth: 920, margin: '48px auto', padding: '32px' }}>
    {state.loading ? <p>Загружаем разбор…</p> : state.error === 'auth' ? <section><h1>Войдите, чтобы посмотреть результат</h1><p>Разбор доступен только в аккаунте с адресом электронной почты, на который отправлено письмо.</p><p><Link to={`/login?next=${encodeURIComponent(next)}`}>Войти</Link> · <Link to={`/register?next=${encodeURIComponent(next)}`}>Зарегистрироваться</Link></p></section> : state.error ? <section><h1>Результат недоступен</h1><p>{state.error}</p><p>Войдите под адресом, на который пришло письмо, или обратитесь к организатору тестирования.</p><Link to={`/login?next=${encodeURIComponent(next)}`}>Сменить аккаунт</Link></section> : <><h1>{state.result.test_title}</h1><p>Ваш результат: {Math.round(state.result.percent)}%</p><Review result={state.result} /></>}
  </main></PublicLayout>
}
