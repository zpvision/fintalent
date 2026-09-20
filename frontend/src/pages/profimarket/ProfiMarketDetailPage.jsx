import { useEffect, useRef, useState } from 'react'
import { useLocation, useParams } from 'react-router-dom'
import { addProfiMarketFavorite, getProfiMarketMeta, getProfiMarketSolution, purchaseProfiMarketSolution, removeProfiMarketFavorite } from '../../api/profimarket'
import { useDocumentPage } from '../../hooks/useDocumentPage'
import usePageStyles from '../../hooks/usePageStyles'
import PublicLayout from '../../layouts/PublicLayout'
import PublishSuccessModal from '../../components/PublishSuccessModal'
import ProfiMarketProductDetail from './ProfiMarketProductDetail'
import ProfiMarketReviews from '../../components/ProfiMarketReviews'

let uiPromise
function loadPresentation() {
  if (window.ProfiMarketUI?.version >= 37) return Promise.resolve(window.ProfiMarketUI)
  if (!uiPromise) uiPromise = new Promise((resolve, reject) => {
    const load = (src, done) => {
      const script = document.createElement('script')
      script.src = src; script.onload = done
      script.onerror = () => reject(new Error('Не удалось загрузить компоненты страницы'))
      document.head.append(script)
    }
    const loadComponents = () => load('/static/profimarket-components.js?v=37', () => resolve(window.ProfiMarketUI))
    if (window.ProfiMarketStylePresets) loadComponents()
    else load('/static/profimarket-style-presets.js?v=3', loadComponents)
  })
  return uiPromise
}

function Notice({ value }) {
  if (!value) return null
  return <div className={`pm-notice${value.bad ? ' bad' : ''}`}>{value.text}</div>
}

function DetailImageLightbox({ image, close }) {
  useEffect(() => {
    const keydown = (event) => { if (event.key === 'Escape') close() }
    document.addEventListener('keydown', keydown)
    const previous = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => { document.removeEventListener('keydown', keydown); document.body.style.overflow = previous }
  }, [close])
  return <div className="pmp-image-lightbox" role="dialog" aria-modal="true" aria-label="Просмотр изображения" onMouseDown={(event) => event.target === event.currentTarget && close()}><button type="button" onClick={close} aria-label="Закрыть">×</button><img src={image.src} alt={image.alt} /></div>
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
  usePageStyles(['/static/profimarket.css?v=3','/static/profimarket-product.css?v=2','/static/vacancy-publish-success.css?v=1'])
  const { key } = useParams(), location = useLocation(), root = useRef(null)
  const [solution, setSolution] = useState(null), [html, setHTML] = useState(''), [error, setError] = useState(''), [modal, setModal] = useState(false), [notice, setNotice] = useState(null), [purchaseSuccess, setPurchaseSuccess] = useState(null), [expandedImage, setExpandedImage] = useState(null)
  useDocumentPage({ title: solution ? `${solution.title} — ПрофиМаркет` : 'Решение — ПрофиМаркет' })
  const preview = new URLSearchParams(location.search).get('preview') === '1'
  function notify(text, bad = false) { setNotice({ text, bad }); window.setTimeout(() => setNotice(null), 3000) }
  useEffect(() => {
    const controller = new AbortController(); setError(''); setSolution(null)
    getProfiMarketSolution(key, { signal: controller.signal }).then(async(data) => { setSolution(data);if(['AUTOMATION','INSTRUCTION','ONEC_INTEGRATION','TEMPLATE','CHECKLIST'].includes(data.type)){setHTML('');return}const ui=await loadPresentation();setHTML(ui.solutionView(data,preview)) }).catch((requestError) => { if (requestError.name !== 'AbortError') setError(requestError.message) })
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
    try { const data = await purchaseProfiMarketSolution(solution.id); setPurchaseSuccess(data) } catch (requestError) { notify(requestError.message, true) }
  }
  function interact(event) {
    const demoImage = event.target.closest('[data-demo-image]'), legacyImage = event.target.closest('.pm-ai-visual>img,.pm-video-stage>img,.pmr-product-art.has-cover>img,.pmr-section-image img'), favoriteButton = event.target.closest('[data-favorite]'), buyButton = event.target.closest('[data-buy]'), tabButton = event.target.closest('[data-section-tab]')
    if (demoImage) {
      const gallery = demoImage.closest('.pm-ai-demo'), stage = gallery?.querySelector('.pm-video-stage'), image = document.createElement('img')
      if (!stage) return
      image.src = demoImage.dataset.demoImage; image.alt = demoImage.querySelector('img')?.alt || 'Демонстрация'
      stage.replaceChildren(image)
      gallery.querySelectorAll('[data-demo-image]').forEach((button) => { const active = button === demoImage; button.classList.toggle('active', active); button.setAttribute('aria-pressed', String(active)) })
    }
    else if (legacyImage) setExpandedImage({ src: legacyImage.currentSrc || legacyImage.src, alt: legacyImage.alt || solution.title })
    else if (favoriteButton) favorite(favoriteButton)
    else if (buyButton) buy()
    else if (tabButton) { root.current.querySelectorAll('[data-section-tab]').forEach((item) => item.classList.toggle('active', item === tabButton)); root.current.querySelectorAll('[data-section]').forEach((item) => item.classList.toggle('hidden', item.dataset.section !== tabButton.dataset.sectionTab)) }
  }
  function reviewsChanged(rating, reviewCount) {
    const next = {...solution, rating, review_count: reviewCount}
    setSolution(next)
    if (!['AUTOMATION','INSTRUCTION','ONEC_INTEGRATION','TEMPLATE','CHECKLIST'].includes(next.type) && window.ProfiMarketUI) setHTML(window.ProfiMarketUI.solutionView(next, preview))
  }
  const modern=solution&&['AUTOMATION','INSTRUCTION','ONEC_INTEGRATION','TEMPLATE','CHECKLIST'].includes(solution.type)
  return <PublicLayout><main ref={root} id="pm-detail" className="pm-detail-page" onClick={interact}>{error ? <div className="pm-detail-loading"><h1>Решение не найдено</h1><p>{error}</p><a href="/profimarket">Вернуться в ПрофиМаркет</a></div> : !solution ? <div className="pm-detail-loading"><i /><b>Загружаем решение…</b></div> : modern?<ProfiMarketProductDetail solution={solution}/>:<div dangerouslySetInnerHTML={{ __html: html }} />}{solution && <ProfiMarketReviews solution={solution} onChanged={reviewsChanged} />}</main>{modal && <PurchaseModal solution={solution} close={() => setModal(false)} done={(text) => { setModal(false); setPurchaseSuccess({ message: text }) }} fail={(text) => notify(text, true)} />}{purchaseSuccess && <PublishSuccessModal eyebrow={solution?.trial_days ? 'БЕСПЛАТНЫЙ ПЕРИОД' : 'ЗАЯВКА ОФОРМЛЕНА'} title="Поздравляем, всё получилось!" description={purchaseSuccess.message || 'Автор получил ваши контакты и свяжется с вами.'} wishTitle="Автор уже получил уведомление" wishText="Ваши контакты сохранены в его кабинете, также ему отправлено письмо." primaryHref="/profile?section=profimarket-purchases" primaryText="Перейти в мои покупки" onClose={() => setPurchaseSuccess(null)} />}{expandedImage && <DetailImageLightbox image={expandedImage} close={() => setExpandedImage(null)} />}<Notice value={notice} /></PublicLayout>
}
