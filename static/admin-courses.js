(() => {
  const nav = document.querySelector('.sidebar nav'), workspace = document.querySelector('.workspace')
  if (!nav || !workspace) return
  const button = document.createElement('button')
  button.id = 'courses-admin-nav'; const navLabel = document.createElement('span'); navLabel.textContent = 'Курсы'; button.append(document.createTextNode('▤ '), navLabel); nav.append(button)
  const section = document.createElement('section'); section.id = 'admin-courses'; section.className = 'hidden'; workspace.insertBefore(section, document.querySelector('#dictionary-editor'))
  const labels = { draft: 'Черновик', moderation: 'На модерации', published: 'Опубликован', hidden: 'Скрыт' }
  let selected = 'moderation'
  let area = 'access'
  const request = async (path, body) => {
    const response = await fetch(path, { credentials: 'include', cache: 'no-store', ...(body ? { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) } : {}) })
    const value = await response.json(); if (!response.ok) throw Error(value.error || 'Не удалось выполнить запрос'); return value
  }
  const node = (tag, text, className) => { const el = document.createElement(tag); if (text) el.textContent = text; if (className) el.className = className; return el }
  async function loadCourses() {
    section.replaceChildren(node('p', 'Загружаем курсы…'))
    try {
      const items = await request('/api/admin/courses')
      section.replaceChildren()
      const filters = node('div', '', 'pm-admin-tabs')
      for (const [key, label] of Object.entries({ all: 'Все курсы', ...labels })) { const tab = node('button', label, selected === key ? 'active' : ''); tab.onclick = () => { selected = key; load() }; filters.append(tab) }
      section.append(filters)
      const table = node('table', '', 'users-table'), head = node('thead'), heading = node('tr')
      for (const label of ['Курс', 'Автор', 'Статус', 'Изменён', 'Действия']) heading.append(node('th', label))
      head.append(heading); table.append(head)
      const body = node('tbody')
      for (const c of items.filter(c => selected === 'all' || c.status === selected)) {
        const row = node('tr'); row.append(node('td', c.title || 'Новый курс'), node('td', c.author_name), node('td', labels[c.status]), node('td', new Date(c.updated_at).toLocaleDateString('ru-RU')))
        const actions = node('td'), preview = node('a', 'Предпросмотр'); preview.href = `/admin/courses/${c.id}/preview`; preview.target = '_blank'; preview.rel = 'noopener'; actions.append(preview)
        for (const [action, label] of c.status === 'moderation' ? [['approve', 'Опубликовать'], ['reject', 'Вернуть в черновики']] : c.status === 'published' ? [['hide', 'Скрыть']] : []) {
          const actionButton = node('button', label, 'secondary'); actionButton.style.margin = '5px'
          actionButton.onclick = async () => { if (!confirm(`${label} курс «${c.title}»?`)) return; const reason=prompt('Причина изменения (не менее 5 символов)');if(!reason||reason.trim().length<5){notify('Укажите причину от 5 символов',true);return;} actionButton.disabled = true; try { await request(`/api/admin/courses/${c.id}/${action}`, {reason:reason.trim(),confirm:true,revision:c.revision}); await load() } catch (e) { notify(e.message, true); actionButton.disabled = false } }
          actions.append(actionButton)
        }
        row.append(actions); body.append(row)
      }
      table.append(body); const wrap = node('div', '', 'users-table-wrap'); wrap.append(table); section.append(wrap)
      if (!body.children.length) section.append(node('p', 'В этой категории пока нет курсов.'))
    } catch (e) { section.replaceChildren(node('p', e.message)) }
  }
  async function loadAccess() {
    section.replaceChildren(tabs(),node('p','Загружаем настройку доступа…'))
    try {
      const settings=await api('/api/admin/course-access')
      section.replaceChildren(tabs())
      const card=node('section','','course-access-settings'),title=node('h2','Доступ к курсам')
      const label=node('label','','course-access-toggle'),checkbox=document.createElement('input');checkbox.type='checkbox';checkbox.checked=settings.public_access
      label.append(checkbox,node('span','Открыть курсы всем'))
      const description=node('p',settings.public_access?'Курсы доступны всем пользователям и посетителям сайта.':'Курсы доступны только выбранным пользователям и администраторам.')
      const hint=node('p','Индивидуальный доступ включается в разделе «Пользователи», в колонке «Курсы». При открытии всем индивидуальные разрешения сохраняются, но больше не ограничивают доступ.')
      checkbox.onchange=async()=>{checkbox.disabled=true;try{const result=await api('/api/admin/course-access',{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({public_access:checkbox.checked})});settings.public_access=result.public_access;description.textContent=result.public_access?'Курсы доступны всем пользователям и посетителям сайта.':'Курсы доступны только выбранным пользователям и администраторам.';notify('Настройка доступа сохранена')}catch(e){checkbox.checked=settings.public_access;notify(e.message,true)}finally{checkbox.disabled=false}}
      card.append(title,label,description,hint);section.append(card)
    } catch(e){section.replaceChildren(tabs(),node('p',e.message))}
  }
  function tabs() {
    const tabs = node('div', '', 'pm-admin-tabs')
    for (const [key, label] of [['access', 'Доступ'], ['courses', 'Курсы и модерация'], ['course_categories', 'Категории курсов'], ['course_features', 'Возможности курса'], ['course_lesson_types', 'Типы уроков']]) {
      const tab = node('button', label, key === area ? 'active' : '')
      tab.onclick = () => { area = key; editor.classList.add('hidden'); load() }
      tabs.append(tab)
    }
    return tabs
  }
  function renderCourseDictionaries() {
    section.replaceChildren(tabs())
    const cards = node('div', '', 'dictionary-list courses-dictionary-list')
    for (const d of dictionaries.filter(d => d.alias === area)) {
      const card = node('button', '', 'dictionary-card')
      card.type = 'button'; card.append(node('h2', d.name), node('p', `${d.items.length} вариантов`), node('small', 'Создать, изменить, отключить или удалить варианты →'))
      card.onclick = () => openDictionary(d.id)
      cards.append(card)
    }
    section.append(cards)
  }
  async function load() {
    list.classList.add('hidden'); editor.classList.add('hidden')
    if (area === 'access') { await loadAccess(); return }
    if (area === 'courses') { await loadCourses(); section.prepend(tabs()); return }
    try { dictionaries = await api('/api/admin/dictionaries'); renderCourseDictionaries() } catch (e) { notify(e.message, true) }
  }
  const originalRenderList = renderList
  renderList = function () {
    originalRenderList()
    if (button.classList.contains('active') && !['courses','access'].includes(area)) { list.classList.add('hidden'); renderCourseDictionaries() }
  }
  document.querySelector('#back').addEventListener('click', event => {
    if (!button.classList.contains('active') || area === 'courses') return
    event.preventDefault(); event.stopImmediatePropagation(); renderList()
  }, true)
  const originalOpenDictionary = openDictionary
  openDictionary = function (id) {
    originalOpenDictionary(id)
    const managed = ['course_categories', 'course_features', 'course_lesson_types'].includes(current?.alias)
    editor.classList.toggle('course-dictionary-editor', managed)
    editor.classList.toggle('course-lesson-type-editor', current?.alias === 'course_lesson_types')
    document.querySelector('#back').textContent = managed ? '← К справочникам курсов' : '← Все справочники'
    editor.querySelector('.item-columns span:nth-child(5)').textContent = current?.alias === 'course_lesson_types' ? 'Иконка и активность' : managed ? 'Активность' : 'Иконка'
    aliasInput.disabled = managed
    document.querySelector('#delete-dictionary').classList.toggle('hidden', managed)
    for (const el of document.querySelectorAll('.dictionary-meta .vacancy-title-option,.dictionary-meta .vacancy-importance-option')) el.classList.toggle('hidden', managed)
    if (button.classList.contains('active')) section.querySelector('.courses-dictionary-list')?.classList.add('hidden')
  }
  button.onclick = () => {
    document.querySelectorAll('.workspace>section,.dictionary-list,.dictionary-editor').forEach(el => el.classList.add('hidden'))
    section.classList.remove('hidden'); document.querySelectorAll('.sidebar nav button').forEach(el => el.classList.remove('active')); button.classList.add('active')
    document.querySelectorAll('.workspace>header .primary').forEach(el => el.classList.add('hidden'))
    document.querySelector('.workspace>header h1').textContent = 'Курсы'
    document.querySelector('.workspace>header p').textContent = 'Модерация курсов и управление категориями и возможностями обучения.'
    load()
  }
  nav.addEventListener('click', e => { if (e.target.closest('button')?.id !== button.id) section.classList.add('hidden') })
})()
