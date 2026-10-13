import Icon from '../../components/Icon'
import IconSelect from '../../components/IconSelect'
import { types } from './shared'

const defaultIcons = { video: 'video', text: 'book', practice: 'briefcase', test: 'check', materials: 'folder' }
export function lessonTypeInfo(lesson, options = []) {
  return options.find(t => t.key === lesson.type) || { key: lesson.type, name: lesson.type_name || types[lesson.type] || lesson.type, icon: lesson.type_icon || defaultIcons[lesson.type] || 'folder' }
}
export function LessonTypeIcon({ icon }) {
  return /^\/static\/uploads\/position-icons\/[a-f0-9]+\.svg$/.test(icon || '') ? <img src={icon} alt="" /> : <Icon name={icon} />
}
export default function CourseLessonType({ lesson, options, onChange }) {
  const current = lessonTypeInfo(lesson, options)
  const choices = options.some(t => t.key === current.key) ? options : [current, ...options]
  return <div className="course-field course-lesson-type-select"><span>Тип урока</span><IconSelect showLabels label="Тип урока" value={lesson.type} options={Object.fromEntries(choices.map(t => [t.key, t.name]))} onChange={onChange} buttonText={current.name} renderIcon={key => <LessonTypeIcon icon={choices.find(t => t.key === key)?.icon} />} /></div>
}
