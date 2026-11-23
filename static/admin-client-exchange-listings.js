(() => {
  const nav = document.querySelector('.sidebar nav'), workspace = document.querySelector('.workspace')
  const button = document.querySelector('#client-exchange-listings-nav')
  if (!button || !workspace) return
  const esc = value => String(value ?? '').replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]))
  const statuses = {draft:'Черновик',active:'Опубликована',has_responses:'Есть отклики',buyer_selected:'Покупатель выбран',transfer_in_progress:'Передача',transferred:'Передана',archived:'Снята с публикации',cancelled:'Отменена'}
  const editable = status => ['draft','active','has_responses','archived','cancelled'].includes(status)
  const date = value => value ? new Date(value).toLocaleString('ru-RU') : '—'
  const money = value => value == null ? '—' : Number(value).toLocaleString('ru-RU') + ' ₽'
  async function request(url, options = {}) {
    const response = await fetch(url, {cache:'no-store', credentials:'same-origin', ...options, headers:{'Content-Type':'application/json', ...options.headers}})
    const data = await response.json().catch(() => ({}))
    if (!response.ok) throw Error(data.error || 'Не удалось выполнить запрос')
    return data
  }
  const section = document.createElement('section'); section.id = 'client-exchange-listings-admin'; section.className = 'hidden ce-listings'
  section.innerHTML = `<div class="ce-admin-head"><div><h2>Карточки пользователей</h2><p>Просмотр, редактирование и управление публикацией карточек биржи.</p></div></div>
    <form class="ce-listing-filters"><label>Поиск<input name="q" maxlength="200" placeholder="Название, ФИО, почта, ИНН или ID"></label><label>Статус<select name="status"><option value="">Все статусы</option>${Object.entries(statuses).map(([key,label]) => `<option value="${key}">${label}</option>`).join('')}</select></label><button type="submit">Найти</button><button type="button" data-refresh>Обновить</button></form>
    <p role="status" data-summary></p><div class="ce-admin-table" data-list></div><div class="ce-listing-pagination"><button type="button" data-prev>Назад</button><span data-page></span><button type="button" data-next>Далее</button></div>`
  workspace.append(section)
  const filters = section.querySelector('form'), summary = section.querySelector('[data-summary]'), list = section.querySelector('[data-list]')
  let page = 1, sequence = 0, modal = null, loadingModal = false
  async function load() {
    const version = ++sequence
    summary.textContent = 'Загружаем карточки…'
    section.querySelector('[data-prev]').disabled = section.querySelector('[data-next]').disabled = true
    try {
      const query = new URLSearchParams(new FormData(filters)); query.set('page', page)
      const data = await request('/api/admin/client-exchange/listings?' + query)
      if (version !== sequence) return
      if (page > 1 && !data.items.length) { page = Math.max(1, Math.ceil(data.total / data.page_size)); return load() }
      summary.textContent = `Всего карточек: ${data.total}`
      section.querySelector('[data-page]').textContent = `Страница ${page} из ${Math.max(1,Math.ceil(data.total / data.page_size))}`
      section.querySelector('[data-prev]').disabled = page <= 1
      section.querySelector('[data-next]').disabled = page * data.page_size >= data.total
      list.innerHTML = data.items.length ? `<table><thead><tr>${['ID','Карточка','Пользователь','Статус','Цена передачи','Обновлена',''].map(v=>`<th>${v}</th>`).join('')}</tr></thead><tbody>${data.items.map(item => `<tr data-card="${item.id}"><td>${item.id}</td><td><button type="button" data-open="${item.id}">${esc(item.title || 'Без названия')}</button><small>${esc(item.city)}</small></td><td>${esc(item.seller_name)}<small>${esc(item.seller_email)} · ID ${item.seller_user_id}</small></td><td>${esc(statuses[item.status] || item.status)}</td><td>${money(item.transfer_price)}</td><td>${date(item.updated_at)}</td><td><button type="button" data-open="${item.id}">Открыть</button></td></tr>`).join('')}</tbody></table>` : '<p class="ce-listing-empty">Карточки не найдены.</p>'
    } catch (error) { if (version === sequence) { summary.textContent = error.message; list.replaceChildren() } }
  }
  function activate() {
    document.querySelectorAll('.workspace>section,.dictionary-list,.dictionary-editor').forEach(node => node.classList.add('hidden'))
    section.classList.remove('hidden')
    nav.querySelectorAll('button').forEach(node => node.classList.toggle('active', node === button))
    workspace.querySelector('header h1').textContent = 'Клиентская биржа'
    workspace.querySelector('header p').textContent = 'Карточки пользователей'
    workspace.querySelectorAll('header .primary').forEach(node => node.classList.add('hidden'))
    void load()
  }
  const single = [['employee_range_id','employee_range','Сотрудники'],['tax_system_id','tax_system','Налогообложение'],['revenue_range_id','revenue_range','Размер бизнеса'],['accounting_state_id','accounting_state','Состояние учёта'],['transfer_type_id','transfer_type','Условия передачи']]
  const multiple = [['industry_ids','industry','Направления'],['transfer_reason_ids','transfer_reason','Причины передачи'],['marketplace_ids','marketplace','Маркетплейсы'],['edo_provider_ids','edo_provider','ЭДО'],['accounting_program_ids','accounting_program','Учётные программы']]
  const textFields = [['title','Название',240],['client_inn','ИНН клиента',12],['client_legal_name','Юридическое название клиента',500],['region','Регион',200],['city','Город',200]]
  const numeric = [['transfer_price','Цена передачи, ₽','0.01'],['current_monthly_fee','Абонплата, ₽','0.01'],['monthly_commission_percent','Ежемесячная комиссия, %','0.01'],['commission_months','Срок комиссии, мес.','1'],['operations_per_month','Операций в месяц','1'],['banks_count','Количество банков','1']]
  async function open(id) {
    if (loadingModal || modal) return
    loadingModal = true
    try {
      const [data, dictionaries] = await Promise.all([request(`/api/admin/client-exchange/listings/${id}`),request('/api/admin/client-exchange/dictionaries')])
      if (section.classList.contains('hidden')) return
      const {listing: item, input} = data
      const canEdit = editable(item.status), canArchive = ['active','has_responses'].includes(item.status)
      const choices = (kind, values) => {
        const selected = new Set((Array.isArray(values) ? values : values == null ? [] : [values]).map(Number))
        const items = dictionaries.items.filter(x => x.kind === kind && (x.active || selected.has(x.id)))
        for (const value of selected) if (!items.some(x => x.id === value)) items.push({id:value,name:`Недоступное значение #${value}`,active:false})
        return items.map(x => `<option value="${x.id}" ${selected.has(x.id)?'selected':''}>${esc(x.name)}${x.active?'':' (отключено)'}</option>`).join('')
      }
      modal = document.createElement('dialog'); modal.className = 'ce-admin-modal ce-listing-modal'; modal.setAttribute('aria-labelledby','ce-listing-title')
      modal.innerHTML = `<form><header><h2 id="ce-listing-title">Карточка #${item.id}</h2><button type="button" data-close aria-label="Закрыть">✕</button></header>
        <div class="ce-listing-info"><b>${esc(item.title || 'Без названия')}</b><p>${esc(item.seller_name)} · ${esc(item.seller_email)} · ID ${item.seller_user_id}</p><p>${esc(statuses[item.status] || item.status)} · Просмотров: ${item.views_count} · Откликов: ${item.responses_count}</p><p>Создана: ${date(item.created_at)}<br>Обновлена: ${date(item.updated_at)}<br>Опубликована: ${date(item.published_at)}</p><small>Публичный ID: ${esc(item.public_id)}</small></div>
        ${canEdit?'':'<p class="ce-listing-warning">Карточка участвует в передаче или уже передана. Доступен только просмотр, чтобы сохранить историю сделки.</p>'}
        <fieldset disabled><legend>Сведения о клиенте и условия передачи</legend><div class="grid">
        ${textFields.map(([key,label,max]) => `<label>${label}<input name="${key}" maxlength="${max}" value="${esc(input[key])}"></label>`).join('')}
        ${single.map(([key,kind,label]) => `<label>${label}<select name="${key}"><option value="">Не указано</option>${choices(kind,input[key])}</select></label>`).join('')}
        ${multiple.map(([key,kind,label]) => `<label>${label}<select name="${key}" multiple size="4">${choices(kind,input[key])}</select><small>Можно выбрать несколько значений.</small></label>`).join('')}
        ${numeric.map(([key,label,step]) => `<label>${label}<input type="number" name="${key}" min="0" ${key==='monthly_commission_percent'?'max="100"':''} step="${step}" value="${esc(input[key])}"></label>`).join('')}
        ${[['client_since','Клиент обслуживается с'],['desired_transfer_date','Желаемая дата передачи']].map(([key,label]) => `<label>${label}<input type="date" name="${key}" value="${esc(input[key])}"></label>`).join('')}
        </div><div class="ce-listing-checks">${[['has_vat','Есть НДС'],['foreign_trade','Внешнеэкономическая деятельность'],['bargain_allowed','Возможен торг']].map(([key,label])=>`<label><input name="${key}" type="checkbox" ${input[key]?'checked':''}> ${label}</label>`).join('')}</div>
        <label>Комментарий о причине передачи<textarea name="transfer_reason_comment" maxlength="2000">${esc(input.transfer_reason_comment)}</textarea></label><label>Комментарий к карточке<textarea name="comment" maxlength="5000">${esc(input.comment)}</textarea></label></fieldset>
        <p role="status" data-message></p><footer><button type="button" data-close>Закрыть</button>${canEdit?`<button type="button" data-edit>Редактировать</button><button type="submit" data-save hidden>Сохранить</button>${canArchive?'<button type="button" data-archive>Снять с публикации</button>':''}<button type="button" class="danger" data-delete>Удалить</button>`:''}</footer></form>`
      const dialog = modal, form = dialog.querySelector('form'), message = dialog.querySelector('[data-message]'), previousFocus = document.activeElement
      let busy = false, editing = false
      const close = () => { if (!busy) dialog.close() }
      dialog.querySelectorAll('[data-close]').forEach(node => node.onclick = close)
      dialog.addEventListener('cancel', event => { if (busy) event.preventDefault() })
      dialog.addEventListener('close', () => { dialog.remove(); if (modal === dialog) modal = null; previousFocus?.focus() }, {once:true})
      dialog.querySelector('[data-edit]')?.addEventListener('click', () => {
        editing = true; form.querySelector('fieldset').disabled = false
        dialog.querySelector('[data-edit]').hidden = true; dialog.querySelector('[data-save]').hidden = false
        // Save or close first: changing publication must not silently discard edits.
        dialog.querySelectorAll('[data-archive],[data-delete]').forEach(node => node.disabled = true)
        form.querySelector('[name=title]').focus()
      })
      async function mutate(method, action = '', body = {}) {
        if (busy) return
        busy = true; message.textContent = 'Сохраняем…'
        dialog.querySelectorAll('button').forEach(node => node.disabled = true)
        try {
          await request(`/api/admin/client-exchange/listings/${id}${action}`, {method, body:JSON.stringify({updated_at:item.updated_at,...body})})
          busy = false; dialog.close(); await load()
        } catch (error) {
          message.textContent = error.message; busy = false
          dialog.querySelectorAll('button').forEach(node => node.disabled = false)
          if (editing) dialog.querySelectorAll('[data-archive],[data-delete]').forEach(node => node.disabled = true)
        }
      }
      form.onsubmit = event => {
        event.preventDefault()
        if (!editing) return
        const values = new FormData(form), result = {...input}
        for (const [key] of textFields) result[key] = values.get(key).trim()
        for (const key of ['comment','transfer_reason_comment']) result[key] = values.get(key).trim()
        for (const [key] of single) result[key] = values.get(key) === '' ? null : Number(values.get(key))
        for (const [key] of multiple) result[key] = values.getAll(key).map(Number)
        result.industry_id = result.industry_ids[0] || null; result.transfer_reason_id = result.transfer_reason_ids[0] || null
        for (const [key] of numeric) result[key] = values.get(key) === '' ? null : Number(values.get(key))
        for (const key of ['client_since','desired_transfer_date']) result[key] = values.get(key) || null
        for (const key of ['has_vat','foreign_trade','bargain_allowed']) result[key] = values.has(key)
        void mutate('PUT','',{input:result})
      }
      dialog.querySelector('[data-archive]')?.addEventListener('click', () => { if (confirm('Снять карточку с публикации? Она останется в архиве пользователя, отклики сохранятся.')) void mutate('POST','/archive') })
      dialog.querySelector('[data-delete]')?.addEventListener('click', () => { if (confirm('Удалить карточку? Она исчезнет из биржи и списка карточек пользователя.')) void mutate('DELETE') })
      document.body.append(dialog); dialog.showModal()
    } catch (error) { summary.textContent = error.message }
    finally { loadingModal = false }
  }
  list.addEventListener('click', event => { const row = event.target.closest('[data-card]'); if (row) void open(Number(row.dataset.card)) })
  filters.onsubmit = event => { event.preventDefault(); page = 1; void load() }
  filters.querySelector('[name=status]').onchange = () => { page = 1; void load() }
  section.querySelector('[data-refresh]').onclick = () => void load()
  section.querySelector('[data-prev]').onclick = () => { page--; void load() }
  section.querySelector('[data-next]').onclick = () => { page++; void load() }
  button.onclick = activate
  nav.addEventListener('click', event => { const target = event.target.closest('button'); if (target && target !== button) section.classList.add('hidden') }, true)
})()
