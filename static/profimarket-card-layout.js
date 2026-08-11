(()=>{
  const UI=window.ProfiMarketUI,key=decodeURIComponent(location.pathname.split('/').filter(Boolean).pop());
  const styleCatalog=window.ProfiMarketStylePresets;
  const applyTheme=(element,key,fallback)=>{const theme=styleCatalog.byKey[key]||styleCatalog.byKey[fallback];if(!element||!theme)return;for(const name of ['section','text','card','iconBg','icon','heading','border'])element.style.setProperty(`--block-${name.replace(/[A-Z]/g,x=>'-'+x.toLowerCase())}`,theme[name]);element.dataset.style=theme.key};
  async function apply(){
    const response=await fetch('/api/profimarket/solution/'+encodeURIComponent(key),{cache:'no-store'});
    if(!response.ok)return;
    const solution=await response.json();
    if(solution.type!=='REGULATION')return;
    for(let i=0;i<30&&!document.querySelector('.pmr-shell');i++)await new Promise(r=>setTimeout(r,100));
    const shell=document.querySelector('.pmr-shell');
    if(!shell)return;

    const metrics=shell.querySelector('.pmr-benefits');
    if(metrics&&solution.key_metrics?.length){
      metrics.innerHTML=solution.key_metrics.map(x=>`<article>${UI.icon(x.icon)}<span><b>${UI.esc(x.title)}</b><small>${UI.esc(x.description)}</small></span></article>`).join('');
    }
    applyTheme(metrics,solution.metric_style,styleCatalog.defaults.metrics);
    const top=shell.querySelector('.pmr-top-grid');
    if(top&&metrics){
      const hero=top.querySelector('.pmr-hero');
      const left=document.createElement('div');
      left.className='pmr-top-left';
      top.prepend(left);
      if(hero)left.append(hero);
      left.append(metrics);
      top.classList.add('has-inline-metrics');
    }

    const accessSection=shell.querySelector('.pmr-access');
    if(accessSection&&!solution.access_features?.length)accessSection.remove();
    else if(accessSection){const title=accessSection.querySelector('h2');if(title)title.textContent=solution.right_block_title||'Формат и доступ'}
    const access=shell.querySelector('.pmr-access>div');
    if(access&&solution.access_features?.length){
      access.innerHTML=solution.access_features.map(x=>`<article><i>${UI.icon(x.icon)}</i><span><b>${UI.esc(x.title)}</b>${x.description?`<small>${UI.esc(x.description)}</small>`:''}</span></article>`).join('');
    }
    applyTheme(shell.querySelector('.pmr-access'),solution.access_style,styleCatalog.defaults.access);

    const bonuses=shell.querySelector('.pmr-bonuses>div');
    if(bonuses&&solution.bonuses?.length){
      bonuses.innerHTML=solution.bonuses.map(x=>`<article><i>${UI.icon(x.icon)}</i><span><b>${UI.esc(x.title)}</b><small>${UI.esc(x.description)}</small></span></article>`).join('');
    }
    const bonusesSection=shell.querySelector('.pmr-bonuses');
    applyTheme(bonusesSection,solution.bonus_style,styleCatalog.defaults.bonuses);

    const left=top?.querySelector('.pmr-top-left');
    const right=top?.querySelector(':scope>aside');
    const packageBlock=shell.querySelector('.pmr-package');
    const bonusesBlock=shell.querySelector('.pmr-bonuses');
    const accessBlock=shell.querySelector('.pmr-access');
    let authorBlock=shell.querySelector('.pmr-publication-author');
    if(right&&!authorBlock){
      authorBlock=document.createElement('section');
      authorBlock.className='pmr-publication-author';
      const initial=UI.esc((solution.author_name||'А').charAt(0).toUpperCase());
      authorBlock.innerHTML=`<small>АВТОР ПУБЛИКАЦИИ</small><div><i>${solution.author_avatar?`<img src="${UI.esc(solution.author_avatar)}" alt="${UI.esc(solution.author_name||'Автор')}">`:initial}</i><span><b>${UI.esc(solution.author_name||'Автор FinTalent')}</b><em>Проверенный автор FinTalent</em></span><strong>✓</strong></div>`;
    }
    if(left){
      if(packageBlock)left.append(packageBlock);
      if(bonusesBlock)left.append(bonusesBlock);
    }
    if(right&&authorBlock)right.append(authorBlock);
    if(right&&accessBlock)right.append(accessBlock);
    shell.querySelector('.pmr-main-grid')?.remove();

    const safe=shell.querySelector('.pmr-safe');
    if(safe){
      safe.querySelector('b').textContent=solution.implementation_title||'Регламенты навсегда в вашей CRM';
      const subtitle=safe.querySelector('small');
      if(subtitle)subtitle.textContent=solution.implementation_subtitle||'Доступ получаете вы, они остаются у вас';
      if(solution.crms?.length)safe.insertAdjacentHTML('beforeend',`<div class="pmr-safe-crms">${solution.crms.map(crm=>`<div class="pmr-safe-crm">${crm.icon?`<img src="${UI.esc(crm.icon)}" alt="">`:UI.icon(crm.code==='other'?'settings':'workflow')}<b>${UI.esc(crm.name)}</b></div>`).join('')}</div>`);
    }
    shell.querySelectorAll('.pmr-buy-card>button[data-buy]').forEach(button=>button.textContent=solution.purchase_button_label||'Купить и внедрить');
  }
  apply().catch(()=>{});
})();
