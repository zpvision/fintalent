import Icon from '../../components/Icon'
import Field from './CourseField'
import { money } from './shared'

export default function CoursePricingStep({ value, update }) {
  const discounted = value.old_price > value.price
  const discount = discounted ? Math.round((1 - value.price / value.old_price) * 100) : 0
  const invalidOldPrice = value.old_price != null && !discounted
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
    </div>
    <aside className="course-price-preview" aria-label="Предпросмотр стоимости курса"><small className="course-price-eyebrow">ВАШЕ ПРЕДЛОЖЕНИЕ</small><h3>{value.title || 'Ваш курс'}</h3><div className="course-price"><strong>{money(value.price)}</strong>{discounted && <><del>{money(value.old_price)}</del><span>−{discount}%</span></>}</div><p className="course-price-caption">Стоимость курса для одного участника</p><ul><li><Icon name="clock" /><span>Доступ на <b>{value.access_months} мес.</b></span></li><li><Icon name="list" /><span>Около <b>{value.hours} ч</b> обучения</span></li>{value.updates && <li><Icon name="replace" /><span>Обновления включены</span></li>}{value.certificate && <li><Icon name="check" /><span>Сертификат после завершения</span></li>}</ul><div className="course-price-preview-note">Цена и условия появятся на странице курса. Продажи откроются после подключения оплаты.</div></aside>
  </div>
}
