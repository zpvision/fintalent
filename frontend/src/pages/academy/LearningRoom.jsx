import { useEffect, useRef, useState } from 'react'
import { Link, useNavigate, useParams, useSearchParams } from 'react-router-dom'
import PublicLayout from '../../layouts/PublicLayout'
import { learningAPI } from '../../api/learning'
import { academyAPI } from '../../api/academy'
import Icon from '../../components/Icon'
import usePageStyles from '../../hooks/usePageStyles'
import { useDocumentPage } from '../../hooks/useDocumentPage'
import { styles } from '../courses/shared'
import LessonRenderer, { blockTypes } from './LessonRenderer'
import AcademyDialog from './AcademyDialog'
import { AccessNotice, ProgressBar, learningDate } from './LearningShared'

function Room({ preview = false }) {
  const { enrollmentId, id, versionId } = useParams(), [query, setQuery] = useSearchParams(), [room, setRoom] = useState(null), [active, setActive] = useState(''), [busy, setBusy] = useState(false), [error, setError] = useState(''), [treeOpen, setTreeOpen] = useState(false), [ack, setAck] = useState(false), [notice, setNotice] = useState(''), [celebrate, setCelebrate] = useState(false)
  const navigate = useNavigate(), flushers = useRef(new Map())
  const flight = useRef(false), heading = useRef(null), alive = useRef(true), current = useRef(null)
  usePageStyles([...styles, '/static/academy.css?v=3', '/static/academy-assessments.css?v=1', '/static/academy-learning.css?v=1'])
  useDocumentPage({ title: `${room?.enrollment?.title || room?.course?.title || 'Учебная комната'} — FinTalent` })
  const fetchRoom = () => preview ? learningAPI.preview(id, versionId) : learningAPI.room(enrollmentId)
  async function refreshDeniedAccess() {
    const closed = { enrollment: { ...current.current?.enrollment, effective_status: 'suspended' }, program: null }
    current.current = closed; setRoom(closed)
    try { const data = await fetchRoom(); if (alive.current) { current.current = data; setRoom(data) } } catch { /* Keep materials closed if the status refresh also fails. */ }
  }
  useEffect(() => { alive.current = true; fetchRoom().then(data => { if (!alive.current) return; current.current = data; setRoom(data); if (data.program) { const requested = query.get('lesson'), available = data.program.modules.flatMap(m => m.lessons).some(l => l.key === requested && l.self_paced); choose(available ? requested : data.resume_lesson_key, data) } }).catch(e => { if (alive.current) setError(e.message) }); return () => { alive.current = false } }, [])
  const enrollment = room?.enrollment, course = enrollment || room?.course, lessons = room?.program?.modules.flatMap(m => m.lessons) || [], available = lessons.filter(l => l.self_paced), lesson = lessons.find(l => l.key === active), index = available.findIndex(l => l.key === active), progress = new Map((enrollment?.progress || []).map(p => [p.lesson_key, p.status])), blocked = room?.completion_blocks?.[active], summary = enrollment?.summary || { required: 0, completed: 0, percent: 0 }
  const register = (key, flush) => { flushers.current.set(key, flush); return () => flushers.current.delete(key) }
  const flushAll = async () => { for (const flush of flushers.current.values()) await flush() }
  async function updated() { const value = await fetchRoom(); if (!alive.current) return; if (value.enrollment?.summary.status === 'completed' && !current.current?.enrollment?.completed_at) setCelebrate(true); current.current = value; setRoom(value) }
  async function choose(key, initial = current.current) {
    if (!key || flight.current) return
    flight.current = true; setBusy(true); setError('')
    try { await flushAll(); if (!preview) { const e = await learningAPI.open(enrollmentId, key); if (!alive.current) return; const next = { ...current.current, enrollment: e }; current.current = next; setRoom(next) } else if (initial && !current.current) current.current = initial
      if (alive.current) { setActive(key); setQuery({ lesson: key }, { replace: true }); setAck(false); setNotice(''); setTreeOpen(false); heading.current?.scrollIntoView({ block: 'start', behavior: 'smooth' }) }
    } catch (e) { if (alive.current) { setError(e.message); if (e.status === 403) await refreshDeniedAccess() } } finally { flight.current = false; if (alive.current) setBusy(false) }
  }
  async function complete() {
    if (preview || !lesson || flight.current) return
    flight.current = true; setBusy(true); setError('')
    try { const e = await learningAPI.complete(enrollmentId, lesson.key, ack); if (!alive.current) return; const next = { ...current.current, enrollment: e }; current.current = next; setRoom(next); setNotice('Урок завершён. Ваш прогресс сохранён.'); if (e.summary.status === 'completed' && !enrollment.completed_at) setCelebrate(true) }
    catch (e) { if (alive.current) { setError(e.message); if (e.status === 403) await refreshDeniedAccess() } } finally { flight.current = false; if (alive.current) setBusy(false) }
  }
  const tree = <nav className="learning-room-tree" aria-label="Программа обучения"><header><small>ВАШ ПУТЬ ОБУЧЕНИЯ</small><h2>Программа курса</h2></header>{room?.program?.modules.map((m, i) => <details key={m.key} open><summary><span>{String(i + 1).padStart(2, '0')}</span><b>{m.title}</b><span>⌄</span></summary><ol>{m.lessons.map((l, j) => { const state = !l.self_paced ? 'unavailable' : progress.get(l.key) || 'not_started'; return <li key={l.key}><button type="button" className={`${active === l.key ? 'current' : ''} ${state}`} aria-current={active === l.key ? 'step' : undefined} disabled={busy || !l.self_paced} onClick={() => choose(l.key)}><i><Icon name={state === 'completed' ? 'check' : blockTypes[l.blocks[0]?.type]?.[1] || 'book'} /></i><span><b>{i + 1}.{j + 1} · {l.title}</b><small>{({ not_started: 'Не начат', in_progress: 'В процессе', completed: 'Завершён', unavailable: 'Недоступен для самостоятельного обучения' })[state]} · {l.duration_minutes} мин</small></span></button></li> })}</ol></details>)}</nav>
  const back = preview ? `/profile/courses/${id}/academy/students` : '/profile/learning'
  if (!room) return <PublicLayout><main className="course-page learning-room-page"><Link to={back} onClick={async e => { e.preventDefault(); try { await flushAll(); navigate(back) } catch (e) { setError(e.message) } }}>← {preview ? 'В кабинет курса' : 'Моё обучение'}</Link><p role={error ? 'alert' : 'status'}>{error || 'Открываем учебную комнату…'}</p>{error && <button className="course-secondary" onClick={() => window.location.reload()}>Повторить</button>}</main></PublicLayout>
  if (!preview && !room.program) return <PublicLayout><main className="course-page learning-room-page"><AccessNotice enrollment={enrollment}><Link className="course-primary" to={back}>Вернуться в моё обучение</Link></AccessNotice></main></PublicLayout>
  return <PublicLayout><main className="course-page learning-room-page"><header className="learning-room-heading"><div><Link to={back}>← {preview ? 'В кабинет курса' : 'Моё обучение'}</Link><small>{preview ? 'ПРЕДПРОСМОТР АВТОРА · без зачисления и записи прогресса' : `FINTALENT ACADEMY · ВЕРСИЯ ${enrollment.version_number}`}</small><h1>{course.title}</h1></div><div>{!preview && <ProgressBar summary={summary} />}<button type="button" className="course-secondary learning-program-toggle" onClick={() => setTreeOpen(true)}><Icon name="list" />Программа курса</button></div></header>
    {error && <div className="course-error" role="alert">{error}<button className="course-secondary" onClick={() => choose(active || room.resume_lesson_key)}>Повторить</button></div>}{notice && <p className="academy-notice" role="status"><Icon name="check" />{notice}</p>}
    <div className="learning-room-grid"><aside>{tree}</aside><div className="learning-room-material" ref={heading}>{lesson ? <><small className="learning-module-label">{room.program.modules.find(m => m.lessons.some(l => l.key === active))?.title}</small><LessonRenderer lesson={lesson} courseID={course.course_id || course.id} assets={room.assets} tests={room.tests} mode="learn" assessmentContext={preview ? undefined : { enrollmentId, lessonKey: lesson.key, updated, denied: refreshDeniedAccess, register }} assetURL={(asset, download = false) => preview ? academyAPI.assetURL(id, asset, download) : learningAPI.assetURL(enrollmentId, lesson.key, asset, download)} />
      {blocked && <div className="learning-requirement" role="note"><Icon name="lightbulb" /><p>{blocked}</p></div>}
      {!preview && !blocked && lesson.completion_rule === 'materials' && progress.get(active) !== 'completed' && <label className="learning-materials-ack"><input type="checkbox" checked={ack} onChange={e => setAck(e.target.checked)} />Я изучил материалы урока. Это моё подтверждение, а не автоматическая проверка просмотра.</label>}
      <footer className="learning-lesson-navigation"><button type="button" className="course-secondary" disabled={busy || index <= 0} onClick={() => choose(available[index - 1].key)}>← Предыдущий урок</button><button type="button" className="course-primary" disabled={busy || preview || !!blocked || progress.get(active) === 'completed' || lesson.completion_rule === 'materials' && !ack} onClick={complete}>{progress.get(active) === 'completed' ? '✓ Урок завершён' : busy ? 'Сохраняем…' : 'Завершить урок'}</button><button type="button" className="course-secondary" disabled={busy || index < 0 || index >= available.length - 1} onClick={() => choose(available[index + 1].key)}>Следующий урок →</button></footer>{index === available.length - 1 && summary.status !== 'completed' && <p className="learning-last-lesson">Это последний доступный урок. Курс завершится, когда будут выполнены все обязательные требования.</p>}{summary.status === 'completed' && <button className="learning-completed-link" onClick={() => setCelebrate(true)}>✓ Поздравляем! Вы завершили курс — посмотреть итог</button>}
    </> : <div className="learning-empty"><img src="/static/academy/program-start.png" alt="" /><h2>Выберите урок в программе</h2><p>Откройте доступный урок, чтобы начать обучение.</p></div>}</div></div>
    {treeOpen && <AcademyDialog title="Программа курса" drawer close={() => setTreeOpen(false)}>{tree}</AcademyDialog>}
    {celebrate && !preview && <AcademyDialog title="Обучение завершено" wide close={() => setCelebrate(false)}><section className="learning-empty learning-success"><img src="/static/academy/learning-success.png" alt="Книга и знак завершения обучения" /><small>ВАШ НОВЫЙ ШАГ В ПРОФЕССИИ</small><h2>Поздравляем! Вы завершили курс</h2><h3>{course.title}</h3><p>Преподаватель: {course.author_name}</p><p>Дата завершения: {learningDate(enrollment.completed_at)} · Пройдено уроков: {summary.completed_lessons}</p><div><Link className="course-primary" to="/profile/learning">Вернуться к моим курсам</Link><button className="course-secondary" onClick={() => { setCelebrate(false); choose(available[0]?.key) }}>Посмотреть курс ещё раз</button></div><small>Сертификаты и подтверждение компетенций на этом этапе не выдаются.</small></section></AcademyDialog>}
  </main></PublicLayout>
}
export default function LearningRoom(props) { const p = useParams(); return <Room key={`${p.enrollmentId || p.id}:${p.versionId || ''}`} {...props} /> }
