import '../../../../static/profimarket-style-presets.js'
import Icon from '../../components/Icon'

export const levels = { beginner: 'Для начинающих', intermediate: 'Средний уровень', advanced: 'Продвинутый' }
export const statuses = { draft: 'Черновик', moderation: 'На модерации', published: 'Опубликован', hidden: 'Скрыт' }
export const types = { video: 'Видео', text: 'Текст', practice: 'Практическое задание', test: 'Тест', materials: 'Материалы' }
export const benefitIcons = { sparkles: 'Технологии', clock: 'Время', folder: 'Материалы', check: 'Практика', users: 'Команда', calculator: 'Расчёты', workflow: 'Процессы', bot: 'ИИ', list: 'Знания', message: 'Общение' }
export const benefitColors = Object.fromEntries(window.ProfiMarketStylePresets.presets.filter(p => p.key !== 'metrics-default').map(p => [p.key, p]))
export const filledBenefit = b => Boolean(b.title?.trim() || b.description?.trim())
export function withBenefitSlots(course) { const benefits = [...(course.benefits || [])]; while (benefits.length < 4) benefits.push({ id: 0, icon: 'sparkles', color: '', title: '', description: '' }); return { ...course, benefits } }
export const money = value => `${new Intl.NumberFormat('ru-RU', { maximumFractionDigits: 2 }).format((value || 0) / 100)} ₽`
export const duration = minutes => minutes >= 60 ? `${Math.floor(minutes / 60)} ч ${minutes % 60 ? `${minutes % 60} мин` : ''}` : `${minutes} мин`
export const date = value => value ? new Date(value).toLocaleDateString('ru-RU') : ''
export const styles = ['/static/courses.css?v=1']
export const profileStyles = ['/static/profile.css', '/static/profile-sidebar-v2.css?v=2', '/static/profile-buttons.css', '/static/profile-layout-stability.css?v=2', ...styles]
export function Benefits({ items, showEmpty = false }) { return <div className="course-benefits">{items.filter(b => showEmpty || filledBenefit(b)).map((b, i) => { const palette = benefitColors[b.color]; return <article key={b.id || i}><i style={palette ? { background: palette.iconBg, color: palette.icon } : undefined}><Icon name={b.icon} /></i><h3>{b.title || 'Название преимущества'}</h3><p>{b.description || (showEmpty ? 'Незаполненный блок не появится на странице курса.' : '')}</p></article> })}</div> }
export function Avatar({ person }) { return person.avatar ? <img className="course-avatar" src={person.avatar} alt="" /> : <span className="course-avatar">{person.name?.charAt(0) || 'А'}</span> }
export function EmptyArt() { return <div className="course-empty-art" aria-hidden="true"><span className="course-art-orbit" /><div className="course-art-book"><Icon name="list" /><b>ВАШИ ЗНАНИЯ</b><span>Новые возможности</span><i>✦</i></div><span className="course-art-play">▶</span><span className="course-art-check">✓</span><span className="course-art-spark">✧</span></div> }
