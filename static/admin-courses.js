(() => {
  const nav = document.querySelector('.sidebar nav'), workspace = document.querySelector('.workspace')
  if (!nav || !workspace) return
  const studentStyles=document.createElement('link');studentStyles.rel='stylesheet';studentStyles.href='/static/admin-course-students.css?v=1';document.head.append(studentStyles)
  const button = document.createElement('button')
  button.id = 'courses-admin-nav'; const navLabel = document.createElement('span'); navLabel.textContent = 'Курсы'; button.append(document.createTextNode('▤ '), navLabel); nav.append(button)
  const section = document.createElement('section'); section.id = 'admin-courses'; section.className = 'hidden'; workspace.insertBefore(section, document.querySelector('#dictionary-editor'))
  const labels = { draft: 'Черновик', moderation: 'На модерации', published: 'Опубликован', hidden: 'Скрыт' }
  let selected = 'moderation'
  let area = 'access'
  let studentsPage = 1
  let studentsRequest = 0
  let studentFilters = {author_id:'',student_name:'',email:''}
  const money = value => value==null?'—':new Intl.NumberFormat('ru-RU',{style:'currency',currency:'RUB',maximumFractionDigits:2}).format(Number(value)/100)
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
  async function loadStudents() {
    section.replaceChildren(tabs(),node('p','Загружаем заявки учеников…'))
    const page=studentsPage,version=++studentsRequest
    try {
      const query=new URLSearchParams({...studentFilters,page:String(page)})
      const data=await request(`/api/admin/courses/students?${query}`)
      if(area!=='students'||studentsPage!==page||version!==studentsRequest)return
      if(page>1&&!data.items.length){studentsPage=Math.max(1,Math.ceil(data.total/data.page_size));return loadStudents()}
      section.replaceChildren(tabs(),node('h2','Ученики'),node('p','Заявки на курсы и контакты для связи. Для старых заявок без сохранённых контактов показаны доступные данные аккаунта.'))
      const overall=node('div','','course-students-totals');overall.dataset.overall=''
      for(const [label,value] of [['Всего заявок',data.overall.applications],['Всего учеников',data.overall.students],['Общая сумма тарифов по заявкам',money(data.overall.total_price)]]){const card=node('div');card.append(node('small',label),node('strong',String(value)));overall.append(card)}
      section.append(overall,node('p','Ученики считаются без повторов по аккаунту. Сумма тарифов — стоимость заявок, а не подтверждённая выручка.','course-students-hint'))
      const form=node('form','','course-students-filters')
      const authorLabel=node('label','Автор курса'),authorSelect=node('select');authorSelect.name='author_id'
      const allAuthors=node('option','Все авторы');allAuthors.value='';authorSelect.append(allAuthors)
      for(const author of data.authors){const option=node('option',author.name);option.value=String(author.id);authorSelect.append(option)}
      if(studentFilters.author_id&&!data.authors.some(a=>String(a.id)===studentFilters.author_id)){const option=node('option','Выбранный автор');option.value=studentFilters.author_id;authorSelect.append(option)}
      authorSelect.value=studentFilters.author_id;authorLabel.append(authorSelect);form.append(authorLabel)
      for(const [key,label,placeholder,max] of [['student_name','ФИО ученика','Имя или часть ФИО',200],['email','Email ученика','Email или его часть',254]]){const field=node('label',label),input=node('input');input.name=key;input.type='search';input.placeholder=placeholder;input.maxLength=max;input.value=studentFilters[key];field.append(input);form.append(field)}
      const find=node('button','Найти','secondary'),reset=node('button','Сбросить','secondary');find.type='submit';reset.type='button';form.append(find,reset)
      form.onsubmit=event=>{event.preventDefault();studentFilters=Object.fromEntries([...new FormData(form)].map(([key,value])=>[key,value.trim()]));studentsPage=1;loadStudents()}
      reset.onclick=()=>{studentFilters={author_id:'',student_name:'',email:''};studentsPage=1;loadStudents()}
      section.append(form)
      const found=node('p',`Найдено заявок: ${data.total} · Учеников: ${data.summary.students} · Сумма тарифов: ${money(data.summary.total_price)}`,'course-students-found');found.setAttribute('role','status');found.dataset.filtered='';section.append(found)
      for(const [stats,label] of [[data.overall,'Всего'],[data.summary,'По фильтрам']])if(stats.unknown_prices||stats.current_prices)section.append(node('p',`${label}: ${stats.current_prices} заявок рассчитано по текущей цене (историческая цена не сохранена); ${stats.unknown_prices} заявок без известной цены не включено в сумму.`,'course-students-hint'))
      const table=node('table','','users-table'),head=node('thead'),heading=node('tr'),body=node('tbody')
      for(const label of ['Название курса','Выбранный тариф','Сумма тарифа','Автор курса','ФИО ученика','Email ученика','Телефон ученика','Дата заявки','Действия'])heading.append(node('th',label))
      head.append(heading);table.append(head)
      for(const item of data.items){
        const row=node('tr'),course=node('td'),link=node('a',item.course_title||'Без названия');row.dataset.applicationId=String(item.id)
        link.href=`/admin/courses/${item.course_id}/preview`;link.target='_blank';link.rel='noopener noreferrer';course.append(link);row.append(course)
        row.append(node('td',item.tariff_name||'Без тарифа'))
        const price=node('td',money(item.selected_price));if(item.price_is_current&&item.selected_price!=null)price.append(node('small','Текущая цена'));row.append(price)
        for(const value of [item.author_name,item.student_name,item.contact_email,item.contact_phone||'—',new Date(item.created_at).toLocaleString('ru-RU')])row.append(node('td',value||'—'))
        const actions=node('td')
        if(item.can_delete){const remove=node('button','Удалить','secondary');remove.type='button';remove.dataset.deleteApplication=String(item.id);remove.onclick=async()=>{
          if(!confirm(`Удалить заявку «${item.student_name}» на курс «${item.course_title}»? Аккаунт ученика и доступ к обучению сохранятся.`))return
          remove.disabled=true
          try{await api(`/api/admin/courses/students/${item.id}`,{method:'DELETE'});if(area==='students')await loadStudents()}catch(error){notify(error.message,true);remove.disabled=false}
        };actions.append(remove)}else{actions.append(node('span','Оплачена / зачислена'));actions.append(node('small','Удаление недоступно'))}
        row.append(actions)
        body.append(row)
      }
      table.append(body);const wrap=node('div','','users-table-wrap course-students-table');wrap.append(table);section.append(wrap)
      if(!data.items.length)section.append(node('p','По выбранным фильтрам заявок нет.'))
      const pager=node('div','','pm-admin-tabs'),prev=node('button','← Назад'),next=node('button','Далее →')
      prev.disabled=page<=1;next.disabled=page*data.page_size>=data.total
      prev.onclick=()=>{studentsPage--;loadStudents()};next.onclick=()=>{studentsPage++;loadStudents()}
      pager.append(prev,node('span',`Найдено заявок: ${data.total} · Страница ${page} из ${Math.max(1,Math.ceil(data.total/data.page_size))}`),next);section.append(pager)
    }catch(e){if(area==='students'&&version===studentsRequest)section.replaceChildren(tabs(),node('p',e.message))}
  }
  function tabs() {
    const tabs = node('div', '', 'pm-admin-tabs')
    for (const [key, label] of [['access', 'Доступ'], ['courses', 'Курсы и модерация'], ['students', 'Ученики'], ['course_categories', 'Категории курсов'], ['course_features', 'Возможности курса'], ['course_lesson_types', 'Типы уроков']]) {
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
    if (area === 'students') { await loadStudents(); return }
    if (area === 'courses') { await loadCourses(); section.prepend(tabs()); return }
    try { dictionaries = await api('/api/admin/dictionaries'); renderCourseDictionaries() } catch (e) { notify(e.message, true) }
  }
  const originalRenderList = renderList
  renderList = function () {
    originalRenderList()
    if (button.classList.contains('active') && !['courses','access','students'].includes(area)) { list.classList.add('hidden'); renderCourseDictionaries() }
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
