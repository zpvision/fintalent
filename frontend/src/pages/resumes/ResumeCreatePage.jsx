import { useEffect, useState } from 'react'
import { useDocumentPage } from '../../hooks/useDocumentPage'
import usePageStyles from '../../hooks/usePageStyles'

function loadScript(src, ready) {
  if (ready?.()) return Promise.resolve(null)
  return new Promise((resolve, reject) => {
    const script = document.createElement('script')
    script.src = src; script.dataset.reactResumeCreate = 'true'
    script.onload = () => resolve(script)
    script.onerror = () => reject(new Error('Не удалось загрузить мастер создания резюме'))
    document.head.append(script)
  })
}

export default function ResumeCreatePage() {
  useDocumentPage({ title: 'Создание резюме — FinTalent' })
  usePageStyles(['/static/resume-create.css?v=2', '/static/wizard-step-progress.css?v=4'])
  const [error, setError] = useState('')
  useEffect(() => {
    let cancelled = false
    const loaded = []
    async function start() {
      try {
        const picker = await loadScript('/static/duty-picker.js?v=4', () => window.DutyPicker)
        if (picker) loaded.push(picker)
        if (cancelled) return
        const controller = await loadScript('/static/resume-create.js?v=25')
        if (controller) loaded.push(controller)
      } catch (loadError) { if (!cancelled) setError(loadError.message) }
    }
    start()
    return () => {
      cancelled = true; loaded.forEach((script) => script.remove()); document.body.removeAttribute('data-step')
      document.querySelectorAll('.resume-help-modal,.resume-experience-duty-modal,.publish-success-modal').forEach((element) => element.remove())
    }
  }, [])
  return <><header className="resume-header"><a className="resume-brand" href="/"><span>F</span><b>Fin <em>Talent</em></b></a><div className="header-title"><b>Создание резюме</b><small>Быстрое создание</small></div><a className="cancel" href="/">× Отменить</a><button className="help">? Нужна помощь?</button></header>
    <main className="resume-layout"><section className="quiz"><div className="progress-head wizard-progress-head"><div className="wizard-progress-main"><div id="resume-step-track" className="wizard-step-track" aria-label="Прогресс создания резюме" /><div className="wizard-progress-copy"><b id="step-number">Шаг 1 из 7</b><strong id="step-title">Загрузка…</strong></div></div></div><div className="question"><h1 id="question-title">Кем вы хотите работать?</h1><p id="question-description">Выберите одну или несколько должностей, которые наиболее точно отражают вашу цель.</p><div id="position-options" className="position-options"><div className={`loading${error ? ' error' : ''}`}>{error || 'Загружаем варианты…'}</div></div><div className="quiz-note">▷ Результат можно будет отредактировать в любой момент</div><div className="navigation"><button id="back-step" className="back-step hidden">← Назад</button><button id="continue" className="continue">Продолжить <span>→</span></button></div></div></section>
      <aside className="profile-progress"><h2>Ваш профиль</h2><p>Заполненность резюме</p><b><span id="percent">0</span>%</b><div className="bar"><i id="bar" /></div><div className="tip">✣ <span>Мы зададим несколько простых вопросов и создадим сильное резюме</span></div><hr /><h3>Что будет в вашем резюме</h3><ul><li>▷ Опыт работы и навыки</li><li>⌘ Программы и сервисы</li><li>▤ Участки учета</li><li>◫ Отрасли и клиенты</li><li>♙ Обязанности</li><li>♢ Тестирование и подтверждения</li></ul><div className="secure">♢ <span>Ваши данные защищены<br />и не будут переданы третьим лицам</span></div></aside>
    </main><footer className="resume-footer">◷ Среднее время создания резюме: 5–7 минут</footer></>
}
