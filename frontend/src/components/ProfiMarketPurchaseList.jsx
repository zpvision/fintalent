import { useCallback, useEffect, useState } from 'react'
import { getMyProfiMarketOrders, getMyProfiMarketPurchases, markMyProfiMarketOrderRead } from '../api/profimarket'

const typeLabel = type => ({REGULATION:'Регламент',AI_ASSISTANT:'ИИ-ассистент',AUTOMATION:'Автоматизация',INSTRUCTION:'Инструкция',ONEC_INTEGRATION:'1С Интеграция',TEMPLATE:'Шаблон',CHECKLIST:'Чек-лист'})[type] || type || 'Решение'
const money = value => new Intl.NumberFormat('ru-RU').format(value || 0)
const date = value => new Date(value).toLocaleDateString('ru-RU')
const purchaseStatus = status => ({PENDING:'Ожидает оплаты',COMPLETED:'Оплачено',CANCELLED:'Отменено',REFUNDED:'Возврат'})[status] || status
const implementationStatus = status => ({NEW:'Новая заявка',CONTACTED:'Связались с покупателем',IN_PROGRESS:'В работе',COMPLETED:'Завершено',CANCELLED:'Отменено'})[status] || status

export default function ProfiMarketPurchaseList({ orders = false, onSummary }) {
  const [items,setItems]=useState([]),[loading,setLoading]=useState(true),[error,setError]=useState(''),[copied,setCopied]=useState(0)
  const load=useCallback(()=>{setLoading(true);setError('');(orders?getMyProfiMarketOrders():getMyProfiMarketPurchases()).then(data=>{setItems(data.items||[]);if(orders)onSummary?.({total_count:data.total_count||0,new_count:data.new_count||0})}).catch(e=>setError(e.message)).finally(()=>setLoading(false))},[orders,onSummary])
  useEffect(load,[load])
  async function copyEmail(item){
    try{await navigator.clipboard.writeText(item.buyer_email)}catch{
      const input=document.createElement('textarea');input.value=item.buyer_email;input.style.position='fixed';input.style.opacity='0';document.body.append(input);input.select();document.execCommand('copy');input.remove()
    }
    setCopied(item.id)
    window.setTimeout(()=>setCopied(current=>current===item.id?0:current),2200)
  }
  async function markRead(item){
    try{
      await markMyProfiMarketOrderRead(item.id)
      setItems(current=>current.map(value=>value.id===item.id?{...value,is_new:false}:value))
      onSummary?.(current=>{const next={total_count:current.total_count,new_count:Math.max(0,current.new_count-1)};window.dispatchEvent(new CustomEvent('profimarket:orders-read',{detail:next}));return next})
    }catch(e){setError(e.message)}
  }
  if(loading)return <div className="profile-market-loading">Загружаем данные…</div>
  if(error)return <div className="profile-market-empty"><h2>Не удалось загрузить данные</h2><p>{error}</p><button onClick={load}>Повторить</button></div>
  if(!items.length)return <div className="profile-market-empty"><h2>{orders?'Новых заказов пока нет':'Покупок пока нет'}</h2><p>{orders?'Здесь появятся заявки покупателей и их контакты.':'Купленные решения появятся в этом разделе.'}</p>{!orders&&<a href="/profimarket" target="_self">Перейти в ПрофиМаркет</a>}</div>
  return <div className="profile-market-order-list">{items.map(item=><article className={`profile-market-order${orders&&item.is_new?' is-new':''}`} key={item.id}>
    <div className="profile-market-order-cover">{item.cover_image?<img src={item.cover_image} alt=""/>:<span>{typeLabel(item.type).slice(0,2)}</span>}</div>
    <header><div><small className="profile-market-order-type">{typeLabel(item.type)} · заказ №{item.id}</small><h3>{item.title}</h3>{item.short_description&&<p>{item.short_description}</p>}<div className="profile-market-order-author"><i>{(item.author_name||'А').charAt(0)}</i><span><small>Автор решения</small><b>{item.author_name||'Автор FinTalent'}</b></span></div></div><div className="profile-market-order-head-actions">{item.solution_available?<a href={`/profimarket/solution/${encodeURIComponent(item.slug)}`} target="_self">Открыть решение →</a>:<span className="profile-market-unavailable">Карточка снята с публикации</span>}</div></header>
    <dl><dt>{orders?'Покупатель':'Стоимость'}</dt><dd>{orders?item.buyer_name:`${money(item.amount)} ₽`}</dd><dt>{orders?'Контакт':'Статус'}</dt><dd>{orders?item.buyer_email:purchaseStatus(item.status)}</dd>{(item.crm||item.custom_crm_name)&&<><dt>CRM</dt><dd>{item.custom_crm_name||item.crm}</dd><dt>E-mail в CRM</dt><dd>{item.crm_email}</dd></>}</dl>
    <dl>{item.comment&&<><dt>Комментарий</dt><dd>{item.comment}</dd></>}{item.implementation_status&&<><dt>Внедрение</dt><dd><span className={`profile-market-order-status ${item.implementation_status}`}>{implementationStatus(item.implementation_status)}</span></dd></>}<dt>Дата покупки</dt><dd>{date(item.created_at)}</dd>{orders&&<><dt>Действие</dt><dd><div className="profile-market-order-actions">{item.is_new?<button className="profile-market-mark-read" type="button" onClick={()=>markRead(item)}>✓ Отметить прочитанным</button>:<span className="profile-market-read-state">✓ Прочитано</span>}<button className={`profile-market-copy${copied===item.id?' copied':''}`} type="button" onClick={()=>copyEmail(item)}>{copied===item.id?'✓ Email скопирован':'Скопировать email'}</button></div></dd></>}</dl>
  </article>)}</div>
}
