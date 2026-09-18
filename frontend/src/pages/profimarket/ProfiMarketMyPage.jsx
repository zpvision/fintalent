import { useCallback, useEffect, useState } from 'react'
import { getProfiMarketSolution, getProfiMarketSolutions } from '../../api/profimarket'
import { useDocumentPage } from '../../hooks/useDocumentPage'
import usePageStyles from '../../hooks/usePageStyles'
import PublicLayout from '../../layouts/PublicLayout'

const money=value=>new Intl.NumberFormat('ru-RU').format(value||0)
const price=item=>item.is_free?'Бесплатно':`${money(item.price)} ₽`

export default function ProfiMarketMyPage(){
  useDocumentPage({title:'Избранное — ПрофиМаркет'})
  usePageStyles(['/static/profimarket.css?v=1'])
  const[items,setItems]=useState([]),[loading,setLoading]=useState(true),[error,setError]=useState('')
  const load=useCallback(()=>{setLoading(true);setError('');getProfiMarketSolutions({}).then(data=>Promise.all((data.items||[]).map(item=>getProfiMarketSolution(item.slug).catch(()=>null)))).then(values=>setItems(values.filter(item=>item?.is_favorite))).catch(e=>setError(e.message)).finally(()=>setLoading(false))},[])
  useEffect(load,[load])
  return <PublicLayout><main className="pm-my-page"><div className="pm-shell"><header className="pm-dashboard-head"><div><small>ПРОФИМАРКЕТ</small><h1>Избранное</h1><p>Решения, которые вы сохранили, чтобы вернуться к ним позже.</p></div><a href="/profimarket">Вернуться в каталог</a></header>{loading?<div className="pm-loading">Загружаем данные…</div>:error?<div className="pm-loading">{error}</div>:items.length?<div className="pm-grid">{items.map(item=><article className="pm-card" key={item.id}><a href={`/profimarket/solution/${encodeURIComponent(item.slug)}`}><h3>{item.title}</h3><p>{item.short_description}</p><strong>{price(item)}</strong></a></article>)}</div>:<div className="pm-loading">В избранном пока нет решений</div>}</div></main></PublicLayout>
}
