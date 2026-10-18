import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import UserLayout from '../../layouts/UserLayout'
import PublicLayout from '../../layouts/PublicLayout'
import usePageStyles from '../../hooks/usePageStyles'
import { profileStyles } from '../courses/shared'
import { communicationAPI } from '../../api/communications'
import { communicationStyles } from './CommunicationShared'
export default function CommunicationPreferences() {
 const { token } = useParams(), [items, setItems] = useState([]), [info, setInfo] = useState(null), [done, setDone] = useState(false), [busy, setBusy] = useState(false), [error, setError] = useState('')
 usePageStyles([...profileStyles, ...communicationStyles])
 useEffect(() => { (token ? communicationAPI.unsubscribe(token).then(setInfo) : communicationAPI.preferences().then(setItems)).catch(e => setError(e.message)) }, [token])
 async function change(item, patch) { setBusy(true); setError(''); try { await communicationAPI.preference({ ...item, ...patch, confirm_marketing_consent: (patch.email_marketing ?? item.email_marketing) === true }); setItems(await communicationAPI.preferences()) } catch (e) { setError(e.message) } finally { setBusy(false) } }
 const content = <main className="course-page communication-preferences"><Link to="/profile/learning">← Моё обучение</Link><h1>{token ? 'Уведомления курса' : 'Настройки уведомлений Академии'}</h1>{error && <p role="alert" className="course-error">{error}</p>}{token ? info && <section className="teaching-panel"><h2>{info.course_title}</h2><p>{done ? 'Необязательные email этого курса отключены. Вновь включить их можно в настройках профиля.' : info.message}</p>{!done && <button className="course-primary" disabled={busy} onClick={async () => { setBusy(true); try { await communicationAPI.unsubscribe(token, true); setDone(true) } catch (e) { setError(e.message) } finally { setBusy(false) } }}>Отписаться от email этого курса</button>}</section> : <><p>Вы выбираете, как получать сопровождение обучения. Письма входа, восстановления доступа и безопасности настраиваются платформой отдельно.</p>{items.map(item => <section className="teaching-panel" key={item.course_id}><h2>{item.title}</h2><label className="cohort-check"><input type="checkbox" disabled={busy} checked={item.email_service} onChange={e => change(item, { email_service: e.target.checked })} />Полезные email об обучении</label><label className="cohort-check"><input type="checkbox" disabled={busy} checked={item.email_marketing} onChange={e => { if (e.target.checked && !window.confirm('Согласны получать необязательные рекламные сообщения этого курса? Согласие можно отозвать в любой момент.')) return; change(item, { email_marketing: e.target.checked }) }} />Рекламные сообщения · только с моим согласием</label><label className="cohort-check"><input type="checkbox" disabled={busy} checked={item.in_app} onChange={e => change(item, { in_app: e.target.checked })} />Сообщения автора внутри FinTalent</label></section>)}</>}</main>
 return token ? <PublicLayout>{content}</PublicLayout> : <UserLayout active="learning">{content}</UserLayout>
}
