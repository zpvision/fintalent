import Icon from '../../components/Icon'

export default function CourseOutcomes({ items = [] }) {
  const results = (items || []).filter(text => text.trim())
  if (!results.length) return null
  return <section id="outcomes" className="course-outcomes"><div className="course-outcomes-surface"><span className="course-outcomes-eyebrow">ВАШ РЕЗУЛЬТАТ</span><h2>Чему вы научитесь</h2><p className="course-outcomes-subtitle">Знания и навыки, которые останутся с вами после курса</p><ul>{results.map((text, i) => <li key={i}><i className="course-outcome-check"><Icon name="check" viewBox="4 5 16 15" /></i><span>{text}</span></li>)}</ul></div></section>
}
