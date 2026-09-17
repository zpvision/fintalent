import { useLocation } from 'react-router-dom'
import { useAuth } from '../../context/AuthContext'
import { useDocumentPage } from '../../hooks/useDocumentPage'
import usePageStyles from '../../hooks/usePageStyles'
import PublicLayout from '../../layouts/PublicLayout'

const styles = ['/static/client-exchange-access.css?v=1']

export default function ClientExchangeAccess({ children }) {
  const { user, loading } = useAuth()
  const location = useLocation()
  useDocumentPage({ title: 'Клиентская биржа — FinTalent' })
  usePageStyles(styles)

  if (loading) return <PublicLayout><main className="ce-access-loading">Загружаем…</main></PublicLayout>
  if (user) return children

  const next = `${location.pathname}${location.search}`
  const query = encodeURIComponent(next)

  return <PublicLayout><main className="ce-access-page"><section className="ce-access-card"><div className="ce-access-copy"><span className="ce-access-kicker"><i>◇</i> КЛИЕНТСКАЯ БИРЖА</span><h1>Доступ к разделу<br/>после регистрации</h1><p>Создайте аккаунт, чтобы находить новых клиентов, передавать проекты коллегам и безопасно обсуждать условия сотрудничества.</p><div className="ce-access-benefits"><span><i>✓</i> Обезличенные карточки клиентов</span><span><i>✓</i> Предложения проверенных компаний</span><span><i>✓</i> Контакты открываются после выбора</span></div><div className="ce-access-actions"><a className="primary" href={`/register?next=${query}`}>Зарегистрироваться <i>→</i></a><a className="secondary" href={`/login?next=${query}`}>Войти</a></div><small>Регистрация бесплатная и займёт несколько минут.</small></div><div className="ce-access-visual"><img src="/static/images/client-exchange-transfer-promo.jpg" alt="Передача клиентов между бухгалтерскими компаниями"/><div className="ce-access-float top"><i>↗</i><span><b>Передавайте клиентов</b><small>на выгодных условиях</small></span></div><div className="ce-access-float bottom"><i>✓</i><span><b>Находите проекты</b><small>для своей компании</small></span></div></div></section></main></PublicLayout>
}
