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
