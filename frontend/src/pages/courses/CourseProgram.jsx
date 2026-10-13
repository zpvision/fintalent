import { useState } from 'react'
import Icon from '../../components/Icon'
import { LessonTypeIcon, lessonTypeInfo } from './CourseLessonType'
import { duration } from './shared'

export default function CourseProgram({ modules }) {
  const [open, setOpen] = useState(() => new Set(modules.length ? [0] : []))
  const lessons = modules.flatMap(m => m.lessons)
  const expanded = modules.length > 0 && open.size === modules.length
  function syncOpen(index, isOpen) { setOpen(previous => { if (previous.has(index) === isOpen) return previous; const next = new Set(previous); isOpen ? next.add(index) : next.delete(index); return next }) }

  return <section id="program" className="course-program-view" aria-labelledby="course-program-title">
    <div className="cp-heading"><div><small>ОТ ПЕРВЫХ ШАГОВ К ПРАКТИКЕ</small><h2 id="course-program-title">Программа курса</h2><p><span><Icon name="folder" />{modules.length} модулей</span><span><Icon name="list" />{lessons.length} уроков</span><span><Icon name="clock" />{duration(lessons.reduce((sum, l) => sum + l.duration_minutes, 0))}</span></p></div>{modules.length > 0 && <button type="button" className="course-secondary cp-toggle-all" aria-expanded={expanded} onClick={() => setOpen(expanded ? new Set() : new Set(modules.map((_, i) => i)))}>{expanded ? 'Свернуть все' : 'Развернуть все'}<span aria-hidden="true">{expanded ? '−' : '+'}</span></button>}</div>
    <div className="cp-modules">{modules.map((m, i) => <details className="cp-module" key={m.id || i} open={open.has(i)} onToggle={event => syncOpen(i, event.currentTarget.open)}>
      <summary><span className="cp-module-number" aria-hidden="true">{String(i + 1).padStart(2, '0')}</span><span className="cp-module-copy"><small>МОДУЛЬ {i + 1}</small><strong>{m.title}</strong><span className="cp-module-meta">{m.lessons.length} уроков<span aria-hidden="true">·</span>{duration(m.lessons.reduce((sum, l) => sum + l.duration_minutes, 0))}</span></span><span className="cp-chevron" aria-hidden="true">⌄</span></summary>
      <div className="cp-module-content">{m.description && <p className="cp-module-description">{m.description}</p>}<ol className="cp-lessons">{m.lessons.map((l, j) => <li key={l.id || j}><span className="cp-lesson-marker" aria-hidden="true"><i><LessonTypeIcon icon={lessonTypeInfo(l).icon} /></i><small>{i + 1}.{j + 1}</small></span><div className="cp-lesson-copy"><div className="cp-lesson-heading"><strong>{l.title}</strong><span className="cp-lesson-duration"><Icon name="clock" />{duration(l.duration_minutes)}</span></div>{l.description && <p>{l.description}</p>}<span className="cp-lesson-type">{lessonTypeInfo(l).name}</span></div></li>)}</ol>{!m.lessons.length && <p className="course-muted">Уроки появятся здесь после добавления автором.</p>}</div>
    </details>)}</div>
    {!modules.length && <div className="cp-empty"><Icon name="book" /><p>Автор пока готовит программу курса.</p></div>}
  </section>
}
