(() => {
  const nav = document.querySelector('.sidebar nav'), workspace = document.querySelector('.workspace')
  if (!nav || !workspace) return
  document.head.insertAdjacentHTML('beforeend', '<link rel="stylesheet" href="/static/admin-community.css?v=1">')
  nav.insertAdjacentHTML('beforeend', '<small>СООБЩЕСТВО</small><button id="community-nav">♧ <span>Сообщество</span></button>')
  workspace.insertAdjacentHTML('beforeend', '<section id="admin-community" class="admin-community hidden"><div class="community-head"><div><h2>Бухгалтерские компании</h2><p>Компании, опубликованные в открытом каталоге сообщества.</p></div><b id="community-count">0</b></div><label class="community-search"><span>⌕</span><input type="search" placeholder="Поиск по компании, городу или владельцу"></label><div id="community-list" class="community-list"><div class="community-loading">Загружаем компании…</div></div></section>')

  const section = document.querySelector('#admin-community'), list = document.querySelector('#community-list'), search = section.querySelector('input')
  let timer
  const escape = value => { const node = document.createElement('span'); node.textContent = value ?? ''; return node.innerHTML }
  const date = value => value ? new Date(value).toLocaleDateString('ru-RU') : '—'
  async function request(url, options = {}) {
    const response = await fetch(url, { cache: 'no-store', ...options })
    const data = await response.json().catch(() => ({}))
    if (!response.ok) throw Error(data.error || 'Ошибка запроса')
    return data
  }
  function activate() {
    document.querySelectorAll('.workspace>section,.dictionary-list,.dictionary-editor').forEach(element => element.classList.add('hidden'))
    section.classList.remove('hidden')
    document.querySelectorAll('.sidebar nav button').forEach(button => button.classList.remove('active'))
    document.querySelector('#community-nav').classList.add('active')
    document.querySelector('.workspace>header h1').textContent = 'Сообщество'
    document.querySelector('.workspace>header p').textContent = 'Управление опубликованными бухгалтерскими компаниями'
    document.querySelectorAll('.workspace>header .primary').forEach(button => button.classList.add('hidden'))
    load()
  }
  async function load() {
    list.innerHTML = '<div class="community-loading">Загружаем компании…</div>'
    try {
      const query = search.value.trim(), data = await request('/api/admin/community/accounting-companies' + (query ? `?q=${encodeURIComponent(query)}` : '')), items = data.items || []
      document.querySelector('#community-count').textContent = items.length
      if (!items.length) {
        list.innerHTML = '<div class="community-empty"><i>♧</i><b>Опубликованных компаний не найдено</b><span>Измените запрос или дождитесь публикации новых страниц.</span></div>'
        return
      }
      list.innerHTML = `<div class="community-table"><table><thead><tr><th>Компания</th><th>Владелец</th><th>Город</th><th>Наполнение</th><th>Опубликована</th><th></th></tr></thead><tbody>${items.map(company => `<tr><td><div class="community-company"><i>${company.logo ? `<img src="${escape(company.logo)}" alt="">` : escape((company.name || '?').charAt(0))}</i><span><b>${escape(company.name)}</b><small>${company.verified ? '✓ Подтверждена' : 'Опубликована'}</small></span></div></td><td><b>${escape(company.owner_name)}</b><small>${escape(company.owner_email)}</small></td><td>${escape(company.city || 'Не указан')}</td><td><span class="community-metrics">${company.directions_count} напр. · ${company.services_count} услуг</span></td><td>${date(company.published_at)}</td><td><div class="community-actions"><a href="/accounting-companies/view?slug=${encodeURIComponent(company.slug || '')}" target="_blank" rel="noopener">Открыть</a><button type="button" data-archive="${company.id}" data-name="${escape(company.name)}">Снять с публикации</button></div></td></tr>`).join('')}</tbody></table></div>`
      list.querySelectorAll('[data-archive]').forEach(button => button.onclick = () => archiveCompany(button))
    } catch (error) {
      list.innerHTML = `<div class="community-empty"><i>!</i><b>Не удалось загрузить компании</b><span>${escape(error.message)}</span></div>`
    }
  }
  async function archiveCompany(button) {
    if (!confirm(`Снять компанию «${button.dataset.name}» с публикации? Она исчезнет из открытого каталога.`)) return
    button.disabled = true
    button.textContent = 'Снимаем…'
    try {
      await request(`/api/admin/community/accounting-companies/${button.dataset.archive}/archive`, { method: 'POST' })
      if (typeof notify === 'function') notify('Компания снята с публикации')
      await load()
    } catch (error) {
      if (typeof notify === 'function') notify(error.message, true)
      button.disabled = false
      button.textContent = 'Снять с публикации'
    }
  }
  search.addEventListener('input', () => { clearTimeout(timer); timer = setTimeout(load, 300) })
  document.querySelector('#community-nav').onclick = activate
  document.querySelectorAll('.sidebar nav button').forEach(button => button.addEventListener('click', () => { if (button.id !== 'community-nav') section.classList.add('hidden') }))
})()
