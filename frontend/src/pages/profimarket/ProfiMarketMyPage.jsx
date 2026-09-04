import { useCallback, useEffect, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { deleteProfiMarketSolution, getMyProfiMarketOrders, getMyProfiMarketPurchases, getMyProfiMarketSolutions, getProfiMarketSolution, getProfiMarketSolutions, unpublishProfiMarketSolution } from '../../api/profimarket'
import { useDocumentPage } from '../../hooks/useDocumentPage'
import usePageStyles from '../../hooks/usePageStyles'
import PublicLayout from '../../layouts/PublicLayout'

const typeLabel = (type) => type === 'REGULATION' ? 'Регламент' : type === 'AI_SOLUTION' ? 'AI-решение' : type || 'Решение'
const money = (value) => new Intl.NumberFormat('ru-RU').format(value || 0)
const price = (item) => item.is_free ? 'Бесплатно' : `${money(item.price)} ₽`
const date = (value) => new Date(value).toLocaleDateString('ru-RU')

function SolutionRows({ items, reload }) {
  async function action(item) {
    if (item.status === 'PUBLISHED') await unpublishProfiMarketSolution(item.id)
    else if (window.confirm('Удалить черновик?')) await deleteProfiMarketSolution(item.id)
    else return
    reload()
  }
  return <div className="pm-dashboard-list">{items.length ? items.map((item) => <article className="pm-dashboard-row" key={item.id}>
    <div><h3>{item.title || 'Без названия'}</h3><p>{typeLabel(item.type)} · создано {date(item.created_at)}</p></div>
    <span><small>Статус</small><b className={`pm-status ${item.status}`}>{item.status}</b></span><span><small>Цена</small><b>{price(item)}</b></span>
    <span><small>Просмотры</small><b>{item.views_count || 0}</b></span><span><small>Покупки</small><b>{item.purchases_count || 0}</b></span>
    <div className="pm-row-actions"><a href={`${item.type === 'REGULATION' ? '/profimarket/regulation/edit?id=' : '/profimarket/create?id='}${item.id}`}>Редактировать</a><a href={`/profimarket/solution/${item.id}?preview=1`}>Предпросмотр</a><button onClick={() => action(item)}>{item.status === 'PUBLISHED' ? 'Снять' : 'Удалить'}</button></div>
  </article>) : <div className="pm-loading">У вас пока нет решений. Создайте первое профессиональное решение.</div>}</div>
}

function Orders({ items, orders }) {
  return <div className="pm-dashboard-list">{items.length ? items.map((item) => <article className="pm-order-card" key={item.id}>
    <header><div><h3>{item.title}</h3><small>{typeLabel(item.type)} · {date(item.created_at)}</small></div><a href={`/profimarket/solution/${encodeURIComponent(item.slug)}`}>Открыть решение →</a></header>
    <dl><dt>{orders ? 'Покупатель' : 'Стоимость'}</dt><dd>{orders ? item.buyer_name : `${money(item.amount)} ₽`}</dd><dt>{orders ? 'Контакт' : 'Статус'}</dt><dd>{orders ? item.buyer_email : item.status}</dd>{(item.crm || item.custom_crm_name) && <><dt>CRM</dt><dd>{item.custom_crm_name || item.crm}</dd><dt>E-mail в CRM</dt><dd>{item.crm_email}</dd></>}</dl>
    <dl>{item.comment && <><dt>Комментарий</dt><dd>{item.comment}</dd></>}{item.implementation_status && <><dt>Внедрение</dt><dd><span className={`pm-status ${item.implementation_status}`}>{item.implementation_status}</span></dd></>}<dt>Покупка</dt><dd>№{item.id}</dd></dl>
  </article>) : <div className="pm-loading">{orders ? 'Новых заказов пока нет' : 'Покупок пока нет'}</div>}</div>
}

function Favorites({ items }) {
  return items.length ? <div className="pm-grid">{items.map((item) => <article className="pm-card" key={item.id}><a href={`/profimarket/solution/${encodeURIComponent(item.slug)}`}><h3>{item.title}</h3><p>{item.short_description}</p><strong>{price(item)}</strong></a></article>)}</div> : <div className="pm-loading">В избранном пока нет решений</div>
}

export default function ProfiMarketMyPage() {
  useDocumentPage({ title: 'Мои решения — ПрофиМаркет' })
  usePageStyles(['/static/profimarket.css?v=1'])
  const [params, setParams] = useSearchParams()
  const requestedTab = params.get('tab')
  const tab = ['solutions', 'purchases', 'orders', 'favorites'].includes(requestedTab) ? requestedTab : 'solutions'
  const [items, setItems] = useState([]), [loading, setLoading] = useState(true), [error, setError] = useState('')
  const load = useCallback(() => {
    setLoading(true); setError('')
    let request
    if (tab === 'solutions') request = getMyProfiMarketSolutions()
    else if (tab === 'purchases') request = getMyProfiMarketPurchases()
    else if (tab === 'orders') request = getMyProfiMarketOrders()
    else request = getProfiMarketSolutions({}).then((data) => Promise.all((data.items || []).map((item) => getProfiMarketSolution(item.slug).catch(() => null)))).then((values) => ({ items: values.filter((item) => item?.is_favorite) }))
    request.then((data) => setItems(data.items || [])).catch((requestError) => setError(requestError.message)).finally(() => setLoading(false))
  }, [tab])
  useEffect(load, [load])
  const tabs = [['solutions', 'Мои решения'], ['purchases', 'Мои покупки'], ['orders', 'Заказы и внедрения']]
  if (tab === 'favorites') tabs.unshift(['favorites', 'Избранное'])
  return <PublicLayout><main className="pm-my-page"><div className="pm-shell">
    <header className="pm-dashboard-head"><div><small>ЛИЧНЫЙ КАБИНЕТ</small><h1>ПрофиМаркет</h1><p>Управляйте решениями, покупками и запросами на внедрение.</p></div><a href="/profimarket/create">＋ Создать новое решение</a></header>
    <nav className="pm-dashboard-tabs">{tabs.map(([key, label]) => <button className={tab === key ? 'active' : ''} key={key} onClick={() => setParams({ tab: key })}>{label}</button>)}</nav>
    <section>{loading ? <div className="pm-loading">Загружаем данные…</div> : error ? <div className="pm-loading">{error}</div> : tab === 'solutions' ? <SolutionRows items={items} reload={load} /> : tab === 'favorites' ? <Favorites items={items} /> : <Orders items={items} orders={tab === 'orders'} />}</section>
  </div></main></PublicLayout>
}
