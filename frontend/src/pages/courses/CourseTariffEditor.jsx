import Field from './CourseField'
import Reorder from './CourseReorder'

export default function CourseTariffEditor({ value, update, item, remove, moveItem }) {
  const tariffs = value.tariffs || []
  const multiple = value.pricing_mode === 'tariffs'
  function mode(next) {
    if (next === 'tariffs' && tariffs.length < 2) update('tariffs', [...tariffs, ...Array.from({ length: 2 - tariffs.length }, (_, i) => ({ id: 0, title: '', description: '', price: i === 0 ? value.price : 0, old_price: null, hours: value.hours || 0, recommended: false }))])
    update('pricing_mode', next)
  }
  return <><div className="course-pricing-mode" role="group" aria-label="Режим стоимости">{[['single', 'Одна цена'], ['tariffs', 'Несколько тарифов']].map(([key, title]) => <button type="button" key={key} aria-pressed={(value.pricing_mode || 'single') === key} onClick={() => mode(key)}>{title}</button>)}</div>
    {multiple && <div className="course-tariff-editor"><p className="course-muted">От 2 до 4 вариантов. Опишите отличия: что входит в каждый тариф и кому он подойдёт.</p>{tariffs.map((t, i) => <article key={t.id || i}><header><b>Тариф {i + 1}</b><Reorder deleteIcon index={i} count={tariffs.length} label={'тариф ' + (i + 1)} move={(a, b) => update('tariffs', moveItem(tariffs, a, b))} remove={tariffs.length > 2 ? () => remove('tariffs', i) : undefined} /></header><div className="course-tariff-fields"><Field label="Название тарифа" max={80} value={t.title} onChange={title => item('tariffs', i, { title })} placeholder="Например, С поддержкой" /><Field label="Цена тарифа, ₽" type="number" min="0" max="100000000" step="0.01" value={t.price / 100} onChange={v => item('tariffs', i, { price: Math.round(Number(v) * 100) })} /><Field label="Старая цена, ₽" type="number" min="0" max="100000000" step="0.01" value={t.old_price == null ? '' : t.old_price / 100} onChange={v => item('tariffs', i, { old_price: v === '' ? null : Math.round(Number(v) * 100) })} hint="Необязательно, выше текущей цены." /></div>
{t.old_price != null && t.old_price <= t.price && <p className="course-pricing-error" role="alert">Старая цена тарифа должна быть выше текущей.</p>}
<Field label="Примерно часов обучения" type="number" min="0" max="100000" step="0.5" value={t.hours ?? 0} onChange={v => item('tariffs', i, { hours: Number(v) })} hint="Время на уроки и практику в этом тарифе." />
<Field label="Описание тарифа" area max={300} value={t.description} onChange={description => item('tariffs', i, { description })} placeholder="Что получит участник на этом тарифе?" /><label className="course-tariff-recommend"><input type="checkbox" checked={t.recommended} onChange={e => update('tariffs', tariffs.map((tariff, j) => ({ ...tariff, recommended: j === i && e.target.checked })))} />Рекомендуемый — выбран первым</label></article>)}<button type="button" className="course-add" disabled={tariffs.length >= 4} onClick={() => update('tariffs', [...tariffs, { id: 0, title: '', description: '', price: 0, old_price: null, hours: value.hours || 0, recommended: false }])}>＋ Добавить тариф <small>{tariffs.length}/4</small></button></div>}
  </>
}
