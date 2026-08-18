(()=>{
  if(new URLSearchParams(location.search).get('section')!=='settings')return;
  document.head.insertAdjacentHTML('beforeend','<link rel="stylesheet" href="/static/profile-settings.css?v=1">');
  const main=document.querySelector('.dashboard-main');
  const links=[...document.querySelectorAll('.profile-menu a')];
  const settingsLink=links.find(link=>link.getAttribute('href')==='/profile?section=settings');
  links.forEach(link=>link.classList.remove('active'));
  settingsLink?.classList.add('active');
  const escapeHTML=value=>{const node=document.createElement('span');node.textContent=value??'';return node.innerHTML};
  const api=async(url,value)=>{const response=await fetch(url,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(value)});const data=await response.json().catch(()=>({}));if(!response.ok)throw Error(data.error||'Не удалось сохранить изменения');return data};
  const message=(form,text,error=false)=>{const box=form.querySelector('.settings-message');box.textContent=text;box.classList.toggle('error',error);box.hidden=false};

  async function render(){
    const response=await fetch('/api/me',{cache:'no-store'});
    if(!response.ok){location.href='/login';return}
    const user=await response.json();
    main.innerHTML=`<section class="profile-settings-page"><header><small>НАСТРОЙКИ ПРОФИЛЯ</small><h1>Личные данные и безопасность</h1><p>Обновите имя, которое видят другие пользователи, или установите новый пароль.</p></header><div class="settings-grid"><form id="name-settings"><div class="settings-icon">Aa</div><h2>ФИО</h2><p>Используется в профиле, резюме и ваших публикациях.</p><label><span>Фамилия, имя и отчество</span><input name="full_name" maxlength="200" autocomplete="name" value="${escapeHTML(user.full_name)}" required></label><div class="settings-message" hidden></div><button>Сохранить ФИО</button></form><form id="password-settings"><div class="settings-icon lock">●</div><h2>Смена пароля</h2><p>Для безопасности подтвердите, что аккаунт принадлежит вам.</p><label><span>Текущий пароль</span><input name="current_password" type="password" autocomplete="current-password" required></label><label><span>Новый пароль</span><input name="new_password" type="password" minlength="8" maxlength="72" autocomplete="new-password" required></label><label><span>Повторите новый пароль</span><input name="confirm_password" type="password" minlength="8" maxlength="72" autocomplete="new-password" required></label><div class="settings-message" hidden></div><button>Изменить пароль</button></form></div></section>`;
    const nameForm=main.querySelector('#name-settings'),passwordForm=main.querySelector('#password-settings');
    nameForm.onsubmit=async event=>{event.preventDefault();const button=nameForm.querySelector('button');button.disabled=true;try{const result=await api('/api/profile/name',{full_name:nameForm.elements.full_name.value});nameForm.elements.full_name.value=result.full_name;document.querySelectorAll('#header-name,#sidebar-name').forEach(node=>node.textContent=result.full_name);const initial=result.full_name.trim().charAt(0).toUpperCase();document.querySelectorAll('#header-avatar,#sidebar-avatar').forEach(node=>node.textContent=initial);message(nameForm,'ФИО успешно обновлено')}catch(error){message(nameForm,error.message,true)}finally{button.disabled=false}};
    passwordForm.onsubmit=async event=>{event.preventDefault();const current=passwordForm.elements.current_password.value,next=passwordForm.elements.new_password.value,confirm=passwordForm.elements.confirm_password.value;if(next!==confirm){message(passwordForm,'Новые пароли не совпадают',true);return}const button=passwordForm.querySelector('button');button.disabled=true;try{await api('/api/profile/password',{current_password:current,new_password:next});passwordForm.reset();message(passwordForm,'Пароль успешно изменён')}catch(error){message(passwordForm,error.message,true)}finally{button.disabled=false}};
  }
  render().catch(()=>{main.innerHTML='<section class="profile-settings-page"><h1>Не удалось загрузить настройки</h1></section>'});
})();
