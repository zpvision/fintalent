import { useId, useState } from 'react'
import Icon from '../../components/Icon'
import { money } from './shared'

export const initialTariff = course => Math.max(0, (course.tariffs || []).findIndex(t => t.recommended))
export const hasTariffs = course => course.pricing_mode === 'tariffs' && course.tariffs?.length > 0

export default function CourseTariffs({ course, selected, onSelect }) {
  const name = useId(), [local, setLocal] = useState(initialTariff(course))
  const [animating, setAnimating] = useState(null)
  const index = selected ?? local
  return <fieldset className="course-tariffs"><legend>Выберите тариф</legend>{(course.tariffs || []).map((t, i) => <label className={`course-tariff${index === i ? ' selected' : ''}${animating === i && index === i ? ' is-selecting' : ''}`} onAnimationEnd={e => { if (e.target === e.currentTarget) setAnimating(current => current === i ? null : current) }} key={t.id || i}>
    {t.recommended && <small className="course-tariff-badge">Рекомендуем</small>}
    <span className="course-tariff-heading"><input type="radio" name={name} checked={index === i} onChange={() => { setAnimating(i); setLocal(i); onSelect?.(i) }} /><b>{t.title || 'Название тарифа'}</b><span className="course-tariff-prices"><strong>{money(t.price)}</strong>{t.old_price > t.price && <span><del>{money(t.old_price)}</del><small>−{Math.round((1 - t.price / t.old_price) * 100)}%</small></span>}</span></span>
    <span className="course-tariff-hours"><Icon name="clock" />{t.hours || 0} ч обучения</span>
    <span className="course-tariff-description">{t.description || 'Добавьте описание тарифа'}</span>
  </label>)}</fieldset>
}
