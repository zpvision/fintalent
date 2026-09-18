import { useEffect, useState } from 'react'
import { getProfiMarketReviews, saveProfiMarketReview } from '../api/profimarket'

const dateText = value => new Date(value).toLocaleDateString('ru-RU', { day: 'numeric', month: 'long', year: 'numeric' })

export default function ProfiMarketReviews({ solution, onChanged }) {
  const [data, setData] = useState({ reviews: [], can_review: false })
  const [open, setOpen] = useState(false), [rating, setRating] = useState(5), [comment, setComment] = useState('')
  const [busy, setBusy] = useState(false), [error, setError] = useState('')
  async function load(signal) {
    const value = await getProfiMarketReviews(solution.id, signal ? { signal } : undefined)
    setData(value)
    const mine = value.reviews?.find(item => item.is_mine)
    if (mine) { setRating(mine.rating); setComment(mine.comment || '') }
    return value
  }
  useEffect(() => { const controller = new AbortController(); load(controller.signal).catch(e => { if (e.name !== 'AbortError') setError(e.message) }); return () => controller.abort() }, [solution.id])
  async function submit(event) {
    event.preventDefault(); setBusy(true); setError('')
    try {
      await saveProfiMarketReview(solution.id, rating, comment)
      const value = await load(), reviews = value.reviews || []
      onChanged?.(reviews.length ? reviews.reduce((sum, item) => sum + Number(item.rating), 0) / reviews.length : 0, reviews.length)
      setOpen(false)
    } catch (e) { setError(e.message) } finally { setBusy(false) }
  }
  const mine = data.reviews?.find(item => item.is_mine)
  return <section className="pm-customer-reviews" id="reviews"><header><div><small>МНЕНИЯ ПОКУПАТЕЛЕЙ</small><h2>Отзывы о решении</h2><p>{data.reviews.length ? `${data.reviews.length} отзывов от пользователей FinTalent` : 'Отзывов пока нет. Вы можете стать первым.'}</p></div>{data.can_review && <button type="button" onClick={() => setOpen(true)}>{mine ? 'Изменить отзыв' : 'Оставить отзыв'}</button>}</header>{error && !open && <div className="pm-review-error">{error}</div>}<div className="pm-review-list">{data.reviews.map(item => <article key={item.id}><div className="pm-review-avatar">{item.author_avatar ? <img src={item.author_avatar} alt="" /> : (item.author_name || 'П').charAt(0)}</div><div><header><span><b>{item.author_name || 'Пользователь FinTalent'}</b><small>{dateText(item.created_at)}</small></span><strong>{'★'.repeat(item.rating)}<i>{'★'.repeat(5-item.rating)}</i></strong></header>{item.comment && <p>{item.comment}</p>}{item.is_mine && <em>Ваш отзыв</em>}</div></article>)}</div>{open && <div className="pm-review-modal" onMouseDown={event => { if (event.target === event.currentTarget) setOpen(false) }}><form onSubmit={submit}><button className="pm-review-close" type="button" onClick={() => setOpen(false)}>×</button><small>ВАШЕ МНЕНИЕ</small><h2>{mine ? 'Изменить отзыв' : 'Оценить решение'}</h2><p>Поделитесь опытом использования — это поможет другим пользователям.</p><div className="pm-review-stars">{[1,2,3,4,5].map(value => <button type="button" className={value<=rating?'active':''} onClick={() => setRating(value)} aria-label={`${value} из 5`} key={value}>★</button>)}</div><label>Комментарий <span>необязательно</span><textarea maxLength="2000" rows="5" value={comment} onChange={event => setComment(event.target.value)} placeholder="Что вам понравилось? Как решение помогло в работе?" /></label>{error && <div className="pm-review-error">{error}</div>}<footer><button type="button" onClick={() => setOpen(false)}>Отмена</button><button className="primary" disabled={busy}>{busy?'Сохраняем…':'Опубликовать отзыв'}</button></footer></form></div>}</section>
}
