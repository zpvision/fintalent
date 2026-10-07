import Icon from '../../components/Icon'

export const levels = { beginner: 'Для начинающих', intermediate: 'Средний уровень', advanced: 'Продвинутый' }
export const statuses = { draft: 'Черновик', moderation: 'На модерации', published: 'Опубликован', hidden: 'Скрыт' }
export const types = { video: 'Видео', text: 'Текст', practice: 'Практическое задание', test: 'Тест', materials: 'Материалы' }
export const benefitIcons = { sparkles: 'Технологии', clock: 'Время', folder: 'Материалы', check: 'Практика', users: 'Команда', calculator: 'Расчёты', workflow: 'Процессы', bot: 'ИИ', list: 'Знания', message: 'Общение' }
export const money = value => `${new Intl.NumberFormat('ru-RU', { maximumFractionDigits: 2 }).format((value || 0) / 100)} ₽`
export const duration = minutes => minutes >= 60 ? `${Math.floor(minutes / 60)} ч ${minutes % 60 ? `${minutes % 60} мин` : ''}` : `${minutes} мин`
export const date = value => value ? new Date(value).toLocaleDateString('ru-RU') : ''
export const styles = ['/static/courses.css?v=1']
export const profileStyles = ['/static/profile.css', '/static/profile-sidebar-v2.css?v=2', '/static/profile-buttons.css', '/static/profile-layout-stability.css?v=2', ...styles]
export function Benefits({ items }) { return <div className="course-benefits">{items.map((b, i) => <article key={b.id || i}><i><Icon name={b.icon} /></i><h3>{b.title || 'Название преимущества'}</h3><p>{b.description || 'Расскажите, чему научится участник'}</p></article>)}</div> }
export function Avatar({ person }) { return person.avatar ? <img className="course-avatar" src={person.avatar} alt="" /> : <span className="course-avatar">{person.name?.charAt(0) || 'А'}</span> }
export function EmptyArt() { return <div className="course-empty-art" aria-hidden="true"><span className="course-art-orbit" /><div className="course-art-book"><Icon name="list" /><b>ВАШИ ЗНАНИЯ</b><span>Новые возможности</span><i>✦</i></div><span className="course-art-play">▶</span><span className="course-art-check">✓</span><span className="course-art-spark">✧</span></div> }
