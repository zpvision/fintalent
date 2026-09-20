(() => {
  const nav = document.querySelector('.sidebar nav');
  const workspace = document.querySelector('.workspace');
  if (!nav || !workspace) return;

  document.head.insertAdjacentHTML('beforeend', '<link rel="stylesheet" href="/static/admin-profimarket.css?v=4"><link rel="stylesheet" href="/static/admin-profimarket-solutions.css?v=1">');
  nav.insertAdjacentHTML('beforeend', '<small>ПРОФИМАРКЕТ</small><button id="profimarket-admin-nav">✦ <span>ПрофиМаркет</span></button>');
  workspace.insertAdjacentHTML('beforeend', `
    <section id="profimarket-admin" class="pm-admin-section hidden">
      <div class="pm-admin-tabs" role="tablist">
        <button class="active" data-pm-tab="purchases">Покупки</button>
        <button data-pm-tab="solutions">Карточки</button>
        <button data-pm-tab="platforms">Платформы ИИ-ассистентов</button>
        <button data-pm-tab="dictionaries">Справочники</button>
      </div>
      <div data-pm-view="purchases"></div>
      <div data-pm-view="solutions" class="hidden"></div>
      <div data-pm-view="platforms" class="hidden"></div>
      <div data-pm-view="dictionaries" class="hidden"></div>
    </section>`);

  const section = document.querySelector('#profimarket-admin');
  const purchasesView = section.querySelector('[data-pm-view="purchases"]');
  const solutionsView = section.querySelector('[data-pm-view="solutions"]');
  const platformsView = section.querySelector('[data-pm-view="platforms"]');
  const dictionariesView = section.querySelector('[data-pm-view="dictionaries"]');
  const productTypes = {
    AI_ASSISTANT: 'ИИ-ассистенты',
    REGULATION: 'Регламенты',
    AUTOMATION: 'Автоматизации',
    INSTRUCTION: 'Инструкции',
    ONEC_INTEGRATION: '1С Интеграции',
    TEMPLATE: 'Шаблоны',
    CHECKLIST: 'Чек-листы'
  };
  const statusNames = {PENDING: 'Ожидает', COMPLETED: 'Оформлена', CANCELLED: 'Отменена', REFUNDED: 'Возврат'};
  const solutionStatusNames = {PUBLISHED:'Опубликованы', DRAFT:'Черновики', MODERATION:'На модерации', ARCHIVED:'Сняты'};
  let activeTab = 'purchases';
  let platforms = [];
  let onecConfigurations = [];
  let compatibilityOptions = [];
  let query = {q: '', status: '', type: '', page: 1};
  let solutionQuery = {q: '', status: '', owner_id: '', page: 1};
  let searchTimer;
  let solutionSearchTimer;

  const esc = value => String(value ?? '').replace(/[&<>"']/g, char => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[char]));
  const formatDate = value => new Intl.DateTimeFormat('ru-RU', {dateStyle:'medium', timeStyle:'short'}).format(new Date(value));
  const formatPrice = item => item.amount > 0
    ? new Intl.NumberFormat('ru-RU', {style:'currency', currency:item.currency || 'RUB', maximumFractionDigits:0}).format(item.amount)
    : 'Бесплатно';

  async function request(url, options = {}) {
    const response = await fetch(url, {cache: 'no-store', ...options});
    let data = {};
    try { data = await response.json(); } catch {}
    if (!response.ok) throw Error(data.error || 'Не удалось выполнить запрос');
    return data;
  }

  function activate() {
    document.querySelectorAll('.workspace>section,.dictionary-list,.dictionary-editor').forEach(node => node.classList.add('hidden'));
    section.classList.remove('hidden');
    document.querySelectorAll('.sidebar nav button').forEach(button => button.classList.remove('active'));
    document.querySelector('#profimarket-admin-nav').classList.add('active');
    document.querySelectorAll('.workspace>header .primary').forEach(button => button.classList.add('hidden'));
    renderActiveTab();
  }

  function setHeader(title, description) {
    document.querySelector('.workspace>header h1').textContent = title;
    document.querySelector('.workspace>header p').textContent = description;
  }

  function renderActiveTab() {
    section.querySelectorAll('[data-pm-tab]').forEach(button => button.classList.toggle('active', button.dataset.pmTab === activeTab));
    purchasesView.classList.toggle('hidden', activeTab !== 'purchases');
    solutionsView.classList.toggle('hidden', activeTab !== 'solutions');
    platformsView.classList.toggle('hidden', activeTab !== 'platforms');
    dictionariesView.classList.toggle('hidden', activeTab !== 'dictionaries');
    if (activeTab === 'purchases') {
      setHeader('ПрофиМаркет', 'Все покупки и заявки на решения сервиса');
      loadPurchases();
    } else if (activeTab === 'solutions') {
      setHeader('ПрофиМаркет', 'Управление публикацией карточек сервиса');
      loadSolutions();
    } else if (activeTab === 'platforms') {
      setHeader('ПрофиМаркет', 'ИИ-ассистенты → справочник платформ');
      loadPlatforms();
    } else {
      setHeader('ПрофиМаркет', 'Справочники для карточек решений');
      loadDictionaries();
    }
  }

  async function loadPurchases() {
    purchasesView.innerHTML = '<div class="pm-loading"><span></span>Загружаем покупки…</div>';
    try {
      const parameters = new URLSearchParams({...query, page: String(query.page)});
      const data = await request('/api/admin/profimarket/purchases?' + parameters);
      renderPurchases(data);
    } catch (error) {
      purchasesView.innerHTML = `<div class="pm-empty"><b>Не удалось загрузить покупки</b><p>${esc(error.message)}</p><button data-retry>Повторить</button></div>`;
      purchasesView.querySelector('[data-retry]').onclick = loadPurchases;
    }
  }

  function renderPurchases(data) {
    const counts = data.status_counts || {};
    const pages = Math.max(1, Math.ceil(data.total / data.limit));
    purchasesView.innerHTML = `
      <div class="pm-purchases-head">
        <div><small>КОНТРОЛЬ ПРОДАЖ</small><h2>Покупки на сервисе</h2><p>Контакты участников, продукт и состояние каждой сделки в одном месте.</p></div>
        <div class="pm-purchase-total"><span>Всего покупок</span><b>${Object.values(counts).reduce((sum, value) => sum + Number(value || 0), 0).toLocaleString('ru-RU')}</b></div>
      </div>
      <div class="pm-status-cards">
        ${statusCard('', 'Все', Object.values(counts).reduce((sum, value) => sum + Number(value || 0), 0))}
        ${statusCard('COMPLETED', 'Оформлены', counts.COMPLETED)}
        ${statusCard('PENDING', 'Ожидают', counts.PENDING)}
        ${statusCard('CANCELLED', 'Отменены', counts.CANCELLED)}
        ${statusCard('REFUNDED', 'Возвраты', counts.REFUNDED)}
      </div>
      <div class="pm-purchase-filters">
        <label class="pm-search"><span>⌕</span><input type="search" value="${esc(query.q)}" placeholder="Имя или почта владельца либо покупателя"></label>
        <select data-filter="status" aria-label="Статус">
          <option value="">Все статусы</option>${Object.entries(statusNames).map(([value,label]) => `<option value="${value}" ${query.status===value?'selected':''}>${label}</option>`).join('')}
        </select>
        <select data-filter="type" aria-label="Направление">
          <option value="">Все направления</option>${Object.entries(productTypes).map(([value,label]) => `<option value="${value}" ${query.type===value?'selected':''}>${label}</option>`).join('')}
        </select>
        ${(query.q || query.status || query.type) ? '<button class="pm-clear" data-clear>Сбросить</button>' : ''}
      </div>
      <div class="pm-results-line"><span>Найдено: <b>${Number(data.total).toLocaleString('ru-RU')}</b></span><span>Сначала новые</span></div>
      <div class="pm-purchase-table-wrap">
        <table class="pm-purchase-table">
          <thead><tr><th>Покупка</th><th>Решение</th><th>Владелец</th><th>Покупатель</th><th>Сумма</th><th>Статус</th></tr></thead>
          <tbody>${data.items.map(purchaseRow).join('') || '<tr><td colspan="6"><div class="pm-table-empty"><b>Покупок не найдено</b><span>Измените параметры поиска или фильтры.</span></div></td></tr>'}</tbody>
        </table>
      </div>
      ${pages > 1 ? `<div class="pm-pagination"><button data-page="${query.page-1}" ${query.page<=1?'disabled':''}>← Назад</button><span>Страница <b>${query.page}</b> из ${pages}</span><button data-page="${query.page+1}" ${query.page>=pages?'disabled':''}>Вперёд →</button></div>` : ''}`;

    const search = purchasesView.querySelector('.pm-search input');
    search.oninput = event => {
      clearTimeout(searchTimer);
      searchTimer = setTimeout(() => { query.q = event.target.value.trim(); query.page = 1; loadPurchases(); }, 350);
    };
    purchasesView.querySelectorAll('[data-filter]').forEach(select => select.onchange = () => {
      query[select.dataset.filter] = select.value;
      query.page = 1;
      loadPurchases();
    });
    purchasesView.querySelectorAll('[data-status-card]').forEach(button => button.onclick = () => {
      query.status = button.dataset.statusCard;
      query.page = 1;
      loadPurchases();
    });
    purchasesView.querySelector('[data-clear]')?.addEventListener('click', () => { query = {q:'', status:'', type:'', page:1}; loadPurchases(); });
    purchasesView.querySelectorAll('[data-page]').forEach(button => button.onclick = () => { query.page = Number(button.dataset.page); loadPurchases(); });
  }

  function statusCard(status, label, count = 0) {
    return `<button class="pm-status-card ${query.status===status?'active':''} status-${status.toLowerCase() || 'all'}" data-status-card="${status}"><span>${esc(label)}</span><b>${Number(count || 0).toLocaleString('ru-RU')}</b></button>`;
  }

  function purchaseRow(item) {
    const product = item.solution_visible && item.slug
      ? `<a class="pm-product-title" href="/profimarket/solution/${encodeURIComponent(item.slug)}?preview=1" target="_blank" rel="noopener">${esc(item.title)} <span>↗</span></a>`
      : `<b class="pm-product-title">${esc(item.title)}</b>`;
    return `<tr>
      <td><div class="pm-order-number">№ ${item.id}</div><time>${formatDate(item.created_at)}</time></td>
      <td><div class="pm-product"><div class="pm-product-cover">${item.cover_image?`<img src="${esc(item.cover_image)}" alt="">`:'<span>F</span>'}</div><div>${product}<small>${esc(productTypes[item.product_type] || item.product_type || 'Без направления')}</small></div></div></td>
      <td>${person(item.seller_name, item.seller_email, 'Владелец')}</td>
      <td>${person(item.buyer_name, item.buyer_email, 'Покупатель')}</td>
      <td><b class="pm-price">${esc(formatPrice(item))}</b><small class="pm-price-kind">${item.pricing_type==='TRIAL'?'Пробный период':'Покупка'}</small></td>
      <td><span class="pm-status status-${item.status.toLowerCase()}"><i></i>${esc(statusNames[item.status] || item.status)}</span></td>
    </tr>`;
  }

  function person(name, email, role) {
    const initials = String(name || email || '?').split(/\s+/).slice(0,2).map(part => part[0]).join('').toUpperCase();
    return `<div class="pm-person"><span>${esc(initials)}</span><div><b>${esc(name || role)}</b><a href="mailto:${esc(email)}">${esc(email)}</a></div></div>`;
  }

  async function loadSolutions() {
    solutionsView.innerHTML = '<div class="pm-loading"><span></span>Загружаем карточки…</div>';
    try {
      const parameters = new URLSearchParams({...solutionQuery, page: String(solutionQuery.page)});
      const data = await request('/api/admin/profimarket/solutions?' + parameters);
      const items = data.items || [];
      const counts = data.status_counts || {};
      const pages = Math.max(1, Math.ceil(Number(data.total || 0) / Number(data.limit || 100)));
      solutionsView.innerHTML = `
        <div class="pm-admin-head"><div><small>УПРАВЛЕНИЕ КАТАЛОГОМ</small><h2>Все карточки</h2><p>Снимайте решения с публикации или удаляйте их из сервиса.</p></div></div>
        <div class="pm-solution-summary">
          <div class="pm-purchase-total"><span>Всего карточек</span><b>${Number(data.all_total || 0).toLocaleString('ru-RU')}</b></div>
          ${solutionStatusCard('PUBLISHED', solutionStatusNames.PUBLISHED, counts.PUBLISHED)}
          ${solutionStatusCard('DRAFT', solutionStatusNames.DRAFT, counts.DRAFT)}
          ${solutionStatusCard('MODERATION', solutionStatusNames.MODERATION, counts.MODERATION)}
          ${solutionStatusCard('ARCHIVED', solutionStatusNames.ARCHIVED, counts.ARCHIVED)}
        </div>
        <div class="pm-solution-filters">
          <label class="pm-search"><span>⌕</span><input type="search" value="${esc(solutionQuery.q)}" placeholder="Поиск карточки по названию"></label>
          <select data-solution-filter="owner_id" aria-label="Аккаунт владельца">
            <option value="">Все аккаунты</option>${(data.owners || []).map(owner => `<option value="${owner.id}" ${String(solutionQuery.owner_id)===String(owner.id)?'selected':''}>${esc(owner.name || owner.email)} · ${esc(owner.email)}</option>`).join('')}
          </select>
          <select data-solution-filter="status" aria-label="Статус карточки">
            <option value="">Все статусы</option>${Object.entries(solutionStatusNames).map(([value,label]) => `<option value="${value}" ${solutionQuery.status===value?'selected':''}>${label}</option>`).join('')}
          </select>
          ${(solutionQuery.q || solutionQuery.status || solutionQuery.owner_id) ? '<button class="pm-clear" data-solution-clear>Сбросить</button>' : ''}
        </div>
        <div class="pm-results-line"><span>Найдено: <b>${Number(data.total || 0).toLocaleString('ru-RU')}</b></span><span>По 100 карточек на странице</span></div>
        <div class="pm-admin-table pm-solutions-table"><table><thead><tr><th>Карточка</th><th>Владелец</th><th>Статус</th><th>Покупки</th><th>Обновлена</th><th>Действия</th></tr></thead><tbody>${items.map(solutionRow).join('') || '<tr><td colspan="6"><div class="pm-table-empty"><b>Карточки не найдены</b><span>Измените поиск или фильтры.</span></div></td></tr>'}</tbody></table></div>
        ${pages > 1 ? `<div class="pm-pagination"><button data-solution-page="${solutionQuery.page-1}" ${solutionQuery.page<=1?'disabled':''}>← Назад</button><span>Страница <b>${solutionQuery.page}</b> из ${pages}</span><button data-solution-page="${solutionQuery.page+1}" ${solutionQuery.page>=pages?'disabled':''}>Вперёд →</button></div>` : ''}`;
      solutionsView.querySelectorAll('[data-unpublish-solution]').forEach(button => button.onclick = () => unpublishSolution(button));
      solutionsView.querySelectorAll('[data-delete-solution]').forEach(button => button.onclick = () => deleteSolution(button));
      const search = solutionsView.querySelector('.pm-search input');
      search.oninput = event => {
        clearTimeout(solutionSearchTimer);
        solutionSearchTimer = setTimeout(() => { solutionQuery.q = event.target.value.trim(); solutionQuery.page = 1; loadSolutions(); }, 350);
      };
      solutionsView.querySelectorAll('[data-solution-filter]').forEach(select => select.onchange = () => {
        solutionQuery[select.dataset.solutionFilter] = select.value;
        solutionQuery.page = 1;
        loadSolutions();
      });
      solutionsView.querySelectorAll('[data-solution-status]').forEach(button => button.onclick = () => {
        solutionQuery.status = button.dataset.solutionStatus;
        solutionQuery.page = 1;
        loadSolutions();
      });
      solutionsView.querySelector('[data-solution-clear]')?.addEventListener('click', () => { solutionQuery = {q:'', status:'', owner_id:'', page:1}; loadSolutions(); });
      solutionsView.querySelectorAll('[data-solution-page]').forEach(button => button.onclick = () => { solutionQuery.page = Number(button.dataset.solutionPage); loadSolutions(); });
    } catch (error) {
      solutionsView.innerHTML = `<div class="pm-empty"><b>Не удалось загрузить карточки</b><p>${esc(error.message)}</p><button data-retry>Повторить</button></div>`;
      solutionsView.querySelector('[data-retry]').onclick = loadSolutions;
    }
  }

  function solutionStatusCard(status, label, count = 0) {
    return `<button class="pm-solution-count status-${status.toLowerCase() || 'all'} ${solutionQuery.status===status?'active':''}" data-solution-status="${status}"><span>${esc(label)}</span><b>${Number(count || 0).toLocaleString('ru-RU')}</b></button>`;
  }

  function solutionRow(item) {
    const status = {PUBLISHED:'Опубликована', DRAFT:'Черновик', MODERATION:'На модерации', ARCHIVED:'Снята'}[item.status] || item.status;
    return `<tr><td><div class="pm-product"><div class="pm-product-cover">${item.cover_image?`<img src="${esc(item.cover_image)}" alt="">`:'<span>F</span>'}</div><div><a class="pm-product-title" href="/profimarket/solution/${encodeURIComponent(item.slug)}?preview=1" target="_blank" rel="noopener">${esc(item.title)} <span>↗</span></a><small>${esc(productTypes[item.product_type] || item.product_type)}</small></div></div></td><td>${person(item.owner_name,item.owner_email,'Владелец')}</td><td><span class="pm-solution-status status-${item.status.toLowerCase()}">${esc(status)}</span></td><td><b>${Number(item.purchases).toLocaleString('ru-RU')}</b></td><td><time>${formatDate(item.updated_at)}</time></td><td><div class="pm-solution-actions">${item.status==='PUBLISHED'?`<button data-unpublish-solution="${item.id}" data-title="${esc(item.title)}">Снять</button>`:''}<button class="delete" data-delete-solution="${item.id}" data-title="${esc(item.title)}">Удалить</button></div></td></tr>`;
  }

  async function unpublishSolution(button) {
    button.disabled = true;
    try {
      await request(`/api/admin/profimarket/solutions/${button.dataset.unpublishSolution}/unpublish`, {method:'POST'});
      await loadSolutions();
      notify('Карточка снята с публикации');
    } catch (error) {
      notify(error.message, true);
      button.disabled = false;
    }
  }

  async function deleteSolution(button) {
    if (!confirm(`Удалить карточку «${button.dataset.title}»? История покупок сохранится, но карточка исчезнет из сервиса.`)) return;
    button.disabled = true;
    try { await request(`/api/admin/profimarket/solutions/${button.dataset.deleteSolution}`, {method:'DELETE'}); loadSolutions(); }
    catch (error) { alert(error.message); button.disabled = false; }
  }

  async function loadDictionaries() {
    dictionariesView.innerHTML = '<div class="pm-loading"><span></span>Загружаем справочники…</div>';
    try {
      const [onecData, compatibilityData] = await Promise.all([
        request('/api/admin/profimarket/onec-configurations'),
        request('/api/admin/profimarket/compatibility')
      ]);
      onecConfigurations = onecData.items || [];
      compatibilityOptions = compatibilityData.items || [];
      dictionariesView.innerHTML = `<section class="pm-dictionary-section"><div class="pm-admin-head"><div><small>АВТОМАТИЗАЦИИ</small><h2>Совместимость</h2><p>Варианты показываются на шаге «Совместимость». Переименование автоматически обновит уже заполненные карточки.</p></div><button class="primary" id="pm-compatibility-new">＋ Добавить вариант</button></div><div class="pm-admin-table"><table><thead><tr><th>Порядок</th><th>Название</th><th>Code</th><th>Статус</th><th>Использование</th><th></th></tr></thead><tbody>${compatibilityOptions.map(compatibilityRow).join('') || '<tr><td colspan="6">Вариантов пока нет</td></tr>'}</tbody></table></div></section><section class="pm-dictionary-section"><div class="pm-admin-head"><div><small>1С ИНТЕГРАЦИИ</small><h2>Конфигурации 1С</h2><p>Список используется при заполнении совместимости карточек 1С. Переименование автоматически обновит уже заполненные карточки.</p></div><button class="primary" id="pm-onec-new">＋ Добавить конфигурацию</button></div><div class="pm-admin-table"><table><thead><tr><th>Порядок</th><th>Логотип</th><th>Название</th><th>Code</th><th>Статус</th><th>Использование</th><th></th></tr></thead><tbody>${onecConfigurations.map(onecConfigurationRow).join('') || '<tr><td colspan="7">Конфигураций пока нет</td></tr>'}</tbody></table></div></section>`;
      dictionariesView.querySelector('#pm-compatibility-new').onclick = () => editCompatibilityOption();
      dictionariesView.querySelectorAll('[data-compatibility-edit]').forEach(button => button.onclick = () => editCompatibilityOption(compatibilityOptions.find(item => item.id === Number(button.dataset.compatibilityEdit))));
      dictionariesView.querySelectorAll('[data-compatibility-delete]').forEach(button => button.onclick = () => removeCompatibilityOption(Number(button.dataset.compatibilityDelete)));
      dictionariesView.querySelector('#pm-onec-new').onclick = () => editOneCConfiguration();
      dictionariesView.querySelectorAll('[data-onec-edit]').forEach(button => button.onclick = () => editOneCConfiguration(onecConfigurations.find(item => item.id === Number(button.dataset.onecEdit))));
      dictionariesView.querySelectorAll('[data-onec-delete]').forEach(button => button.onclick = () => removeOneCConfiguration(Number(button.dataset.onecDelete)));
    } catch (error) {
      dictionariesView.innerHTML = `<div class="pm-empty"><b>Не удалось загрузить справочники</b><p>${esc(error.message)}</p></div>`;
    }
  }

  function compatibilityRow(item) {
    return `<tr class="${item.active?'':'inactive'}"><td>${item.sort_order}</td><td><b>${esc(item.name)}</b></td><td><code>${esc(item.code)}</code></td><td>${item.active?'Активен':'Отключён'}</td><td>${item.used?'Есть в карточках':'Не используется'}</td><td><button data-compatibility-edit="${item.id}">Изменить</button> <button class="delete" data-compatibility-delete="${item.id}">${item.used?'Отключить':'Удалить'}</button></td></tr>`;
  }

  function editCompatibilityOption(item = {name:'',code:'',sort_order:compatibilityOptions.length+1,active:true}) {
    const modal = document.createElement('div');
    modal.className = 'pm-admin-modal';
    modal.innerHTML = `<form><h2>${item.id?'Изменить вариант':'Новый вариант совместимости'}</h2><p>Название появится на шаге «Совместимость» при создании автоматизации.</p><div class="grid"><label>Название<input name="name" maxlength="160" required value="${esc(item.name)}" placeholder="Например, FinKoper"></label><label>Code<input name="code" maxlength="80" required pattern="[a-z][a-z0-9_-]*" value="${esc(item.code)}" placeholder="finkoper"></label></div><div class="grid"><label>Порядок<input name="sort_order" type="number" value="${item.sort_order||0}"></label><label class="check"><input name="active" type="checkbox" ${item.active!==false?'checked':''}> Показывать в редакторе карточки</label></div><footer><button type="button" data-cancel>Отмена</button><button class="primary">Сохранить</button></footer></form>`;
    document.body.append(modal);
    modal.querySelector('[data-cancel]').onclick = () => modal.remove();
    modal.onclick = event => { if (event.target === modal) modal.remove(); };
    modal.querySelector('form').onsubmit = async event => {
      event.preventDefault();
      const form = new FormData(event.currentTarget);
      const payload = {name:form.get('name').trim(),code:form.get('code').trim(),sort_order:Number(form.get('sort_order') || 0),active:form.has('active')};
      try {
        await request(item.id?`/api/admin/profimarket/compatibility/${item.id}`:'/api/admin/profimarket/compatibility', {method:item.id?'PUT':'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(payload)});
        modal.remove(); loadDictionaries();
      } catch (error) { alert(error.message); }
    };
  }

  async function removeCompatibilityOption(id) {
    const item = compatibilityOptions.find(value => value.id === id);
    const message = item?.used
      ? `Вариант «${item.name}» используется в карточках и будет отключён для нового выбора. Продолжить?`
      : `Удалить вариант «${item?.name || ''}»?`;
    if (!confirm(message)) return;
    try { await request(`/api/admin/profimarket/compatibility/${id}`, {method:'DELETE'}); loadDictionaries(); }
    catch (error) { alert(error.message); }
  }

  function onecConfigurationRow(item) {
    return `<tr class="${item.active?'':'inactive'}"><td>${item.sort_order}</td><td>${item.logo?`<img src="${esc(item.logo)}" alt="">`:'<i>—</i>'}</td><td><b>${esc(item.name)}</b></td><td><code>${esc(item.code)}</code></td><td>${item.active?'Активна':'Отключена'}</td><td>${item.used?'Есть в карточках':'Не используется'}</td><td><button data-onec-edit="${item.id}">Изменить</button> <button class="delete" data-onec-delete="${item.id}">${item.used?'Отключить':'Удалить'}</button></td></tr>`;
  }

  function editOneCConfiguration(item = {name:'',code:'',logo:'',sort_order:onecConfigurations.length+1,active:true}) {
    const modal = document.createElement('div');
    modal.className = 'pm-admin-modal';
    modal.innerHTML = `<form><h2>${item.id?'Изменить конфигурацию 1С':'Новая конфигурация 1С'}</h2><p>Название и логотип появятся в разделе совместимости карточки.</p><div class="grid"><label>Название<input name="name" maxlength="160" required value="${esc(item.name)}" placeholder="1С:Бухгалтерия предприятия"></label><label>Code<input name="code" maxlength="80" required pattern="[a-z][a-z0-9_-]*" value="${esc(item.code)}" placeholder="accounting"></label></div><label>Ссылка на логотип<input name="logo" maxlength="1000" type="url" value="${esc(item.logo)}" placeholder="https://example.ru/logo.svg"></label><div class="icon-preview">${item.logo?`<img src="${esc(item.logo)}" alt="">`:'<span>Предпросмотр логотипа</span>'}</div><div class="grid"><label>Порядок<input name="sort_order" type="number" value="${item.sort_order||0}"></label><label class="check"><input name="active" type="checkbox" ${item.active!==false?'checked':''}> Показывать в редакторе карточки</label></div><footer><button type="button" data-cancel>Отмена</button><button class="primary">Сохранить</button></footer></form>`;
    document.body.append(modal);
    const logoInput = modal.querySelector('[name=logo]');
    logoInput.oninput = () => { modal.querySelector('.icon-preview').innerHTML = logoInput.value ? `<img src="${esc(logoInput.value)}" alt="">` : '<span>Предпросмотр логотипа</span>'; };
    modal.querySelector('[data-cancel]').onclick = () => modal.remove();
    modal.onclick = event => { if (event.target === modal) modal.remove(); };
    modal.querySelector('form').onsubmit = async event => {
      event.preventDefault();
      const form = new FormData(event.currentTarget);
      const payload = {name:form.get('name').trim(),code:form.get('code').trim(),logo:form.get('logo').trim(),sort_order:Number(form.get('sort_order') || 0),active:form.has('active')};
      try {
        await request(item.id?`/api/admin/profimarket/onec-configurations/${item.id}`:'/api/admin/profimarket/onec-configurations', {method:item.id?'PUT':'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(payload)});
        modal.remove(); loadDictionaries();
      } catch (error) { alert(error.message); }
    };
  }

  async function removeOneCConfiguration(id) {
    const item = onecConfigurations.find(value => value.id === id);
    const message = item?.used
      ? `Конфигурация «${item.name}» используется в карточках и будет отключена для нового выбора. Продолжить?`
      : `Удалить конфигурацию «${item?.name || ''}»?`;
    if (!confirm(message)) return;
    try { await request(`/api/admin/profimarket/onec-configurations/${id}`, {method:'DELETE'}); loadDictionaries(); }
    catch (error) { alert(error.message); }
  }

  async function loadPlatforms() {
    platformsView.innerHTML = '<div class="pm-loading"><span></span>Загружаем платформы…</div>';
    try {
      platforms = (await request('/api/admin/profimarket/platforms')).items || [];
      platformsView.innerHTML = `<div class="pm-admin-head"><div><small>ПРОФИМАРКЕТ → ИИ-АССИСТЕНТ</small><h2>Платформы</h2><p>Варианты из этого справочника показываются на шаге «Где работает».</p></div><button class="primary" id="pm-platform-new">＋ Добавить платформу</button></div><div class="pm-admin-table"><table><thead><tr><th>Порядок</th><th>Иконка</th><th>Название</th><th>Code</th><th>Статус</th><th>Использование</th><th></th></tr></thead><tbody>${platforms.map(platformRow).join('') || '<tr><td colspan="7">Платформ пока нет</td></tr>'}</tbody></table></div>`;
      platformsView.querySelector('#pm-platform-new').onclick = () => editPlatform();
      platformsView.querySelectorAll('[data-edit]').forEach(button => button.onclick = () => editPlatform(platforms.find(item => item.id === Number(button.dataset.edit))));
      platformsView.querySelectorAll('[data-delete]').forEach(button => button.onclick = () => removePlatform(Number(button.dataset.delete)));
    } catch (error) {
      platformsView.innerHTML = `<div class="pm-empty"><b>Не удалось загрузить платформы</b><p>${esc(error.message)}</p></div>`;
    }
  }

  function platformRow(item) {
    return `<tr class="${item.active?'':'inactive'}"><td>${item.sort_order}</td><td>${item.icon?`<img src="${esc(item.icon)}" alt="">`:'<i>—</i>'}</td><td><b>${esc(item.name)}</b></td><td><code>${esc(item.code)}</code></td><td>${item.active?'Активна':'Отключена'}</td><td>${item.used?'Используется':'Свободна'}</td><td><button data-edit="${item.id}">Изменить</button><button data-delete="${item.id}">Удалить</button></td></tr>`;
  }

  function editPlatform(item = {code:'', name:'', icon:'', sort_order:platforms.length+1, active:true}) {
    const modal = document.createElement('div');
    modal.className = 'pm-admin-modal';
    modal.innerHTML = `<form><h2>${item.id?'Изменить платформу':'Новая платформа'}</h2><p>Название и иконка появятся в форме создания ИИ-ассистента.</p><div class="grid"><label>Название<input name="name" maxlength="160" value="${esc(item.name)}" required></label><label>Code<input name="code" maxlength="80" pattern="[a-z][a-z0-9_-]*" value="${esc(item.code)}" required></label></div><label>Ссылка на иконку<input name="icon" maxlength="1000" type="url" value="${esc(item.icon)}" placeholder="https://example.ru/icon.svg"></label><div class="icon-preview">${item.icon?`<img src="${esc(item.icon)}" alt="">`:'<span>Предпросмотр иконки</span>'}</div><div class="grid"><label>Порядок<input name="sort_order" type="number" value="${item.sort_order||0}"></label><label class="check"><input name="active" type="checkbox" ${item.active!==false?'checked':''}> Показывать в мастере</label></div><footer><button type="button" data-cancel>Отмена</button><button class="primary">Сохранить</button></footer></form>`;
    document.body.append(modal);
    const iconInput = modal.querySelector('[name=icon]');
    iconInput.oninput = () => { modal.querySelector('.icon-preview').innerHTML = iconInput.value ? `<img src="${esc(iconInput.value)}" alt="">` : '<span>Предпросмотр иконки</span>'; };
    modal.querySelector('[data-cancel]').onclick = () => modal.remove();
    modal.querySelector('form').onsubmit = async event => {
      event.preventDefault();
      const form = new FormData(event.currentTarget);
      const value = {name:form.get('name').trim(), code:form.get('code').trim(), icon:form.get('icon').trim(), sort_order:Number(form.get('sort_order'))||0, active:form.get('active')==='on'};
      try {
        await request(item.id?`/api/admin/profimarket/platforms/${item.id}`:'/api/admin/profimarket/platforms', {method:item.id?'PUT':'POST', headers:{'Content-Type':'application/json'}, body:JSON.stringify(value)});
        modal.remove(); loadPlatforms();
      } catch (error) { alert(error.message); }
    };
  }

  async function removePlatform(id) {
    if (!confirm('Удалить платформу? Если она используется, платформа будет отключена.')) return;
    try { await request(`/api/admin/profimarket/platforms/${id}`, {method:'DELETE'}); loadPlatforms(); }
    catch (error) { alert(error.message); }
  }

  section.querySelectorAll('[data-pm-tab]').forEach(button => button.onclick = () => { activeTab = button.dataset.pmTab; renderActiveTab(); });
  nav.addEventListener('click', event => {
    const button = event.target.closest('button');
    if (button && button.id !== 'profimarket-admin-nav') section.classList.add('hidden');
  }, true);
  document.querySelector('#profimarket-admin-nav').onclick = activate;
})();
