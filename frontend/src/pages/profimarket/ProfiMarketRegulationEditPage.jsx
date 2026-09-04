import { useEffect, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { useDocumentPage } from '../../hooks/useDocumentPage'
import usePageStyles from '../../hooks/usePageStyles'
import PublicLayout from '../../layouts/PublicLayout'

const editorStyles = [
  '/static/profimarket.css?v=1',
  '/static/vacancy-publish-success.css?v=1',
  '/static/profimarket-regulation-editor.css?v=2',
  '/static/profimarket-regulation-editor-v2.css?v=2',
  '/static/profimarket-regulation-editor-v3.css?v=2',
  '/static/profimarket-regulation-editor-v4.css?v=2',
  '/static/profimarket-regulation-editor-v5.css?v=20',
  '/static/profimarket-regulation-editor-v6.css?v=8',
]

function loadScript(src, ready) {
  if (ready?.()) return Promise.resolve(null)
  return new Promise((resolve, reject) => {
    const script = document.createElement('script')
    script.src = src
    script.dataset.reactRegulationEditor = 'true'
    script.onload = () => resolve(script)
    script.onerror = () => reject(new Error('Не удалось загрузить редактор регламента'))
    document.head.append(script)
  })
}

export default function ProfiMarketRegulationEditPage() {
  useDocumentPage({ title: 'Редактор регламента — ПрофиМаркет' })
  usePageStyles(editorStyles)
  const [params] = useSearchParams(), id = params.get('id'), [error, setError] = useState('')
  useEffect(() => {
    if (!id) { setError('Не указан идентификатор регламента'); return undefined }
    let cancelled = false
    const loaded = []
    async function start() {
      try {
        const presets = await loadScript('/static/profimarket-style-presets.js?v=3', () => window.ProfiMarketStylePresets)
        if (presets) loaded.push(presets)
        const components = await loadScript('/static/profimarket-components.js?v=27', () => window.ProfiMarketUI)
        if (components) loaded.push(components)
        if (cancelled) return
        const editor = await loadScript('/static/profimarket-regulation-editor.js?v=40')
        if (editor) loaded.push(editor)
      } catch (loadError) { if (!cancelled) setError(loadError.message) }
    }
    start()
    return () => {
      cancelled = true
      loaded.forEach((script) => script.remove())
      document.querySelectorAll('.pme-startup-screen,.pme-notice,.publish-success-modal').forEach((element) => element.remove())
    }
  }, [id])
  return <PublicLayout><main id="pm-create" className="pme-booting">{error && <div className="pm-loading">{error}</div>}</main>{!error && <div className="pme-startup-screen"><div><i /><b>Готовим редактор регламента</b><small>Загружаем оформление и данные…</small></div></div>}</PublicLayout>
}
