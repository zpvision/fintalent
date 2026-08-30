(()=>{
  const type=document.body.dataset.catalog,form=document.querySelector('#catalog-search'),list=document.querySelector('#catalog-list'),meta=document.querySelector('#catalog-meta');
  const esc=value=>{const n=document.createElement('span');n.textContent=value??'';return n.innerHTML};
  const money=value=>new Intl.NumberFormat('ru-RU',{maximumFractionDigits:0}).format(value||0);
  const avatar=item=>{
    const initials=esc((item.name||'').split(' ').filter(Boolean).map(x=>x[0]).slice(0,2).join('').toUpperCase());
    const photo=type==='resumes'&&item.avatar?`<img src="${esc(item.avatar)}" alt="Фото ${esc(item.name)}">`:'';
    return `<div class="catalog-card-icon"><span>${initials}</span>${photo}</div>`;
  };
  if(type==='resumes')document.head.insertAdjacentHTML('beforeend','<style>.catalog-card-icon{position:relative;overflow:hidden}.catalog-card-icon img{position:absolute;inset:0;width:100%;height:100%;object-fit:cover}</style>');
  const render=async()=>{
    list.innerHTML='<div class="catalog-empty">Загружаем предложения…</div>';
    const params=new URLSearchParams({kind:type,q:form.q.value.trim(),city:form.city.value.trim()});
    try{
      const data=await fetch('/api/public/catalog?'+params,{cache:'no-store'}).then(r=>r.json());
      meta.textContent=`Найдено: ${data.total||0}`;
      list.innerHTML=data.items?.length?data.items.map(item=>`<a class="catalog-card" href="${type==='resumes'?`/resume/view/${item.id}`:`/vacancies/view?id=${item.id}`}">${avatar(item)}<div><h2>${esc(item.title)}</h2><span class="company">${esc(item.name)} · ${esc(item.city||'Россия')}</span>${item.description?`<p>${esc(item.description)}</p>`:''}<div class="catalog-tags">${(item.tags||[]).map(tag=>`<span>${esc(tag)}</span>`).join('')}</div></div><div class="catalog-side"><strong>${money(item.salary)} ₽</strong><small>${type==='resumes'?'желаемый доход':'зарплата от'}</small></div></a>`).join(''):'<div class="catalog-empty">По вашему запросу ничего не найдено</div>';
      list.querySelectorAll('.catalog-card-icon img').forEach(image=>image.onerror=()=>image.remove());
    }catch{
      list.innerHTML='<div class="catalog-empty">Не удалось загрузить каталог</div>';
    }
  };
  form.onsubmit=e=>{e.preventDefault();render()};
  let timer;
  form.querySelectorAll('input').forEach(input=>input.oninput=()=>{clearTimeout(timer);timer=setTimeout(render,300)});
  render();
})();
