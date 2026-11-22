(() => {
  const nav = document.querySelector('.sidebar nav'), workspace = document.querySelector('.workspace')
  if (!nav || !workspace || document.querySelector('#errors-nav')) return
  const button = document.createElement('button'); button.id = 'errors-nav'; button.type = 'button'; button.textContent = '⚠ '; button.title = 'Ошибки сайта'; button.setAttribute('aria-label', 'Ошибки сайта')
  const buttonText = document.createElement('span'); buttonText.textContent = 'Ошибки сайта'; button.append(buttonText); nav.append(button)
  const section = document.createElement('section'); section.id = 'error-journal-section'; section.className = 'hidden'; workspace.append(section)
  const controls = document.createElement('div'); controls.style.cssText = 'display:flex;gap:12px;flex-wrap:wrap;align-items:center;margin:20px 0'
  const label = document.createElement('label'); label.textContent = 'Период: '
  const period = document.createElement('select'); period.setAttribute('aria-label', 'Период отчёта'); period.style.cssText = 'min-height:44px;max-width:100%;font:inherit'
  for (const [value, text] of [[1, 'Час'], [24, 'Сутки'], [168, '7 дней'], [336, '14 дней']]) { const option = document.createElement('option'); option.value = value; option.textContent = text; period.append(option) }
  period.value = '24'; label.append(period)
  const refresh = document.createElement('button'); refresh.type = 'button'; refresh.className = 'secondary'; refresh.textContent = 'Обновить'
  const download = document.createElement('a'); download.className = 'secondary'; download.textContent = 'Скачать JSON'; download.href = '/api/admin/diagnostics?hours=24&download=1'; download.style.cssText = 'display:inline-flex;align-items:center;min-height:44px;text-decoration:none;font:inherit'
  refresh.style.cssText = 'min-height:44px;font:inherit'
  controls.append(label, refresh, download)
  const summary = document.createElement('p'); summary.setAttribute('role', 'status')
  const content = document.createElement('div'); section.append(controls, summary, content)
  const names = { backend: 'Сервер', frontend: 'Браузер', worker: 'Фоновая задача', startup: 'Запуск' }
  const formatTime = value => new Date(value).toLocaleString('ru-RU')
  async function loadErrors() {
    refresh.disabled = true; summary.textContent = 'Загружаем журнал…'
    download.href = `/api/admin/diagnostics?hours=${period.value}&download=1`
    try {
      const report = await api(`/api/admin/diagnostics?hours=${period.value}`)
      summary.textContent = `Событий: ${report.events}. Групп: ${report.groups.length}. Журнал: ${(report.stored_bytes / 1048576).toFixed(1)} из ${(report.max_bytes / 1048576).toFixed(0)} МБ. Хранение до ${report.retention_days} дней. Пропущено с запуска: ${report.dropped_since_start}. Ошибок записи: ${report.write_errors_since_start}.${report.truncated ? ' Показаны не все группы — сократите период.' : ''}`
      content.replaceChildren()
      if (!report.groups.length) { const empty = document.createElement('p'); empty.textContent = 'За выбранный период зарегистрированных ошибок нет.'; content.append(empty); return }
      const wrap = document.createElement('div'); wrap.className = 'users-table-wrap'
      const table = document.createElement('table'); table.className = 'users-table'
      const head = document.createElement('thead'), headers = document.createElement('tr')
      for (const title of ['Последний случай', 'Источник', 'Ошибка', 'Повторов', 'Контекст']) { const th = document.createElement('th'); th.textContent = title; headers.append(th) }
      head.append(headers); table.append(head)
      const body = document.createElement('tbody')
      for (const group of report.groups) {
        const event = group.sample, row = document.createElement('tr')
        for (const value of [formatTime(group.last), names[event.source] || event.source, `${event.code} · ${event.operation || event.route || ''}`, group.count]) { const cell = document.createElement('td'); cell.textContent = value; row.append(cell) }
        const cell = document.createElement('td'), details = document.createElement('details'), title = document.createElement('summary'), pre = document.createElement('pre')
        title.textContent = `Затронуто пользователей: ${group.affected_users || 0}${group.more_users ? '+' : ''} · последний: ${event.user_id || 'не определён'}`
        pre.textContent = JSON.stringify({ first: group.first, last: group.last, ...event }, null, 2); pre.style.cssText = 'white-space:pre-wrap;overflow-wrap:anywhere;max-width:480px'
        details.append(title, pre); cell.append(details); row.append(cell); body.append(row)
      }
      table.append(body); wrap.append(table); content.append(wrap)
    } catch (error) { summary.textContent = error.message || 'Не удалось загрузить журнал' }
    finally { refresh.disabled = false }
  }
  button.onclick = () => {
    if (window.__academyOnly) return
    nav.querySelectorAll('button').forEach(item => item.classList.toggle('active', item === button))
    workspace.querySelectorAll(':scope > section').forEach(item => item.classList.toggle('hidden', item !== section))
    const header = workspace.querySelector(':scope > header')
    header.querySelector('h1').textContent = 'Ошибки сайта'; header.querySelector('p').textContent = 'Сбои браузера, сервера и фоновых задач'
    header.querySelectorAll('.primary').forEach(item => item.classList.add('hidden'))
    section.classList.remove('hidden'); void loadErrors()
  }
  nav.addEventListener('click', event => { const target = event.target.closest('button'); if (target && target !== button) section.classList.add('hidden') }, true)
  refresh.onclick = loadErrors; period.onchange = loadErrors
})()
