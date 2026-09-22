(()=>{
  document.head.insertAdjacentHTML('beforeend','<link rel="stylesheet" href="/static/profile-help.css?v=2">')
  const escapeHTML=value=>{const node=document.createElement('span');node.textContent=value??'';return node.innerHTML}
  const statusText={new:'Новое',accepted:'Принято',declined:'Отклонено',completed:'Завершено',cancelled:'Отменено'}
  let activeScope='incoming',me=null
  async function api(path,options={}){
    const response=await fetch(path,{cache:'no-store',headers:{'Content-Type':'application/json',...(options.headers||{})},...options})
    const data=await response.json().catch(()=>({}))
    if(response.status===401){location.href='/login';throw Error('Требуется авторизация')}
    if(!response.ok)throw Error(data.error||'Не удалось выполнить действие')
    return data
  }
  function notify(text,error=false){
    window.showFinTalentError&&error?window.showFinTalentError(text):showToast(text,error)
  }
  function showToast(text,error=false){
    let box=document.querySelector('.profile-help-toast')
    if(!box){box=document.createElement('div');box.className='profile-vacancy-message profile-help-toast';document.body.append(box)}
    box.textContent=text;box.classList.toggle('error',error);box.classList.add('visible')
    setTimeout(()=>box.classList.remove('visible'),2600)
  }
  function topicIcon(topic){
    const icon=String(topic.icon||'').trim()
    return/^\/|^https?:\/\//i.test(icon)?`<img src="${escapeHTML(icon)}" alt="">`:escapeHTML(icon||'◇')
  }
  function date(value){return value?new Date(value).toLocaleDateString('ru-RU',{day:'2-digit',month:'long',year:'numeric'}):''}
  function person(item){return activeScope==='incoming'?item.requester:item.expert}
  function card(item){
    const p=person(item),incoming=activeScope==='incoming'
    return`<article class="profile-help-card" data-request="${item.id}">
      <header><img src="${escapeHTML(p.avatar||'/static/avatar-placeholder.svg')}" alt=""><div><h2>${p.profile_id?`<a class="profile-help-person-link" href="/profiles/view/${Number(p.profile_id)}">${escapeHTML(p.name)}</a>`:escapeHTML(p.name)}</h2><small>${topicIcon(item.topic)} ${escapeHTML(item.topic.name)} · создано ${escapeHTML(date(item.created_at))}</small></div><span class="help-status ${escapeHTML(item.status)}">${escapeHTML(statusText[item.status]||item.status)}</span></header>
      <p>${escapeHTML(item.text)}</p>
      ${!incoming&&['accepted','completed'].includes(item.status)&&item.acceptance_message?`<div class="profile-help-accept-message"><span>Ответ специалиста</span><p>${escapeHTML(item.acceptance_message)}</p></div>`:''}
      ${!incoming&&item.status==='declined'&&item.decline_reason?`<div class="profile-help-decline-reason"><span>Почему специалист не смог помочь</span><p>${escapeHTML(item.decline_reason)}</p></div>`:''}
      <div class="profile-help-actions">
        ${incoming&&item.status==='new'?`<button class="good" data-action="accept">Принять</button><button class="danger" data-action="decline">Отклонить</button>`:''}
        ${incoming&&item.status==='accepted'?`<button class="good" data-action="complete">Отметить завершенной</button>`:''}
        ${!incoming&&(item.status==='new'||item.status==='accepted')?`<button class="danger" data-action="cancel">Отменить запрос</button>`:''}
        ${(item.status==='accepted'||item.status==='completed')?`<button class="primary" data-open-messages>Переписка · ${item.messages_count||0}</button>`:''}
        ${item.can_review?'<button class="good" data-open-review>Оставить отзыв</button>':''}
      </div>
    </article>`
  }
  async function render(){
    const main=document.querySelector('.dashboard-main')
    main.innerHTML=`<section class="profile-help-page"><header><div><small>ПРОФЕССИОНАЛЬНЫЙ ПРОФИЛЬ</small><h1>Помощь коллегам</h1><p>Входящие обращения по вашим направлениям помощи и ваши запросы другим специалистам.</p></div><div class="profile-help-tabs"><button data-scope="incoming" class="${activeScope==='incoming'?'active':''}">Мне написали</button><button data-scope="outgoing" class="${activeScope==='outgoing'?'active':''}">Мои запросы</button></div></header><div class="profile-help-list"><div class="profile-help-empty">Загружаем обращения…</div></div></section>`
    main.querySelectorAll('[data-scope]').forEach(button=>button.onclick=()=>{activeScope=button.dataset.scope;render()})
    try{
      const items=await api(`/api/v1/help/requests?scope=${activeScope}`)
      const list=main.querySelector('.profile-help-list')
      list.innerHTML=items.length?items.map(card).join(''):`<div class="profile-help-empty">${activeScope==='incoming'?'Новых обращений пока нет':'Вы пока не отправляли запросы помощи'}</div>`
      list.querySelectorAll('[data-action]').forEach(button=>button.onclick=()=>{const id=button.closest('[data-request]').dataset.request,action=button.dataset.action;action==='decline'?openDecline(id):action==='accept'?openAccept(id):runAction(id,action)})
      list.querySelectorAll('[data-open-messages]').forEach(button=>button.onclick=()=>openMessages(button.closest('[data-request]').dataset.request,items.find(item=>String(item.id)===String(button.closest('[data-request]').dataset.request))))
      list.querySelectorAll('[data-open-review]').forEach(button=>button.onclick=()=>openReview(button.closest('[data-request]').dataset.request))
    }catch(error){
      main.querySelector('.profile-help-list').innerHTML=`<div class="profile-help-empty">Не удалось загрузить обращения: ${escapeHTML(error.message)}</div>`
    }
  }
  function openDecline(id){
    const modal=document.createElement('div')
    modal.className='profile-help-modal'
    modal.innerHTML=`<div class="profile-help-dialog profile-help-decline-dialog" role="dialog" aria-modal="true"><header><div><small>ОТКЛОНЕНИЕ ЗАПРОСА</small><h2>Коротко укажите причину</h2><p>Коллега увидит ваше пояснение и поймёт, почему сейчас вы не можете помочь.</p></div><button class="close" type="button" aria-label="Закрыть">×</button></header><textarea maxlength="500" placeholder="Например: сейчас нет возможности взять запрос или вопрос не относится к моей специализации"></textarea><div class="profile-help-reason-count">0 / 500</div><footer><button type="button" class="cancel">Отмена</button><button type="button" class="danger-primary" disabled>Отклонить запрос</button></footer></div>`
    document.body.append(modal)
    const textarea=modal.querySelector('textarea'),submit=modal.querySelector('.danger-primary'),close=()=>modal.remove()
    modal.querySelector('.close').onclick=close;modal.querySelector('.cancel').onclick=close;modal.onclick=event=>{if(event.target===modal)close()}
    textarea.oninput=()=>{modal.querySelector('.profile-help-reason-count').textContent=`${textarea.value.length} / 500`;submit.disabled=textarea.value.trim().length<3}
    submit.onclick=async()=>{if(await runAction(id,'decline',{reason:textarea.value.trim()}))close()}
    textarea.focus()
  }
  function openAccept(id){
    const modal=document.createElement('div')
    modal.className='profile-help-modal'
    modal.innerHTML=`<div class="profile-help-dialog profile-help-accept-dialog" role="dialog" aria-modal="true"><header><div><small>ПРИНЯТИЕ ЗАПРОСА</small><h2>Ответьте коллеге</h2><p>Ваш ответ и контакты сразу увидит автор запроса. После этого откроется переписка.</p></div><button class="close" type="button" aria-label="Закрыть">×</button></header><textarea maxlength="1000" placeholder="Напишите ответ и укажите удобные контакты для связи"></textarea><div class="profile-help-reason-count">0 / 1000</div><footer><button type="button" class="cancel">Отмена</button><button type="button" class="accept-primary" disabled>Принять и отправить ответ</button></footer></div>`
    document.body.append(modal)
    const textarea=modal.querySelector('textarea'),submit=modal.querySelector('.accept-primary'),close=()=>modal.remove()
    modal.querySelector('.close').onclick=close;modal.querySelector('.cancel').onclick=close;modal.onclick=event=>{if(event.target===modal)close()}
    textarea.oninput=()=>{modal.querySelector('.profile-help-reason-count').textContent=`${textarea.value.length} / 1000`;submit.disabled=textarea.value.trim().length<3}
    submit.onclick=async()=>{if(await runAction(id,'accept',{message:textarea.value.trim()}))close()}
    textarea.focus()
  }
  async function runAction(id,action,payload=null){
    try{
      await api(`/api/v1/help/requests/${id}/${action}`,{method:'POST',body:payload?JSON.stringify(payload):null})
      notify('Статус обращения обновлен')
      render();updateBadge()
      return true
    }catch(error){notify(error.message,true);return false}
  }
  async function openMessages(id,item){
    const modal=document.createElement('div')
    modal.className='profile-help-modal'
    modal.innerHTML=`<div class="profile-help-dialog"><header><div><h2>Переписка</h2><p>${escapeHTML(item?.topic?.name||'Обращение')}</p></div><button class="close" type="button">×</button></header><div class="help-message-list">Загружаем сообщения…</div>${item?.status==='accepted'?'<textarea maxlength="4000" placeholder="Напишите сообщение"></textarea><footer><button type="button" class="primary">Отправить</button></footer>':''}</div>`
    document.body.append(modal)
    const close=()=>modal.remove()
    modal.querySelector('.close').onclick=close
    modal.onclick=event=>{if(event.target===modal)close()}
    async function loadMessages(){
      const messages=await api(`/api/v1/help/requests/${id}/messages`)
      modal.querySelector('.help-message-list').innerHTML=messages.length?messages.map(message=>`<article class="help-message ${me&&Number(message.author.id)===Number(me.id)?'mine':''}"><b>${escapeHTML(message.author.name)}</b><span>${escapeHTML(message.text)}</span><small>${new Date(message.created_at).toLocaleString('ru-RU')}</small></article>`).join(''):'<div class="profile-help-empty">Сообщений пока нет</div>'
    }
    try{await loadMessages()}catch(error){modal.querySelector('.help-message-list').textContent=error.message}
    modal.querySelector('footer .primary')?.addEventListener('click',async()=>{
      const textarea=modal.querySelector('textarea'),text=textarea.value.trim()
      try{await api(`/api/v1/help/requests/${id}/messages`,{method:'POST',body:JSON.stringify({text})});textarea.value='';await loadMessages();render()}catch(error){notify(error.message,true)}
    })
  }
  function openReview(id){
    let rating=5
    const modal=document.createElement('div')
    modal.className='profile-help-modal'
    modal.innerHTML=`<div class="profile-help-dialog"><header><div><h2>Оставить отзыв</h2><p>Отзыв будет привязан к завершенному обращению.</p></div><button class="close" type="button">×</button></header><div class="review-stars">${[1,2,3,4,5].map(value=>`<button type="button" class="${value<=rating?'active':''}" data-rating="${value}">★</button>`).join('')}</div><textarea maxlength="4000" placeholder="Расскажите, чем специалист помог"></textarea><footer><button type="button" class="primary">Опубликовать отзыв</button></footer></div>`
    document.body.append(modal)
    const close=()=>modal.remove()
    modal.querySelector('.close').onclick=close
    modal.onclick=event=>{if(event.target===modal)close()}
    modal.querySelectorAll('[data-rating]').forEach(button=>button.onclick=()=>{rating=Number(button.dataset.rating);modal.querySelectorAll('[data-rating]').forEach(star=>star.classList.toggle('active',Number(star.dataset.rating)<=rating))})
    modal.querySelector('footer .primary').onclick=async()=>{try{await api(`/api/v1/help/requests/${id}/review`,{method:'POST',body:JSON.stringify({rating,text:modal.querySelector('textarea').value.trim()})});notify('Отзыв опубликован');close();render()}catch(error){notify(error.message,true)}}
  }
  async function updateBadge(){
    try{
      const data=await api('/api/v1/help/notifications')
      const link=document.querySelector('[data-profile-help-link]')
      link?.querySelector('.help-menu-badge')?.remove()
      if(link&&data.incoming_new>0)link.insertAdjacentHTML('beforeend',`<b class="help-menu-badge">${data.incoming_new}</b>`)
    }catch{}
  }
  function installMenu(){
    if(document.querySelector('[data-profile-help-link]'))return
    const group=document.querySelector('.profile-menu .menu-group')||document.querySelector('.profile-menu>div')
    if(!group)return
    group.insertAdjacentHTML('beforeend','<a href="/profile?section=help" data-profile-help-link><i>🤝</i><span>Помощь коллегам</span></a>')
  }
  async function init(){
    installMenu()
    me=await api('/api/me').catch(()=>null)
    updateBadge()
    if(new URLSearchParams(location.search).get('section')==='help'){
      document.querySelectorAll('.profile-menu a').forEach(item=>item.classList.remove('active'))
      document.querySelector('[data-profile-help-link]')?.classList.add('active')
      render()
    }
  }
  init()
})()
