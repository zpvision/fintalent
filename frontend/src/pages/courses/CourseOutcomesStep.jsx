import Icon from '../../components/Icon'
import Field from './CourseField'
import Reorder from './CourseReorder'

export default function CourseOutcomesStep({ items, onChange }) {
  function move(from, to) { const next = [...items]; next.splice(to, 0, next.splice(from, 1)[0]); onChange(next) }
  return <section className="course-outcomes-editor"><div className="course-outcomes-intro"><Icon name="check" /><div><h3>После курса участник сможет…</h3><p>Один конкретный результат — один пункт. Например: «Составлять финансовую модель и оценивать рентабельность проекта».</p></div></div>{!items.length && <p className="course-muted">Раздел необязательный. Добавьте первый результат, чтобы показать практическую пользу обучения.</p>}<div className="course-outcome-fields">{items.map((text, i) => <article className="course-outcome-field" key={i}><i className="course-outcome-check"><Icon name="check" /></i><Field label={`Результат ${i + 1}`} area rows={2} max={300} value={text} onChange={value => onChange(items.map((s, j) => j === i ? value : s))} placeholder="Научитесь применять знания на практике" /><Reorder deleteIcon label={`результат ${i + 1}`} index={i} count={items.length} move={move} remove={() => onChange(items.filter((_, j) => j !== i))} /></article>)}</div><button type="button" className="course-add course-editor-add-primary" disabled={items.length >= 20} onClick={() => onChange([...items, ''])}>＋ Добавить результат</button><p className="course-muted">До 20 пунктов. Пустые строки не отображаются на странице курса.</p></section>
}
