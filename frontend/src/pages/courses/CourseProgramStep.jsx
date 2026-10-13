import { useRef, useState } from 'react'
import Icon from '../../components/Icon'
import Field from './CourseField'
import Reorder from './CourseReorder'
import CourseLessonType, { lessonTypeInfo, LessonTypeIcon } from './CourseLessonType'

export default function CourseProgramStep({ value, meta, update, moduleUpdate, lessonUpdate, remove, moveItem, lesson }) {
  const [active, setActive] = useState('')
  const targets = useRef(new Map())
  const moduleKey = (m, i) => `module-${m.id || `new-${i}`}`
  const lessonKey = (m, i, l, j) => `${moduleKey(m, i)}-lesson-${l.id || `new-${j}`}`
  const register = key => node => { if (node) targets.current.set(key, node); else targets.current.delete(key) }
  function jump(key) {
    const node = targets.current.get(key)
    if (!node) return
    if (node.tagName === 'DETAILS') node.open = true
    setActive(key)
    node.scrollIntoView({ behavior: 'smooth', block: 'start' })
    node.focus({ preventScroll: true })
  }
  const typeOptions = meta.lesson_types || []
  const lessonCount = value.modules.reduce((total, m) => total + m.lessons.length, 0)

  return <div className="course-program-layout">
    <aside className="course-program-sidebar">
      <header><Icon name="list" /><div><h3>Структура курса</h3><p>{value.modules.length} модулей · {lessonCount} уроков</p></div></header>
      <nav aria-label="Модули и уроки курса">
        {value.modules.length ? value.modules.map((m, i) => <div className="course-program-nav-group" key={moduleKey(m, i)}>
          <button type="button" className="course-program-nav-module" aria-current={active === moduleKey(m, i) ? 'location' : undefined} onClick={() => jump(moduleKey(m, i))}><span>{i + 1}</span><b>{m.title || `Модуль ${i + 1}`}</b><small>{m.lessons.length}</small></button>
          {m.lessons.map((l, j) => <button type="button" className="course-program-nav-lesson" key={lessonKey(m, i, l, j)} aria-current={active === lessonKey(m, i, l, j) ? 'location' : undefined} onClick={() => jump(lessonKey(m, i, l, j))}><span>{i + 1}.{j + 1}</span><LessonTypeIcon icon={lessonTypeInfo(l, typeOptions).icon} /><span>{l.title || 'Новый урок'}</span></button>)}
          {!m.lessons.length && <p className="course-program-nav-empty">Уроков пока нет</p>}
        </div>) : <p className="course-program-nav-empty">Добавьте модуль — здесь появится программа курса.</p>}
      </nav>
      <p className="course-program-nav-hint">Нажмите на модуль или урок, чтобы перейти к редактированию.</p>
    </aside>
    <div className="course-program-content">
      {!value.modules.length && <div className="course-inline-empty">Начните с первого модуля. Например, «Знакомство с инструментами». Затем добавьте в него уроки.</div>}
      {value.modules.map((m, i) => <section className="course-module-editor" key={moduleKey(m, i)} ref={register(moduleKey(m, i))} tabIndex={-1} aria-label={`Модуль ${i + 1}`} onFocusCapture={() => setActive(moduleKey(m, i))}>
        <div className="course-row-head"><div className="course-program-module-title"><span>{i + 1}</span><div><h3>Модуль {i + 1}</h3><small>{m.lessons.length} уроков · {m.lessons.reduce((n, l) => n + l.duration_minutes, 0)} мин</small></div></div><Reorder index={i} count={value.modules.length} label={`модуль ${i + 1} и его уроки`} move={(a, b) => update('modules', moveItem(value.modules, a, b))} remove={() => remove('modules', i)} /></div>
        <Field label="Название модуля" value={m.title} onChange={title => moduleUpdate(i, { title })} placeholder="Например, первые шаги и основные инструменты" />
        {m.lessons.map((l, j) => <details className="course-lesson-editor" key={lessonKey(m, i, l, j)} ref={register(lessonKey(m, i, l, j))} tabIndex={-1} open onFocusCapture={event => { event.stopPropagation(); setActive(lessonKey(m, i, l, j)) }}>
          <summary><span className="course-program-lesson-title"><span>{i + 1}.{j + 1}</span><b>{l.title || 'Новый урок'}</b></span><small>{lessonTypeInfo(l, typeOptions).name} · {l.duration_minutes} мин</small></summary>
          <div><div className="course-row-head"><h4>Настройки урока</h4><Reorder index={j} count={m.lessons.length} label={`урок ${j + 1}`} move={(a, b) => moduleUpdate(i, { lessons: moveItem(m.lessons, a, b) })} remove={() => { if (confirm('Удалить урок?')) moduleUpdate(i, { lessons: m.lessons.filter((_, k) => k !== j) }) }} /></div>
            <div className="course-lesson-fields">
              <Field label="Название урока" value={l.title} onChange={title => lessonUpdate(i, j, { title })} placeholder="Какой теме посвящён урок?" />
              <CourseLessonType lesson={l} options={typeOptions} onChange={type => lessonUpdate(i, j, { type })} />
              <Field label="Короткое описание (необязательно)" max={1200} area rows={3} value={l.description} onChange={description => lessonUpdate(i, j, { description })} placeholder="Что участник узнает или сделает на этом уроке" />
              <Field label="Длительность, мин" type="number" min="0" value={l.duration_minutes} onChange={v => lessonUpdate(i, j, { duration_minutes: Number(v) })} />
            </div>
          </div>
        </details>)}
        {!typeOptions.length && <p className="course-muted">Нет активных типов уроков. Добавьте их в справочнике «Типы уроков» админки.</p>}
        <button type="button" className="course-add" disabled={!typeOptions.length} onClick={() => moduleUpdate(i, { lessons: [...m.lessons, { ...lesson(), type: typeOptions[0]?.key || '' }] })}>＋ Добавить урок</button>
      </section>)}
      <button type="button" className="course-add course-program-add-module" onClick={() => update('modules', [...value.modules, { id: 0, title: '', description: '', lessons: [] }])}>＋ Добавить модуль</button>
    </div>
  </div>
}
