import { useId } from 'react'

export default function CourseField({ label, value, onChange, max = 180, area = false, hint, ...props }) {
  const id = useId()
  const Input = area ? 'textarea' : 'input'
  return <div className="course-field"><label htmlFor={id}>{label}</label><Input id={id} aria-describedby={hint ? `${id}-hint` : undefined} value={value ?? ''} onChange={e => onChange(e.target.value)} maxLength={max} rows={area ? 3 : undefined} {...props} />{hint && <small id={`${id}-hint`}>{hint}</small>}{props.type !== 'number' && <small className="course-counter">{String(value || '').length}/{max}</small>}</div>
}
