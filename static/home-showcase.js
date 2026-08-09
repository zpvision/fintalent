(()=>{
  const esc=value=>{const node=document.createElement('span');node.textContent=value??'';return node.innerHTML};
  const money=value=>new Intl.NumberFormat('ru-RU',{maximumFractionDigits:0}).format(value||0);
  const icons=['1C','%','▥','X','₽','✓'];
  const difficulty={easy:'Начальный',medium:'Средний',hard:'Продвинутый',expert:'Эксперт'};

  function renderShowcase(data){
    const jobs=document.querySelector('#jobs'),candidates=document.querySelector('#candidates');
    if(jobs&&data.vacancies?.length){
      jobs.querySelectorAll('.job').forEach(item=>item.remove());
      const more=jobs.querySelector('.more');
      data.vacancies.slice(0,4).reverse().forEach((item,index)=>more.insertAdjacentHTML('beforebegin',`<a class="job" href="/vacancies/view?id=${item.id}"><div class="company c${index%3+1}">${esc(item.name.slice(0,2).toUpperCase())}</div><div><b>${esc(item.title)}</b><small>${esc(item.name)}</small><span>⌖ ${esc(item.city)}　 ◇ Опубликована</span></div><strong>от ${money(item.salary)} ₽</strong></a>`));
    }
    if(candidates&&data.resumes?.length){
      candidates.querySelectorAll('.candidate').forEach(item=>item.remove());
      const more=candidates.querySelector('.more');
      data.resumes.slice(0,4).reverse().forEach((item,index)=>more.insertAdjacentHTML('beforebegin',`<a class="candidate" href="/resume/view/${item.id}"><div class="avatar av${index%3+1}">${esc(item.name.split(' ').map(part=>part[0]).slice(0,2).join(''))}</div><div><b>${esc(item.name)}</b><small>${esc(item.title)}</small><span>⌖ ${esc(item.city||'Россия')}</span><i>${(item.tags||[]).slice(0,4).map(esc).join('　')}</i></div><strong>${money(item.salary)} ₽</strong></a>`));
    }
  }

  function renderTests(tests){
    const list=document.querySelector('.test-list');
    if(!list)return;
    const random=[...tests].sort(()=>Math.random()-.5).slice(0,4);
    list.innerHTML=random.length?random.map((test,index)=>`<a href="/tests/take?id=${test.id}"><i>${icons[index%icons.length]}</i><b>${esc(test.title)}</b><small>${esc(difficulty[test.difficulty]||'Тест')} · ${test.question_count} вопр.</small></a>`).join(''):'<span class="test-loading">В Маркетплейсе пока нет опубликованных тестов</span>';
  }

  fetch('/api/public/home-showcase',{cache:'no-store'}).then(response=>response.json()).then(renderShowcase).catch(()=>{});
  fetch('/api/marketplace/tests',{cache:'no-store'}).then(response=>response.json()).then(tests=>renderTests(Array.isArray(tests)?tests:[])).catch(()=>{const list=document.querySelector('.test-list');if(list)list.innerHTML='<span class="test-loading">Не удалось загрузить тесты</span>'});
})();
