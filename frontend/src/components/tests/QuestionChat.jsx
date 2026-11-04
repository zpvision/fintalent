import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import QuestionMaterials from './QuestionMaterials'
import usePageStyles from '../../hooks/usePageStyles'

const formatTimer = seconds => `${String(Math.floor(seconds / 60)).padStart(2, '0')}:${String(seconds % 60).padStart(2, '0')}`
const wait = milliseconds => new Promise(resolve => setTimeout(resolve, milliseconds))

export default function QuestionChat({ title, questions, initialIndex = 0, initialAnswers = [], startedAt, remainingSeconds = 0, timeLimitSeconds = 0, employee = false, materialAccess, saveAnswer, finish, onDone, onError }) {
  usePageStyles(['/static/test-timer.css?v=1'])
  const [index, setIndex] = useState(initialIndex)
  const [selected, setSelected] = useState([])
  const [text, setText] = useState('')
  const [log, setLog] = useState(() => questions.slice(0, initialIndex).flatMap(q => {
    const saved = initialAnswers.filter(answer => answer.question_id === q.id)
    if (!saved.length) return []
    const selectedIds = new Set(saved.map(answer => answer.selected_answer_id))
    const answer = q.question_type === 'text'
      ? saved[0].text_answer || ''
      : (q.answers || []).filter(option => selectedIds.has(option.id)).map(option => option.answer).join(', ')
    return [{ q, answer }]
  }))
  const [busy, setBusy] = useState(false)
  const [typing, setTyping] = useState(false)
  const [error, setError] = useState('')
  const [seconds, setSeconds] = useState(() => timeLimitSeconds ? remainingSeconds : Math.max(0, Math.floor((Date.now() - new Date(startedAt || Date.now()).getTime()) / 1000)))
  const [expired, setExpired] = useState(false)
  const clock = useRef({ start: performance.now(), remaining: Math.max(0, remainingSeconds) })
  const expiredRef = useRef(false), finishing = useRef(false), completed = useRef(false), pendingAnswer = useRef(null), retryAt = useRef(0)
  const callbacks = useRef({ finish, onDone })
  callbacks.current = { finish, onDone }
  const urgent = timeLimitSeconds > 0 && seconds <= 60 && !expired
  const timerBadge = urgent && <span className="test-time-warning"><span aria-hidden="true">◷</span> Осталось {formatTimer(seconds)}</span>
  const locked = useRef(false)
  const timers = useRef([])
  const messagesRef = useRef(null)
  const activeQuestionRef = useRef(null)
  const q = questions[index]
  const multiple = q?.question_type === 'multiple_choice'
  const textual = q?.question_type === 'text'

  async function complete(timedOut = false) {
    if (finishing.current || completed.current) return
    finishing.current = true
    locked.current = true
    setBusy(true)
    try {
      // Let an answer already sent before the deadline settle before scoring.
      if (pendingAnswer.current) await pendingAnswer.current.catch(() => {})
      const result = await callbacks.current.finish()
      completed.current = true
      callbacks.current.onDone(result, { timedOut: timedOut || expiredRef.current })
    } catch (failure) {
      if (expiredRef.current) {
        retryAt.current = performance.now() + 5000
        setError('Время вышло. Не удалось подтвердить сохранение результата. Повторяем отправку…')
      } else handleError(failure)
    } finally { finishing.current = false }
  }

  useEffect(() => {
    const tick = () => {
      if (completed.current) return
      const elapsed = (performance.now() - clock.current.start) / 1000
      const remaining = timeLimitSeconds ? Math.max(0, Math.ceil(clock.current.remaining - elapsed)) : Math.max(0, Math.floor((Date.now() - new Date(startedAt || Date.now()).getTime()) / 1000))
      setSeconds(remaining)
      if (timeLimitSeconds && remaining === 0) {
        expiredRef.current = true
        setExpired(true)
        locked.current = true
        timers.current.forEach(clearTimeout)
        if (performance.now() >= retryAt.current) void complete(true)
      }
    }
    tick()
    const timer = setInterval(tick, 250)
    document.addEventListener('visibilitychange', tick)
    return () => { clearInterval(timer); document.removeEventListener('visibilitychange', tick); timers.current.forEach(clearTimeout) }
  }, [])

  useLayoutEffect(() => {
    const messages = messagesRef.current
    const question = activeQuestionRef.current
    if (typing || !messages || !question) return
    const messagesRect = messages.getBoundingClientRect()
    const questionRect = question.getBoundingClientRect()
    messages.scrollTo({
      top: Math.max(0, messages.scrollTop + questionRect.top - messagesRect.top - 12),
      behavior: 'auto',
    })
  }, [index, typing])

  function handleError(failure) {
    const message = failure?.message || 'Не удалось сохранить ответ'
    setError(message)
    onError?.(message)
    locked.current = false
    setBusy(false)
    setTyping(false)
  }

  async function send(ids, textAnswer) {
    try {
      if (expiredRef.current) return
      pendingAnswer.current = saveAnswer({ question_id: q.id, selected_answer_ids: textual ? [] : ids, text_answer: textual ? textAnswer : '' })
      await pendingAnswer.current
      pendingAnswer.current = null
      if (expiredRef.current) return
      const answer = textual ? textAnswer : q.answers.filter(item => ids.includes(item.id)).map(item => item.answer).join(', ')
      setLog(value => [...value, { q, answer }])
      if (index + 1 < questions.length) {
        setTyping(true)
        await wait(650)
        if (expiredRef.current) return
        setIndex(value => value + 1)
        setSelected([])
        setText('')
        setTyping(false)
        locked.current = false
        setBusy(false)
      } else {
        await complete()
      }
    } catch (failure) { pendingAnswer.current = null; if (!expiredRef.current) handleError(failure) }
  }

  function submit(ids = selected, textAnswer = text.trim(), delay = 0) {
    if (expiredRef.current || locked.current || (!textAnswer && !ids.length)) return
    locked.current = true
    setBusy(true)
    setError('')
    if (delay) timers.current.push(setTimeout(() => send(ids, textAnswer), delay))
    else send(ids, textAnswer)
  }

  function choose(answer) {
    if (locked.current) return
    if (multiple) {
      setSelected(value => value.includes(answer.id) ? value.filter(id => id !== answer.id) : [...value, answer.id])
      return
    }
    setSelected([answer.id])
    submit([answer.id], '', 140)
  }

  function keySelect(event, answer) {
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault()
      choose(answer)
    }
  }

  if (expired) return <div className="chat-shell"><div className="chat-messages"><section className="bot-message result-message test-time-expired" role="status"><span className="test-time-expired-icon" aria-hidden="true">◷</span><h2>Время на прохождение закончилось</h2><p>Завершаем тест и сохраняем полученные ответы. Пожалуйста, не закрывайте страницу.</p>{error && <p role="alert">{error}</p>}</section></div></div>
  if (!q) return <div className="chat-shell"><div className="chat-messages"><p>Все ответы сохранены.</p><button disabled={busy} onClick={() => complete()}>Завершить тест</button>{error && <p role="alert">{error}</p>}</div></div>
  return <div className={`chat-shell${employee ? ' employee-chat-shell' : ''}`}>
    <div className="chat-top"><a href={employee ? undefined : '/tests'} aria-hidden={employee || undefined}>←</a><div className="chat-avatar">FT</div><div><h1>{title}</h1><p><i /> Тестирование идёт</p></div><div className="chat-progress"><b>{index + 1} / {questions.length}</b><span className={`test-countdown${urgent ? ' is-urgent' : ''}`} aria-label={timeLimitSeconds ? 'Осталось времени' : 'Прошло времени'}>{urgent && <i aria-hidden="true">◷ </i>}{formatTimer(seconds)}</span></div></div>
    <div className="chat-bar"><i style={{ width: `${index / questions.length * 100}%` }} /></div>
    <div className="chat-messages" ref={messagesRef}><div className="day-label">Сегодня</div><div className="bot-message intro"><b>FinTalent</b><p>Я буду задавать вопросы по одному. Выберите все подходящие ответы.</p></div>
      {log.map((item, itemIndex) => <div key={`${item.q.id}-${itemIndex}`}><div className="bot-message question-message"><p className="test-question-content">{item.q.question}</p><QuestionMaterials question={item.q} access={materialAccess} /></div><div className="user-wrap"><div className="user-message"><p>{item.answer}</p></div></div></div>)}
      {!typing && <><div className="bot-message question-message" ref={activeQuestionRef}><div className="question-label">Вопрос {index + 1}</div><p className="test-question-content">{q.question}</p><QuestionMaterials question={q} access={materialAccess} /><small>{q.points} балл(а)</small></div>
        {!textual && <div className={`bot-message options-message${busy ? ' answered' : ''}`}><div className="options-title"><span>Варианты ответа</span><small>Можно нажать мышкой</small></div><ol>{q.answers.map(answer => <li key={answer.id} role="button" tabIndex={0} className={selected.includes(answer.id) ? 'selected' : ''} onClick={() => choose(answer)} onKeyDown={event => keySelect(event, answer)}><b /><span>{answer.answer}</span><i>✓</i></li>)}</ol>{multiple && <button type="button" className="confirm-options" disabled={busy || !selected.length} onClick={() => submit()}>Ответить</button>}</div>}</>}
      {typing && <div className="bot-message typing" aria-label="Загружается следующий вопрос"><i /><i /><i /></div>}
      {error && <small className="bad">{error}</small>}
    </div>
    {urgent && <div className="test-time-reminder" role="status">{timerBadge}<small>После окончания времени тест завершится автоматически</small></div>}
    {textual && <form className="chat-compose" onSubmit={event => { event.preventDefault(); submit() }}><div><input value={text} autoComplete="off" placeholder="Напишите развернутый ответ…" onChange={event => setText(event.target.value)} /><small>Введите текст ответа и нажмите Enter</small></div><button title="Отправить" disabled={busy || !text.trim()}>➤</button></form>}
  </div>
}
