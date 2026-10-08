import { useEffect, useRef, useState } from 'react'

function drawCover(canvas, image, zoom, positionX, positionY) {
  if (!canvas || !image) return
  const context = canvas.getContext('2d')
  const targetWidth = canvas.width
  const targetHeight = canvas.height
  const scale = Math.max(targetWidth / image.naturalWidth, targetHeight / image.naturalHeight) * zoom
  const width = image.naturalWidth * scale
  const height = image.naturalHeight * scale
  const x = (targetWidth - width) * (positionX / 100)
  const y = (targetHeight - height) * (positionY / 100)
  context.clearRect(0, 0, targetWidth, targetHeight)
  context.drawImage(image, x, y, width, height)
}

export default function ProfiMarketCoverCropper({ file, onCancel, onReady, eyebrow = 'ОБЛОЖКА РЕШЕНИЯ', width = 1200, height = 675 }) {
  const canvasRef = useRef(null)
  const [image, setImage] = useState(null)
  const [positionX, setPositionX] = useState(50)
  const [positionY, setPositionY] = useState(50)
  const [zoom, setZoom] = useState(1)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    if (!file) return undefined
    const url = URL.createObjectURL(file)
    const next = new Image()
    next.onload = () => setImage(next)
    next.onerror = () => setError('Не удалось открыть изображение')
    next.src = url
    return () => URL.revokeObjectURL(url)
  }, [file])

  useEffect(() => { drawCover(canvasRef.current, image, zoom, positionX, positionY) }, [image, zoom, positionX, positionY, width, height])
  useEffect(() => {
    const close = (event) => { if (event.key === 'Escape' && !busy) onCancel() }
    document.addEventListener('keydown', close)
    const previous = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => { document.removeEventListener('keydown', close); document.body.style.overflow = previous }
  }, [busy, onCancel])

  async function confirm() {
    if (!image || busy) return
    setBusy(true); setError('')
    try {
      drawCover(canvasRef.current, image, zoom, positionX, positionY)
      const blob = await new Promise((resolve, reject) => canvasRef.current.toBlob(value => value ? resolve(value) : reject(new Error('Не удалось подготовить обложку')), 'image/webp', 0.9))
      const baseName = file.name.replace(/\.[^.]+$/, '') || 'cover'
      const accepted = await onReady(new File([blob], `${baseName}-${width}x${height}.webp`, { type: 'image/webp' }))
      if (accepted === false) setBusy(false)
    } catch (cropError) {
      setError(cropError.message)
      setBusy(false)
    }
  }

  return <div className="pmp-crop-overlay" onMouseDown={event => event.target === event.currentTarget && !busy && onCancel()}>
    <section className="pmp-crop-dialog" role="dialog" aria-modal="true" aria-labelledby="pmp-crop-title">
      <header><div><small>{eyebrow}</small><h2 id="pmp-crop-title">Настройте кадрирование</h2><p>Выберите область, которая будет видна в каталоге. Итоговый формат — {width} × {height}.</p></div><button type="button" onClick={onCancel} disabled={busy} aria-label="Закрыть">×</button></header>
      <div className="pmp-crop-preview"><canvas ref={canvasRef} width={width} height={height}/>{!image && !error && <span>Подготавливаем изображение…</span>}</div>
      <div className="pmp-crop-controls">
        <label><span>По горизонтали</span><input type="range" min="0" max="100" value={positionX} onChange={event => setPositionX(Number(event.target.value))}/></label>
        <label><span>По вертикали</span><input type="range" min="0" max="100" value={positionY} onChange={event => setPositionY(Number(event.target.value))}/></label>
        <label><span>Масштаб</span><input type="range" min="1" max="2.5" step="0.01" value={zoom} onChange={event => setZoom(Number(event.target.value))}/></label>
      </div>
      {error && <p className="pmp-crop-error">{error}</p>}
      <footer><button type="button" className="secondary" onClick={onCancel} disabled={busy}>Отмена</button><button type="button" className="primary" onClick={confirm} disabled={!image || busy}>{busy ? 'Подготавливаем…' : 'Использовать обложку'}</button></footer>
    </section>
  </div>
}
