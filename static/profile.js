document.head.insertAdjacentHTML('beforeend','<link rel="stylesheet" href="/static/profile-logo.css"><link rel="stylesheet" href="/static/profile-sidebar-v2.css">');document.querySelector('.brand-symbol').innerHTML='<img src="/static/logo.png" alt="">';document.querySelector('.profile-menu').innerHTML=`<div class="menu-group"><a class="home-link" href="/"><i>⌂</i><span>На главную</span></a><a href="/#jobs"><i>▣</i><span>Поиск работы</span></a><a href="#"><i>♧</i><span>Мои отклики</span></a><a href="#"><i>☆</i><span>Избранные вакансии</span></a><a href="#"><i>◇</i><span>Сообщения</span><b class="menu-badge">8</b></a></div><div class="menu-group"><small>Для компаний</small><a href="#"><i>⌕</i><span>Поиск специалистов</span></a><a href="#"><i>▣</i><span>Мои вакансии</span></a><a href="#"><i>□</i><span>Отклики</span></a><a href="#"><i>♧</i><span>Тестирование</span></a><a href="#"><i>▥</i><span>Аналитика</span></a></div><div class="menu-group"><small>О компании</small><a class="active" href="/profile"><i>▤</i><span>Профиль</span></a><a href="#"><i>▣</i><span>Отзывы и рейтинги</span></a><a href="#"><i>♙</i><span>Мои сотрудники</span><b class="menu-badge">24</b></a><a href="#"><i>▢</i><span>Подписка и услуги</span></a><a href="#"><i>⚙</i><span>Настройки</span></a></div>`;
document.head.insertAdjacentHTML('beforeend','<link rel="stylesheet" href="/static/profile-buttons.css">');
document.head.insertAdjacentHTML('beforeend','<link rel="stylesheet" href="/static/fintalent-theme.css">');
(async()=>{const response=await fetch('/api/me');if(!response.ok){location.href='/login';return}const user=await response.json(),name=String(user.full_name||'Пользователь').trim()||'Пользователь',initial=name.charAt(0).toUpperCase(),set=(selector,value)=>{const element=document.querySelector(selector);if(element)element.textContent=value};set('#header-name',name);set('#sidebar-name',name);set('#sidebar-email',user.email||'');set('#header-avatar',initial);set('#sidebar-avatar',initial)})();
document.querySelector('#logout')?.addEventListener('click',async()=>{await fetch('/api/logout',{method:'POST'});location.href='/'});
const testsLink=[...document.querySelectorAll('.profile-menu a')].find(a=>a.textContent.includes('Тест'));if(testsLink)testsLink.href='/tests';
const companyGroup=document.querySelectorAll('.profile-menu .menu-group')[2];if(companyGroup){companyGroup.insertAdjacentHTML('afterbegin','<a class="profile-market-link" href="/profile?section=profimarket"><i>◇</i><span>ПрофиМаркет</span></a>')}const marketScript=document.createElement('script');marketScript.src='/static/profile-profimarket.js?v=2';document.body.appendChild(marketScript);if(location.pathname==='/tests'){const employeeTestingScript=document.createElement('script');employeeTestingScript.src='/static/employee-testing.js?v=4';document.body.appendChild(employeeTestingScript)}
(async()=>{
 try{
  const response=await fetch('/api/v1/resumes/status',{cache:'no-store'});
  if(!response.ok)return;
  const resume=await response.json();
  if(!resume.published)return;
  document.querySelectorAll('.resume-action,.welcome-resume').forEach(link=>{
   const title=link.querySelector('b'),description=link.querySelector('small');
   if(title)title.textContent='Моё резюме (просмотр)';
   if(description)description.textContent='Посмотреть опубликованное резюме';
   link.href=`/resume/view/${resume.id}`;
   link.setAttribute('aria-label','Моё резюме — просмотр');
  });
 }catch{}
})();

// Keep the profile navigation focused on the sections that are currently useful.
(()=>{
 const groups=[...document.querySelectorAll('.profile-menu .menu-group')],links=group=>[...(group?.querySelectorAll(':scope > a')||[])];
 const personal=links(groups[0]),company=links(groups[1]),about=links(groups[2]);
 if(company[1])company[1].href='/profile?section=vacancies';
 if(about.length)about[about.length-1].href='/profile?section=settings';
 [personal[1],personal[2],personal[4],company[0],company[2],company[4]].forEach(link=>link?.remove());
 const icons=[
  '<path d="M3 11.5 12 4l9 7.5M5.5 10v10h13V10M9 20v-6h6v6"/>',
  '<path d="m12 3 2.7 5.5 6.1.9-4.4 4.3 1 6.1-5.4-2.9-5.4 2.9 1-6.1-4.4-4.3 6.1-.9L12 3Z"/>',
  '<path d="M4 5.5h16v13H4zM8 5.5v-2h8v2M8 10h8M8 14h5"/>',
  '<path d="M5 4h14v16H5zM8 8h8M8 12h8M8 16h5"/>',
  '<path d="M12 21c4-2.1 7-5.5 7-10V5l-7-2-7 2v6c0 4.5 3 7.9 7 10Zm-3-9 2 2 4-4"/>',
  '<path d="M4 19V9l8-5 8 5v10M8 19v-5h8v5M3 19h18"/>',
  '<path d="M8 12h8M12 8v8M5 4h14v16H5z"/>',
  '<path d="M7 13.5 10 16l7-8M4 4h16v16H4z"/>',
  '<path d="M16 20v-2a4 4 0 0 0-4-4H7a4 4 0 0 0-4 4v2M9.5 10a4 4 0 1 0 0-8 4 4 0 0 0 0 8ZM17 11l2 2 3-4"/>',
  '<path d="M5 7h14M7 3v4m10-4v4M5 7v14h14V7M9 12h6M9 16h4"/>',
  '<path d="M12 15.5a3.5 3.5 0 1 0 0-7 3.5 3.5 0 0 0 0 7Zm8-3.5 2-1-2-3-2.2.5a8 8 0 0 0-1.3-.8L16 4h-4l-.5 2.7a8 8 0 0 0-1.3.8L8 7 6 10l2 1a8 8 0 0 0 0 2l-2 1 2 3 2.2-.5a8 8 0 0 0 1.3.8L12 20h4l.5-2.7a8 8 0 0 0 1.3-.8l2.2.5 2-3-2-1a8 8 0 0 0 0-2Z"/>'
 ];
 const remaining=[...document.querySelectorAll('.profile-menu a')];
 remaining.forEach((link,index)=>{const icon=link.querySelector('i');if(icon)icon.innerHTML=`<svg viewBox="0 0 24 24" aria-hidden="true">${icons[index%icons.length]}</svg>`});
 [testsLink,document.querySelector('.profile-market-link')].forEach(link=>{if(link&&!link.querySelector('.menu-new'))link.insertAdjacentHTML('beforeend','<b class="menu-new"><span class="menu-fire" aria-hidden="true">🔥</span>Новое</b>')});
})();

if(location.pathname==='/profile'){
 const settingsScript=document.createElement('script');
 settingsScript.src='/static/profile-settings.js?v=1';
 document.body.appendChild(settingsScript);
}
