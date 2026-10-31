import { useEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { apiClient } from '../../api/client'
import usePageStyles from '../../hooks/usePageStyles'
import Icon from '../Icon'

const size = value => value >= 1048576 ? `${(value / 1048576).toFixed(1)} МБ` : `${Math.ceil(value / 1024)} КБ`

function Preview({ file, src, close }) {
  const dialog = useRef(null), image = file.mime.startsWith('image/')
  useEffect(() => {
    const node = dialog.current, previous = document.documentElement.style.overflow
    document.documentElement.style.overflow = 'hidden'; node.showModal()
    return () => { node.close(); document.documentElement.style.overflow = previous }
  }, [])
  return createPortal(<dialog ref={dialog} className={`tqm-modal ${image ? 'tqm-full-image' : ''}`} onKeyDown={e => { if (e.key === 'Escape') { e.preventDefault(); close() } }} onCancel={e => { e.preventDefault(); close() }} onClick={e => { if (e.target === e.currentTarget) close() }}><header><div><strong>{file.name}</strong><small>{image ? 'Изображение к вопросу' : 'Документ PDF'} · {size(file.size)}</small></div><a href={src} download={file.name}>Скачать</a><button type="button" aria-label="Закрыть предпросмотр" onClick={close}>×</button></header><div className="tqm-preview-body">{image ? <img src={src} alt={file.name} /> : <iframe src={src} title={`Предпросмотр PDF: ${file.name}`} />}</div>{!image && <p className="tqm-pdf-note">Если браузер не отображает PDF, скачайте документ по кнопке выше.</p>}</dialog>, document.body)
}

function Material({ file, questionID, access, onRemove, removeDisabled }) {
  const [src, setSrc] = useState(''), [error, setError] = useState(''), [busy, setBusy] = useState(false), [open, setOpen] = useState(false), [retry, setRetry] = useState(0)
  const container = useRef(null), [visible, setVisible] = useState(false)
  const image = file.mime?.startsWith('image/'), headers = JSON.stringify(access || {})
  useEffect(() => {
    if (!image) return
    if (typeof IntersectionObserver === 'undefined') { setVisible(true); return }
    const observer = new IntersectionObserver(entries => {
      if (entries.some(entry => entry.isIntersecting)) { setVisible(true); observer.disconnect() }
    }, { rootMargin: '250px' })
    if (container.current) observer.observe(container.current)
    return () => observer.disconnect()
  }, [image, file.id])
  useEffect(() => {
    if (image && !visible) return
    if (!image && !open) return
    let active = true, url = ''; const abort = new AbortController()
    setBusy(true); setError(''); setSrc('')
    apiClient.get(`/api/test-materials/${file.id}?question=${questionID || 0}`, { responseType: 'blob', redirectOnUnauthorized: false, headers: JSON.parse(headers), signal: abort.signal }).then(blob => {
      if (!active) return
      if (blob.type !== file.mime) throw new Error('Не удалось открыть материал')
      url = URL.createObjectURL(blob); setSrc(url)
    }).catch(e => { if (active) setError(e.message) }).finally(() => { if (active) setBusy(false) })
    return () => { active = false; abort.abort(); if (url) URL.revokeObjectURL(url) }
  }, [file.id, file.mime, questionID, headers, image, visible, image ? false : open, retry])
  return <div ref={container} className={`tqm-material ${image ? 'tqm-picture' : 'tqm-document'}`}>
    {image ? <button type="button" className="tqm-image-button" onClick={() => setOpen(true)} disabled={!src}><span className="tqm-image-frame">{src ? <img src={src} alt={file.name} /> : <span>{busy ? 'Загружаем изображение…' : error ? 'Изображение недоступно' : 'Изображение к вопросу'}</span>}</span><span className="tqm-image-caption">{file.name}<b>На весь экран ↗</b></span></button> : <button type="button" className="tqm-file-button" onClick={() => setOpen(true)}><span className="tqm-pdf-icon">PDF</span><span><strong>{file.name}</strong><small>{size(file.size)} · Просмотреть ↗</small></span></button>}
    {onRemove && <button type="button" className="tqm-remove" disabled={removeDisabled} onClick={onRemove} aria-label={`Убрать материал ${file.name}`} title="Убрать из вопроса"><Icon name="trash" /></button>}
    {error && <p className="tqm-error" role="alert">{error}<button type="button" onClick={() => { setRetry(x => x + 1); if (!image) setOpen(true) }}>Повторить</button></p>}
    {open && src && <Preview file={file} src={src} close={() => { setOpen(false); if (!image) setSrc('') }} />}
    {open && busy && !image && <p role="status">Открываем PDF…</p>}
  </div>
}

export default function QuestionMaterials({ question, access, onRemove, removeDisabled }) {
  usePageStyles(['/static/test-question-materials.css?v=2'])
  const items = Array.isArray(question?.settings?.materials) ? question.settings.materials : [], images = items.filter(f => f.mime?.startsWith('image/')), files = items.filter(f => f.mime === 'application/pdf')
  if (!items.length) return null
  return <section className={`tqm-layout ${images.length && files.length ? 'tqm-mixed' : ''}`} aria-label="Материалы к вопросу">{images.length > 0 && <div className="tqm-images">{images.map(file => <Material key={file.id} file={file} questionID={question.id} access={access} onRemove={onRemove ? () => onRemove(file.id) : undefined} removeDisabled={removeDisabled} />)}</div>}{files.length > 0 && <div className="tqm-files"><h4>Документы к вопросу</h4>{files.map(file => <Material key={file.id} file={file} questionID={question.id} access={access} onRemove={onRemove ? () => onRemove(file.id) : undefined} removeDisabled={removeDisabled} />)}</div>}</section>
}

export function QuestionMaterialsEditor({ testID, question, onChange, disabled, onUploading }) {
  const [error, setError] = useState(''), [busy, setBusy] = useState(false), items = Array.isArray(question.settings?.materials) ? question.settings.materials : []
  async function upload(event) {
    const files = [...event.target.files]; event.target.value = ''
    if (!files.length) return
    if (!testID) { setError('Сначала сохраните информацию о тесте.'); return }
    if (items.length + files.length > 6) { setError('К вопросу можно добавить до 6 материалов.'); return }
    if (files.some(file => file.size > 6 * 1048576)) { setError('Каждый файл должен быть не больше 6 МБ.'); return }
    setBusy(true); onUploading?.(true); setError('')
    const next = [...items]
    try {
      for (const file of files) {
        const data = new FormData(); data.append('test_id', String(testID)); data.append('file', file)
        const asset = await apiClient.post('/api/test-materials', data)
        next.push(asset); onChange([...next])
      }
    } catch (e) { setError(e.message) } finally { setBusy(false); onUploading?.(false) }
  }
  return <div className="tqm-editor"><div className="tqm-editor-heading"><div><strong>Материалы к вопросу</strong><small>Изображения JPG, PNG и документы PDF · до 6 файлов по 6 МБ</small></div><label className={`tqm-upload ${busy || disabled ? 'disabled' : ''}`}>{busy ? 'Загружаем…' : '＋ Добавить материалы'}<input type="file" accept="image/jpeg,image/png,application/pdf" multiple disabled={busy || disabled} onChange={upload} /></label></div>{error && <p className="tqm-error" role="alert">{error}</p>}<QuestionMaterials question={question} onRemove={id => onChange(items.filter(file => file.id !== id))} removeDisabled={busy || disabled} /></div>
}
