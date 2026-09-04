import { useEffect, useRef, useState } from 'react'
import { useLocation, useParams } from 'react-router-dom'
import { addProfiMarketFavorite, getProfiMarketMeta, getProfiMarketSolution, purchaseProfiMarketSolution, removeProfiMarketFavorite } from '../../api/profimarket'
import { useDocumentPage } from '../../hooks/useDocumentPage'
import usePageStyles from '../../hooks/usePageStyles'
import PublicLayout from '../../layouts/PublicLayout'

let uiPromise
function loadPresentation() {
  if (window.ProfiMarketUI) return Promise.resolve(window.ProfiMarketUI)
  if (!uiPromise) uiPromise = new Promise((resolve, reject) => {
    const script = document.createElement('script')
    script.src = '/static/profimarket-components.js?v=12'
    script.onload = () => resolve(window.ProfiMarketUI)
    script.onerror = () => reject(new Error('Не удалось загрузить компоненты страницы'))
    document.head.append(script)
  })
  return uiPromise
}

function Notice({ value }) {
  if (!value) return null
  return <div className={`pm-notice${value.bad ? ' bad' : ''}`}>{value.text}</div>
}

function PurchaseModal({ solution, close, done, fail }) {
  const [crms, setCrms] = useState(solution.crms || []), [crmID, setCrmID] = useState(''), [submitting, setSubmitting] = useState(false)
  useEffect(() => {
    if (crms.length) { setCrmID(String(crms[0].id)); return }
    getProfiMarketMeta().then((data) => { setCrms(data.crms || []); setCrmID(String(data.crms?.[0]?.id || '')) }).catch((error) => { fail(error.message); close() })
  }, [])
  async function submit(event) {
    event.preventDefault(); setSubmitting(true)
    const form = new FormData(event.currentTarget)
    try {
      const data = await purchaseProfiMarketSolution(solution.id, { crm_id: Number(form.get('crm_id')), custom_crm_name: form.get('custom_crm_name'), crm_email: form.get('crm_email'), comment: form.get('comment') })
      close(); done(data.message || 'Покупка оформлена')
    } catch (error) { fail(error.message) } finally { setSubmitting(false) }
  }
  const selected = crms.find((item) => String(item.id) === crmID)
  return <div className="pm-modal"><section role="dialog" aria-modal="true"><header><div><small>ПОКУПКА И ВНЕДРЕНИЕ</small><h2>{solution.title}</h2></div><button className="pm-modal-close" onClick={close}>×</button></header><p>Укажите учетную запись, в которую автор поможет внедрить регламенты. Пароль от CRM никогда не требуется.</p><form onSubmit={submit}><div className="pm-form-grid"><label className="pm-field wide">CRM<select name="crm_id" required value={crmID} onChange={(event) => setCrmID(event.target.value)}>{crms.map((item) => <option value={item.id} key={item.id}>{item.name}</option>)}</select></label>{selected?.code === 'other' && <label className="pm-field wide">Название CRM<input name="custom_crm_name" required /></label>}<label className="pm-field wide">E-mail учетной записи в CRM<input name="crm_email" type="email" required placeholder="name@company.ru" /></label><label className="pm-field wide">Комментарий для внедрения<textarea name="comment" rows="3" placeholder="Необязательно" /></label></div><footer><button type="button" className="secondary" onClick={close}>Отмена</button><button className="primary" disabled={submitting}>Купить за {new Intl.NumberFormat('ru-RU').format(solution.price || 0)} ₽</button></footer></form></section></div>
}

export default function ProfiMarketDetailPage() {
  usePageStyles(['/static/profimarket.css?v=1'])
  const { key } = useParams(), location = useLocation(), root = useRef(null)
  const [solution, setSolution] = useState(null), [html, setHTML] = useState(''), [error, setError] = useState(''), [modal, setModal] = useState(false), [notice, setNotice] = useState(null)
  useDocumentPage({ title: solution ? `${solution.title} — ПрофиМаркет` : 'Решение — ПрофиМаркет' })
  const preview = new URLSearchParams(location.search).get('preview') === '1'
  function notify(text, bad = false) { setNotice({ text, bad }); window.setTimeout(() => setNotice(null), 3000) }
  useEffect(() => {
    const controller = new AbortController(); setError(''); setSolution(null)
    Promise.all([getProfiMarketSolution(key, { signal: controller.signal }), loadPresentation()]).then(([data, ui]) => { setSolution(data); setHTML(ui.solutionView(data, preview)) }).catch((requestError) => { if (requestError.name !== 'AbortError') setError(requestError.message) })
    return () => controller.abort()
  }, [key, preview])
  useEffect(() => {
    if (!solution || solution.type !== 'REGULATION' || !root.current) return
    const art = root.current.querySelector('.pmr-product-art')
    if (solution.cover_image && art) {
      art.classList.add('has-cover')
      const image = document.createElement('img')
      image.src = solution.cover_image; image.alt = solution.title
      art.replaceChildren(image)
    }
    const access = root.current.querySelector('.pmr-access')
    if (!solution.access_features?.length) access?.remove()
    else if (solution.right_block_title) {
      const title = access?.querySelector('h2')
      if (title) title.textContent = solution.right_block_title
    }
  }, [html, solution])
  async function favorite(button) {
    try {
      const data = button.classList.contains('active') ? await removeProfiMarketFavorite(solution.id) : await addProfiMarketFavorite(solution.id)
      root.current.querySelectorAll('[data-favorite]').forEach((item) => { item.classList.toggle('active', data.active); item.lastChild.textContent = data.active ? ' В избранном' : ' Добавить в избранное' })
    } catch (requestError) { notify(requestError.message, true) }
  }
  async function buy() {
    if (solution.type === 'REGULATION') { setModal(true); return }
    try { const data = await purchaseProfiMarketSolution(solution.id); notify(data.message || 'Доступ оформлен') } catch (requestError) { notify(requestError.message, true) }
  }
  function interact(event) {
    const favoriteButton = event.target.closest('[data-favorite]'), buyButton = event.target.closest('[data-buy]'), tabButton = event.target.closest('[data-section-tab]')
    if (favoriteButton) favorite(favoriteButton)
    else if (buyButton) buy()
    else if (tabButton) { root.current.querySelectorAll('[data-section-tab]').forEach((item) => item.classList.toggle('active', item === tabButton)); root.current.querySelectorAll('[data-section]').forEach((item) => item.classList.toggle('hidden', item.dataset.section !== tabButton.dataset.sectionTab)) }
  }
  return <PublicLayout><main ref={root} id="pm-detail" className="pm-detail-page" onClick={interact}>{error ? <div className="pm-detail-loading"><h1>Решение не найдено</h1><p>{error}</p><a href="/profimarket">Вернуться в ПрофиМаркет</a></div> : !solution ? <div className="pm-detail-loading"><i /><b>Загружаем решение…</b></div> : <div dangerouslySetInnerHTML={{ __html: html }} />}</main>{modal && <PurchaseModal solution={solution} close={() => setModal(false)} done={notify} fail={(text) => notify(text, true)} />}<Notice value={notice} /></PublicLayout>
}
