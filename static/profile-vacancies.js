(()=>{
  const escapeHTML=value=>{const node=document.createElement('span');node.textContent=value??'';return node.innerHTML}
  const money=value=>Number(value).toLocaleString('ru-RU')
  const date=value=>new Date(value).toLocaleDateString('ru-RU',{day:'2-digit',month:'long',year:'numeric'})
  const statusLabel=status=>({published:'Опубликована',draft:'Черновик',archived:'В архиве'})[status]||status
  const salary=vacancy=>{
    const value=vacancy.salary_from??vacancy.salary_to
    if(value==null)return'Сумма не указана'
    return`${vacancy.salary_to!=null&&vacancy.salary_from==null?'До':'От'} ${money(value)} ₽ · ${vacancy.salary_tax_mode==='gross'?'до вычета налогов':'на руки'}`
  }
  async function api(path,options){
    const response=await fetch(path,options)
    if(response.status===401){location.href='/login';throw Error('Требуется авторизация')}
    const data=await response.json().catch(()=>({}))
    if(!response.ok)throw Error(data.error||'Не удалось выполнить действие')
    return data
  }
  function card(vacancy){
    const published=vacancy.status==='published'
    return `<article class="my-vacancy-card" data-open-vacancy="${vacancy.id}" tabindex="0" role="link" aria-label="Редактировать вакансию ${escapeHTML(vacancy.title)}">
      <header><div class="my-vacancy-icon">▣</div><div><span class="vacancy-status ${escapeHTML(vacancy.status)}"><i></i>${escapeHTML(statusLabel(vacancy.status))}</span><h2>${escapeHTML(vacancy.title||'Вакансия')}</h2></div><div class="vacancy-card-actions">${published?`<a href="/vacancies/view?id=${vacancy.id}" aria-label="Посмотреть опубликованную вакансию" title="Открыть страницу вакансии">◉</a>`:''}<button type="button" data-delete-vacancy="${vacancy.id}" aria-label="Удалить вакансию навсегда" title="Удалить вакансию навсегда">⌫</button><span class="vacancy-edit-arrow">→</span></div></header>
      <div class="vacancy-card-facts"><span><i>₽</i><b>${escapeHTML(salary(vacancy))}</b></span><span><i>⌖</i><b>${escapeHTML(vacancy.city||'Город не указан')}</b></span><span><i>◷</i><b>Обновлена ${escapeHTML(date(vacancy.updated_at))}</b></span></div>
      ${vacancy.description?`<p>${escapeHTML(vacancy.description)}</p>`:''}
      <footer><span>Нажмите на карточку, чтобы открыть все шаги редактирования</span>${published?`<label class="publication-switch"><input type="checkbox" checked data-unpublish="${vacancy.id}"><em></em><b>Выключить публикацию</b></label>`:'<span class="draft-hint">Не показывается кандидатам</span>'}</footer>
    </article>`
  }
  function showMessage(text,error=false){
    let box=document.querySelector('.profile-vacancy-message')
    if(!box){box=document.createElement('div');box.className='profile-vacancy-message';document.body.append(box)}
    box.textContent=text;box.classList.toggle('error',error);box.classList.add('visible')
    setTimeout(()=>box.classList.remove('visible'),2800)
  }
  async function unpublish(input){
    const id=Number(input.dataset.unpublish)
    input.disabled=true
    try{
      await api(`/api/v1/vacancies/${id}/unpublish`,{method:'POST'})
      showMessage('Вакансия снята с публикации')
      await render()
    }catch(error){input.checked=true;input.disabled=false;showMessage(error.message,true)}
  }
  async function deleteVacancy(button){
    const id=Number(button.dataset.deleteVacancy),card=button.closest('.my-vacancy-card'),title=card?.querySelector('h2')?.textContent||'эту вакансию'
    if(!confirm(`Удалить вакансию «${title}» навсегда?\n\nБудут удалены все выбранные параметры, обязанности и тесты. Отменить это действие нельзя.`))return
    button.disabled=true
    try{
      await api(`/api/v1/vacancies/${id}`,{method:'DELETE'})
      if(localStorage.getItem('fintalent_vacancy_draft')===String(id))localStorage.removeItem('fintalent_vacancy_draft')
      showMessage('Вакансия удалена навсегда')
      await render()
    }catch(error){button.disabled=false;showMessage(error.message,true)}
  }
  function bind(){
    document.querySelectorAll('[data-open-vacancy]').forEach(item=>{
      const open=()=>location.href=`/vacancies/create?id=${item.dataset.openVacancy}`
      item.onclick=event=>{if(!event.target.closest('.publication-switch,.vacancy-card-actions'))open()}
      item.onkeydown=event=>{if((event.key==='Enter'||event.key===' ')&&!event.target.closest('.publication-switch,.vacancy-card-actions')){event.preventDefault();open()}}
    })
    document.querySelectorAll('[data-unpublish]').forEach(input=>input.onchange=()=>{if(!input.checked)unpublish(input)})
    document.querySelectorAll('[data-delete-vacancy]').forEach(button=>button.onclick=event=>{event.stopPropagation();deleteVacancy(button)})
  }
  async function render(){
    const main=document.querySelector('.dashboard-main')
    main.innerHTML=`<section class="my-vacancies-page"><header><div><small>УПРАВЛЕНИЕ ВАКАНСИЯМИ</small><h1>Мои вакансии</h1><p>Все созданные вакансии в одном месте. Откройте карточку, чтобы изменить любой шаг.</p></div><div class="my-vacancies-tools"><label><span>Показывать</span><select id="vacancy-status-filter"><option value="all">Все вакансии</option><option value="published">Только активные</option><option value="draft">Только черновики</option></select></label><a href="/vacancies/create">＋ Создать вакансию</a></div></header><div class="my-vacancies-loading">Загружаем вакансии…</div></section>`
    try{
      const vacancies=await api('/api/v1/vacancies')
      const page=main.querySelector('.my-vacancies-page')
      page.insertAdjacentHTML('beforeend',vacancies.length?`<div class="my-vacancies-stats"><span><b>${vacancies.length}</b><small>Всего</small></span><span><b>${vacancies.filter(item=>item.status==='published').length}</b><small>Опубликовано</small></span><span><b>${vacancies.filter(item=>item.status==='draft').length}</b><small>Черновиков</small></span></div><div class="my-vacancies-grid">${vacancies.map(card).join('')}</div>`:`<div class="my-vacancies-empty"><i>▣</i><h2>Пока нет вакансий</h2><p>Создайте первую вакансию — она появится в этом разделе.</p><a href="/vacancies/create">Создать вакансию</a></div>`)
      page.querySelector('.my-vacancies-loading').remove()
      bind()
      const filter=page.querySelector('#vacancy-status-filter'),grid=page.querySelector('.my-vacancies-grid')
      if(filter&&grid)filter.onchange=()=>{const items=filter.value==='all'?vacancies:vacancies.filter(item=>item.status===filter.value);grid.innerHTML=items.length?items.map(card).join(''):'<div class="my-vacancies-filter-empty">В этой категории вакансий пока нет</div>';bind()}
    }catch(error){main.querySelector('.my-vacancies-loading').innerHTML=`<b>Не удалось загрузить вакансии</b><span>${escapeHTML(error.message)}</span>`}
  }
  document.querySelectorAll('.create-action.vacancy-action,.welcome-vacancy').forEach(link=>link.href='/vacancies/create')
  const link=[...document.querySelectorAll('.profile-menu a')].find(item=>item.textContent.includes('Мои вакансии'))
  if(link){link.href='/profile?section=vacancies';if(new URLSearchParams(location.search).get('section')==='vacancies'){document.querySelectorAll('.profile-menu a').forEach(item=>item.classList.remove('active'));link.classList.add('active');render()}}
})()
