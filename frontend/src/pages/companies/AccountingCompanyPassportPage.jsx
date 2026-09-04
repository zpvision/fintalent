import { useEffect, useRef, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { getAccountingCompany, getAccountingCompanyPassport } from '../../api/companies'
import { useDocumentPage } from '../../hooks/useDocumentPage'
import usePageStyles from '../../hooks/usePageStyles'
import PublicLayout from '../../layouts/PublicLayout'

const formatDate = value => value ? new Date(value).toLocaleDateString('ru-RU') : '—'

function Radar({ scores, color }) {
  const ref = useRef(null)
  useEffect(() => {
    const canvas = ref.current, shown = scores.length > 12 ? scores.slice(0, 12) : scores, n = shown.length
    if (!canvas || n < 3) return
    const ctx = canvas.getContext('2d'), cx = 450, cy = 450, radius = 270
    ctx.clearRect(0, 0, canvas.width, canvas.height)
    ctx.font = '600 19px Inter'; ctx.textAlign = 'center'; ctx.textBaseline = 'middle'
    for (let level = 1; level <= 5; level++) {
      ctx.beginPath(); shown.forEach((_, index) => { const angle=-Math.PI/2+index*2*Math.PI/n,x=cx+Math.cos(angle)*radius*level/5,y=cy+Math.sin(angle)*radius*level/5; index?ctx.lineTo(x,y):ctx.moveTo(x,y) }); ctx.closePath(); ctx.strokeStyle='#dce5f0'; ctx.stroke()
    }
    shown.forEach((score,index)=>{const angle=-Math.PI/2+index*2*Math.PI/n;ctx.beginPath();ctx.moveTo(cx,cy);ctx.lineTo(cx+Math.cos(angle)*radius,cy+Math.sin(angle)*radius);ctx.strokeStyle='#e4eaf3';ctx.stroke();ctx.fillStyle='#4f5f83';const label=score.name.length>20?`${score.name.slice(0,19)}…`:score.name;ctx.fillText(label,cx+Math.cos(angle)*(radius+75),cy+Math.sin(angle)*(radius+60))})
    ctx.beginPath(); shown.forEach((score,index)=>{const angle=-Math.PI/2+index*2*Math.PI/n,r=radius*score.percent/100,x=cx+Math.cos(angle)*r,y=cy+Math.sin(angle)*r;index?ctx.lineTo(x,y):ctx.moveTo(x,y)});ctx.closePath();ctx.fillStyle=`${color}30`;ctx.fill();ctx.strokeStyle=color;ctx.lineWidth=6;ctx.stroke()
  }, [scores, color])
  return <canvas ref={ref} className="ac-radar" width="900" height="900" />
}

export default function AccountingCompanyPassportPage() {
  usePageStyles(['/static/accounting-company.css?v=1'])
  const [params] = useSearchParams(), id = params.get('id')
  const [company,setCompany]=useState(null),[passport,setPassport]=useState(null),[error,setError]=useState(''),[selected,setSelected]=useState(null)
  useDocumentPage({ title: company ? `Паспорт компетенций — ${company.name}` : 'Паспорт компетенций — FinTalent' })
  useEffect(()=>{if(!id){setError('Не указана компания');return}const controller=new AbortController();Promise.all([getAccountingCompany(encodeURIComponent(id),{signal:controller.signal}),getAccountingCompanyPassport(id,{signal:controller.signal})]).then(([value,data])=>{setCompany(value.company);setPassport(data)}).catch(e=>{if(e.name!=='AbortError')setError(e.message)});return()=>controller.abort()},[id])
  if(error)return <PublicLayout><main className="ac-page"><div className="ac-empty"><i>!</i><h3>Не удалось открыть Паспорт</h3><p>{error}</p></div></main></PublicLayout>
  if(!company||!passport)return <PublicLayout><main className="ac-page"><div className="ac-loading-page"><div className="ac-skeleton"/></div></main></PublicLayout>
  const color=company.accent?.color_value||'#1768e8',scores=passport.scores||[],strongest=scores.slice(0,3)
  return <PublicLayout><main className="ac-page" style={{'--company-accent':color}}><section className="ac-passport-hero"><a href={`/accounting-companies/view?slug=${encodeURIComponent(company.slug)}`}>← Страница компании</a><h1>Паспорт компетенций</h1><p>{company.name}{scores.length?' · автоматически сформирован по результатам тестирования FinTalent':''}</p></section>{!scores.length?<div className="ac-content-panel" style={{marginTop:20}}><div className="ac-passport-empty"><i>◇</i><h3>Паспорт компетенций пока формируется</h3><p>После первого завершённого тестирования здесь появятся подтверждённые компетенции команды.</p><a className="ac-button" href="/profile?section=employee-testing">Начать тестирование</a></div></div>:<><section className="ac-passport-layout"><div className="ac-panel ac-passport-chart-panel"><span className="ac-kicker">ОБЩИЙ ИНДЕКС</span><div className="ac-passport-big-index">{Math.round(passport.overall_index)}%</div><Radar scores={scores} color={color}/><div className="ac-passport-stats"><div><b>{passport.confirmed_competencies}</b><small>компетенций</small></div><div><b>{passport.tests_count}</b><small>тестирований</small></div><div><b>{passport.specialists_count}</b><small>специалистов</small></div></div></div><aside className="ac-panel"><div className="ac-section-title"><h2>Все компетенции</h2></div><div className="ac-score-list">{scores.map(score=><article className="ac-score-row" key={score.name} onClick={()=>setSelected(score)}><div><span>{score.name}</span><b>{Math.round(score.percent)}%</b></div><div className="ac-score-bar"><i style={{width:`${score.percent}%`}}/></div><small style={{fontSize:8,color:'#7180a4'}}>Подтвердили: {score.specialists} · тестов: {score.tests} · {formatDate(score.last_confirmed_at)}</small></article>)}</div></aside></section><section className="ac-panel ac-history"><div className="ac-section-title"><h2>Сильнейшие компетенции</h2></div><div className="ac-advantages">{strongest.map(score=><span key={score.name}>◆ {score.name} — {Math.round(score.percent)}%</span>)}</div></section>{passport.history?.length>0&&<section className="ac-panel ac-history"><div className="ac-section-title"><h2>История тестирования</h2></div>{passport.history.map((item,index)=><div className="ac-history-row" key={`${item.test_title}-${item.finished_at}-${index}`}><b>{item.test_title}</b><span>{item.specialist_name}</span><b>{Math.round(item.percent)}%</b><span>{formatDate(item.finished_at)}</span></div>)}</section>}</>}</main>{selected&&<div className="ac-modal" onMouseDown={e=>e.target===e.currentTarget&&setSelected(null)}><section className="ac-modal-box"><div className="ac-modal-head"><h2>{selected.name}</h2><button onClick={()=>setSelected(null)}>×</button></div><div className="ac-passport-big-index" style={{fontSize:32,margin:'20px 0'}}>{Math.round(selected.percent)}%</div><p style={{fontSize:11,color:'#7180a4'}}>Средний подтверждённый результат · {selected.specialists} специалистов · {selected.tests} тестов</p>{(passport.history||[]).filter(item=>item.test_title===selected.name).map((item,index)=><div className="ac-history-row" key={index}><b>{item.specialist_name}</b><b>{Math.round(item.percent)}%</b><span>{formatDate(item.finished_at)}</span></div>)}</section></div>}</PublicLayout>
}
