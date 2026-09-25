(() => {
  const nav = document.querySelector('.sidebar nav'), workspace = document.querySelector('.workspace')
  if (!nav || !workspace) return
  document.head.insertAdjacentHTML('beforeend', '<link rel="stylesheet" href="/static/admin-community.css?v=2">')
  nav.insertAdjacentHTML('beforeend', '<small>СООБЩЕСТВО</small><button id="community-nav">♧ <span>Сообщество</span></button>')
  workspace.insertAdjacentHTML('beforeend', `<section id="admin-community" class="admin-community hidden"><div class="community-tabs"><button type="button" data-community-tab="companies">Компании</button><button type="button" data-community-tab="vacancies">Вакансии</button><button type="button" data-community-tab="profiles">Профили</button></div><div class="community-head"><div><h2 id="community-title"></h2><p id="community-description"></p></div><b id="community-count">0</b></div><label class="community-search"><span>⌕</span><input type="search"></label><div id="community-list" class="community-list"></div></section>`)
  const section = document.querySelector('#admin-community'), list = document.querySelector('#community-list'), search = section.querySelector('input')
  const configs = {
    companies: { title: 'Бухгалтерские компании', description: 'Компании, опубликованные в открытом каталоге сообщества.', placeholder: 'Поиск по компании, городу или владельцу', endpoint: '/api/admin/community/accounting-companies', action: 'archive', noun: 'компанию' },
    vacancies: { title: 'Вакансии', description: 'Все вакансии, которые сейчас видны пользователям сайта.', placeholder: 'Поиск по вакансии, городу или владельцу', endpoint: '/api/admin/community/vacancies', action: 'unpublish', noun: 'вакансию' },
    profiles: { title: 'Профили специалистов', description: 'Опубликованные профили специалистов из открытого каталога.', placeholder: 'Поиск по имени, должности, городу или почте', endpoint: '/api/admin/community/profiles', action: 'unpublish', noun: 'профиль' },
  }
  let current = 'companies', timer
  const escape = value => { const node = document.createElement('span'); node.textContent = value ?? ''; return node.innerHTML }
  const date = value => value ? new Date(value).toLocaleDateString('ru-RU') : '—'
  const money = value => value == null ? '' : new Intl.NumberFormat('ru-RU', { maximumFractionDigits: 0 }).format(value)
  const salary = item => item.salary_from != null && item.salary_to != null ? `${money(item.salary_from)}–${money(item.salary_to)} ${item.currency || 'RUB'}` : item.salary_from != null ? `от ${money(item.salary_from)} ${item.currency || 'RUB'}` : item.salary_to != null ? `до ${money(item.salary_to)} ${item.currency || 'RUB'}` : 'Не указана'
  async function request(url, options = {}) { const response = await fetch(url, { cache: 'no-store', ...options }); const data = await response.json().catch(() => ({})); if (!response.ok) throw Error(data.error || 'Ошибка запроса'); return data }
  function setTab(tab, updatePath = false) {
    if (!configs[tab]) tab = 'companies'
    current = tab
    const config = configs[current]
    section.querySelectorAll('[data-community-tab]').forEach(button => button.classList.toggle('active', button.dataset.communityTab === current))
    document.querySelector('#community-title').textContent = config.title
    document.querySelector('#community-description').textContent = config.description
    search.placeholder = config.placeholder
    search.value = ''
    if (updatePath && location.pathname !== `/admin/community/${current}`) history.pushState({}, '', `/admin/community/${current}`)
    load()
  }
  function activate() {
    document.querySelectorAll('.workspace>section,.dictionary-list,.dictionary-editor').forEach(element => element.classList.add('hidden'))
    section.classList.remove('hidden')
    document.querySelectorAll('.sidebar nav button').forEach(button => button.classList.remove('active'))
    document.querySelector('#community-nav').classList.add('active')
    document.querySelector('.workspace>header h1').textContent = 'Сообщество'
    document.querySelector('.workspace>header p').textContent = 'Управление публичными компаниями, вакансиями и профилями'
    document.querySelectorAll('.workspace>header .primary').forEach(button => button.classList.add('hidden'))
    const routeTab = location.pathname.split('/').filter(Boolean)[2]
    setTab(configs[routeTab] ? routeTab : current)
  }
  function identity(icon, title, subtitle) { return `<div class="community-company"><i>${icon ? `<img src="${escape(icon)}" alt="">` : escape((title || '?').charAt(0))}</i><span><b>${escape(title)}</b><small>${escape(subtitle)}</small></span></div>` }
  function actions(item, href) { return `<div class="community-actions"><a href="${href}" target="_blank" rel="noopener">Открыть</a><button type="button" data-unpublish="${item.id}" data-name="${escape(item.name || item.title || item.position)}">Снять с публикации</button></div>` }
  function renderCompanies(items) { return `<div class="community-table"><table><thead><tr><th>Компания</th><th>Владелец</th><th>Город</th><th>Наполнение</th><th>Опубликована</th><th></th></tr></thead><tbody>${items.map(item => `<tr><td>${identity(item.logo, item.name, item.verified ? '✓ Подтверждена' : 'Опубликована')}</td><td><b>${escape(item.owner_name)}</b><small>${escape(item.owner_email)}</small></td><td>${escape(item.city || 'Не указан')}</td><td><span class="community-metrics">${item.directions_count} напр. · ${item.services_count} услуг</span></td><td>${date(item.published_at)}</td><td>${actions(item, `/accounting-companies/view?slug=${encodeURIComponent(item.slug || '')}`)}</td></tr>`).join('')}</tbody></table></div>` }
  function renderVacancies(items) { return `<div class="community-table"><table><thead><tr><th>Вакансия</th><th>Владелец</th><th>Город</th><th>Условия</th><th>Опубликована</th><th></th></tr></thead><tbody>${items.map(item => `<tr><td>${identity('', item.title || 'Без названия', item.employment_type || 'Вакансия')}</td><td><b>${escape(item.owner_name)}</b><small>${escape(item.owner_email)}</small></td><td>${escape(item.city || 'Не указан')}</td><td><b>${escape(salary(item))}</b><small>${escape(item.work_format || 'Формат не указан')}</small></td><td>${date(item.published_at)}</td><td>${actions(item, `/vacancies/view?id=${item.id}`)}</td></tr>`).join('')}</tbody></table></div>` }
  function renderProfiles(items) { return `<div class="community-table"><table><thead><tr><th>Специалист</th><th>Контакты</th><th>Город</th><th>Ожидания</th><th>Опубликован</th><th></th></tr></thead><tbody>${items.map(item => `<tr><td>${identity(item.avatar, item.name, item.position)}</td><td><b>${escape(item.name)}</b><small>${escape(item.email)}</small></td><td>${escape(item.city || 'Не указан')}</td><td><b>${item.desired_salary ? `${money(item.desired_salary)} ₽` : 'Не указаны'}</b><small>${escape(item.search_status || '')}</small></td><td>${date(item.published_at)}</td><td>${actions(item, `/profiles/view/${item.id}`)}</td></tr>`).join('')}</tbody></table></div>` }
  async function load() {
    const config = configs[current]
    list.innerHTML = `<div class="community-loading">Загружаем ${config.title.toLowerCase()}…</div>`
    try {
      const query = search.value.trim(), data = await request(config.endpoint + (query ? `?q=${encodeURIComponent(query)}` : '')), items = data.items || []
      document.querySelector('#community-count').textContent = items.length
      if (!items.length) { list.innerHTML = '<div class="community-empty"><i>♧</i><b>Опубликованных записей не найдено</b><span>Измените запрос или выберите другой раздел.</span></div>'; return }
      list.innerHTML = current === 'companies' ? renderCompanies(items) : current === 'vacancies' ? renderVacancies(items) : renderProfiles(items)
      list.querySelectorAll('[data-unpublish]').forEach(button => button.onclick = () => unpublish(button))
    } catch (error) { list.innerHTML = `<div class="community-empty"><i>!</i><b>Не удалось загрузить данные</b><span>${escape(error.message)}</span></div>` }
  }
  async function unpublish(button) {
    const config = configs[current]
    if (!confirm(`Снять ${config.noun} «${button.dataset.name}» с публикации? Запись исчезнет из открытого каталога, а владелец сможет исправить и опубликовать её снова.`)) return
    button.disabled = true; button.textContent = 'Снимаем…'
    try { await request(`${config.endpoint}/${button.dataset.unpublish}/${config.action}`, { method: 'POST' }); if (typeof notify === 'function') notify('Запись снята с публикации'); await load() }
    catch (error) { if (typeof notify === 'function') notify(error.message, true); button.disabled = false; button.textContent = 'Снять с публикации' }
  }
  section.querySelectorAll('[data-community-tab]').forEach(button => button.onclick = () => setTab(button.dataset.communityTab, true))
  search.addEventListener('input', () => { clearTimeout(timer); timer = setTimeout(load, 300) })
  document.querySelector('#community-nav').onclick = activate
  document.querySelectorAll('.sidebar nav button').forEach(button => button.addEventListener('click', () => { if (button.id !== 'community-nav') section.classList.add('hidden') }))
})()
