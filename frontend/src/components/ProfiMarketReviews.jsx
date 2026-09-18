import { useEffect, useState } from 'react'
import { getProfiMarketReviews, saveProfiMarketReview } from '../api/profimarket'

const dateText = value => new Date(value).toLocaleDateString('ru-RU', { day: 'numeric', month: 'long', year: 'numeric' })

export default function ProfiMarketReviews({ solution, onChanged }) {
  const [data, setData] = useState({ reviews: [], can_review: false })
  const [rating, setRating] = useState(5), [comment, setComment] = useState('')
  const [busy, setBusy] = useState(false), [error, setError] = useState(''), [saved, setSaved] = useState(false)

  async function load(signal) {
    const value = await getProfiMarketReviews(solution.id, signal ? { signal } : undefined)
    setData(value)
    const mine = value.reviews?.find(item => item.is_mine)
    if (mine) { setRating(mine.rating); setComment(mine.comment || '') }
    return value
  }

  useEffect(() => {
    const controller = new AbortController()
    setError('')
    load(controller.signal).catch(e => { if (e.name !== 'AbortError') setError(e.message) })
    return () => controller.abort()
  }, [solution.id])

  async function submit(event) {
    event.preventDefault(); setBusy(true); setError(''); setSaved(false)
    try {
      await saveProfiMarketReview(solution.id, rating, comment)
      const value = await load(), reviews = value.reviews || []
      onChanged?.(reviews.length ? reviews.reduce((sum, item) => sum + Number(item.rating), 0) / reviews.length : 0, reviews.length)
      setSaved(true)
    } catch (e) { setError(e.message) } finally { setBusy(false) }
  }

  const reviews = data.reviews || []
  const mine = reviews.find(item => item.is_mine)
  return <section className="pm-customer-reviews" id="reviews">
    <header className="pm-reviews-heading">
      <span className="pm-reviews-mark">★</span>
      <div><small>МНЕНИЯ ПОКУПАТЕЛЕЙ</small><h2>Отзывы о решении</h2><p>{reviews.length ? `${reviews.length} отзывов от пользователей FinTalent` : 'Здесь появятся впечатления тех, кто уже использует решение.'}</p></div>
    </header>
    <form className={`pm-review-composer${data.can_review ? '' : ' locked'}`} onSubmit={submit}>
      <div className="pm-review-composer-copy"><b>{mine ? 'Обновите свой отзыв' : 'Поделитесь впечатлением'}</b><span>{data.can_review ? 'Ваш опыт поможет другим специалистам принять решение.' : 'Отправка отзыва будет доступна после покупки решения.'}</span></div>
      <div className="pm-review-inline-stars" aria-label="Оценка решения">{[1,2,3,4,5].map(value => <button type="button" disabled={!data.can_review} className={value<=rating?'active':''} onClick={() => setRating(value)} aria-label={`${value} из 5`} key={value}>★</button>)}</div>
      <label><span className="sr-only">Текст отзыва</span><textarea disabled={!data.can_review} maxLength="2000" rows="4" value={comment} onChange={event => setComment(event.target.value)} placeholder={data.can_review ? 'Расскажите, что вам понравилось и как решение помогло в работе…' : 'Напишите здесь о своём опыте после покупки…'} /></label>
      <footer><span>{saved ? 'Спасибо! Отзыв опубликован.' : data.can_review ? 'Можно изменить отзыв в любое время' : 'Только подтверждённые покупатели могут оставлять отзывы'}</span><button disabled={!data.can_review || busy}>{busy ? 'Сохраняем…' : mine ? 'Сохранить изменения' : 'Опубликовать отзыв'}</button></footer>
      {error && <div className="pm-review-error">{error}</div>}
    </form>
    {reviews.length ? <div className="pm-review-list">{reviews.map(item => <article key={item.id}><div className="pm-review-avatar">{item.author_avatar ? <img src={item.author_avatar} alt="" /> : (item.author_name || 'П').charAt(0)}</div><div><header><span><b>{item.author_name || 'Пользователь FinTalent'}</b><small>{dateText(item.created_at)}</small></span><strong>{'★'.repeat(item.rating)}<i>{'★'.repeat(5-item.rating)}</i></strong></header>{item.comment && <p>{item.comment}</p>}{item.is_mine && <em>Ваш отзыв</em>}</div></article>)}</div> : <div className="pm-review-empty"><span>☆</span><b>Отзывов пока нет</b><p>После покупки вы сможете первым поделиться опытом использования.</p></div>}
  </section>
}
