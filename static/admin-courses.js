(() => {
  const nav = document.querySelector('.sidebar nav'), workspace = document.querySelector('.workspace')
  if (!nav || !workspace) return
  const button = document.createElement('button')
  button.id = 'courses-admin-nav'; button.textContent = '▤ Курсы'; nav.append(button)
  const section = document.createElement('section'); section.id = 'admin-courses'; section.className = 'hidden'; workspace.append(section)
  const labels = { draft: 'Черновик', moderation: 'На модерации', published: 'Опубликован', hidden: 'Скрыт' }
  let selected = 'moderation'
  const request = async (path, body) => {
    const response = await fetch(path, { credentials: 'include', cache: 'no-store', ...(body ? { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) } : {}) })
    const value = await response.json(); if (!response.ok) throw Error(value.error || 'Не удалось выполнить запрос'); return value
  }
  const node = (tag, text, className) => { const el = document.createElement(tag); if (text) el.textContent = text; if (className) el.className = className; return el }
  async function load() {
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
          actionButton.onclick = async () => { if (!confirm(`${label} курс «${c.title}»?`)) return; actionButton.disabled = true; try { await request(`/api/admin/courses/${c.id}/${action}`, {}); await load() } catch (e) { notify(e.message, true); actionButton.disabled = false } }
          actions.append(actionButton)
        }
        row.append(actions); body.append(row)
      }
      table.append(body); const wrap = node('div', '', 'users-table-wrap'); wrap.append(table); section.append(wrap)
      if (!body.children.length) section.append(node('p', 'В этой категории пока нет курсов.'))
    } catch (e) { section.replaceChildren(node('p', e.message)) }
  }
  button.onclick = () => {
    document.querySelectorAll('.workspace>section,.dictionary-list,.dictionary-editor').forEach(el => el.classList.add('hidden'))
    section.classList.remove('hidden'); document.querySelectorAll('.sidebar nav button').forEach(el => el.classList.remove('active')); button.classList.add('active')
    document.querySelectorAll('.workspace>header .primary').forEach(el => el.classList.add('hidden'))
    document.querySelector('.workspace>header h1').textContent = 'Курсы'
    document.querySelector('.workspace>header p').textContent = 'Проверка страниц перед публикацией. Категории курсов доступны в общих справочниках.'
    load()
  }
  nav.addEventListener('click', e => { if (e.target.closest('button')?.id !== button.id) section.classList.add('hidden') })
})()
