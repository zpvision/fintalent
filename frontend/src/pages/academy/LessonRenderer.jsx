import { useEffect, useState } from 'react'
import Icon from '../../components/Icon'
import { academyAPI } from '../../api/academy'

export const blockTypes = { text: ['Текст', 'book'], video: ['Видео', 'video'], image: ['Изображение', 'globe'], file: ['Файл', 'folder'], link: ['Ссылка', 'globe'], note: ['Примечание', 'lightbulb'], test: ['Тест FinTalent', 'check'], assignment: ['Практическое задание', 'briefcase'] }
export const noteTypes = { info: 'Информация', tip: 'Совет', important: 'Важно', warning: 'Предупреждение', example: 'Пример' }
const safeURL = value => { try { const u = new URL(value); return ['https:', 'http:', 'mailto:'].includes(u.protocol) ? value : undefined } catch { return undefined } }
// Render an allowlist into React nodes; never insert author HTML into the page.
export function Inline({ html = '' }) {
  const doc = new DOMParser().parseFromString(html, 'text/html')
  function render(node, key) {
    if (node.nodeType === 3) return node.textContent
    if (node.nodeType !== 1 || ['SCRIPT','STYLE','IFRAME','OBJECT'].includes(node.tagName)) return null
    const children = [...node.childNodes].map((child, i) => render(child, i)), tag = node.tagName.toLowerCase()
    if (tag === 'a') return <a key={key} href={safeURL(node.getAttribute('href'))} target="_blank" rel="noopener noreferrer">{children}</a>
    if (tag === 'br') return <br key={key} />
    const Tag = ['b', 'strong', 'i', 'em', 'u', 'code', 'mark'].includes(tag) ? tag : 'span'
    return <Tag key={key}>{children}</Tag>
  }
  return [...doc.body.childNodes].map((node, i) => render(node, i))
}
function Rich({ blocks = [] }) { return blocks.map((b, i) => {
  if (b.type === 'table') return <div className="academy-table-wrap" key={i}><table><tbody>{b.rows?.map((row, j) => <tr key={j}>{row.map((cell, k) => <td key={k}><Inline html={cell} /></td>)}</tr>)}</tbody></table></div>
  if (['bullets', 'numbered', 'checklist'].includes(b.type)) { const List = b.type === 'numbered' ? 'ol' : 'ul'; return <List key={i}>{b.items?.map((item, j) => <li key={j}><Inline html={item} /></li>)}</List> }
  if (b.type === 'divider') return <hr key={i} />
  const Tag = ({ h2: 'h2', h3: 'h3', quote: 'blockquote', code: 'pre' })[b.type] || 'p'
  return <Tag key={i}><Inline html={b.text} /></Tag>
}) }
export function videoSource(value) {
  try {
    const u = new URL(value); if (u.protocol !== 'https:' || u.username) return null
    const host = u.hostname, parts = u.pathname.split('/').filter(Boolean), valid = id => /^[\w-]+$/.test(id || '')
    if (host === 'youtu.be' && valid(parts[0])) return ['iframe', `https://www.youtube-nocookie.com/embed/${parts[0]}`]
    if (['youtube.com','www.youtube.com'].includes(host)) { const id = u.searchParams.get('v') || (parts[0] === 'embed' && parts[1]); if (valid(id)) return ['iframe', `https://www.youtube-nocookie.com/embed/${id}`] }
    if (['vimeo.com','player.vimeo.com'].includes(host) && /^\d+$/.test(parts.at(-1))) return ['iframe', `https://player.vimeo.com/video/${parts.at(-1)}`]
    if (['rutube.ru','www.rutube.ru'].includes(host) && ['video','play'].includes(parts[0]) && valid(parts.at(-1))) return ['iframe', `https://rutube.ru/play/embed/${parts.at(-1)}`]
    if (/\.(mp4|webm)$/i.test(u.pathname)) return ['video', u.href]
  } catch {}
  return null
}
export function Video({ data, courseID }) {
  const [playing, setPlaying] = useState(false)
  useEffect(() => setPlaying(false), [data.url, data.poster_id])
  const source = videoSource(data.url)
  if (!source) return <div className="academy-media-empty"><Icon name="video" /><span>Добавьте ссылку на YouTube, Vimeo, Rutube или MP4/WebM</span></div>
  return <figure className="academy-video">{source[0] === 'video' ? <video controls preload="metadata" src={source[1]} poster={data.poster_id ? academyAPI.assetURL(courseID, data.poster_id) : undefined} /> : data.poster_id && !playing ? <button type="button" className="academy-video-poster" onClick={() => setPlaying(true)} aria-label="Показать видеоплеер"><img src={academyAPI.assetURL(courseID, data.poster_id)} alt="Постер видео" /><span>▶</span></button> : <iframe src={source[1]} title={data.title || 'Видео урока'} loading="lazy" allow="fullscreen; picture-in-picture" allowFullScreen referrerPolicy="strict-origin-when-cross-origin" />}{data.title && <figcaption>{data.title}{data.duration_minutes > 0 && ` · ${data.duration_minutes} мин`}</figcaption>}</figure>
}
export const fileSize = size => size >= 1048576 ? `${(size / 1048576).toFixed(1)} МБ` : `${Math.ceil((size || 0) / 1024)} КБ`
export function AssetLink({ id, courseID, assets = [] }) {
  const asset = assets.find(a => a.id === id)
  return <a className="academy-asset" href={academyAPI.assetURL(courseID, id, true)}><Icon name="folder" /><span><b>{asset?.name || 'Учебный материал'}</b><small>{asset ? `${asset.mime.split('/').at(-1)} · ${fileSize(asset.size)}` : 'Скачать файл'}</small></span><span aria-hidden="true">↓</span></a>
}
export default function LessonRenderer({ lesson, courseID, assets = [], tests = [] }) {
  return <article className="academy-lesson-renderer"><header>{lesson.cover_asset_id > 0 && <img className="academy-lesson-cover" src={academyAPI.assetURL(courseID, lesson.cover_asset_id)} alt="" />}<small>ПРЕДПРОСМОТР УРОКА · {lesson.duration_minutes || 0} МИН</small><h1>{lesson.title || 'Новый урок'}</h1>{lesson.description && <p>{lesson.description}</p>}</header>
    {lesson.blocks.map(b => { const d = b.data; return <section key={b.key} className={`academy-render-block type-${b.type}`}>
      {b.type === 'text' && <Rich blocks={d.rich} />}
      {b.type === 'video' && <Video data={d} courseID={courseID} />}
      {b.type === 'image' && (d.asset_id ? <figure><img src={academyAPI.assetURL(courseID, d.asset_id)} alt={d.caption || ''} />{d.caption && <figcaption>{d.caption}</figcaption>}</figure> : <p className="course-muted">Изображение ещё не добавлено</p>)}
      {b.type === 'file' && (d.asset_id ? <AssetLink id={d.asset_id} {...{ courseID, assets }} /> : <p className="course-muted">Файл ещё не добавлен</p>)}
      {b.type === 'link' && <div className="academy-render-link"><Icon name="globe" /><a href={safeURL(d.url)} target="_blank" rel="noopener noreferrer">{d.title || d.url || 'Ссылка'}</a>{d.text && <p>{d.text}</p>}</div>}
      {b.type === 'note' && <aside className={`academy-callout ${d.variant}`}><Icon name={d.variant === 'warning' ? 'shield' : 'lightbulb'} /><div><b>{d.title || noteTypes[d.variant]}</b><p>{d.text}</p></div></aside>}
      {b.type === 'test' && <div className="academy-future"><Icon name="check" /><h3>{tests.find(t => t.id === d.test_version_id)?.title || 'Тест FinTalent'}</h3><p>Тест прикреплён к уроку. Прохождение в учебной комнате появится на следующем этапе.</p></div>}
      {b.type === 'assignment' && <div className="academy-assignment"><small>ПРАКТИКА</small><h3>{d.title || 'Практическое задание'}</h3><p>{d.text}</p><Rich blocks={d.rich} />{d.material_ids?.map(id => <AssetLink key={id} id={id} {...{ courseID, assets }} />)}<p className="course-muted">Формат ответа: {({ text: 'текст', file: 'файл', link: 'ссылка' })[d.answer_format]}. Сдача и проверка работ появятся на следующем этапе.</p></div>}
    </section> })}
    {!lesson.blocks.length && <div className="academy-empty"><img src="/static/academy/program-start.png" alt="" /><h2>Содержимое урока ещё готовится</h2><p>Добавьте первый блок в редакторе.</p></div>}
  </article>
}
