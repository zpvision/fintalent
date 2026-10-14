import { useId, useState } from 'react'
import { money } from './shared'

export const initialTariff = course => Math.max(0, (course.tariffs || []).findIndex(t => t.recommended))
export const hasTariffs = course => course.pricing_mode === 'tariffs' && course.tariffs?.length > 0

export default function CourseTariffs({ course, selected, onSelect }) {
  const name = useId(), [local, setLocal] = useState(initialTariff(course))
  const index = selected ?? local
  return <fieldset className="course-tariffs"><legend>Выберите тариф</legend>{(course.tariffs || []).map((t, i) => <label className={index === i ? 'course-tariff selected' : 'course-tariff'} key={t.id || i}>
    {t.recommended && <small className="course-tariff-badge">Рекомендуем</small>}
    <span className="course-tariff-heading"><input type="radio" name={name} checked={index === i} onChange={() => { setLocal(i); onSelect?.(i) }} /><b>{t.title || 'Название тарифа'}</b><strong>{money(t.price)}</strong></span>
    <span className="course-tariff-description">{t.description || 'Добавьте описание тарифа'}</span>
  </label>)}</fieldset>
}
