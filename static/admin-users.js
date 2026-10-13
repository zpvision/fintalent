document.head.insertAdjacentHTML('beforeend','<link rel="stylesheet" href="/static/admin-users.css?v=4">');
const usersNav=document.querySelector('#users-nav'),usersSection=document.querySelector('#users-section'),usersList=document.querySelector('#users-list'),usersEmpty=document.querySelector('#users-empty'),userModal=document.querySelector('#user-modal'),userForm=document.querySelector('#user-form'),passwordModal=document.querySelector('#password-modal'),passwordForm=document.querySelector('#password-form');
let adminUsers=[];
let usersStatus='active',usersSearch='';
const normalizeUserSearch=value=>String(value||'').trim().toLocaleLowerCase('ru-RU').replace(/ё/g,'е').replace(/\s+/g,' ');
const registrationDate=value=>value?new Date(value).toLocaleString('ru-RU',{day:'2-digit',month:'2-digit',year:'numeric',hour:'2-digit',minute:'2-digit'}):'—';

function hideUsersSection(){usersSection.classList.add('hidden');userModal.classList.add('hidden');passwordModal.classList.add('hidden')}
document.querySelector('.sidebar nav').addEventListener('click',event=>{const button=event.target.closest('button');if(button&&button!==usersNav)hideUsersSection()},true);

async function showUsers(){
  document.querySelectorAll('.sidebar nav button').forEach(button=>button.classList.remove('active'));usersNav.classList.add('active');
  [list,editor,document.querySelector('#survey-section'),document.querySelector('#admin-testing'),document.querySelector('#admin-other-dictionaries'),document.querySelector('#admin-duties')].filter(Boolean).forEach(section=>section.classList.add('hidden'));
  usersSection.classList.remove('hidden');
  const header=document.querySelector('.workspace>header');header.querySelector('h1').textContent='Пользователи';header.querySelector('p').textContent='Управление доступом и паролями пользователей';header.querySelectorAll('.primary').forEach(button=>button.classList.add('hidden'));
  try{adminUsers=await api('/api/admin/users');renderUsers()}catch(error){notify(error.message,true)}
}

function renderUsers(){
  let summary=usersSection.querySelector('.users-summary');
  if(!summary){summary=document.createElement('div');summary.className='users-summary';usersSection.prepend(summary)}
  const blocked=adminUsers.filter(user=>user.is_blocked).length;
  const active=adminUsers.length-blocked;
  summary.innerHTML=`<article><i>♙</i><span><small>ВСЕГО ПРОФИЛЕЙ</small><b>${adminUsers.length}</b><em>зарегистрированных пользователей</em></span></article><article class="actual"><i>✓</i><span><small>АКТУАЛЬНЫЕ ПРОФИ</small><b>${active}</b><em>незаблокированных пользователей</em></span></article><article class="blocked"><i>!</i><span><small>ЗАБЛОКИРОВАНО</small><b>${blocked}</b><em>${blocked?'доступ к аккаунтам закрыт':'заблокированных нет'}</em></span></article>`;
  let controls=usersSection.querySelector('.users-controls');
  if(!controls){
    controls=document.createElement('div');controls.className='users-controls';
    controls.innerHTML='<div class="users-tabs" role="group" aria-label="Статус пользователей"><button type="button" data-status="active">Актуальные <span></span></button><button type="button" data-status="blocked">Заблокированные <span></span></button></div><label class="users-search"><span>Поиск по почте или ФИО</span><input type="search" placeholder="Введите почту или ФИО" autocomplete="off"></label><p class="users-found" role="status" aria-live="polite"></p>';
    summary.after(controls);
    controls.querySelector('input').addEventListener('input',event=>{usersSearch=event.target.value;renderUsers()});
    controls.querySelectorAll('[data-status]').forEach(button=>button.onclick=()=>{usersStatus=button.dataset.status;renderUsers()});
  }
  controls.querySelectorAll('[data-status]').forEach(button=>{const selected=button.dataset.status===usersStatus;button.classList.toggle('active',selected);button.setAttribute('aria-pressed',String(selected));button.querySelector('span').textContent=button.dataset.status==='active'?active:blocked});
  const query=normalizeUserSearch(usersSearch);
  const visible=adminUsers.filter(user=>Boolean(user.is_blocked)===(usersStatus==='blocked')&&(!query||normalizeUserSearch(user.email).includes(query)||normalizeUserSearch(user.full_name).includes(query)));
  controls.querySelector('.users-found').textContent=`Показано: ${visible.length}`;
  usersEmpty.textContent=query?'Пользователи не найдены. Попробуйте изменить запрос.':usersStatus==='blocked'?'Заблокированных пользователей нет.':'Актуальных пользователей пока нет.';
  usersEmpty.classList.toggle('hidden',visible.length!==0);usersSection.querySelector('.users-table-wrap').classList.toggle('hidden',visible.length===0);
  usersList.innerHTML=visible.map(user=>`<tr class="${user.is_blocked?'blocked':''}" data-id="${user.id}"><td><b>${esc(user.email)}</b>${user.is_blocked?'<small>Аккаунт заблокирован</small>':''}</td><td>${esc(user.full_name)}</td><td class="user-created">${registrationDate(user.created_at)}</td><td>${user.resume_id?`<a class="secondary user-profile" href="/profiles/view/${user.resume_id}" target="_blank" rel="noopener">Открыть профиль ↗</a>`:'<span class="user-profile-empty">Нет опубликованного профиля</span>'}</td><td><button class="secondary user-courses" aria-pressed="${Boolean(user.courses_access)}">${user.courses_access?'Разрешён':'Разрешить'}</button></td><td><button class="secondary user-edit">Изменить</button></td><td><button class="${user.is_blocked?'secondary':'danger'} user-block">${user.is_blocked?'Разблокировать':'Заблокировать'}</button></td><td><button class="secondary user-password">Изменить пароль</button></td></tr>`).join('');
  usersList.querySelectorAll('.user-courses').forEach(button=>button.onclick=()=>toggleUserCourses(button));
  usersList.querySelectorAll('.user-edit').forEach(button=>button.onclick=()=>openUserModal(button.closest('tr')));
  usersList.querySelectorAll('.user-block').forEach(button=>button.onclick=()=>toggleUserBlock(button.closest('tr')));
  usersList.querySelectorAll('.user-password').forEach(button=>button.onclick=()=>openPasswordModal(button.closest('tr')));
}

function openUserModal(row){const user=adminUsers.find(item=>item.id===Number(row.dataset.id));userForm.elements.user_id.value=user.id;userForm.elements.full_name.value=user.full_name;userForm.elements.email.value=user.email;userModal.classList.remove('hidden');userForm.elements.full_name.focus()}
document.querySelector('#close-user-modal').onclick=()=>userModal.classList.add('hidden');
userModal.addEventListener('click',event=>{if(event.target===userModal)userModal.classList.add('hidden')});
userForm.addEventListener('submit',async event=>{event.preventDefault();const userID=Number(userForm.elements.user_id.value),full_name=userForm.elements.full_name.value,email=userForm.elements.email.value;const submit=userForm.querySelector('button[type="submit"],button:not([type])');submit.disabled=true;try{const updated=await api(`/api/admin/users/${userID}/profile`,{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({full_name,email})});const index=adminUsers.findIndex(item=>item.id===userID);if(index>=0)adminUsers[index]={...adminUsers[index],...updated};userModal.classList.add('hidden');renderUsers();notify('Данные пользователя изменены')}catch(error){notify(error.message,true)}finally{submit.disabled=false}});

async function toggleUserBlock(row){
  const user=adminUsers.find(item=>item.id===Number(row.dataset.id)),next=!user.is_blocked;
  if(next&&!confirm(`Заблокировать пользователя ${user.email}? Все активные сессии будут завершены.`))return;
  try{await api(`/api/admin/users/${user.id}/block`,{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({is_blocked:next})});user.is_blocked=next;renderUsers();notify(next?'Пользователь заблокирован':'Пользователь разблокирован')}catch(error){notify(error.message,true)}
}

function openPasswordModal(row){const user=adminUsers.find(item=>item.id===Number(row.dataset.id));passwordForm.reset();passwordForm.elements.user_id.value=user.id;document.querySelector('#password-user').textContent=`Новый пароль для ${user.email}. Активные сессии пользователя будут завершены.`;passwordModal.classList.remove('hidden');passwordForm.elements.password.focus()}
document.querySelector('#close-password-modal').onclick=()=>passwordModal.classList.add('hidden');
passwordModal.addEventListener('click',event=>{if(event.target===passwordModal)passwordModal.classList.add('hidden')});
passwordForm.addEventListener('submit',async event=>{event.preventDefault();const userID=Number(passwordForm.elements.user_id.value),password=passwordForm.elements.password.value;try{await api(`/api/admin/users/${userID}/password`,{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({password})});passwordModal.classList.add('hidden');notify('Пароль пользователя изменён')}catch(error){notify(error.message,true)}});
usersNav.onclick=showUsers;

async function toggleUserCourses(button){
 const user=adminUsers.find(item=>item.id===Number(button.closest('tr').dataset.id));button.disabled=true;
 try{const result=await api('/api/admin/users/'+user.id+'/courses-access',{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({courses_access:!user.courses_access})});user.courses_access=result.courses_access;renderUsers();notify(user.courses_access?'Доступ к курсам разрешён':'Индивидуальный доступ к курсам отключён')}catch(error){notify(error.message,true);button.disabled=false}
}
