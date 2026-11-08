import { useEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { Link } from 'react-router-dom'
import Icon from '../../components/Icon'
import { AnalyticsTable, number } from './AnalyticsShared'
import { zonedDate } from './CohortShared'

const commission = bps => bps == null ? '—' : `${number(bps / 100)}%`
const deliveryNames = { pending:'Приглашение в очереди', sending:'Письмо отправляется', sent:'Приглашение отправлено', failed:'Ошибка отправки письма' }

function PaymentConfirmation({ row, versions, id, api, onClose, onPaid }) {
  const ref = useRef(null), [version,setVersion] = useState(versions[0]?.id || ''), [busy,setBusy] = useState(false), [error,setError] = useState('')
  useEffect(()=>{const previous=document.body.style.overflow,rootPrevious=document.documentElement.style.overflow;document.body.style.overflow='hidden';document.documentElement.style.overflow='hidden';ref.current.showModal();return()=>{ref.current?.close();document.body.style.overflow=previous;document.documentElement.style.overflow=rootPrevious}},[])
  async function confirm(event) {
    event.preventDefault(); if(busy || !version) return
    setBusy(true);setError('')
    try { await api.markPaid(id,row.id,version);onClose();onPaid() }
    catch(e){setError(e.message)} finally {setBusy(false)}
  }
  return createPortal(<dialog ref={ref} className="course-page analytics-payment-dialog" aria-labelledby="payment-confirm-title" onCancel={e=>{e.preventDefault();if(!busy)onClose()}} onClick={e=>{if(e.target!==e.currentTarget||busy)return;const r=e.currentTarget.getBoundingClientRect();if(e.clientX<r.left||e.clientX>r.right||e.clientY<r.top||e.clientY>r.bottom)onClose()}}>
    <form onSubmit={confirm}><span className="acquisition-dialog-icon"><Icon name="check" /></span><small>ОПЛАТА ВНЕ FINTALENT</small><h2 id="payment-confirm-title">Подтвердить оплату?</h2><p>Вы подтверждаете, что получили оплату от <b>{row.name}</b>. Ученик получит доступ к выбранной программе, а приглашение на обучение будет отправлено на его email.</p>
    <div className="acquisition-confirm-rate"><span>Комиссия FinTalent</span><b>{commission(row.commission_bps)}</b></div>
    <label className="course-field">Опубликованная версия программы<select value={version} onChange={e=>setVersion(e.target.value)} disabled={busy} required>{!versions.length && <option value="">Нет опубликованных версий</option>}{versions.map(v=><option value={v.id} key={v.id}>Версия {v.number}</option>)}</select></label>
    {!versions.length && <p className="course-notice">Сначала <Link to={`/profile/courses/${id}/academy/students`}>опубликуйте учебную версию</Link>, затем подтвердите оплату.</p>}
    <p className="analytics-caption">Процент фиксируется при подтверждении оплаты. Повторное нажатие не создаст ещё одно зачисление или письмо. Снять отметку здесь нельзя.</p>
    {error && <p className="course-error" role="alert">{error}</p>}<div className="acquisition-dialog-actions"><button className="course-secondary" type="button" disabled={busy} onClick={onClose}>Отмена</button><button className="course-primary" disabled={busy || !version} type="submit">{busy?'Подтверждаем…':'Оплата получена'}</button></div></form>
  </dialog>,document.body)
}

export default function AnalyticsSources({ data, query, change, zone, owner, id, versions = [], api, onPaid }) {
  const sources = data.sources || [], interests = data.interests, [confirm,setConfirm] = useState(null)
  const metric = value => <span className="acquisition-value">{value==null?'—':number(value)}</span>
  return <>
    <div className="acquisition-controls"><label className="course-field analytics-source-filter">Источник<select value={query.get('source') || ''} onChange={e => change({source:e.target.value})}><option value="">Все источники</option><option value="platform">FinTalent</option>{sources.filter(s => s.id).map(s => <option key={s.id} value={s.id}>{s.name}{s.deleted ? ' · удалена' : ''}</option>)}</select></label>
    {owner && <label className="course-field">Оплата заявки<select value={query.get('interest_payment') || ''} onChange={e=>change({interest_payment:e.target.value})}><option value="">Все заявки</option><option value="unpaid">Не оплачены</option><option value="paid">Оплачены</option></select></label>}</div>
    {owner && <div className="analytics-acquisition-grid">
      <section className="teaching-panel acquisition-panel acquisition-sources"><header><span className="acquisition-panel-icon"><Icon name="globe" /></span><div><small>КАНАЛЫ ПРИВЛЕЧЕНИЯ</small><h3>Источники интереса к курсу</h3></div></header><p className="analytics-caption">За всё время. Посещения и заявки учитываются отдельно от оплат и зачислений.</p>
      <AnalyticsTable rows={sources} columns={[{key:'name',label:'Источник',render:r=><><span className={`acquisition-source ${r.id?'partner':''}`}>{r.name}</span>{r.deleted && <small>Ссылка удалена</small>}</>},{key:'visits',label:'Переходы',render:r=>metric(r.visits)},{key:'visitors',label:'Посетители',render:r=>metric(r.visitors)},{key:'accounts',label:'Аккаунты',render:r=>metric(r.accounts)},{key:'interests',label:'Заявки',render:r=>metric(r.interests)}]} />
      <p className="analytics-caption acquisition-footnote">Гостевые посещения помогают оценить продвижение. В заявки попадают только пользователи, подтвердившие выбор курса после входа.</p></section>
      {interests && <section className="teaching-panel acquisition-panel acquisition-applications"><header><span className="acquisition-panel-icon green"><Icon name="users" /></span><div><small>ОТ ИНТЕРЕСА К ОБУЧЕНИЮ</small><h3>Выбрали курс <span className="acquisition-total">{number(interests.total)}</span></h3></div></header><p className="analytics-caption">По выбранному периоду, поиску, источнику и статусу оплаты. Поток и прогресс относятся к таблице учеников ниже.</p>
      <AnalyticsTable rows={interests.rows || []} columns={[{key:'name',label:'Пользователь',render:r=><div className="acquisition-person"><span className="acquisition-avatar" aria-hidden="true">{r.name?.charAt(0)}</span><div>{r.profile_url?<Link className="analytics-text-link" to={r.profile_url}>{r.name} ↗</Link>:<b>{r.name}</b>}<small>{zonedDate(r.first_clicked_at,zone)}</small>{!r.profile_url && <small>Профиль не опубликован · ID {r.user_id}</small>}</div></div>},{key:'source_name',label:'Источник',render:r=><span className="acquisition-source">{r.source_name}</span>},{key:'commission_bps',label:'Комиссия',render:r=><><span className={`acquisition-commission ${r.paid_at?'fixed':''}`}>{commission(r.commission_bps)}</span><small>{r.paid_at?'Зафиксирована':r.commission_override_bps!=null?'Индивидуальная':'При оплате'}</small></>},{key:'paid_at',label:'Оплачен',render:r=><><label className={`acquisition-paid ${r.paid_at?'is-paid':''}`}><input type="checkbox" checked={Boolean(r.paid_at)} disabled={Boolean(r.paid_at)||!data.can_mark_paid} aria-label={`Оплачен: ${r.name}`} onChange={()=>setConfirm(r)}/><span>{r.paid_at?'Оплачен':'Отметить'}</span></label>{r.paid_at && <><small>{zonedDate(r.paid_at,zone)}</small><small className={r.invitation_status==='failed'?'acquisition-delivery-error':''}>{deliveryNames[r.invitation_status] || 'Приглашение подготовлено'}</small></>}</>}]} />
      <div className="acquisition-pagination"><span>{interests.total?`Страница ${interests.page}`:'Заявок пока нет'}</span><div><button type="button" className="course-secondary" aria-label="Предыдущая страница заявок" disabled={interests.page<=1} onClick={()=>change({page:interests.page-1})}>←</button><button type="button" className="course-secondary" aria-label="Следующая страница заявок" disabled={interests.page*interests.size>=interests.total} onClick={()=>change({page:interests.page+1})}>→</button></div></div>
      {data.manual_payments && <p className="analytics-caption acquisition-footnote">Ставки этого курса: FinTalent — {commission(data.manual_payments.standard_bps)}, партнёрские ссылки — {commission(data.manual_payments.partner_bps)}. Оплату получает автор вне сайта.</p>}
      </section>}
    </div>}
    {confirm && <PaymentConfirmation row={confirm} versions={versions} id={id} api={api} onClose={()=>setConfirm(null)} onPaid={onPaid}/>}
  </>
}
