(function(){
  document.head.insertAdjacentHTML('beforeend','<link rel="stylesheet" href="/static/publication-editor-enhancements.css?v=1"><link rel="stylesheet" href="/static/publication-preview-accurate.css?v=1">');
  const $=selector=>document.querySelector(selector);
  const escapeHTML=value=>{const node=document.createElement('span');node.textContent=value??'';return node.innerHTML};

  async function init(){
    buildArticleHero();
    hideAdvancedFields();
    decoratePublicationSettings();
    enableAutomaticSEO();
    try{
      const response=await fetch('/api/publications/meta',{cache:'no-store'});
      const meta=await response.json();
      await waitForBaseEditor();
      await enableEditorJS();
      window.renderPreview=renderEditorPreview;
      await buildDictionaryPickers(meta);
      document.addEventListener('pointerdown',event=>{if(!event.target.closest('.dictionary-picker'))document.querySelectorAll('.picker-menu.open').forEach(menu=>menu.classList.remove('open'))});
      enableAutomaticSummary();
    }catch(error){console.warn('Publication editor enhancements:',error)}
  }

  function buildArticleHero(){
    const mode=$('#editor-mode'),cover=$('#cover-drop'),title=$('#publication-title'),subtitle=$('#publication-subtitle'),excerpt=$('#publication-excerpt');
    if(!mode||!cover||!title||mode.querySelector('.publication-hero-editor'))return;
    const hero=document.createElement('section'),copy=document.createElement('div');
    hero.className='publication-hero-editor';copy.className='publication-hero-editor-copy';
    hero.append(copy,cover);copy.append(title,subtitle,excerpt);mode.prepend(hero);
  }

  function hideAdvancedFields(){
    ['#difficulty','#series','#series-order','#language','#change-summary','#skills'].forEach(selector=>$(selector)?.closest('label')?.classList.add('editor-setting-removed'));
    const seoTitle=$('#seo-title'),seoDescription=$('#seo-description');
    seoTitle?.closest('label')?.classList.add('editor-setting-removed');
    seoDescription?.closest('label')?.classList.add('editor-setting-removed');
    const anchor=$('#slug')?.closest('label');
    if(anchor&&!$('.seo-auto-card')){
      const card=document.createElement('section');card.className='seo-auto-card';
      card.innerHTML='<i>✓</i><div><b>SEO настроено автоматически</b><span id="seo-auto-preview">Заголовок и описание сформируются из публикации</span></div>';
      anchor.after(card);
    }
  }

  function decoratePublicationSettings(){
    const icons={
      category:'<svg viewBox="0 0 24 24"><path d="M4 5h6v6H4zM14 5h6v6h-6zM4 15h6v4H4zM14 15h6v4h-6z"/></svg>',
      topics:'<svg viewBox="0 0 24 24"><path d="M4 6.5h16M4 12h11M4 17.5h8"/><circle cx="18" cy="12" r="2.5"/></svg>',
      tags:'<svg viewBox="0 0 24 24"><path d="M4 5h7l9 9-6 6-9-9Z"/><circle cx="8.5" cy="8.5" r="1"/></svg>',
      'reading-time':'<svg viewBox="0 0 24 24"><circle cx="12" cy="13" r="8"/><path d="M12 9v4l3 2M9 3h6"/></svg>',
      test:'<svg viewBox="0 0 24 24"><path d="M7 3h10v4H7zM5 5h2M17 5h2v16H5V5"/><path d="m8 13 2 2 5-5"/></svg>',
      visibility:'<svg viewBox="0 0 24 24"><path d="M2.5 12s3.5-6 9.5-6 9.5 6 9.5 6-3.5 6-9.5 6-9.5-6-9.5-6Z"/><circle cx="12" cy="12" r="2.5"/></svg>',
      slug:'<svg viewBox="0 0 24 24"><path d="M9.5 14.5 14.5 9M7.5 16.5l-1 1a3.5 3.5 0 0 1-5-5l3-3a3.5 3.5 0 0 1 5 0M16.5 7.5l1-1a3.5 3.5 0 0 1 5 5l-3 3a3.5 3.5 0 0 1-5 0"/></svg>',
    };
    Object.entries(icons).forEach(([id,icon])=>{
      const control=$(`#${id}`),label=control?.closest('label');if(!label||label.classList.contains('editor-setting-removed')||label.querySelector('.setting-label-head'))return;
      const textNode=[...label.childNodes].find(node=>node.nodeType===Node.TEXT_NODE&&node.textContent.trim());if(!textNode)return;
      const title=textNode.textContent.trim(),head=document.createElement('span');head.className='setting-label-head';head.innerHTML=`<i>${icon}</i><b>${escapeHTML(title)}</b>`;textNode.remove();label.prepend(head);
    });
    const comments=$('#allow-comments')?.closest('label');if(comments&&!comments.querySelector('.setting-check-icon'))comments.insertAdjacentHTML('afterbegin','<i class="setting-check-icon"><svg viewBox="0 0 24 24"><path d="M4 5h16v12H9l-5 4Z"/><path d="M8 9h8M8 13h5"/></svg></i>');
  }

  function waitForBaseEditor(){
    return new Promise(resolve=>{let attempts=0;const timer=setInterval(()=>{attempts++;if($('#category option:nth-child(2)')||attempts>30){clearInterval(timer);setTimeout(resolve,120)}},60)});
  }

  async function buildDictionaryPickers(meta){
    let existingTopics=[];
    const match=location.pathname.match(/\/publications\/(\d+)\/edit/);
    if(match){try{const response=await fetch(`/api/publications/${match[1]}`,{cache:'no-store'});if(response.ok)existingTopics=(await response.json()).topics||[]}catch{}}
    createTagPicker(meta.tags||[]);
    createTopicPicker(meta.topics||[],existingTopics);
  }

  function createTagPicker(items){
    const source=$('#tags');if(!source||source.nextElementSibling?.classList.contains('dictionary-picker'))return;
    source.classList.add('picker-source');
    const selected=new Map();
    source.value.split(',').map(value=>value.trim()).filter(Boolean).forEach(value=>selected.set(value.toLocaleLowerCase('ru'),value));
    const picker=createPicker('Начните вводить тег…');source.after(picker.root);
    const sync=()=>{source.value=[...selected.values()].join(', ');source.dispatchEvent(new Event('input',{bubbles:true}));renderTokens(picker.tokens,selected,value=>{selected.delete(value.toLocaleLowerCase('ru'));sync();showSuggestions('')})};
    const add=value=>{value=value.trim().replace(/^#+/,'');if(!value)return;selected.set(value.toLocaleLowerCase('ru'),value);picker.input.value='';sync();showSuggestions('')};
    const showSuggestions=query=>renderSuggestions(picker.menu,items.map(item=>item.name),query,[...selected.values()],add,true);
    picker.input.addEventListener('input',event=>showSuggestions(event.target.value));
    picker.input.addEventListener('focus',()=>showSuggestions(picker.input.value));
    picker.input.addEventListener('keydown',event=>{if(event.key==='Enter'||event.key===','){event.preventDefault();add(picker.input.value)}if(event.key==='Backspace'&&!picker.input.value&&selected.size){selected.delete([...selected.keys()].pop());sync()}});
    sync();
  }

  function createTopicPicker(items,existingNames){
    const source=$('#topics');if(!source||source.nextElementSibling?.classList.contains('dictionary-picker'))return;
    source.classList.add('picker-source');source.innerHTML=items.map(item=>`<option value="${escapeHTML(item.slug)}">${escapeHTML(item.name)}</option>`).join('');
    const selected=new Map();
    existingNames.forEach(name=>{const item=items.find(option=>option.name===name);if(item)selected.set(item.slug,item.name)});
    const picker=createPicker('Найдите и выберите тему…');source.after(picker.root);
    const sync=()=>{[...source.options].forEach(option=>option.selected=selected.has(option.value));source.dispatchEvent(new Event('input',{bubbles:true}));renderTokens(picker.tokens,selected,(value,key)=>{selected.delete(key);sync();showSuggestions('')})};
    const add=name=>{const item=items.find(option=>option.name===name);if(!item)return;selected.set(item.slug,item.name);picker.input.value='';sync();showSuggestions('')};
    const showSuggestions=query=>renderSuggestions(picker.menu,items.map(item=>item.name),query,[...selected.values()],add,false);
    picker.input.addEventListener('input',event=>showSuggestions(event.target.value));picker.input.addEventListener('focus',()=>showSuggestions(picker.input.value));sync();
  }

  function createPicker(placeholder){
    const root=document.createElement('div');root.className='dictionary-picker';
    root.innerHTML=`<div class="picker-tokens"></div><input class="picker-input" placeholder="${placeholder}" autocomplete="off"><div class="picker-menu"></div>`;
    return{root,tokens:root.querySelector('.picker-tokens'),input:root.querySelector('.picker-input'),menu:root.querySelector('.picker-menu')};
  }

  function renderTokens(container,selected,remove){
    container.innerHTML='';selected.forEach((value,key)=>{const token=document.createElement('span');token.innerHTML=`${escapeHTML(value)}<button type="button" aria-label="Удалить">×</button>`;token.querySelector('button').onclick=()=>remove(value,key);container.append(token)});
  }

  function renderSuggestions(menu,values,query,selected,choose,allowCustom){
    const normalized=query.trim().toLocaleLowerCase('ru');
    const matches=values.filter(value=>!selected.includes(value)&&(!normalized||value.toLocaleLowerCase('ru').includes(normalized))).slice(0,8);
    if(allowCustom&&query.trim()&&!values.some(value=>value.toLocaleLowerCase('ru')===normalized))matches.unshift(query.trim());
    menu.innerHTML=matches.map((value,index)=>`<button type="button" data-value="${escapeHTML(value)}">${index===0&&allowCustom&&value===query.trim()?'<i>＋</i> Добавить ':''}<b>${escapeHTML(value)}</b></button>`).join('');
    menu.classList.toggle('open',matches.length>0);menu.querySelectorAll('button').forEach(button=>button.onclick=()=>choose(button.dataset.value));
  }

  async function enableEditorJS(){
    const legacy=$('#content-blocks'),toolbar=$('.block-toolbar');if(!legacy||!window.FinTalentEditorJS)return;
    const shell=document.createElement('section');shell.className='editorjs-shell';shell.innerHTML='<header><div><h2>Текст публикации</h2><p>Нажмите «+», чтобы добавить блок. Выделите текст для форматирования.</p></div><span>Блоки можно перетаскивать</span></header><div id="publication-editorjs"></div>';
    toolbar.before(shell);toolbar.classList.add('legacy-editor-hidden');legacy.classList.add('legacy-editor-hidden');
    let syncing=false,timer=null;
    const initial=[...legacy.children].map(blockData);
    const mounted=await window.FinTalentEditorJS.mount({holder:'publication-editorjs',data:initial,onChange:()=>{if(syncing)return;clearTimeout(timer);timer=setTimeout(syncNow,100)}});
    async function syncNow(){
      if(syncing)return;syncing=true;
      try{const content=await mounted.save();legacy.innerHTML='';content.forEach(block=>addBlock(block.type,block,false));changed()}finally{syncing=false}
    }
    window.syncPublicationEditor=syncNow;
    document.querySelectorAll('.view-switch button').forEach(control=>{const original=control.onclick;if(!original)return;control.onclick=async function(event){if(this.dataset.view!=='editor')await syncNow();return original.call(this,event)}});
  }

  function safeInline(value){
    const template=document.createElement('template');template.innerHTML=String(value||'');const allowed=new Set(['B','STRONG','I','EM','U','CODE','MARK','BR','A']);
    [...template.content.querySelectorAll('*')].forEach(node=>{if(!allowed.has(node.tagName)){node.replaceWith(document.createTextNode(node.textContent||''));return}const href=node.tagName==='A'?node.getAttribute('href')||'':'';[...node.attributes].forEach(attribute=>node.removeAttribute(attribute.name));if(node.tagName==='A'&&/^https?:\/\//i.test(href)){node.setAttribute('href',href);node.setAttribute('target','_blank');node.setAttribute('rel','noopener noreferrer')}});return template.innerHTML;
  }

  function renderEditorPreview(){
    const data=payload(),html=data.content.map(block=>{
      if(block.type==='h2'||block.type==='h3')return`<${block.type}>${safeInline(block.text)}</${block.type}>`;
      if(block.type==='paragraph')return`<p>${safeInline(block.text)}</p>`;
      if(block.type==='quote')return`<blockquote>${safeInline(block.text)}</blockquote>`;
      if(['avoid','note','example','conclusion','warning','info'].includes(block.type))return`<aside class="article-callout ${block.type}"><b>${escapeHTML(labels[block.type])}</b><p>${safeInline(block.text)}</p></aside>`;
      if(['bullets','numbered','checklist'].includes(block.type)){const tag=block.type==='numbered'?'ol':'ul';return`<${tag}>${(block.items||[]).map(item=>`<li>${safeInline(item)}</li>`).join('')}</${tag}>`}
      if(block.type==='code')return`<pre><code>${escapeHTML(block.text)}</code></pre>`;
      if(block.type==='divider')return'<hr>';
      if(block.type==='table')return`<div class="article-table"><table>${(block.rows||[]).map((row,index)=>`<tr>${row.map(cell=>`<${index?'td':'th'}>${safeInline(cell)}</${index?'td':'th'}>`).join('')}</tr>`).join('')}</table></div>`;
      if(block.type==='image'&&block.url)return`<figure><img src="${escapeHTML(block.url)}" alt="${escapeHTML(block.caption||'')}"><figcaption>${escapeHTML(block.caption||'')}</figcaption></figure>`;
      return'';
    }).join('');
    const categorySelect=$('#category'),category=categorySelect?.value&&categorySelect.value!=='0'?categorySelect.selectedOptions[0]?.textContent:'Экспертный материал';
    const tags=(data.tags||[]).slice(0,5),skills=(data.skills||[]).slice(0,4);
    const cover=coverImage?`<img src="${escapeHTML(coverImage)}" alt="${escapeHTML(data.title)}">`:'<div class="preview-cover-empty"><span>▧</span><small>Обложка публикации</small></div>';
    $('#publication-preview').innerHTML=`
      <nav class="preview-breadcrumbs"><span>Публикации</span><i>›</i><span>${escapeHTML(category)}</span><i>›</i><b>Предпросмотр</b></nav>
      <section class="preview-article-top">
        <div class="preview-article-hero">
          <div class="preview-article-labels"><b>${escapeHTML(category)}</b><em>Актуально</em></div>
          <div class="preview-article-dates">Предпросмотр публикации · ${Math.max(1,data.readingTime||1)} мин чтения</div>
          <h1>${escapeHTML(data.title||'Заголовок публикации')}</h1>
          ${data.subtitle?`<h2>${escapeHTML(data.subtitle)}</h2>`:''}
          <p>${escapeHTML(data.excerpt||'Краткое описание публикации появится здесь.')}</p>
          <div class="preview-taxonomy">${tags.map(tag=>`<span>${escapeHTML(tag)}</span>`).join('')}${skills.map(skill=>`<span class="skill">✓ ${escapeHTML(skill)}</span>`).join('')}</div>
        </div>
        <div class="preview-article-cover">${cover}</div>
      </section>
      <div class="preview-actions"><span><i>♡</i><b>Полезно</b><small>0</small></span><span><i>▣</i><b>Использовал в работе</b><small>0</small></span><span><i>♧</i><b>Решило проблему</b><small>0</small></span><span><i>☆</i><b>Сохранить</b><small>0</small></span></div>
      <div class="preview-summary"><b>✦ За 30 секунд</b><ul>${data.summaryPoints.length?data.summaryPoints.map(point=>`<li>${escapeHTML(point)}</li>`).join(''):'<li>Тезисы будут сформированы автоматически</li>'}</ul></div>
      <h2 class="preview-full-title">Полная статья</h2><div class="preview-content">${html||'<p class="preview-content-empty">Текст публикации появится здесь.</p>'}</div>`;
  }

  function enableAutomaticSEO(){
    const title=$('#publication-title'),subtitle=$('#publication-subtitle'),excerpt=$('#publication-excerpt'),seoTitle=$('#seo-title'),seoDescription=$('#seo-description');
    const trim=(value,limit)=>{value=value.trim().replace(/\s+/g,' ');if([...value].length<=limit)return value;const short=[...value].slice(0,limit-1).join('');return short.replace(/\s+\S*$/,'').trim()+'…'};
    const update=()=>{seoTitle.value=trim(title.value,60);seoDescription.value=trim(excerpt.value||subtitle.value,160);const preview=$('#seo-auto-preview');if(preview)preview.textContent=seoTitle.value||'Заголовок и описание сформируются из публикации'};
    [title,subtitle,excerpt].forEach(field=>field?.addEventListener('input',update));setTimeout(update,500);
  }

  function enableAutomaticSummary(){
    const button=$('#ai-summary');if(!button)return;
    button.textContent='✦ Сформировать по статье';
    const hint=$('.summary-editor small');if(hint)hint.textContent='Сформируется автоматически · каждый пункт можно изменить';
    if(!location.pathname.match(/\/publications\/(\d+)\/edit/)&&![...summary.querySelectorAll('input')].some(input=>input.value.trim()))summary.innerHTML='<p class="summary-auto-empty">Напишите статью — тезисы сформируются автоматически перед сохранением</p>';
    const generate=async(showNotice=true)=>{await window.syncPublicationEditor?.();const data=payload();if(data.content.filter(block=>block.text||block.items?.length).length<2){if(showNotice)notify('Добавьте хотя бы несколько содержательных блоков');return false}button.disabled=true;button.textContent='Формируем тезисы…';try{const result=await api('/api/publications/summary',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(data)});summary.innerHTML='';result.points.forEach(point=>addSummary(point));changed();if(showNotice)notify('Тезисы сформированы — их можно отредактировать');return true}catch(error){if(showNotice)notify(error.message);return false}finally{button.disabled=false;button.textContent='✦ Сформировать по статье'}};
    button.onclick=()=>generate(true);
    const addButton=$('#add-summary'),originalAdd=addButton?.onclick;if(addButton&&originalAdd)addButton.onclick=function(event){summary.querySelector('.summary-auto-empty')?.remove();return originalAdd.call(this,event)};
    ['#save-draft','#publish-button'].forEach(selector=>{const control=$(selector),original=control?.onclick;if(!control||!original)return;control.onclick=async function(event){await window.syncPublicationEditor?.();const hasSummary=[...summary.querySelectorAll('input')].some(input=>input.value.trim());if(!hasSummary)await generate(false);return original.call(this,event)}});
  }

  if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',init);else init();
})();
