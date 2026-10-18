import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { assessmentAPI } from '../../api/assessments'
export default function AcademyNotifications() {
  const [items, setItems] = useState([]), [error, setError] = useState('')
  useEffect(() => { let live = true; const load = () => assessmentAPI.notifications().then(v => { if (live && Array.isArray(v)) setItems(v) }).catch(() => {}); load(); window.addEventListener('focus', load); return () => { live = false; window.removeEventListener('focus', load) } }, [])
  if (!items.length) return <Link className="academy-notification-settings" to="/profile/academy/notifications">Настройки уведомлений</Link>
  return <details className="academy-notifications"><summary>Уведомления об обучении <b>{items.filter(v => !v.is_read).length || '✓'}</b></summary><div>{items.map(n => <Link className={n.is_read ? '' : 'unread'} to={n.href} key={n.id} onClick={() => assessmentAPI.read([n.id]).then(() => setItems(list => list.map(v => v.id === n.id ? { ...v, is_read: true } : v))).catch(e => setError(e.message))}><b>{n.title}</b><span>{n.body}</span><small>{new Date(n.created_at).toLocaleString('ru-RU')}</small></Link>)}<Link to="/profile/academy/notifications">Настройки уведомлений</Link>{error && <p role="alert">{error}</p>}</div></details>
}
