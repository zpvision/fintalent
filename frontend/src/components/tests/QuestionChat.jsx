import { useEffect, useRef, useState } from 'react'

const formatTimer = seconds => `${String(Math.floor(seconds / 60)).padStart(2, '0')}:${String(seconds % 60).padStart(2, '0')}`
const wait = milliseconds => new Promise(resolve => setTimeout(resolve, milliseconds))

export default function QuestionChat({ title, questions, initialIndex = 0, startedAt, remainingSeconds = 0, timeLimitSeconds = 0, employee = false, saveAnswer, finish, onDone, onError }) {
  const [index, setIndex] = useState(initialIndex)
  const [selected, setSelected] = useState([])
  const [text, setText] = useState('')
  const [log, setLog] = useState([])
  const [busy, setBusy] = useState(false)
  const [typing, setTyping] = useState(false)
  const [error, setError] = useState('')
  const [seconds, setSeconds] = useState(() => timeLimitSeconds ? remainingSeconds : Math.max(0, Math.floor((Date.now() - new Date(startedAt || Date.now()).getTime()) / 1000)))
  const locked = useRef(false)
  const timers = useRef([])
  const q = questions[index]
  const multiple = q?.question_type === 'multiple_choice'
  const textual = q?.question_type === 'text'

  useEffect(() => {
    const timer = setInterval(() => setSeconds(value => {
      if (!timeLimitSeconds) return value + 1
      if (value > 1) return value - 1
      clearInterval(timer)
      if (!locked.current) {
        locked.current = true
        finish().then(onDone).catch(handleError)
      }
      return 0
    }), 1000)
    return () => { clearInterval(timer); timers.current.forEach(clearTimeout) }
  }, [])

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
      await saveAnswer({ question_id: q.id, selected_answer_ids: textual ? [] : ids, text_answer: textual ? textAnswer : '' })
      const answer = textual ? textAnswer : q.answers.filter(item => ids.includes(item.id)).map(item => item.answer).join(', ')
      setLog(value => [...value, { q, answer }])
      if (index + 1 < questions.length) {
        setTyping(true)
        await wait(650)
        setIndex(value => value + 1)
        setSelected([])
        setText('')
        setTyping(false)
        locked.current = false
        setBusy(false)
      } else {
        onDone(await finish())
      }
    } catch (failure) { handleError(failure) }
  }

  function submit(ids = selected, textAnswer = text.trim(), delay = 0) {
    if (locked.current || (!textAnswer && !ids.length)) return
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

  if (!q) return null
  return <div className={`chat-shell${employee ? ' employee-chat-shell' : ''}`}>
    <div className="chat-top"><a href={employee ? undefined : '/tests'} aria-hidden={employee || undefined}>←</a><div className="chat-avatar">FT</div><div><h1>{title}</h1><p><i /> Тестирование идёт</p></div><div className="chat-progress"><b>{index + 1} / {questions.length}</b><span>{formatTimer(seconds)}</span></div></div>
    <div className="chat-bar"><i style={{ width: `${index / questions.length * 100}%` }} /></div>
    <div className="chat-messages"><div className="day-label">Сегодня</div><div className="bot-message intro"><b>FinTalent</b><p>Я буду задавать вопросы по одному. Выберите все подходящие ответы.</p></div>
      {log.map((item, itemIndex) => <div key={`${item.q.id}-${itemIndex}`}><div className="bot-message question-message"><p>{item.q.question}</p></div><div className="user-wrap"><div className="user-message"><p>{item.answer}</p></div></div></div>)}
      {!typing && <><div className="bot-message question-message"><div className="question-label">Вопрос {index + 1}</div><p>{q.question}</p><small>{q.points} балл(а)</small></div>
        {!textual && <div className={`bot-message options-message${busy ? ' answered' : ''}`}><div className="options-title"><span>Варианты ответа</span><small>Можно нажать мышкой</small></div><ol>{q.answers.map(answer => <li key={answer.id} role="button" tabIndex={0} className={selected.includes(answer.id) ? 'selected' : ''} onClick={() => choose(answer)} onKeyDown={event => keySelect(event, answer)}><b /><span>{answer.answer}</span><i>✓</i></li>)}</ol>{multiple && <button type="button" className="confirm-options" disabled={busy || !selected.length} onClick={() => submit()}>Ответить</button>}</div>}</>}
      {typing && <div className="bot-message typing" aria-label="Загружается следующий вопрос"><i /><i /><i /></div>}
      {error && <small className="bad">{error}</small>}
    </div>
    {textual && <form className="chat-compose" onSubmit={event => { event.preventDefault(); submit() }}><div><input value={text} autoComplete="off" placeholder="Напишите развернутый ответ…" onChange={event => setText(event.target.value)} /><small>Введите текст ответа и нажмите Enter</small></div><button title="Отправить" disabled={busy || !text.trim()}>➤</button></form>}
  </div>
}
