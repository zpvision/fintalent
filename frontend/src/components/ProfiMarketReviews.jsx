import { useEffect, useState } from 'react'
import { answerProfiMarketQuestion, askProfiMarketQuestion, getProfiMarketQuestions, getProfiMarketReviews, saveProfiMarketReview } from '../api/profimarket'

const dateText = value => new Date(value).toLocaleDateString('ru-RU', { day: 'numeric', month: 'long', year: 'numeric' })

function Reviews({ solution, onChanged }) {
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
  return <div className="pm-discussion-panel" id="reviews">
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
  </div>
}

function AnswerForm({ item, onSaved }) {
  const [answer, setAnswer] = useState(item.answer || ''), [busy, setBusy] = useState(false), [error, setError] = useState('')
  async function submit(event) {
    event.preventDefault(); setBusy(true); setError('')
    try { await answerProfiMarketQuestion(item.id, answer); await onSaved() } catch (e) { setError(e.message) } finally { setBusy(false) }
  }
  return <form className="pm-answer-form" onSubmit={submit}><label><span>{item.answer ? 'Изменить ответ' : 'Ответить как автор решения'}</span><textarea minLength="2" maxLength="4000" required value={answer} onChange={event => setAnswer(event.target.value)} placeholder="Дайте понятный и полезный ответ…" /></label><footer>{error ? <span className="pm-review-error">{error}</span> : <span>Ответ будет виден всем посетителям карточки</span>}<button disabled={busy}>{busy ? 'Сохраняем…' : item.answer ? 'Обновить ответ' : 'Опубликовать ответ'}</button></footer></form>
}

function Questions({ solution }) {
  const [data, setData] = useState({ questions: [], authenticated: false, is_author: false })
  const [question, setQuestion] = useState(''), [busy, setBusy] = useState(false), [error, setError] = useState(''), [saved, setSaved] = useState(false)
  async function load(signal) { const value = await getProfiMarketQuestions(solution.id, signal ? { signal } : undefined); setData(value); return value }
  useEffect(() => { const controller = new AbortController(); setError(''); load(controller.signal).catch(e => { if (e.name !== 'AbortError') setError(e.message) }); return () => controller.abort() }, [solution.id])
  async function submit(event) {
    event.preventDefault(); setBusy(true); setError(''); setSaved(false)
    try { await askProfiMarketQuestion(solution.id, question); setQuestion(''); setSaved(true); await load() } catch (e) { setError(e.message) } finally { setBusy(false) }
  }
  const questions = data.questions || []
  return <div className="pm-discussion-panel" id="questions">
    <header className="pm-reviews-heading"><span className="pm-reviews-mark pm-question-mark">?</span><div><small>СВЯЗЬ С АВТОРОМ</small><h2>Вопрос–Ответ</h2><p>{questions.length ? `${questions.length} ${questions.length === 1 ? 'вопрос' : 'вопросов'} о решении` : 'Уточните детали до покупки — автор ответит прямо здесь.'}</p></div></header>
    {data.authenticated ? <form className="pm-review-composer pm-question-composer" onSubmit={submit}><div className="pm-review-composer-copy"><b>Задайте вопрос автору</b><span>Спросите о составе, совместимости, внедрении или условиях получения.</span></div><label><span className="sr-only">Ваш вопрос</span><textarea minLength="10" maxLength="2000" required value={question} onChange={event => setQuestion(event.target.value)} placeholder="Например: подойдёт ли решение для нашей конфигурации и входит ли помощь с настройкой?" /></label><footer><span>{saved ? 'Вопрос отправлен. Автор получил уведомление.' : 'Ваш вопрос и ответ автора будут видны на странице'}</span><button disabled={busy}>{busy ? 'Отправляем…' : 'Задать вопрос'}</button></footer>{error && <div className="pm-review-error">{error}</div>}</form> : <div className="pm-question-login"><span>?</span><div><b>Есть вопрос по решению?</b><p>Войдите в FinTalent, чтобы задать его автору.</p></div><a href={`/login?next=${encodeURIComponent(location.pathname + location.search + '#questions')}`}>Войти и спросить</a></div>}
    {questions.length ? <div className="pm-question-list">{questions.map(item => <article key={item.id}><div className="pm-question-head"><div className="pm-review-avatar">{item.author_avatar ? <img src={item.author_avatar} alt="" /> : (item.author_name || 'П').charAt(0)}</div><span><b>{item.author_name || 'Пользователь FinTalent'}</b><small>{dateText(item.created_at)}{item.is_mine ? ' · ваш вопрос' : ''}</small></span></div><p>{item.question}</p>{item.answer && <div className="pm-author-answer"><small>ОТВЕТ АВТОРА</small><p>{item.answer}</p>{item.answered_at && <time>{dateText(item.answered_at)}</time>}</div>}{data.is_author && <AnswerForm item={item} onSaved={() => load()} />}</article>)}</div> : <div className="pm-review-empty"><span>?</span><b>Вопросов пока нет</b><p>Задайте первый вопрос — он поможет и другим покупателям.</p></div>}
  </div>
}

export default function ProfiMarketReviews({ solution, onChanged }) {
  const initial = window.location.hash === '#questions' ? 'questions' : 'reviews'
  const [tab, setTab] = useState(initial)
  useEffect(() => { const change = () => { if (window.location.hash === '#questions') setTab('questions') }; window.addEventListener('hashchange', change); return () => window.removeEventListener('hashchange', change) }, [])
  function select(value) { setTab(value); window.history.replaceState(null, '', value === 'questions' ? '#questions' : '#reviews') }
  return <section className="pm-customer-reviews"><nav className="pm-discussion-tabs" aria-label="Обсуждение решения"><button className={tab === 'reviews' ? 'active' : ''} onClick={() => select('reviews')}>Отзывы</button><button className={tab === 'questions' ? 'active' : ''} onClick={() => select('questions')}>Вопрос–Ответ</button></nav>{tab === 'reviews' ? <Reviews solution={solution} onChanged={onChanged} /> : <Questions solution={solution} />}</section>
}
