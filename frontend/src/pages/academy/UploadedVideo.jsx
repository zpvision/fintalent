import { useEffect, useRef, useState } from 'react'
import { apiClient } from '../../api/client'

export default function UploadedVideo({ url, poster }) {
  const ref = useRef(null), position = useRef(0), wasPlaying = useRef(false), attempts = useRef(0), retryWindow = useRef(0)
  const [source, setSource] = useState(''), [error, setError] = useState(''), [revision, setRevision] = useState(0)
  useEffect(() => { position.current = 0; attempts.current = 0 }, [url])
  useEffect(() => {
    let alive = true
    apiClient.get(`${url}${url.includes('?') ? '&' : '?'}resolve=1`).then(result => { if (alive) { setSource(result.url); setError('') } }).catch(() => { if (alive) { setSource(''); setError('Видео недоступно. Проверьте доступ и повторите.') } })
    return () => { alive = false }
  }, [url, revision])
  const retry = () => { const v = ref.current; if (v) { position.current = v.currentTime; wasPlaying.current = !v.paused }; setSource(''); setRevision(n => n + 1) }
  return <><video ref={ref} controls playsInline preload="metadata" poster={poster} src={source || undefined} onLoadedMetadata={() => { if (position.current > 0) ref.current.currentTime = position.current; if (wasPlaying.current) ref.current.play().catch(() => {}) }} onError={() => { if (Date.now() - retryWindow.current > 30000) { attempts.current = 0; retryWindow.current = Date.now() }; if (attempts.current++ < 2) retry(); else setError('Не удалось воспроизвести видео. Повторите загрузку ссылки.') }} />{error && <p className="course-error" role="alert">{error} <button className="course-secondary" onClick={() => { attempts.current = 0; retry() }}>Повторить</button></p>}</>
}
