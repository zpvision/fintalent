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
  const clear = document.createElement('button'); clear.type = 'button'; clear.id = 'clear-errors'; clear.className = 'secondary'; clear.textContent = 'Удалить все ошибки'; clear.style.cssText = 'min-height:44px;font:inherit;color:#b42318'
  const zone = Intl.DateTimeFormat().resolvedOptions().timeZone || 'местное время'
  const dates = ['С', 'По'].map(text => {
    const label = document.createElement('label'); label.textContent = text + ' (' + zone + '): '
    const input = document.createElement('input'); input.type = 'datetime-local'; input.step = '1'; input.setAttribute('aria-label', text + ' (' + zone + ')')
    input.style.cssText = 'min-height:44px;max-width:100%;min-width:0;font:inherit;box-sizing:border-box'
    label.style.cssText = 'max-width:100%;min-width:0'; label.append(input); return { label, input }
  })
  const [since, until] = dates.map(item => item.input)
  controls.append(label, ...dates.map(item => item.label), refresh, download, clear)
  const summary = document.createElement('p'); summary.setAttribute('role', 'status')
  const content = document.createElement('div'); section.append(controls, summary, content)
  const names = { backend: 'Сервер', frontend: 'Браузер', worker: 'Фоновая задача', startup: 'Запуск' }
  const formatTime = value => new Date(value).toLocaleString('ru-RU')
  let loadID = 0
  let clearing = false
  async function loadErrors() {
    if (clearing) return
    const currentLoad = ++loadID
    download.removeAttribute('href')
    refresh.disabled = true; summary.textContent = 'Загружаем журнал…'
    try {
      const params = new URLSearchParams({ hours: period.value })
      for (const [name, input] of [['since', since], ['until', until]]) {
        if (!input.validity.valid) throw Error('Проверьте дату и время')
        if (input.value) params.set(name, new Date(input.value).toISOString())
      }
      const report = await api(`/api/admin/diagnostics?${params}`)
      if (currentLoad !== loadID) return
      params.set('since', report.since); params.set('until', report.until)
      download.href = `/api/admin/diagnostics?${params}&download=1`
      summary.textContent = `Событий: ${report.events}. Групп: ${report.groups.length}. Журнал: ${(report.stored_bytes / 1048576).toFixed(1)} из ${(report.max_bytes / 1048576).toFixed(0)} МБ. Хранение до ${report.retention_days} дней. Пропущено с запуска: ${report.dropped_since_start}. Ошибок записи: ${report.write_errors_since_start}.${report.truncated ? ' Показаны не все группы — сократите период.' : ''}`
      summary.textContent += ` Интервал: ${formatTime(report.since)} — ${formatTime(report.until)} (${zone}).`
      content.replaceChildren()
      if (!report.groups.length) { const empty = document.createElement('p'); empty.textContent = 'За выбранный период зарегистрированных ошибок нет.'; content.append(empty); return }
      for (const category of ['application', 'external_resource']) {
        const groups = report.groups.filter(group => (group.category || 'application') === category)
        const heading = document.createElement('h2'); heading.textContent = `${category === 'application' ? 'Ошибки приложения' : 'Внешние ресурсы'}: ${groups.reduce((sum, group) => sum + group.count, 0)}`; content.append(heading)
        if (!groups.length) continue
        const wrap = document.createElement('div'); wrap.className = 'users-table-wrap'
        const table = document.createElement('table'); table.className = 'users-table'
        const head = document.createElement('thead'), headers = document.createElement('tr')
        for (const title of ['Последний случай', 'Источник', 'Ошибка', 'Повторов', 'Контекст']) { const th = document.createElement('th'); th.textContent = title; headers.append(th) }
        head.append(headers); table.append(head)
        const body = document.createElement('tbody')
        for (const group of groups) {
          const event = group.sample, row = document.createElement('tr')
          for (const value of [formatTime(group.last), names[event.source] || event.source, `${event.code} · ${event.operation || event.route || ''}`, group.count]) { const cell = document.createElement('td'); cell.textContent = value; row.append(cell) }
          const cell = document.createElement('td'), details = document.createElement('details'), title = document.createElement('summary'), pre = document.createElement('pre')
          title.textContent = `Затронуто пользователей: ${group.affected_users || 0}${group.more_users ? '+' : ''} · последний: ${event.user_id || 'не определён'}`
          pre.textContent = JSON.stringify({ first: group.first, last: group.last, ...event, version_mismatch: Boolean(event.client_release && event.release && event.client_release !== event.release) }, null, 2); pre.style.cssText = 'white-space:pre-wrap;overflow-wrap:anywhere;max-width:480px'
          details.append(title, pre); cell.append(details); row.append(cell); body.append(row)
        }
        table.append(body); wrap.append(table); content.append(wrap)
      }
    } catch (error) { if (currentLoad === loadID) summary.textContent = error.message || 'Не удалось загрузить журнал' }
    finally { if (currentLoad === loadID) refresh.disabled = false }
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
  refresh.onclick = loadErrors
  clear.onclick = async () => {
    if (clearing || !confirm('Удалить все ошибки за все периоды, включая ошибки внешних ресурсов? Новые ошибки продолжат записываться.')) return
    clearing = true; ++loadID
    clear.disabled = refresh.disabled = period.disabled = since.disabled = until.disabled = true
    download.removeAttribute('href')
    summary.textContent = 'Удаляем все ошибки…'
    try {
      await api('/api/admin/diagnostics', { method: 'DELETE' })
      content.replaceChildren()
      since.value = ''; until.value = ''
      clearing = false
      await loadErrors()
    } catch (error) { summary.textContent = error.message || 'Не удалось удалить ошибки' }
    finally {
      clearing = false
      clear.disabled = refresh.disabled = period.disabled = since.disabled = until.disabled = false
    }
  }
  period.onchange = () => { since.value = ''; until.value = ''; void loadErrors() }
  since.onchange = until.onchange = loadErrors
})()
