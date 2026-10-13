import Icon from '../../components/Icon'
import Field from './CourseField'
import Reorder from './CourseReorder'
import IconSelect from '../../components/IconSelect'
import { money, benefitColors, selectedCourseFeatures, courseFeatureItems, CourseBonuses } from './shared'

export default function CoursePricingStep({ value, update, meta, item, remove, moveItem }) {
  const discounted = value.old_price > value.price
  const discount = discounted ? Math.round((1 - value.price / value.old_price) * 100) : 0
  const invalidOldPrice = value.old_price != null && !discounted
  const features = [...(meta.features || []), ...(value.features || []).filter(f => !(meta.features || []).some(m => m.id === f.id))]
  const selectedFeatures = selectedCourseFeatures(value, features)
  const customFeatures = value.custom_features || []
  const previewFeatures = courseFeatureItems(value, features)
  function toggleFeature(feature, checked) {
    const ids = value.feature_ids || selectedFeatures.map(f => f.id)
    update('feature_ids', checked ? [...ids, feature.id] : ids.filter(id => id !== feature.id))
    if (feature.key) update(feature.key, checked)
  }
  return <div className="course-pricing-layout">
    <div className="course-pricing-settings">
      <section className="course-pricing-section"><header><i><Icon name="calculator" /></i><div><h3>Цена вашего курса</h3><p>Укажите стоимость для участника. Старая цена нужна только для предложения со скидкой.</p></div></header>
        <div className="course-form-grid">
          <Field label="Текущая цена, ₽" type="number" min="0" max="100000000" step="0.01" value={value.price / 100} onChange={v => update('price', Math.round(Number(v) * 100))} hint="Именно эту сумму увидит покупатель." />
          <Field label="Старая цена, ₽" type="number" min="0" max="100000000" step="0.01" value={value.old_price == null ? '' : value.old_price / 100} onChange={v => update('old_price', v === '' ? null : Math.round(Number(v) * 100))} hint="Необязательно. Должна быть выше текущей цены." />
        </div>
        {invalidOldPrice && <p className="course-pricing-error" role="alert">Старая цена должна быть больше текущей. Измените её или очистите поле.</p>}
        {discounted && <div className="course-discount-result"><Icon name="check" /><span>Скидка <b>{discount}%</b><small>Участник экономит {money(value.old_price - value.price)}</small></span></div>}
      </section>
      <section className="course-pricing-section"><header><i><Icon name="clock" /></i><div><h3>Обучение и доступ</h3><p>Помогите участнику спланировать обучение и понять условия курса.</p></div></header>
        <div className="course-form-grid">
          <Field label="Примерно часов обучения" type="number" min="0" max="100000" step="0.5" value={value.hours} onChange={v => update('hours', Number(v))} hint="Общее время на уроки и самостоятельную работу." />
          <Field label="Срок доступа, месяцев" type="number" min="1" max="1200" value={value.access_months} onChange={v => update('access_months', Number(v))} hint="Как долго будут доступны материалы курса." />
        </div>
      </section>
      <section className="course-pricing-section"><header><i><Icon name="check" /></i><div><h3>Возможности курса</h3><p>Выберите то, что входит в обучение, и дополните список своими вариантами.</p></div></header><div className="course-toggle-grid">{features.map(f => <label key={f.id}><input type="checkbox" checked={selectedFeatures.some(s => s.id === f.id)} onChange={e => toggleFeature(f, e.target.checked)} /><span>{f.name}</span></label>)}</div>{!features.length && <p className="course-muted">В справочнике пока нет возможностей курса.</p>}
        <div className="course-custom-features"><div className="course-custom-features-heading"><h4>Ваши варианты</h4><span>{customFeatures.length}/10</span></div><p>Добавьте до 10 пунктов по 180 символов. Они появятся только в этом курсе.</p>
          {customFeatures.map((f, i) => <div className="course-custom-feature-row" key={f.id || `new-${i}`}><Field label={`Свой вариант ${i + 1}`} max={180} value={f.title} onChange={title => item('custom_features', i, { title })} placeholder="Например, проверка домашних заданий автором" /><button type="button" className="course-custom-feature-delete" title="Удалить вариант" aria-label={`Удалить свой вариант ${i + 1}`} onClick={() => remove('custom_features', i)}><Icon name="trash" /></button></div>)}
          <button type="button" className="course-add course-custom-feature-add" disabled={customFeatures.length >= 10} onClick={() => update('custom_features', [...customFeatures, { id: 0, title: '' }])}>＋ Добавить свой вариант</button>
        </div>
      </section>
      <section className="course-pricing-section"><header><i><Icon name="folder" /></i><div><h3>Бонусы при покупке</h3><p>Добавьте материалы и подарки, которые получит участник. Блок сразу появится справа.</p></div></header><div className="course-field course-bonus-palette"><span>Цвет фона блока</span><IconSelect label="Цвет фона бонусов" buttonText="Выбрать цвет" value={value.bonuses_color || ''} options={{ '': 'Автоматически', ...Object.fromEntries(Object.entries(benefitColors).map(([key, p]) => [key, p.name])) }} onChange={color => update('bonuses_color', color)} renderIcon={key => <span className="course-color-swatch" style={{ background: benefitColors[key]?.card || '#ffffff' }} />} /></div>{value.bonuses.map((b, i) => <div className="course-repeat-row" key={b.id || 'new-' + i}><Field label={'Бонус ' + (i + 1)} value={b.title} onChange={title => item('bonuses', i, { title })} placeholder="Например, библиотека промптов" /><Reorder index={i} count={value.bonuses.length} label={'бонус ' + (i + 1)} move={(a, b) => update('bonuses', moveItem(value.bonuses, a, b))} remove={() => remove('bonuses', i)} /></div>)}<button type="button" className="course-add" disabled={value.bonuses.length >= 30} onClick={() => update('bonuses', [...value.bonuses, { id: 0, title: '' }])}>＋ Добавить бонус</button></section>
    </div>
    <aside className="course-price-preview" aria-label="Предпросмотр стоимости курса"><small className="course-price-eyebrow">ВАШЕ ПРЕДЛОЖЕНИЕ</small><h3>{value.title || 'Ваш курс'}</h3><div className="course-price"><strong>{money(value.price)}</strong>{discounted && <><del>{money(value.old_price)}</del><span>−{discount}%</span></>}</div><p className="course-price-caption">Стоимость курса для одного участника</p><ul><li><Icon name="clock" /><span>Доступ на <b>{value.access_months} мес.</b></span></li><li><Icon name="list" /><span>Около <b>{value.hours} ч</b> обучения</span></li>{previewFeatures.map(f => <li key={f.id}><Icon name="check" /><span>{f.name}</span></li>)}</ul><CourseBonuses course={value} /><div className="course-price-preview-note">Цена и условия появятся на странице курса. Продажи откроются после подключения оплаты.</div></aside>
  </div>
}
