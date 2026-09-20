import { useCallback, useEffect, useState } from 'react'
import { getProfiMarketSolution, getProfiMarketSolutions } from '../../api/profimarket'
import Icon from '../../components/Icon'
import { useDocumentPage } from '../../hooks/useDocumentPage'
import usePageStyles from '../../hooks/usePageStyles'
import PublicLayout from '../../layouts/PublicLayout'

const money=value=>new Intl.NumberFormat('ru-RU').format(value||0)
const price=item=>item.is_free||item.pricing_type==='FREE'||!Number(item.price)?'Бесплатно':`${money(item.price)} ₽`
const visuals={AI_ASSISTANT:['bot','AI','ai'],REGULATION:['workflow','PRO','reg'],AUTOMATION:['sparkles','AUTO','automation'],INSTRUCTION:['list','DOC','instruction'],ONEC_INTEGRATION:['calculator','1C','onec'],TEMPLATE:['folder','FILE','template'],CHECKLIST:['check','CHECK','checklist']}
const labels={AI_ASSISTANT:'ИИ-ассистенты',REGULATION:'Регламенты',AUTOMATION:'Автоматизации',INSTRUCTION:'Инструкции',ONEC_INTEGRATION:'1С Интеграции',TEMPLATE:'Шаблоны',CHECKLIST:'Чек-листы'}

function FavoriteCard({item}){
  const visual=visuals[item.type]||visuals.REGULATION
  return <article className="pmh-card"><div className={`pmh-card-cover ${visual[2]}${item.cover_image?' has-image':''}`}>{item.is_new?<em>НОВИНКА</em>:null}{item.cover_image?<img src={item.cover_image} alt={item.title} loading="lazy"/>:<><i><Icon name={visual[0]}/></i><b>{visual[1]}</b></>}</div><div className="pmh-card-body"><small className="pmh-card-type">{labels[item.type]||'Профессиональное решение'}</small><h3>{item.title}</h3><p>{item.short_description}</p><div className="pmh-author">{item.author_avatar?<img src={item.author_avatar} alt=""/>:<i>{(item.author_name||'А').charAt(0)}</i>}<span>{item.author_name||'Автор FinTalent'}</span></div><footer><span><b>★ {Number(item.rating||0).toFixed(1)}</b> ({item.review_count||0})</span><strong>{price(item)}</strong></footer></div><a href={`/profimarket/solution/${encodeURIComponent(item.slug)}`} aria-label={`Открыть ${item.title}`}/></article>
}

export default function ProfiMarketMyPage(){
  useDocumentPage({title:'Избранное — ПрофиМаркет'})
  usePageStyles(['/static/profimarket.css?v=1','/static/profimarket-home.css?v=9','/static/profimarket-product.css?v=3'])
  const[items,setItems]=useState([]),[loading,setLoading]=useState(true),[error,setError]=useState('')
  const load=useCallback(()=>{setLoading(true);setError('');getProfiMarketSolutions({}).then(data=>Promise.all((data.items||[]).map(item=>getProfiMarketSolution(item.slug).catch(()=>null)))).then(values=>setItems(values.filter(item=>item?.is_favorite))).catch(e=>setError(e.message)).finally(()=>setLoading(false))},[])
  useEffect(load,[load])
  return <PublicLayout><main className="pm-my-page"><div className="pm-shell"><header className="pm-dashboard-head"><div><small>ПРОФИМАРКЕТ</small><h1>Избранное</h1><p>Решения, которые вы сохранили, чтобы вернуться к ним позже.</p></div><a href="/profimarket">Вернуться в каталог</a></header>{loading?<div className="pm-loading">Загружаем данные…</div>:error?<div className="pm-loading">{error}</div>:items.length?<div className="pmh-cards">{items.map(item=><FavoriteCard item={item} key={item.id}/>)}</div>:<div className="pm-loading">В избранном пока нет решений</div>}</div></main></PublicLayout>
}
