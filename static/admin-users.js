document.head.insertAdjacentHTML('beforeend','<link rel="stylesheet" href="/static/admin-users.css?v=1">');
const usersNav=document.querySelector('#users-nav'),usersSection=document.querySelector('#users-section'),usersList=document.querySelector('#users-list'),usersEmpty=document.querySelector('#users-empty'),passwordModal=document.querySelector('#password-modal'),passwordForm=document.querySelector('#password-form');
let adminUsers=[];

function hideUsersSection(){usersSection.classList.add('hidden');passwordModal.classList.add('hidden')}
document.querySelector('.sidebar nav').addEventListener('click',event=>{const button=event.target.closest('button');if(button&&button!==usersNav)hideUsersSection()},true);

async function showUsers(){
  document.querySelectorAll('.sidebar nav button').forEach(button=>button.classList.remove('active'));usersNav.classList.add('active');
  [list,editor,document.querySelector('#survey-section'),document.querySelector('#admin-testing'),document.querySelector('#admin-other-dictionaries'),document.querySelector('#admin-duties')].filter(Boolean).forEach(section=>section.classList.add('hidden'));
  usersSection.classList.remove('hidden');
  const header=document.querySelector('.workspace>header');header.querySelector('h1').textContent='Пользователи';header.querySelector('p').textContent='Управление доступом и паролями пользователей';header.querySelectorAll('.primary').forEach(button=>button.classList.add('hidden'));
  try{adminUsers=await api('/api/admin/users');renderUsers()}catch(error){notify(error.message,true)}
}

function renderUsers(){
  usersEmpty.classList.toggle('hidden',adminUsers.length!==0);document.querySelector('.users-table-wrap').classList.toggle('hidden',adminUsers.length===0);
  usersList.innerHTML=adminUsers.map(user=>`<tr class="${user.is_blocked?'blocked':''}" data-id="${user.id}"><td><b>${esc(user.email)}</b>${user.is_blocked?'<small>Аккаунт заблокирован</small>':''}</td><td>${esc(user.full_name)}</td><td><button class="${user.is_blocked?'secondary':'danger'} user-block">${user.is_blocked?'Разблокировать':'Заблокировать'}</button></td><td><button class="secondary user-password">Изменить пароль</button></td></tr>`).join('');
  usersList.querySelectorAll('.user-block').forEach(button=>button.onclick=()=>toggleUserBlock(button.closest('tr')));
  usersList.querySelectorAll('.user-password').forEach(button=>button.onclick=()=>openPasswordModal(button.closest('tr')));
}

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
