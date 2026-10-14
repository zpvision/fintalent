import '../../../../static/profimarket-style-presets.js'
import Icon from '../../components/Icon'

export const levels = { beginner: 'Для начинающих', intermediate: 'Средний уровень', advanced: 'Продвинутый' }
export const statuses = { draft: 'Черновик', moderation: 'На модерации', published: 'Опубликован', hidden: 'Скрыт' }
export const types = { video: 'Видео', text: 'Текст', practice: 'Практическое задание', test: 'Тест', materials: 'Материалы' }
export const benefitIcons = { sparkles: 'Технологии', clock: 'Время', folder: 'Материалы', check: 'Практика', users: 'Команда', calculator: 'Расчёты', workflow: 'Процессы', bot: 'ИИ', list: 'Знания', message: 'Общение', book: 'Обучение', target: 'Цели', award: 'Достижения', chart: 'Аналитика', shield: 'Надёжность', briefcase: 'Карьера', globe: 'Возможности', lightbulb: 'Идеи', video: 'Видеоуроки', rocket: 'Развитие' }
export const benefitColors = Object.fromEntries(window.ProfiMarketStylePresets.presets.filter(p => p.key !== 'metrics-default').map(p => [p.key, p]))
export const filledBenefit = b => Boolean(b.title?.trim() || b.description?.trim())
export function withBenefitSlots(course) { const benefits = [...(course.benefits || [])]; while (benefits.length < 4) benefits.push({ id: 0, icon: 'sparkles', color: '', title: '', description: '' }); return { ...course, benefits, custom_features: course.custom_features || [] } }
export const money = value => `${new Intl.NumberFormat('ru-RU', { maximumFractionDigits: 2 }).format((value || 0) / 100)} ₽`
export const duration = minutes => minutes >= 60 ? `${Math.floor(minutes / 60)} ч ${minutes % 60 ? `${minutes % 60} мин` : ''}` : `${minutes} мин`
export const date = value => value ? new Date(value).toLocaleDateString('ru-RU') : ''
export const catalogPrice = course => `${course.pricing_mode === 'tariffs' ? 'от ' : ''}${money(course.min_price ?? course.price)}`
export const styles = ['/static/course-tariffs.css?v=2', '/static/courses.css?v=4']
export const profileStyles = ['/static/profile.css', '/static/profile-sidebar-v2.css?v=2', '/static/profile-buttons.css', '/static/profile-layout-stability.css?v=2', ...styles]
export function Benefits({ items, showEmpty = false }) { return <div className="course-benefits">{items.filter(b => showEmpty || filledBenefit(b)).map((b, i) => { const palette = benefitColors[b.color]; return <article key={b.id || i}><i style={palette ? { background: palette.iconBg, color: palette.icon } : undefined}><Icon name={b.icon} /></i><h3>{b.title || 'Название преимущества'}</h3><p>{b.description || (showEmpty ? 'Незаполненный блок не появится на странице курса.' : '')}</p></article> })}</div> }
export function Avatar({ person }) { return person.avatar ? <img className="course-avatar" src={person.avatar} alt="" /> : <span className="course-avatar">{person.name?.charAt(0) || 'А'}</span> }
export function EmptyArt() { return <div className="course-empty-art" aria-hidden="true"><span className="course-art-orbit" /><div className="course-art-book"><Icon name="list" /><b>ВАШИ ЗНАНИЯ</b><span>Новые возможности</span><i>✦</i></div><span className="course-art-play">▶</span><span className="course-art-check">✓</span><span className="course-art-spark">✧</span></div> }

export const selectedCourseFeatures = (course, options = course.features || []) => options.filter(f => course.feature_ids ? course.feature_ids.includes(f.id) : f.key && course[f.key])
export const courseFeatureItems = (course, options) => [...selectedCourseFeatures(course, options), ...(course.custom_features || []).filter(f => f.title?.trim()).map((f, i) => ({ id: `custom-${f.id || i}`, name: f.title.trim(), key: '' }))]
export function CourseBonuses({ course }) { const palette = benefitColors[course.bonuses_color], bonuses = (course.bonuses || []).filter(b => b.title?.trim()); return bonuses.length > 0 && <div className="course-bonuses" style={palette ? { background: palette.card, color: palette.text, borderColor: palette.border } : undefined}><h3>✧ Бонусы при покупке</h3>{bonuses.map((b, i) => <p key={b.id || i}><span>✓</span>{b.title}</p>)}</div> }
