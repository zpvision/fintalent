import { useEffect, useRef, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { createQuestion, createTest, deleteQuestion, forkTestDraft, getTest, getTestCategories, publishTest, updateQuestion, updateTest } from '../../api/tests'
import { useDocumentPage } from '../../hooks/useDocumentPage'
import usePageStyles from '../../hooks/usePageStyles'
import UserLayout from '../../layouts/UserLayout'

const styles = ['/static/profile.css', '/static/test-editor.css', '/static/test-create-profile.css', '/static/profile-logo.css', '/static/profile-sidebar-v2.css', '/static/profile-buttons.css', '/static/test-correct.css', '/static/test-drag.css', '/static/fintalent-theme.css', '/static/test-create-fix.css', '/static/test-create-blue.css', '/static/test-create-readable.css', '/static/test-info-center.css', '/static/test-answers-section.css', '/static/test-question-clarity.css', '/static/test-question-tools.css?v=2', '/static/test-preview.css?v=2', '/static/test-editor-ux.css', '/static/test-sidebar-sticky.css']
const stepNames = ['Информация', 'Вопросы', 'Предпросмотр', 'Публикация']
const blankTest = () => ({ title: '', description: '', category: '', difficulty: 'medium', visibility: 'public', is_free: true, shuffle_answers: false, price: 0, version: 1, status: 'draft' })
const makeAnswer = (answer = '', is_correct = false) => ({ answer, is_correct, key: crypto.randomUUID() })
const blankQuestion = () => ({ id: 0, key: crypto.randomUUID(), question: '', question_type: 'single_choice', explanation: '', points: 1, settings: { shuffle_answers: false }, answers: [makeAnswer('', true), makeAnswer()] })
const prepareQuestion = q => ({ ...q, key: crypto.randomUUID(), answers: (q.answers || []).map(a => ({ ...a, key: crypto.randomUUID() })) })
const infoPayload = t => ({ title: t.title.trim(), description: (t.description || '').trim(), category: t.category, difficulty: t.difficulty, visibility: t.visibility || 'public', is_free: t.is_free, shuffle_answers: !!t.shuffle_answers, price: t.is_free ? 0 : Number(t.price || 0), passing_percent: 60 })

function moveItem(items, from, to) {
  if (from === to || from < 0 || to < 0 || from >= items.length || to >= items.length) return items
  const next = [...items]
  next.splice(to, 0, next.splice(from, 1)[0])
  return next
}

// Preserve the legacy drag handles and visual feedback for questions and answers.
function useOrdering(selector, handle, classes, move) {
  const root = useRef(null), dragged = useRef(null)
  const items = () => [...(root.current?.querySelectorAll(selector) || [])]
  function reset() {
    items().forEach(item => { item.classList.remove(...classes); item.draggable = !handle })
    dragged.current = null
  }
  return {
    ref: root,
    onPointerDown(e) {
      const item = e.target.closest(selector)
      if (item) item.draggable = !handle || !!e.target.closest(handle)
    },
    onPointerUp() { if (!dragged.current) reset() },
    onDragStart(e) {
      const item = e.target.closest(selector)
      if (!item?.draggable) { e.preventDefault(); return }
      e.stopPropagation(); dragged.current = item; item.classList.add(classes[0])
      e.dataTransfer.effectAllowed = 'move'
      e.dataTransfer.setData('text/plain', String(items().indexOf(item)))
    },
    onDragOver(e) {
      if (!dragged.current) return
      e.preventDefault(); e.stopPropagation()
      items().forEach(item => item.classList.remove(...classes.slice(1)))
      const target = e.target.closest(selector)
      if (!target || target === dragged.current) return
      const bounds = target.getBoundingClientRect()
      target.classList.add(classes[e.clientY > bounds.top + bounds.height / 2 ? 2 : 1])
    },
    onDrop(e) {
      if (!dragged.current) return
      e.preventDefault(); e.stopPropagation()
      const target = e.target.closest(selector), list = items()
      const from = list.indexOf(dragged.current), index = list.indexOf(target)
      if (target && target !== dragged.current) {
        const bounds = target.getBoundingClientRect()
        const to = handle ? index + (e.clientY > bounds.top + bounds.height / 2 ? 1 : 0) - (from < index ? 1 : 0) : index
        move(from, to)
      }
      reset()
    },
    onDragEnd(e) { e.stopPropagation(); reset() },
  }
}

function Question({ value, index, set, remove, invalid = {} }) {
  const multiple = value.question_type === 'multiple_choice', text = value.question_type === 'text', boolean = value.question_type === 'boolean'
  const change = (key, next) => set({ ...value, [key]: next })
  const answerOrder = useOrdering('.answer-row', '.drag-handle', ['dragging', 'drop-before', 'drop-after'], (from, to) => change('answers', moveItem(value.answers, from, to)))
  function changeType(type) {
    let answers = value.answers
    if (type === 'boolean') answers = [makeAnswer('Да', true), makeAnswer('Нет')]
    else if (type === 'text') answers = [makeAnswer(answers[0]?.answer || '', true)]
    else {
      if (answers.length < 2) answers = [...answers, ...Array.from({ length: 2 - answers.length }, () => makeAnswer())]
      if (type === 'single_choice') {
        const correct = Math.max(0, answers.findIndex(a => a.is_correct))
        answers = answers.map((a, i) => ({ ...a, is_correct: i === correct }))
      }
    }
    set({ ...value, question_type: type, answers })
  }
  function removeAnswer(index) {
    let answers = value.answers.filter((_, i) => i !== index)
    if (!multiple && answers.length && !answers.some(a => a.is_correct)) answers = answers.map((a, i) => ({ ...a, is_correct: i === 0 }))
    change('answers', answers)
  }
  const selected = value.answers.flatMap((a, i) => a.is_correct ? [String(i)] : [])
  return <article className={`panel question${value.fresh ? ' question-appearing' : ''}${invalid.question ? ' invalid-question' : ''}`} data-key={value.key} data-id={value.id || ''} draggable={false}>
    <div className="question-head"><b className="number" title="Перетащите, чтобы изменить порядок вопросов">{index + 1}</b><span className="question-type-label">Тип вопроса</span><select className="type" aria-label={`Тип вопроса ${index + 1}`} value={value.question_type} onChange={e => changeType(e.target.value)}><option value="single_choice">Один вариант</option><option value="multiple_choice">Несколько вариантов</option><option value="boolean">Да / Нет</option><option value="text">Текст</option></select><button type="button" className="remove" title="Удалить" aria-label={`Удалить вопрос ${index + 1}`} onClick={remove}>×</button></div>
    <label className="question-field"><span className="question-field-title"><i>?</i> Вопрос</span><textarea className={`question-text${invalid.question ? ' invalid-field' : ''}`} rows="3" placeholder="Введите текст вопроса" value={value.question} onChange={e => change('question', e.target.value)} /></label>
    <div className={`answers-section${invalid.answers ? ' invalid-section' : ''}`}>
      <div className="answer-caption">Варианты ответов</div>
      <div className="answers" {...answerOrder}>{!text && value.answers.map((a, i) => <div className="answer-row" key={a.key} draggable={false}>
        <b className="drag-handle" title="Перетащите, чтобы изменить порядок">☰ {i + 1}.</b>
        <input className={`answer-text${invalid.empty?.includes(a.key) ? ' invalid-field' : ''}`} placeholder="Вариант ответа" aria-label={`Вариант ответа ${i + 1}`} value={a.answer} readOnly={boolean} onChange={e => change('answers', value.answers.map((item, n) => n === i ? { ...item, answer: e.target.value } : item))} />
        <button className={`remove-answer${boolean ? ' hidden' : ''}`} type="button" title="Удалить вариант" aria-label="Удалить вариант" onClick={() => removeAnswer(i)}><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 7h16M9 7V4h6v3m-9 0 1 14h10l1-14M10 11v6m4-6v6" /></svg></button>
      </div>)}</div>
      <button type="button" className={`add-answer outline${text || boolean ? ' hidden' : ''}`} onClick={() => change('answers', [...value.answers, makeAnswer('', !multiple && !value.answers.length)])}>＋ Добавить вариант ответа</button>
      <div className="correct-answer">{text ? <><label>Правильный текстовый ответ<input className={`text-correct${invalid.answers ? ' invalid-field' : ''}`} placeholder="Введите эталонный ответ" value={value.answers[0]?.answer || ''} onChange={e => change('answers', [{ ...(value.answers[0] || makeAnswer()), answer: e.target.value, is_correct: true }])} /></label><small>Позже этот ответ можно будет проверять с помощью ИИ.</small></> : <>
        <label>Правильный {multiple ? 'ответ (можно выбрать несколько)' : 'ответ'}<select key={value.question_type} className="correct-select" multiple={multiple} size={multiple ? 4 : undefined} value={multiple ? selected : selected[0] ?? ''} onChange={e => { const correct = new Set([...e.target.selectedOptions].map(option => Number(option.value))); change('answers', value.answers.map((a, i) => ({ ...a, is_correct: correct.has(i) }))) }}>{value.answers.map((a, i) => <option key={a.key} value={String(i)}>{i + 1}. {a.answer.trim() || `Вариант ${i + 1}`}</option>)}</select></label>
        {multiple && <small>Выберите все правильные варианты. Для выбора нескольких удерживайте Ctrl.</small>}
      </>}</div>
    </div>
    <div className="grid"><label>Баллы за ответ<input className="points" type="number" min="0.1" step="0.1" value={value.points} onChange={e => change('points', e.target.value)} /></label><label>Пояснение<input className="explanation" placeholder="Показывается после завершения" value={value.explanation || ''} onChange={e => change('explanation', e.target.value)} /></label></div>
  </article>
}

export default function TestCreatePage() {
  useDocumentPage({ title: 'Создание теста — FinTalent' })
  usePageStyles(styles)
  const [params] = useSearchParams(), initialId = Number(params.get('id')) || 0
  const [id, setId] = useState(initialId), [step, setStep] = useState(1), [maxStep, setMaxStep] = useState(1)
  const [categories, setCategories] = useState([]), [test, setTest] = useState(blankTest)
  const [questions, setQuestions] = useState(() => [blankQuestion()]), [invalid, setInvalid] = useState({})
  const [error, setError] = useState(''), [busy, setBusy] = useState(false)
  const pendingScroll = useRef(null)
  const moveQuestion = (from, to) => setQuestions(items => moveItem(items, from, to))
  const questionOrder = useOrdering('.question', '.number', ['question-dragging', 'question-drop-before', 'question-drop-after'], moveQuestion)
  const previewOrder = useOrdering('.preview-question', null, ['preview-dragging', 'preview-drop', 'preview-drop'], moveQuestion)

  useEffect(() => {
    let cancelled = false
    setId(initialId); setStep(1); setInvalid({}); setError('')
    async function load() {
      try {
        const items = await getTestCategories()
        if (cancelled) return
        setCategories(items)
        if (initialId) {
          let value = await getTest(initialId)
          if (cancelled) return
          if (value.status !== 'draft') { await forkTestDraft(initialId); value = await getTest(initialId) }
          if (cancelled) return
          setTest(value); setQuestions((value.questions || []).map(prepareQuestion)); setMaxStep(value.questions?.length ? 4 : 1)
        } else { setTest(blankTest()); setQuestions([blankQuestion()]); setMaxStep(1) }
      } catch (e) { if (!cancelled) setError(e.message) }
    }
    load()
    return () => { cancelled = true }
  }, [initialId])

  useEffect(() => {
    if (step === 3) window.scrollTo({ top: 0, left: 0, behavior: 'auto' })
    if (step !== 2 || !pendingScroll.current) return
    const key = pendingScroll.current
    const timer = setTimeout(() => { document.querySelector(`[data-key="${key}"]`)?.scrollIntoView({ behavior: 'smooth', block: 'center' }); pendingScroll.current = null }, 60)
    return () => clearTimeout(timer)
  }, [step, questions.length])

  async function saveInfo() {
    const body = infoPayload(test)
    if (body.title.length < 3) throw Error('Введите название теста')
    if (!body.category) throw Error('Выберите категорию теста')
    if (id) { await updateTest(id, body); return id }
    const value = await createTest(body)
    setId(value.id); history.replaceState(null, '', `?id=${value.id}`)
    return value.id
  }
  async function saveQuestions(testId = id) {
    if (!questions.length) throw Error('Добавьте хотя бы один вопрос')
    const issues = {}
    for (const q of questions) {
      const text = q.question_type === 'text', correct = q.answers.filter(a => a.is_correct).length
      const empty = q.answers.filter(a => !a.answer.trim()).map(a => a.key)
      const answers = text ? !q.answers[0]?.answer.trim() : q.answers.length < 2 || (q.question_type === 'multiple_choice' ? correct < 1 : correct !== 1)
      if (!q.question.trim() || answers || empty.length) issues[q.key] = { question: !q.question.trim(), answers, empty }
    }
    setInvalid(issues)
    if (Object.keys(issues).length) {
      setTimeout(() => document.querySelector('.test-create-main .invalid-field,.test-create-main .invalid-section')?.scrollIntoView({ behavior: 'smooth', block: 'center' }), 20)
      throw Error('Заполните поля, выделенные красным')
    }
    for (const [index, q] of questions.entries()) {
      const body = { id: q.id || 0, question: q.question.trim(), question_type: q.question_type, explanation: (q.explanation || '').trim(), points: Number(q.points), settings: { ...q.settings, shuffle_answers: !!test.shuffle_answers }, sort_order: index + 1, answers: q.answers.map((a, i) => ({ answer: a.answer.trim(), is_correct: !!a.is_correct, sort_order: i + 1 })) }
      if (q.id) await updateQuestion(q.id, body)
      else { const value = await createQuestion(testId, body); setQuestions(items => items.map(item => item.key === q.key ? { ...item, id: value.id } : item)) }
    }
  }
  async function navigate(target, continueStep = false) {
    if (busy || target === step || (!continueStep && target > maxStep)) return
    setBusy(true); setError('')
    try {
      if (target > step && step === 1) await saveInfo()
      if (target > step && (step === 2 || step === 3)) await saveQuestions()
      setStep(target); setMaxStep(current => Math.max(current, target))
    } catch (e) { setError(e.message) } finally { setBusy(false) }
  }
  async function removeQuestion(q) {
    if (q.id && !confirm('Удалить вопрос?')) return
    try { if (q.id) await deleteQuestion(q.id); setQuestions(items => items.filter(item => item.key !== q.key)) } catch (e) { setError(e.message) }
  }
  function addQuestion() {
    const question = { ...blankQuestion(), fresh: true }
    pendingScroll.current = question.key
    setQuestions(items => [...items, question])
  }
  function changeQuestion(key, value) {
    setQuestions(items => items.map(item => item.key === key ? value : item))
    setInvalid(items => { const next = { ...items }; delete next[key]; return next })
  }
  async function publish(marketplace = false) {
    setBusy(true); setError('')
    try {
      const testId = await saveInfo()
      await saveQuestions(testId)
      if (marketplace) await updateTest(testId, { ...infoPayload(test), visibility: 'marketplace' })
      await publishTest(testId); location.assign('/tests')
    } catch (e) { setError(e.message) } finally { setBusy(false) }
  }
  const points = questions.reduce((sum, q) => sum + (Number(q.points) || 0), 0)
  const field = (name, value) => setTest(current => ({ ...current, [name]: value }))

  return <UserLayout active="tests"><main className="dashboard-main test-create-main"><div className="create-workspace">
    <div className="editor-breadcrumb"><a href="/tests">Тесты и навыки</a><span>›</span><a href="/tests">Мои тесты</a><span>›</span><b>Новый тест</b></div>
    <div className="editor-title"><div><h1>Создание теста</h1><p>Соберите профессиональный тест и настройте правила прохождения</p></div><div className="draft-state"><i>✓</i><span>Черновик сохраняется</span><a href="/tests">Сохранить и выйти</a></div></div>
    <div className="steps">{stepNames.map((name, i) => <span key={name} className={`${i + 1 <= step ? 'active' : ''} ${i + 1 <= maxStep ? 'available' : ''}`} tabIndex={0} role="button" aria-current={i + 1 === step ? 'step' : 'false'} aria-disabled={i + 1 > maxStep || busy} onClick={() => navigate(i + 1)} onKeyDown={e => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); navigate(i + 1) } }}><i>{i + 1}</i><b>{name}</b></span>)}</div>
    <div id="error" role="alert">{error}</div>
    <div className="editor-layout"><div className="editor-stage">
      <section className={`step${step === 1 ? ' active' : ''}`} data-step="1">
        <div className="section-head"><div><h2>Информация о тесте</h2><p>Заполните основные настройки справа и проверьте описание теста</p></div><span>Шаг 1 из 4</span></div>
        <div className="panel start-card"><div className="start-icon">▤</div><h2>Подготовьте основу теста</h2><p>Укажите понятное название, описание, категорию и уровень сложности. После этого переходите к созданию вопросов.</p><div className="start-tips"><span><i>✓</i> Короткое и ясное название</span><span><i>✓</i> Описание проверяемых навыков</span><span><i>✓</i> Подходящая категория</span></div></div>
        <section className="information-fields"><h3>Настройки теста</h3>
          <label>Название теста<input id="test-title" maxLength="200" placeholder="Например, Основы бухгалтерского учёта" value={test.title} onChange={e => field('title', e.target.value)} /></label>
          <label>Описание теста<textarea id="description" rows="5" placeholder="Какие знания проверяет тест" value={test.description || ''} onChange={e => field('description', e.target.value)} /></label>
          <label>Категория<select id="category" required value={test.category || ''} onChange={e => field('category', e.target.value)}><option value="">Выберите категорию</option>{categories.map(x => <option key={x.id} value={x.name}>{x.name}</option>)}{test.category && !categories.some(item => item.name === test.category) && <option value={test.category}>{test.category} (архивная)</option>}</select></label>
          <label>Уровень сложности<select id="difficulty" value={test.difficulty} onChange={e => field('difficulty', e.target.value)}><option value="easy">Лёгкая</option><option value="medium">Средняя</option><option value="hard">Сложная</option></select></label>
          <label className="shuffle-setting test-shuffle-setting"><i aria-hidden="true">⇄</i><span><b>Перемешивать варианты</b><small id="shuffle-answers-hint">Для всех вопросов теста при каждом прохождении</small></span><input id="shuffle-answers" type="checkbox" aria-describedby="shuffle-answers-hint" checked={!!test.shuffle_answers} onChange={e => field('shuffle_answers', e.target.checked)} /><em aria-hidden="true" /></label>
          <div className="price"><label><input type="checkbox" id="is-free" checked={test.is_free} onChange={e => field('is_free', e.target.checked)} /> Бесплатный тест</label><label id="price-label" className={test.is_free ? 'hidden' : ''}>Стоимость, ₽<input type="number" id="price" min="1" step="1" value={test.price || ''} onChange={e => field('price', Number(e.target.value))} /></label></div>
        </section>
      </section>
      <section className={`step${step === 2 ? ' active' : ''}`} data-step="2">
        <div className="section-head"><div><h2>Вопросы теста</h2><p>Добавьте вопросы, варианты и укажите правильные ответы</p></div><button id="add-question" className="add-question-top" onClick={addQuestion}>＋ Добавить вопрос</button></div>
        <div id="questions" {...questionOrder}>{questions.map((q, i) => <Question key={q.key} value={q} index={i} set={value => changeQuestion(q.key, value)} remove={() => removeQuestion(q)} invalid={invalid[q.key]} />)}</div>
        <button type="button" className="add-question-bottom outline" onClick={addQuestion}>＋ Добавить ещё вопрос</button>
      </section>
      <section className={`step${step === 3 ? ' active' : ''}`} data-step="3">
        <div className="section-head"><div><h2>Предпросмотр теста</h2><p>Проверьте, как вопросы увидят пользователи</p></div><span>Шаг 3 из 4</span></div>
        <div id="preview" {...previewOrder}><div className="panel preview-test-head"><span>Предпросмотр теста</span><h2>{test.title.trim()}</h2><p>{(test.description || '').trim()}</p></div><div className="preview-order-hint">☷ Перетаскивайте вопросы или используйте стрелки справа</div>
          {questions.map((q, i) => <article className="panel preview-question" draggable key={q.key} data-index={i}><div className="preview-question-number">{i + 1}</div><div className="preview-question-body"><h3>{q.question.trim()} <small>{q.points} балл(а)</small></h3><div className="preview-answers">{q.answers.map((a, n) => <div className="preview-answer" key={a.key}><b>{n + 1}</b><span>{a.answer.trim()}</span></div>)}</div></div><div className="preview-order-actions"><button type="button" data-move="up" title="Переместить выше" disabled={i === 0} onClick={() => moveQuestion(i, i - 1)}>↑</button><button type="button" data-move="down" title="Переместить ниже" disabled={i === questions.length - 1} onClick={() => moveQuestion(i, i + 1)}>↓</button><i title="Перетащите вопрос">☷</i></div></article>)}
        </div>
      </section>
      <section className={`step${step === 4 ? ' active' : ''}`} data-step="4">
        <div className="section-head"><div><h2>Публикация</h2><p>Выберите способ публикации готового теста</p></div><span>Шаг 4 из 4</span></div>
        <div className="panel publish"><div>✓</div><h2 id="ready-title">{test.title}</h2><p id="ready-info">{questions.length} вопросов · версия {test.version || 1}</p><button id="publish" disabled={busy} onClick={() => publish()}>Опубликовать тест</button><button id="marketplace" className="outline" disabled={busy} onClick={() => publish(true)}>Опубликовать в Marketplace</button><a href="/tests">Оставить черновиком</a></div>
      </section>
    </div><aside className="test-settings">
      <section className="test-stat-card"><h3>Статистика теста <small>черновик</small></h3><div><span>Вопросов</span><b id="stat-questions">{questions.length}</b></div><div><span>Максимальный балл</span><b id="stat-points">{Number.isInteger(points) ? points : points.toFixed(1)}</b></div><div><span>Версия</span><b id="stat-version">{test.version || 1}</b></div></section>
      <section id="question-navigation" className="question-navigation"><h3>Навигация по вопросам</h3><div id="question-navigation-list">{questions.map((q, i) => { const title = q.question.trim() || 'Вопрос без названия'; return <button type="button" key={q.key} data-question={i} onClick={() => { pendingScroll.current = q.key; if (step === 2) document.querySelector(`[data-key="${q.key}"]`)?.scrollIntoView({ behavior: 'smooth', block: 'center' }); else setStep(2) }}><b>{i + 1}</b><span>{title.slice(0, 30)}{title.length > 30 ? '…' : ''}</span></button> })}</div></section>
      <section className="editor-help"><h3>Нужна помощь?</h3><p>Используйте разные типы вопросов, чтобы точнее оценить знания кандидата.</p><a href="#">Открыть руководство →</a></section>
    </aside></div>
    <footer className="editor-footer"><button id="prev" className={`outline${step === 1 || step === 4 ? ' hidden' : ''}`} disabled={busy} onClick={() => navigate(step - 1)}>← Назад</button><span /><button id="next" className={step === 4 ? 'hidden' : ''} disabled={busy} onClick={() => navigate(step + 1, true)}>Продолжить →</button></footer>
  </div></main></UserLayout>
}
