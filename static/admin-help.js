(()=>{
  document.head.insertAdjacentHTML('beforeend','<link rel="stylesheet" href="/static/admin-help.css?v=1">')
  const escapeHTML=value=>{const node=document.createElement('span');node.textContent=value??'';return node.innerHTML}
  const section=document.createElement('section')
  section.id='help-topics-admin'
  section.className='help-admin-page hidden'
  document.querySelector('.workspace')?.append(section)
  const nav=document.createElement('button')
  nav.id='help-topics-nav'
  nav.innerHTML='🤝 <span>Могу помочь</span>'
  document.querySelector('.sidebar nav')?.append(nav)
  let topics=[]

  function setHeader(){
    const header=document.querySelector('.workspace>header')
    header.querySelector('h1').textContent='Могу помочь'
    header.querySelector('p').textContent='Справочник направлений, в которых специалисты готовы помогать коллегам'
    header.querySelectorAll('.primary').forEach(button=>button.classList.add('hidden'))
  }
  function hideKnownSections(){
    ['#dictionary-list','#dictionary-editor','#survey-section','#vacancy-survey-section','#admin-testing','#other-dictionaries-section','#users-section','#admin-publications'].forEach(selector=>document.querySelector(selector)?.classList.add('hidden'))
    document.querySelectorAll('.workspace header .primary').forEach(button=>button.classList.add('hidden'))
  }
  function iconHTML(topic){
    const icon=String(topic.icon||'').trim()
    if(/^\/|^https?:\/\//i.test(icon))return`<span class="help-topic-icon"><img src="${escapeHTML(icon)}" alt=""></span>`
    return`<span class="help-topic-icon">${escapeHTML(icon||'◇')}</span>`
  }
  function card(topic){
    return`<article class="help-topic-card ${topic.active?'':'inactive'}">
      <header>${iconHTML(topic)}<div><h3>${escapeHTML(topic.name)}</h3><small>${escapeHTML(topic.category||'Без категории')} · ${topic.active?'активен':'выключен'}</small></div></header>
      <p>${escapeHTML(topic.short_description||'Короткое описание пока не заполнено.')}</p>
      <footer><span class="help-topic-order">№ ${Number(topic.sort_order)||0}</span><div class="help-topic-actions"><button type="button" data-edit-help="${topic.id}">Править</button><button type="button" class="delete" data-delete-help="${topic.id}">Удалить</button></div></footer>
    </article>`
  }
  async function loadTopics(){
    topics=await api('/api/admin/help-topics')
  }
  async function render(){
    setHeader()
    section.innerHTML=`<div class="help-admin-toolbar"><div><h2>Направления помощи</h2><p>Название, категория, иконка, описание, активность и порядок сортировки редактируются здесь.</p></div><button id="add-help-topic" class="primary">＋ Добавить направление</button></div><div class="help-admin-loading">Загружаем направления…</div>`
    try{
      await loadTopics()
      section.querySelector('.help-admin-loading').outerHTML=topics.length?`<div class="help-admin-grid">${topics.map(card).join('')}</div>`:'<div class="help-admin-empty">Направлений пока нет</div>'
      section.querySelector('#add-help-topic').onclick=()=>openEditor()
      section.querySelectorAll('[data-edit-help]').forEach(button=>button.onclick=()=>openEditor(topics.find(item=>item.id===Number(button.dataset.editHelp))))
      section.querySelectorAll('[data-delete-help]').forEach(button=>button.onclick=()=>deleteTopic(Number(button.dataset.deleteHelp)))
    }catch(error){
      section.querySelector('.help-admin-loading').innerHTML=`<b>Не удалось загрузить направления</b><span>${escapeHTML(error.message)}</span>`
    }
  }
  function showHelpAdmin(){
    document.querySelectorAll('.sidebar nav button').forEach(button=>button.classList.remove('active'))
    nav.classList.add('active')
    hideKnownSections()
    section.classList.remove('hidden')
    render()
  }
  function openEditor(topic={name:'',category:'',icon:'',short_description:'',active:true,sort_order:(topics.at(-1)?.sort_order||0)+10}){
    const modal=document.createElement('div')
    modal.className='help-topic-modal'
    modal.innerHTML=`<form>
      <h2>${topic.id?'Редактировать направление':'Новое направление'}</h2>
      <label>Название<input name="name" maxlength="200" required value="${escapeHTML(topic.name)}"></label>
      <div class="row"><label>Категория<input name="category" maxlength="160" value="${escapeHTML(topic.category)}"></label><label>Порядок<input name="sort_order" type="number" value="${Number(topic.sort_order)||0}"></label></div>
      <label>Иконка<input name="icon" maxlength="500" placeholder="Emoji или путь к SVG/PNG" value="${escapeHTML(topic.icon)}"></label>
      <label>Короткое описание<textarea name="short_description" maxlength="2000">${escapeHTML(topic.short_description)}</textarea></label>
      <label class="active-line"><input name="active" type="checkbox" ${topic.active!==false?'checked':''}> Активен</label>
      <div class="actions"><button type="button" class="secondary">Отмена</button><button class="primary">Сохранить</button></div>
    </form>`
    document.body.append(modal)
    modal.querySelector('.secondary').onclick=()=>modal.remove()
    modal.onclick=event=>{if(event.target===modal)modal.remove()}
    modal.querySelector('form').onsubmit=async event=>{
      event.preventDefault()
      const form=new FormData(event.currentTarget)
      const payload={
        name:String(form.get('name')).trim(),
        category:String(form.get('category')).trim(),
        icon:String(form.get('icon')).trim(),
        short_description:String(form.get('short_description')).trim(),
        active:form.get('active')==='on',
        sort_order:Number(form.get('sort_order'))||0
      }
      try{
        await api(topic.id?`/api/admin/help-topics/${topic.id}`:'/api/admin/help-topics',{method:topic.id?'PUT':'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(payload)})
        modal.remove()
        notify('Направление помощи сохранено')
        render()
      }catch(error){notify(error.message,true)}
    }
  }
  async function deleteTopic(id){
    const topic=topics.find(item=>item.id===id)
    if(!confirm(`Удалить направление «${topic?.name||id}»? Оно будет выключено и скрыто из выбора.`))return
    try{
      await api(`/api/admin/help-topics/${id}`,{method:'DELETE'})
      notify('Направление помощи удалено')
      render()
    }catch(error){notify(error.message,true)}
  }
  nav.onclick=showHelpAdmin
  document.querySelectorAll('.sidebar nav button:not(#help-topics-nav)').forEach(button=>button.addEventListener('click',()=>section.classList.add('hidden')))
})()
