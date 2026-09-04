import { useDocumentPage } from '../../hooks/useDocumentPage'
import usePageStyles from '../../hooks/usePageStyles'
import PublicLayout from '../../layouts/PublicLayout'

const benefits = [
  ['01', 'Делитесь практическим опытом', 'Создавайте задания на основе реальных рабочих ситуаций и профессиональных задач.'],
  ['02', 'Используйте разные форматы вопросов', 'Добавляйте одиночный и множественный выбор, текстовые ответы, последовательности и кейсы.'],
  ['03', 'Формируйте профессиональное портфолио', 'Опубликованные тесты показывают вашу экспертизу работодателям и коллегам.'],
  ['04', 'Получайте отзывы', 'Пользователи делятся впечатлениями и помогают сделать ваши материалы ещё лучше.'],
  ['05', 'Повышайте рейтинг автора', 'Качественные тесты чаще появляются в рекомендациях и привлекают новую аудиторию.'],
  ['06', 'Помогайте компаниям с наймом', 'Работодатели могут прикреплять ваш тест к вакансии и предлагать его подходящим соискателям.'],
]

export default function MarketplaceCreateTestPage() {
  useDocumentPage({ title: 'Создавайте тесты — FinTalent' })
  usePageStyles(['/static/marketplace-create-test.css?v=1', '/static/marketplace-create-test-compact.css?v=1', '/static/marketplace-create-test-list.css?v=1', '/static/marketplace-create-test-font.css?v=1'])
  return <PublicLayout><main className="creator-page">
    <a className="creator-back" href="/marketplace">← Вернуться в маркетплейс</a>
    <section className="creator-hero"><span className="creator-label">СТАНЬТЕ АВТОРОМ FINTALENT</span><div className="creator-icon">▷</div><h1>Превратите опыт<br />в профессиональный тест</h1><p>Помогайте специалистам подтверждать знания, а компаниям — находить сильных кандидатов.</p></section>
    <section className="creator-benefits">
      {benefits.map(([number, title, description]) => <article key={number}><i>{number}</i><div><h2>{title}</h2><p>{description}</p></div></article>)}
      <article className="income-benefit"><i>07</i><div><span>МОНЕТИЗАЦИЯ</span><h2>Зарабатывайте на своих знаниях.<br />Вы сами устанавливаете цену</h2><p>Пользователи могут проходить тесты самостоятельно или ваш тест могут выбирать для соискателей вакансий.</p></div></article>
      <article className="commission-benefit"><i>08</i><div><h2>Комиссия сервиса — 27%</h2><p>В комиссию входят размещение, приём оплаты, техническая инфраструктура и продвижение внутри FinTalent.</p></div></article>
    </section>
    <section className="income-example"><div className="income-example-head"><span>₽</span><div><small>ПРИМЕР ДОХОДА</small><h2>Сколько можно заработать?</h2></div></div><p>Представьте: ваш тест стоит <b>300 рублей</b>, его прошли <b>100 человек</b>.</p><div className="income-math"><div><small>Выручка за тест</small><strong>300 × 100 = 30 000 ₽</strong></div><span>−</span><div><small>Комиссия FinTalent, 27%</small><strong>8 100 ₽</strong></div><span>=</span><div className="income-total"><small>Ваш заработок</small><strong>21 900 ₽</strong></div></div><p className="income-smile">Неплохо? :)</p></section>
    <section className="creator-start"><p>Создайте свой первый тест и поделитесь знаниями уже сегодня</p><a href="/tests">Начать! <span>→</span></a></section>
  </main></PublicLayout>
}
