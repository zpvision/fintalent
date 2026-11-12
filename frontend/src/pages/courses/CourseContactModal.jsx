import { useEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import Icon from '../../components/Icon'
import { money } from './shared'

export default function CourseContactModal({ user, course, tariff, busy, error, onClose, onSubmit }) {
 const ref=useRef(null),[email,setEmail]=useState(user.email || ''),[phone,setPhone]=useState(user.phone || '')
 useEffect(()=>{const dialog=ref.current,previous=document.activeElement,overflow=document.body.style.overflow;document.body.style.overflow='hidden';dialog.showModal();return()=>{dialog.close();document.body.style.overflow=overflow;if(previous?.isConnected)previous.focus()}},[])
 const price=tariff ? tariff.price : course.price
 function submit(event) {
  event.preventDefault()
  if (busy) return
  const input=event.currentTarget.elements.namedItem('contact_phone')
  const digits=phone.replace(/\D/g,'')
  input.setCustomValidity(digits.length>=7 && digits.length<=15 && /^\+?[\d ()-]+$/.test(phone.trim()) ? '' : 'Укажите телефон: от 7 до 15 цифр')
  if (event.currentTarget.reportValidity()) onSubmit({contact_email:email.trim(),contact_phone:phone.trim()})
 }
 return createPortal(<dialog ref={ref} className="course-contact-dialog" aria-labelledby="course-contact-title" onCancel={e=>{e.preventDefault();if(!busy)onClose()}} onClick={e=>{if(e.target===e.currentTarget&&!busy)onClose()}}>
  <button type="button" className="course-contact-close" aria-label="Закрыть" disabled={busy} onClick={onClose}>×</button>
  <header><span className="course-contact-mark"><Icon name="book" /></span><small>ВАШ СЛЕДУЮЩИЙ ШАГ</small><h2 id="course-contact-title">Начните путь к новым знаниям</h2><p>{course.title}{tariff && <> · {tariff.title}</>}</p></header>
  <form onSubmit={submit}>
   <label className="course-field">ФИО<input value={user.full_name || ''} disabled autoComplete="name" /></label>
   <section className="course-contact-details"><div className="course-contact-heading"><Icon name="message" /><div><h3>Как с вами связаться?</h3><p>Автор курса свяжется с вами, чтобы обсудить обучение и оплату.</p></div></div>
    <div className="course-contact-fields"><label className="course-field">Телефон<input name="contact_phone" type="tel" autoComplete="tel" inputMode="tel" placeholder="+7 (999) 123-45-67" value={phone} onChange={e=>{e.target.setCustomValidity('');setPhone(e.target.value)}} required maxLength={60} disabled={busy}/></label>
    <label className="course-field">Email<input type="email" autoComplete="email" placeholder="you@example.ru" value={email} onChange={e=>setEmail(e.target.value)} required maxLength={254} disabled={busy}/></label></div>
   </section>
   {error && <p className="course-error" role="alert">{error}</p>}
   <button className="course-primary" type="submit" disabled={busy}>{busy?'Отправляем заявку…':`Выбрать курс · ${money(price)}`}</button>
   <small className="course-contact-note">Сейчас оплата проходит через автора курса. Деньги с вашей карты не списываются.</small>
  </form>
 </dialog>,document.body)
}
