import { useEffect, useState } from 'react'
import { apiClient } from '../../api/client'
import Icon from '../../components/Icon'

export default function CourseReferralLinks({ course }) {
  const [links, setLinks] = useState([]), [adding, setAdding] = useState(false), [name, setName] = useState(''), [code, setCode] = useState(''), [busy, setBusy] = useState(false), [error, setError] = useState(''), [notice, setNotice] = useState(''), [deleting, setDeleting] = useState(null)
  const base = `/api/courses/my/${course.id}/referrals`, normalized = code.trim().toLowerCase()
  const linkURL = code => `${window.location.origin}/courses/${encodeURIComponent(course.slug)}?ref=${encodeURIComponent(code)}`
  async function load() { setLinks(await apiClient.get(base)) }
  useEffect(() => { load().catch(e => setError(e.message)) }, [course.id])
  async function create() {
    setError(''); if (!name.trim() || !/^[a-z0-9_-]{3,64}$/.test(normalized)) { setError('Укажите название и код из 3–64 латинских букв, цифр, дефисов или подчёркиваний.'); return }
    if (links.some(l => l.code === normalized)) { setError('Этот код уже используется.'); return }
    setBusy(true); try { await apiClient.post(base, { name: name.trim(), code: normalized }); await load(); setAdding(false); setName(''); setCode(''); setNotice('Партнёрская ссылка создана') } catch (e) { setError(e.message) } finally { setBusy(false) }
  }
  async function remove() { setBusy(true); setError(''); try { await apiClient.delete(`${base}/${deleting.id}`); await load(); setDeleting(null); setNotice('Ссылка удалена. История привлечённых учеников сохранена.') } catch(e) { setError(e.message) } finally { setBusy(false) } }
  async function copy(link) { try { await navigator.clipboard.writeText(linkURL(link.code)); setNotice('Ссылка скопирована') } catch { setError('Не удалось скопировать. Выделите и скопируйте адрес ссылки.') } }
  return <section className="course-pricing-section course-referrals"><header><i><Icon name="users" /></i><div><h3>Партнёрские ссылки</h3><p>Ваши каналы продвижения — отдельная история каждого источника.</p></div></header>
    <p>Привлекайте учеников самостоятельно и снижайте комиссию FinTalent с 20% до 12%. Создавайте отдельные ссылки для Telegram, email-рассылок и других каналов продвижения. Если ученик перейдёт по вашей ссылке, источник закрепится за ним для данного курса и будет учитываться при покупке.</p>
    <p className="course-muted">Статистику переходов и покупок, а также источники учеников вы сможете посмотреть в разделе Академия → Аналитика → Ученики.</p>
    <p className="course-notice">Оплата пока не подключена. Сейчас сохраняются переходы и выбор курса; ставки комиссии будут применяться после запуска продаж.</p>
    {course.status !== 'published' && <p className="course-notice">Ссылки начнут привлекать учеников после публикации лендинга. Адрес уже можно скопировать.</p>}
    {error && <p className="course-error" role="alert">{error}</p>}
    {links.map(link => <article className="course-referral-row" key={link.id}><div><b>{link.name}</b><small>{link.code}</small><input aria-label={`Адрес: ${link.name}`} readOnly value={linkURL(link.code)} onFocus={e => e.target.select()} /></div><div className="course-referral-actions"><button type="button" className="course-secondary" onClick={() => copy(link)}>Копировать</button><button type="button" className="course-secondary" aria-label={`Удалить ${link.name}`} onClick={() => setDeleting(link)}><Icon name="trash" /></button></div></article>)}
    {adding ? <div className="course-repeat-card"><label className="course-field">Название ссылки<input maxLength={120} value={name} onChange={e => setName(e.target.value)} placeholder="Telegram-канал" /></label><label className="course-field">Код ссылки<input maxLength={64} value={code} onChange={e => setCode(e.target.value)} placeholder="telegram2026" autoCapitalize="none" spellCheck={false} /></label><p className="course-referral-preview">{linkURL(normalized)}</p><div className="course-referral-actions"><button type="button" className="course-primary" disabled={busy} onClick={create}>Создать ссылку</button><button type="button" className="course-secondary" disabled={busy} onClick={() => setAdding(false)}>Отмена</button></div></div> : <button type="button" className="course-add" onClick={() => setAdding(true)}>＋ Добавить партнёрскую ссылку</button>}
    {deleting && <div className="course-repeat-card" role="group" aria-label="Подтверждение удаления"><b>Удалить партнёрскую ссылку?</b><p>Новые переходы по этой ссылке больше не будут учитываться как партнёрские. Статистика и источники ранее привлечённых учеников сохранятся.</p><div className="course-referral-actions"><button type="button" className="course-secondary" disabled={busy} onClick={() => setDeleting(null)}>Отмена</button><button type="button" className="course-primary" disabled={busy} onClick={remove}>Удалить ссылку</button></div></div>}
    {notice && <div className="course-notification" role="status"><span>{notice}</span><button type="button" aria-label="Закрыть уведомление" onClick={() => setNotice('')}>×</button></div>}
  </section>
}
