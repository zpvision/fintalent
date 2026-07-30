(function(){
  const oldHeader=document.body.querySelector(':scope > header');
  if(!oldHeader)return;
  const path=location.pathname;
  const active=href=>path===href||href!=='/'&&path.startsWith(href);
  const header=document.createElement('header');
  header.className='ft-site-header';
  header.innerHTML=`<a class="ft-brand" href="/"><span class="ft-logo-crop"><img src="/static/logo.png" alt=""></span><span class="ft-brand-copy"><b>Fin<span>Talent</span></b><small>Биржа вакансий для бухгалтеров</small></span></a><button class="ft-menu-toggle" type="button" aria-label="Открыть меню">☰</button><nav class="ft-main-nav"><a class="${active('/vacancies')?'active':''}" href="/vacancies">Вакансии</a><a class="${active('/resumes')||active('/resume')?'active':''}" href="/resumes">Резюме</a><a href="#">Компании</a><a class="${active('/marketplace')||active('/tests')?'active':''}" href="/marketplace">Тесты и навыки</a><a href="#">Блог</a><a href="#">Цены</a></nav><div class="ft-account"><a class="ft-login" href="/login">Войти</a><a class="ft-register" href="/register">Регистрация</a></div>`;
  oldHeader.replaceWith(header);
  header.querySelector('.ft-menu-toggle').onclick=()=>header.classList.toggle('menu-open');
  fetch('/api/me').then(r=>r.ok?r.json():null).then(user=>{if(!user)return;const name=user.full_name||user.email||'Профиль',initial=name.trim().charAt(0).toUpperCase();header.querySelector('.ft-account').innerHTML=`<a class="ft-profile" href="/profile"><i>${escapeText(initial)}</i><span><small>Личный кабинет</small><b>${escapeText(name)}</b></span></a>`}).catch(()=>{});
  function escapeText(value){const span=document.createElement('span');span.textContent=value;return span.innerHTML}
})();
