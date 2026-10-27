import { useEffect, useRef, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { courseAPI } from '../../api/courses'
import WizardStepProgress from '../../components/WizardStepProgress'
import ProfiMarketCoverCropper from '../../components/ProfiMarketCoverCropper'
import PublicLayout from '../../layouts/PublicLayout'
import CourseLivePreview from './CourseLivePreview'
import Field from './CourseField'
import Reorder from './CourseReorder'
import CoursePricingStep from './CoursePricingStep'
import CourseProgramStep from './CourseProgramStep'
import CourseOutcomesStep from './CourseOutcomesStep'
import IconSelect from '../../components/IconSelect'
import Icon from '../../components/Icon'
import usePageStyles from '../../hooks/usePageStyles'
import { useDocumentPage } from '../../hooks/useDocumentPage'
import { levels, types, benefitIcons, benefitColors, filledBenefit, withBenefitSlots, Benefits, styles } from './shared'

const steps = ['Общая информация', 'Что вы получите на курсе', 'Программа курса', 'Результаты обучения', 'Стоимость и доступ', 'Частые вопросы']
const moveItem = (items, from, to) => { const next = [...items]; next.splice(to, 0, next.splice(from, 1)[0]); return next }
const lesson = () => ({ id: 0, title: '', description: '', duration_minutes: 0, type: 'video' })

export default function CourseEditor() {
  const { id } = useParams(), navigate = useNavigate(), creation = useRef(null)
  const [value, setValue] = useState(null), [meta, setMeta] = useState({ categories: [] }), [step, setStep] = useState(0), [error, setError] = useState(''), [busy, setBusy] = useState(false), [dirty, setDirty] = useState(false), [saved, setSaved] = useState(''), [crop, setCrop] = useState(null), [coverSource, setCoverSource] = useState(null)
  usePageStyles([...styles, '/static/wizard-step-progress.css', '/static/profimarket-product.css?v=3', '/static/course-editor-theme.css?v=1'])
  useDocumentPage({ title: 'Создание курса — FinTalent' })
  useEffect(() => {
    let active = true
    if (!id) { creation.current ||= courseAPI.create(); creation.current.then(c => { if (active) navigate(`/profile/courses/${c.id}/edit`, { replace: true }) }).catch(e => { if (active) setError(e.message) }); return () => { active = false } }
    Promise.all([courseAPI.get(id), courseAPI.meta()]).then(([c, m]) => {
      if (!active) return
      let recovered = null
      try {
        const key = `fintalent.course-draft.${c.owner_id}.${id}`
        const backup = JSON.parse(sessionStorage.getItem(key) || 'null')
        if (backup && confirm('В этой вкладке остались несохранённые изменения курса. Восстановить их?')) recovered = backup
        else sessionStorage.removeItem(key)
      } catch { /* Storage may be unavailable; explicit draft saving still works. */ }
      setValue(withBenefitSlots(recovered || c)); setMeta(m); setDirty(Boolean(recovered))
      if (recovered && recovered.revision !== c.revision) setError('На сервере уже есть более новая версия. Ваши изменения восстановлены для копирования; сохранение потребует обновить страницу и объединить изменения.')
    }).catch(e => { if (active) setError(e.message) })
    return () => { active = false }
  }, [id])
  useEffect(() => {
    if (!value || !dirty) return
    try { sessionStorage.setItem(`fintalent.course-draft.${value.owner_id}.${id}`, JSON.stringify(value)) } catch { /* The server remains the source of saved drafts. */ }
  }, [value, dirty, id])
  useEffect(() => {
    if (!dirty) return
    const unload = e => { e.preventDefault(); e.returnValue = '' }
    const leave = e => { const a = e.target.closest?.('a[href]'); if (a && !a.hash && !confirm('Есть несохранённые изменения. Покинуть страницу?')) { e.preventDefault(); e.stopPropagation() } }
    window.addEventListener('beforeunload', unload); document.addEventListener('click', leave, true)
    return () => { window.removeEventListener('beforeunload', unload); document.removeEventListener('click', leave, true) }
  }, [dirty])
  function update(key, next) { setValue(v => ({ ...v, [key]: next })); setDirty(true); setSaved('') }
  function item(key, index, patch) { update(key, value[key].map((v, i) => i === index ? { ...v, ...patch } : v)) }
  function remove(key, index) { if (confirm('Удалить этот элемент? Изменение применится после сохранения черновика.')) update(key, value[key].filter((_, i) => i !== index)) }
  async function save() {
    if (busy || !value) return null
    if (!dirty) return value
    setBusy(true); setError('')
    try { const next = await courseAPI.save(id, value); try { sessionStorage.removeItem(`fintalent.course-draft.${value.owner_id}.${id}`) } catch {} setValue(withBenefitSlots(next)); setDirty(false); setSaved(`Сохранено в ${new Date().toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' })}`); return next } catch (e) { setError(e.message); return null } finally { setBusy(false) }
  }
  async function go(next) { if (await save()) { setStep(next); window.scrollTo({ top: 0, behavior: 'smooth' }) } }
  async function preview() {
    const tab = window.open('about:blank', '_blank')
    if (!tab) { setError('Браузер заблокировал новое окно. Разрешите его для предпросмотра курса.'); return }
    tab.opener = null
    if (await save()) tab.location.replace(`/profile/courses/${id}/preview`)
    else tab.close()
  }
  async function submit() { if (!await save()) return; setBusy(true); setError(''); try { await courseAPI.action(id, 'submit'); navigate('/profile/courses') } catch (e) { setError(e.message) } finally { setBusy(false) } }
  async function recrop() {
    if (coverSource) { setCrop(coverSource); return }
    setBusy(true); setError('')
    try {
      const response = await fetch(value.cover_image, { credentials: 'same-origin' })
      if (!response.ok) throw new Error('Не удалось загрузить обложку для кадрирования')
      const blob = await response.blob()
      const file = new File([blob], 'course-cover', { type: blob.type })
      setCoverSource(file); setCrop(file)
    } catch (e) { setError(e.message) } finally { setBusy(false) }
  }
  async function upload(file) { try { const data = await courseAPI.upload(file); update('cover_image', data.url); setCoverSource(crop); setCrop(null); return true } catch (e) { setError(e.message); return false } }
  const moduleUpdate = (i, patch) => item('modules', i, patch)
  const lessonUpdate = (i, j, patch) => moduleUpdate(i, { lessons: value.modules[i].lessons.map((l, k) => k === j ? { ...l, ...patch } : l) })
  return <PublicLayout><main className="course-page course-editor"><div className="course-editor-top"><Link to="/profile/courses">← Мои курсы</Link><span>Ваш опыт заслуживает собственного курса</span></div>{error && <p role="alert" className="course-error">{error}</p>}{!value ? <p role="status">{error ? 'Не удалось открыть мастер.' : 'Подготавливаем черновик…'}</p> : <><div className="course-editor-heading"><div><small>МАСТЕР СОЗДАНИЯ КУРСА</small><h1>{value.title || 'Ваш новый курс'}</h1></div><button className="course-secondary" disabled={busy} onClick={preview}>Предпросмотр ↗</button></div><WizardStepProgress steps={steps} current={step} onStepChange={next => !busy && go(next)} />{['published', 'moderation'].includes(value.status) && <p className="course-notice">Сохранение изменений вернёт курс в черновики. После редактирования отправьте его на публикацию повторно.</p>}<fieldset disabled={busy} className="course-editor-fields"><header><span className="course-step-number" aria-hidden="true">{step + 1}</span><small>ШАГ {step + 1} ИЗ {steps.length}</small><h2>{steps[step]}</h2><p>{['Начните с главного: о чём ваш курс и кому он поможет.', 'Покажите конкретную пользу. Заполните нужные карточки. Пустые блоки не появятся на странице курса.', 'Разделите путь обучения на понятные модули и уроки. Содержимое уроков можно будет добавить на следующем этапе.', 'Опишите, какие знания и навыки участник получит к концу курса.', 'Настройте цену и условия доступа. Посмотрите, как предложение будет выглядеть для участника.', 'Ответьте на вопросы, которые могут возникнуть перед покупкой.'][step]}</p></header>
      {step === 0 && <><div className="course-form-grid course-info-layout"><div className="course-editor-panel course-info-card"><div className="course-card-heading"><i><Icon name="book" /></i><div><h3>О вашем курсе</h3><p>Название, описание и обложка — первое знакомство с вашим курсом.</p></div></div><Field label="Название курса" area rows={2} hint="Нажмите Enter, чтобы начать новую строку на обложке. В карточках название будет обычной строкой." value={value.title} onChange={v => update('title', v)} placeholder="ИИ для бухгалтера: от первых шагов до реальных кейсов" /><Field label="Короткое описание" value={value.description} onChange={v => update('description', v)} max={1200} area hint="Какой результат получит участник и для кого этот курс?" /><div className="course-form-grid"><label className="course-field">Категория курса<select aria-label="Категория курса" value={value.category_id || ''} onChange={e => update('category_id', e.target.value ? Number(e.target.value) : null)}><option value="">Выберите категорию</option>{meta.categories.map(c => <option key={c.id} value={c.id}>{c.name}</option>)}</select></label><label className="course-field">Уровень<select aria-label="Уровень" value={value.level} onChange={e => update('level', e.target.value)}>{Object.entries(levels).map(([k, v]) => <option key={k} value={k}>{v}</option>)}</select></label></div><label className="course-cover-upload"><span>Обложка курса</span><div>{value.cover_image ? <img src={value.cover_image} alt="Предпросмотр обложки" /> : <span>＋<b>Загрузить изображение</b><small>JPG, PNG или WebP · до 7 МБ</small></span>}</div><input type="file" accept="image/jpeg,image/png,image/webp" onChange={e => { const file = e.target.files[0]; if (file?.size > 7 * 1024 * 1024) setError('Изображение должно быть не больше 7 МБ'); else if (file) setCrop(file); e.target.value = '' }} /><small>Рекомендуем 1200 × 500 px (12:5). После кадрирования обложка будет именно такого размера. Важные детали располагайте справа: слева на странице находится текст курса.</small></label>{value.cover_image && <button type="button" className="course-secondary course-recrop" onClick={recrop}>Настроить кадрирование</button>}<div className="course-notice">Автор — вы. Имя, фотография и профессиональная информация берутся из вашего профиля FinTalent.</div></div><CourseLivePreview course={value} /></div><section className="course-editor-panel course-promo-card"><Field label="Промо-видео" value={value.promo_url} onChange={v => update('promo_url', v)} max={2000} type="url" placeholder="https://…" hint="Ссылка на трейлер. Видео откроется в новой вкладке." /></section></>}
      {step === 1 && <>
        <section className="course-benefit-preview-panel" aria-label="Предпросмотр преимуществ"><div className="course-row-head course-benefit-heading"><h3><Icon name="users" />Так это увидят участники</h3><span>Заполнено {value.benefits.filter(filledBenefit).length}/4</span></div><Benefits items={value.benefits} showEmpty /></section>
        {value.benefits.length > 4 && <p className="course-error" role="alert">В курсе может быть максимум 4 преимущества. Удалите лишние карточки перед сохранением.</p>}
        {!value.benefits.length && <div className="course-inline-empty">Добавьте первое преимущество — его карточка появится сверху, а поля для редактирования прямо под ней.</div>}
        <div className="course-benefit-columns">{value.benefits.map((b, i) => <section className="course-benefit-column" key={b.id || 'new-' + i} aria-label={'Преимущество ' + (i + 1)}>
          <article className="course-repeat-card course-benefit-editor">
            <div className="course-benefit-appearance"><div className="course-field"><span>Иконка</span><IconSelect compact label={'Иконка преимущества ' + (i + 1)} value={b.icon} options={benefitIcons} onChange={icon => item('benefits', i, { icon })} /></div><div className="course-field"><span>Цвет</span><IconSelect label={'Цвет преимущества ' + (i + 1)} buttonText="Цвет" value={b.color || ''} options={{ '': 'Автоматически', ...Object.fromEntries(Object.entries(benefitColors).map(([key, p]) => [key, p.name])) }} onChange={color => item('benefits', i, { color })} renderIcon={key => <span className="course-color-swatch" style={{ background: benefitColors[key]?.iconBg || '#ffffff' }} />} /></div></div>
            <Field label="Заголовок" max={120} value={b.title} onChange={title => item('benefits', i, { title })} placeholder="Практические навыки" />
            <Field area rows={5} label="Короткое описание" max={500} value={b.description} onChange={description => item('benefits', i, { description })} />
          </article>
        </section>)}</div>

      </>}
      {step === 2 && <CourseProgramStep value={value} meta={meta} update={update} moduleUpdate={moduleUpdate} lessonUpdate={lessonUpdate} remove={remove} moveItem={moveItem} lesson={lesson} />}
      {step === 3 && <CourseOutcomesStep items={value.learning_outcomes || []} onChange={items => update('learning_outcomes', items)} />}
      {step === 4 && <CoursePricingStep value={value} update={update} meta={meta} item={item} remove={remove} moveItem={moveItem} />}
      {step === 5 && <section className="course-faq-editor">{!value.faq.length && <div className="course-inline-empty">Подойдёт ли курс новичкам? Как долго открыт доступ? Помогите будущим участникам принять решение.</div>}{value.faq.map((q, i) => <article className="course-repeat-card" key={q.id || `new-${i}`}><div className="course-row-head"><h3><span className="course-card-number">{i + 1}</span>Вопрос {i + 1}</h3><Reorder deleteIcon index={i} count={value.faq.length} label={`вопрос ${i + 1}`} move={(a, b) => update('faq', moveItem(value.faq, a, b))} remove={() => remove('faq', i)} /></div><Field label="Вопрос" max={300} value={q.question} onChange={question => item('faq', i, { question })} /><Field label="Ответ" max={3000} area value={q.answer} onChange={answer => item('faq', i, { answer })} /></article>)}<button type="button" className="course-add course-editor-add-primary" disabled={value.faq.length >= 50} onClick={() => update('faq', [...value.faq, { id: 0, question: '', answer: '' }])}>＋ Добавить вопрос</button><div className="course-notice"><b>Почти готово!</b><p>Посмотрите страницу глазами участника. После отправки курс проверит администратор; до одобрения он доступен только вам в предпросмотре.</p></div></section>}
    </fieldset><footer className="course-editor-footer"><div role="status">{busy ? 'Сохраняем…' : dirty ? 'Есть несохранённые изменения' : saved || 'Черновик сохранён'}</div><div><button className="course-secondary" disabled={busy} onClick={save}>Сохранить черновик</button>{step > 0 && <button className="course-secondary" disabled={busy} onClick={() => go(step - 1)}>Назад</button>}{step < steps.length - 1 ? <button className="course-primary" disabled={busy} onClick={() => go(step + 1)}>Далее →</button> : <button className="course-primary" disabled={busy || value.status === 'moderation'} onClick={submit}>Отправить на публикацию</button>}</div></footer></>}{crop && <ProfiMarketCoverCropper eyebrow="ОБЛОЖКА КУРСА" width={1200} height={500} file={crop} onCancel={() => setCrop(null)} onReady={upload} />}</main></PublicLayout>
}
