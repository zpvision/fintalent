import { useEffect, useState } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { createProfiMarketSolution, getProfiMarketMeta, getProfiMarketSolution, publishProfiMarketSolution, updateProfiMarketSolution, uploadProfiMarketImage } from '../../api/profimarket'
import Icon from '../../components/Icon'
import ProfiMarketCoverCropper from '../../components/ProfiMarketCoverCropper'
import { useDocumentPage } from '../../hooks/useDocumentPage'
import usePageStyles from '../../hooks/usePageStyles'
import PublicLayout from '../../layouts/PublicLayout'

const creationPayload = (type) => ({ type, pricing_type: type === 'AI_ASSISTANT' ? 'MONTHLY' : 'ONE_TIME', delivery_type: 'MANUAL' })
const solutionPayload = (value) => ({
  type: value.type,
  title: value.title || '',
  short_description: value.short_description || '',
  description: value.description || '',
  cover_image: value.cover_image || '',
  price: Number(value.price || 0),
  old_price: value.old_price ? Number(value.old_price) : null,
  currency: value.currency || 'RUB',
  pricing_type: value.pricing_type || 'MONTHLY',
  trial_days: Number(value.trial_days || 0),
  delivery_type: value.delivery_type || 'MANUAL',
  external_url: value.external_url || '',
  tags: value.tags || [],
  topics: value.topics || [],
  audiences: value.audiences || [],
  sections: value.sections || [],
  access_features: value.access_features || [],
  ai_features: (value.ai_features || []).map((item, index) => ({ ...item, sort_order: index })),
  how_it_works: (value.how_it_works || []).map((item, index) => ({ ...item, sort_order: index })),
  media: (value.media || []).map((item, index) => ({ ...item, sort_order: index })),
  crm_ids: value.crm_ids || [],
  platform_ids: value.platform_ids || [],
  key_metrics: value.key_metrics || [],
  bonuses: value.bonuses || [],
  bonus_style: value.bonus_style || '',
  metric_style: value.metric_style || '',
  access_style: value.access_style || '',
  right_block_title: value.right_block_title || '',
  implementation_title: value.implementation_title || '',
  implementation_subtitle: value.implementation_subtitle || '',
  purchase_button_code: value.purchase_button_code || '',
})
const steps = ['Основная информация', 'Что умеет', 'Где работает', 'Демо и изображения', 'Цена и пробный период', 'Как это работает', 'Публикация']
const defaultHowItWorks = () => [
  { icon: 'bag', title: 'Оформите доступ', description: 'Выберите пробный период или подписку.', sort_order: 0 },
  { icon: 'link', title: 'Получите ссылку', description: 'Получите доступ удобным способом.', sort_order: 1 },
  { icon: 'sparkles', title: 'Решайте задачи', description: 'Используйте помощника в ежедневной работе.', sort_order: 2 },
]
const field = (event) => event.target.type === 'number' ? Number(event.target.value) : event.target.value

function TypeChoice({ choose, busy }) {
  const types=[['REGULATION','workflow','Регламент','Готовый рабочий процесс для внедрения в компании.'],['AI_ASSISTANT','bot','ИИ-ассистент','Бот или помощник для ежедневных профессиональных задач.'],['AUTOMATION','sparkles','Автоматизация','Готовый сценарий автоматизации конкретного процесса.'],['INSTRUCTION','list','Инструкция','Профессиональная пошаговая инструкция с реальным фрагментом.'],['ONEC_INTEGRATION','calculator','1С Интеграция','Расширение, обработка, отчёт или интеграционный модуль.'],['TEMPLATE','folder','Шаблон','Рабочий файл, таблица, документ или набор материалов.'],['CHECKLIST','check','Чек-лист','Структурированный список проверок и контрольных действий.']]
  return <div className="pm-create-shell"><div className="pm-create-head"><Link to="/profimarket">← Вернуться в ПрофиМаркет</Link><span><i>✓</i> Единый кабинет FinTalent</span></div><section className="pm-type-choice"><small>НОВЫЙ ПРОДУКТ</small><h1>Что вы хотите разместить на ПрофиМаркете?</h1><p>Выберите тип продукта — откроется подходящий пошаговый мастер.</p><div className="pm-type-grid pm-type-grid-all">{types.map(([type,icon,title,text])=><button disabled={busy} className={`pm-type-option ${type.toLowerCase()}`} onClick={()=>choose(type)} key={type}><i><Icon name={icon}/></i><h2>{title}</h2><p>{text}</p><strong>Выбрать →</strong></button>)}</div></section></div>
}

function Basic({ value, change, upload }) {
  return <><header><small>ШАГ 1</small><h1>Основная информация</h1><p>Расскажите, какую профессиональную задачу решает ваше решение.</p></header><div className="pm-ai-basic-layout"><div className="pm-ai-basic-copy"><label className="pm-field">Название<input value={value.title} onChange={change('title')} /></label><label className="pm-field">Описание<textarea rows="7" value={value.short_description} onChange={change('short_description')} /></label></div><label className="pm-field pm-ai-basic-cover">Обложка<div className="pm-upload-box">{value.cover_image ? <img src={value.cover_image} alt="Обложка" /> : <span><b>Загрузить обложку</b><small>Рекомендуемый размер 1200 × 675 px<br />JPG, PNG или WebP до 7 МБ</small></span>}<input type="file" accept="image/*" onChange={(event) => { upload(event.target.files[0], 'cover'); event.target.value = '' }} /></div></label><div className="pm-form-grid pm-ai-basic-meta">{[['tags','Теги через запятую'],['topics','Тематика через запятую'],['audiences','Для кого предназначено']].map(([name,label]) => <label className="pm-field" key={name}>{label}<input value={(value[name] || []).join(', ')} onChange={(event) => change(name)({ target: { value: event.target.value.split(',').map((item) => item.trim()).filter(Boolean) } })} /></label>)}</div></div></>
}

function Features({ items, setItems }) {
  return <><header><small>ВОЗМОЖНОСТИ</small><h1>Что умеет помощник</h1><p>Добавьте ключевые возможности ИИ-инструмента.</p></header><div className="pm-repeat-list">{items.map((item, index) => <article className="pm-repeat-item" key={index}><button className="pm-remove" onClick={() => setItems(items.filter((_, i) => i !== index))}>×</button><div className="pm-form-grid"><label className="pm-field">Заголовок<input value={item.title} onChange={(event) => setItems(items.map((x, i) => i === index ? { ...x, title: event.target.value } : x))} /></label><label className="pm-field">Подпись<input value={item.description} onChange={(event) => setItems(items.map((x, i) => i === index ? { ...x, description: event.target.value } : x))} /></label></div></article>)}</div><button className="pm-add" onClick={() => setItems([...items, { icon: 'sparkles', title: '', description: '', sort_order: items.length }])}>＋ Добавить возможность</button></>
}

function Platforms({ meta, selected, setSelected }) {
  return <><header><small>СПРАВОЧНИК</small><h1>Где работает помощник</h1><p>Можно выбрать несколько платформ.</p></header><div className="pm-check-grid">{(meta.platforms || []).map((item) => <label className="pm-choice-check" key={item.id}><input type="checkbox" checked={selected.includes(item.id)} onChange={() => setSelected(selected.includes(item.id) ? selected.filter((id) => id !== item.id) : [...selected, item.id])} />{item.icon ? <img src={item.icon} alt="" /> : <Icon name={item.code === 'telegram' ? 'telegram' : 'cloud'} />}<span><b>{item.name}</b><small>{item.code}</small></span></label>)}</div></>
}

function Media({ value, change, upload, remove }) {
  return <><header><small>ПРОДАЮЩАЯ ДЕМОНСТРАЦИЯ</small><h1>Видео и изображения</h1><p>Добавьте video URL и скриншоты.</p></header><div className="pm-form-grid"><label className="pm-field wide">URL демонстрационного видео<input type="url" value={(value.media.find((item) => item.type === 'VIDEO') || {}).url || ''} onChange={(event) => change(event.target.value)} /></label><label className="pm-field wide">Добавить скриншот<div className="pm-upload-box"><span><b>Загрузить изображение</b><small>Рекомендуемый размер 1440 × 900 px · можно добавить несколько изображений</small></span><input type="file" accept="image/*" onChange={(event) => { upload(event.target.files[0], 'media'); event.target.value = '' }} /></div></label></div><div className="pm-thumbnails">{value.media.filter((item) => item.type === 'IMAGE').map((item) => <span key={item.url}><img src={item.url} alt="" /><button onClick={() => remove(item)}>×</button></span>)}</div></>
}

function HowItWorks({ items, setItems }) {
  return <><header><small>СЦЕНАРИЙ ИСПОЛЬЗОВАНИЯ</small><h1>Как это работает</h1><p>Опишите понятные шаги, которые пройдёт покупатель после оформления доступа.</p></header><div className="pm-repeat-list">{items.map((item, index) => <article className="pm-repeat-item pm-how-editor-item" key={index}><i>{index + 1}</i><button className="pm-remove" onClick={() => setItems(items.filter((_, i) => i !== index))}>×</button><div className="pm-form-grid"><label className="pm-field">Заголовок<input value={item.title} onChange={(event) => setItems(items.map((x, i) => i === index ? { ...x, title: event.target.value } : x))} /></label><label className="pm-field">Описание<input value={item.description} onChange={(event) => setItems(items.map((x, i) => i === index ? { ...x, description: event.target.value } : x))} /></label></div></article>)}</div><button className="pm-add" onClick={() => setItems([...items, { icon: 'sparkles', title: '', description: '', sort_order: items.length }])}>＋ Добавить пункт</button></>
}

function Pricing({ value, change }) {
  return <><header><small>МОНЕТИЗАЦИЯ</small><h1>Цена и пробный период</h1><p>Настройте понятное предложение для покупателя.</p></header><div className="pm-ai-pricing"><section className="pm-ai-price-main"><label className="pm-field">Цена, ₽<input type="number" min="0" value={value.price || ''} onChange={change('price')} placeholder="0" /></label><label className="pm-field">Модель оплаты<select value={value.pricing_type} onChange={change('pricing_type')}><option value="ONE_TIME">Разовая покупка</option><option value="MONTHLY">Цена в месяц</option><option value="YEARLY">Цена в год</option><option value="FREE">Бесплатно</option></select></label></section><section className="pm-ai-price-secondary"><label className="pm-field">Старая цена, ₽<input type="number" min="0" value={value.old_price || ''} onChange={change('old_price')} placeholder="Необязательно" /></label><p>Показывается зачёркнутой рядом с текущей ценой.</p></section><section className="pm-ai-trial"><div><b>Пробный период</b><p>Укажите количество бесплатных дней. Оставьте 0, если пробного периода нет.</p></div><label className="pm-field">Количество дней<input type="number" min="0" value={value.trial_days || ''} onChange={change('trial_days')} placeholder="0" /></label></section></div></>
}

export default function ProfiMarketCreatePage() {
  usePageStyles(['/static/profimarket.css?v=2', '/static/profimarket-ai-editor.css?v=4', '/static/profimarket-product.css?v=2']); useDocumentPage({ title: 'Разместить решение — ПрофиМаркет' })
  const navigate = useNavigate()
  const [params] = useSearchParams(), editID = params.get('id'), [solution, setSolution] = useState(null), [meta, setMeta] = useState({}), [step, setStep] = useState(0), [busy, setBusy] = useState(false), [notice, setNotice] = useState(''), [error, setError] = useState(''), [coverFile, setCoverFile] = useState(null)
  useEffect(() => { setError(''); Promise.all([getProfiMarketMeta(), editID ? getProfiMarketSolution(editID) : null]).then(([dictionary, current]) => { setMeta(dictionary); if (current?.type === 'REGULATION') navigate(`/profimarket/regulation/edit?id=${current.id}`, { replace: true }); else if(current&&current.type!=='AI_ASSISTANT')navigate(`/profimarket/product/edit?id=${current.id}`,{replace:true});else if (current) { setSolution({ ...current, platform_ids: (current.platforms || []).map((item) => item.id), ai_features: current.ai_features || [], how_it_works: current.how_it_works?.length ? current.how_it_works : defaultHowItWorks(), media: current.media || [] }); setBusy(false) } }).catch((requestError) => { setError(requestError.message); setBusy(false) }) }, [editID, navigate])
  const change = (name) => (event) => setSolution((value) => ({ ...value, [name]: field(event) }))
  async function choose(type) { setBusy(true); setError(''); try { const created = await createProfiMarketSolution(creationPayload(type)); window.location.assign(type === 'REGULATION' ? `/profimarket/regulation/edit?id=${created.id}` : type === 'AI_ASSISTANT' ? `/profimarket/create?id=${created.id}` : `/profimarket/product/edit?id=${created.id}`) } catch (requestError) { setError(requestError.message); setBusy(false) } }
  function payload() { return solutionPayload(solution) }
  async function save() { setBusy(true); try { const saved = await updateProfiMarketSolution(solution.id, payload()); setSolution({ ...saved, platform_ids: (saved.platforms || []).map((item) => item.id), ai_features: saved.ai_features || [], how_it_works: saved.how_it_works || [], media: saved.media || [] }); setNotice('Черновик сохранён'); return true } catch (requestError) { setError(requestError.message); return false } finally { setBusy(false) } }
  async function move(next) { if (await save()) setStep(next) }
  async function upload(file, kind) { if (!file) return false; setBusy(true); try { const data = await uploadProfiMarketImage(file); if (kind === 'cover') setSolution((value) => ({ ...value, cover_image: data.url })); else setSolution((value) => ({ ...value, media: [...value.media, { type: 'IMAGE', url: data.url, is_preview: !value.media.length }] })); return true } catch (requestError) { setError(requestError.message); return false } finally { setBusy(false) } }
  function selectImage(file, kind) { if (!file) return; if (kind === 'cover') setCoverFile(file); else upload(file, kind) }
  async function publish() { if (!await save()) return; try { await publishProfiMarketSolution(solution.id); navigate(`/profimarket/solution/${solution.slug}`) } catch (requestError) { setError(requestError.message) } }
  if (error && !solution && editID) return <PublicLayout><main className="pm-create-page"><div className="pm-loading">{error}</div></main></PublicLayout>
  if (!solution) return <PublicLayout><main className="pm-create-page">{error && <div className="pm-notice bad">{error}</div>}<TypeChoice choose={choose} busy={busy} /></main></PublicLayout>
  let content
  if (step === 0) content = <Basic value={solution} change={change} upload={selectImage} />
  else if (step === 1) content = <Features items={solution.ai_features} setItems={(items) => setSolution({ ...solution, ai_features: items })} />
  else if (step === 2) content = <Platforms meta={meta} selected={solution.platform_ids} setSelected={(platform_ids) => setSolution({ ...solution, platform_ids })} />
  else if (step === 3) content = <Media value={solution} change={(url) => setSolution({ ...solution, media: [...solution.media.filter((item) => item.type !== 'VIDEO'), ...(url ? [{ type: 'VIDEO', url, is_preview: true }] : [])] })} upload={upload} remove={(target) => setSolution({ ...solution, media: solution.media.filter((item) => item !== target) })} />
  else if (step === 4) content = <Pricing value={solution} change={change} />
  else if (step === 5) content = <HowItWorks items={solution.how_it_works} setItems={(how_it_works) => setSolution({ ...solution, how_it_works })} />
  else content = <><header><small>ФИНАЛЬНЫЙ ШАГ</small><h1>Всё готово к публикации</h1><p>Сначала посмотрите карточку глазами покупателя, затем откройте её для всех.</p></header><div className="pm-ai-publish-visual"><img src="/static/profimarket-ai-publish.png" alt="ИИ-ассистент готов к публикации" /><div><b>{solution.title || 'Ваш ИИ-ассистент готов к запуску'}</b><p>{solution.short_description || 'Заполните основную информацию и возможности решения.'}</p></div></div><div className="pm-ai-publish-actions"><a className="secondary" href={`/profimarket/solution/${solution.id}?preview=1`} target="_blank" rel="noreferrer">Просмотреть ↗</a><button className="pm-buy pm-ai-publish-button" disabled={!solution.title || !solution.short_description || !solution.ai_features.length || busy} onClick={publish}>Опубликовать <b>→</b></button></div></>
  return <PublicLayout><main className="pm-create-page pm-ai-create-page"><div className="pm-create-shell"><div className="pm-create-head"><Link to="/profile?section=profimarket">← Мои решения</Link><span><i>✓</i><b>{notice || 'Черновик сохранён'}</b></span></div><div className="pm-editor-layout"><aside className="pm-wizard-nav"><small>ИИ-АССИСТЕНТ</small>{steps.map((label, index) => <button className={index === step ? 'active' : ''} key={label} onClick={() => move(index)}><i>{index + 1}</i>{label}</button>)}</aside><div><section className="pm-editor-card">{content}</section><footer className="pm-editor-footer"><span>{step + 1} из {steps.length} · {solution.status === 'PUBLISHED' ? 'Опубликовано' : 'Черновик'}</span>{step > 0 && <button className="secondary" onClick={() => move(step - 1)}>← Назад</button>}<button className="secondary" disabled={busy} onClick={save}>Сохранить</button>{step < steps.length - 1 && <button className="primary" disabled={busy} onClick={() => move(step + 1)}>Продолжить →</button>}</footer></div></div></div>{error && <div className="pm-notice bad">{error}</div>}</main>{coverFile && <ProfiMarketCoverCropper file={coverFile} onCancel={() => setCoverFile(null)} onReady={async file => { const uploaded = await upload(file, 'cover'); if (uploaded) setCoverFile(null); return uploaded }} />}</PublicLayout>
}
