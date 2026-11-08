import { createPortal } from 'react-dom'
import PublishSuccessModal from '../../components/PublishSuccessModal'

export default function CourseInterestModal({ course, onClose, requiresLogin = false }) {
  const next = `${window.location.pathname}${window.location.search}`
  return createPortal(<PublishSuccessModal
    className={`course-interest-success${requiresLogin ? ' is-auth' : ''}`}
    eyebrow={requiresLogin ? 'ДАВАЙТЕ ПОЗНАКОМИМСЯ' : 'ВАШ СЛЕДУЮЩИЙ ШАГ К НОВЫМ ЗНАНИЯМ'}
    title={requiresLogin ? 'Войдите, чтобы оставить заявку' : 'Спасибо за интерес к курсу!'}
    description={requiresLogin ? 'Войдите в аккаунт или зарегистрируйтесь, чтобы автор курса получил вашу заявку и смог связаться с вами.' : 'Ваша заявка передана автору курса. Он свяжется с вами, чтобы рассказать об обучении и ответить на ваши вопросы.'}
    wishTitle={course.title}
    wishText={requiresLogin ? 'После входа вернитесь к курсу и подтвердите выбор. До этого заявка не отправляется.' : 'Рады, что вы выбираете развиваться вместе с FinTalent.'}
    primaryText={requiresLogin ? 'Войти или зарегистрироваться' : 'Отлично, спасибо!'}
    primaryOnClick={requiresLogin ? undefined : onClose}
    primaryHref={requiresLogin ? `/login?next=${encodeURIComponent(next)}` : undefined}
    secondaryText={requiresLogin ? 'Вернуться к курсу' : null}
    footerNote="В будущем оплата будет производиться прямо на сайте FinTalent."
    containFocus
    onClose={onClose}
  />, document.body)
}
