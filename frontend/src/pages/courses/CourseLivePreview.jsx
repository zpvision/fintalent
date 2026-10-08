import { useEffect, useRef, useState } from 'react'
import CourseHero from './CourseHero'

export default function CourseLivePreview({ course }) {
  const [device, setDevice] = useState(() => window.matchMedia('(max-width:600px)').matches ? 'mobile' : 'desktop'), [width, setWidth] = useState(0)
  const viewport = useRef(null)
  useEffect(() => {
    const observer = new ResizeObserver(([entry]) => setWidth(entry.contentRect.width))
    observer.observe(viewport.current)
    return () => observer.disconnect()
  }, [])
  const canvasWidth = device === 'desktop' ? 854 : 360
  const scale = width ? Math.min(width / canvasWidth, 1) : 1
  return <aside className={`course-live-preview ${device}`} aria-label="Живой предпросмотр курса">
    <header><div><h3>Так увидят ваш курс</h3><p>Изменения видны сразу — сохранять для просмотра не нужно.</p></div><div className="course-device-toggle" role="group" aria-label="Устройство предпросмотра">{[['desktop', 'Компьютер'], ['mobile', 'Телефон']].map(([key, label]) => <button type="button" key={key} aria-pressed={device === key} onClick={() => setDevice(key)}>{label}</button>)}</div></header>
    <div className="course-live-viewport" ref={viewport}><div className="course-live-canvas" style={{ width: canvasWidth, zoom: scale }}><CourseHero course={course} /></div></div>
    <small>Проверьте, что текст читается и не закрывает важные детали обложки. Полную страницу можно открыть кнопкой «Предпросмотр».</small>
  </aside>
}
