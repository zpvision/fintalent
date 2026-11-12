// Actual built React + real Academy API/PostgreSQL provided by the Go test.
// Auth is synthetic. All traffic is restricted to loopback.
const fs=require('fs'),path=require('path'),os=require('os'),{spawn}=require('child_process')
const [base,course,version,enrollment,publicSlug]=process.argv.slice(2)
if(!base||new URL(base).hostname!=='127.0.0.1')throw Error('Only isolated loopback server is allowed')
const temp=fs.mkdtempSync(path.join(os.tmpdir(),'ft-revisions-ui-')),sleep=ms=>new Promise(r=>setTimeout(r,ms))
;(async()=>{
 const browser=spawn(process.env.EDGE_PATH||'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe',['--headless=new','--disable-gpu','--no-first-run','--remote-debugging-port=0','--user-data-dir='+path.join(temp,'browser'),'about:blank'],{windowsHide:true,stdio:'ignore'})
 let ws
 try{
  let port;for(let n=0;n<100;n++){try{port=fs.readFileSync(path.join(temp,'browser/DevToolsActivePort'),'utf8').split('\n')[0];break}catch{await sleep(100)}}
  if(!port)throw Error('Headless Edge unavailable')
  const tabs=await(await fetch(`http://127.0.0.1:${port}/json/list`)).json();ws=new WebSocket(tabs.find(t=>t.type==='page').webSocketDebuggerUrl);await new Promise(r=>ws.addEventListener('open',r,{once:true}))
  let seq=0;const pending=new Map(),errors=[]
  ws.addEventListener('message',e=>{const m=JSON.parse(e.data);if(m.id){pending.get(m.id)?.(m);pending.delete(m.id)}if(m.method==='Runtime.exceptionThrown')errors.push(m.params.exceptionDetails.text)})
  const cmd=(method,params={})=>new Promise((resolve,reject)=>{const id=++seq;pending.set(id,m=>m.error?reject(Error(JSON.stringify(m.error))):resolve(m.result));ws.send(JSON.stringify({id,method,params}))})
  const evaluate=async expression=>{const result=await cmd('Runtime.evaluate',{expression,returnByValue:true,awaitPromise:true});if(result.exceptionDetails)throw Error(result.exceptionDetails.exception.description);return result.result.value}
  const wait=async expression=>{for(let n=0;n<150;n++){if(await evaluate(`Boolean(${expression})`))return;await sleep(100)}throw Error('UI timeout: '+expression)}
  const click=async text=>{await wait(`Array.from(document.querySelectorAll('button')).some(b=>b.textContent.trim()===${JSON.stringify(text)}&&!b.disabled)`);await evaluate(`Array.from(document.querySelectorAll('button')).find(b=>b.textContent.trim()===${JSON.stringify(text)}&&!b.disabled).click()`)}
  const navigate=async url=>{await cmd('Page.navigate',{url:base+url});await sleep(500)}
  await cmd('Runtime.enable');await cmd('Page.enable')
  await navigate(`/profile/courses/${course}/academy/?version=${version}`)
  await wait(`document.querySelector('select[aria-label="Версия программы"]')?.value==='${version}'`)
  await wait(`document.querySelector('.academy-section-nav a[aria-current="page"]')?.textContent.includes('Программа и уроки')`)
  await evaluate(`window.__academyHeader=document.querySelector('.academy-course-header');window.__siteHeader=document.querySelector('.ft-site-header');document.querySelector('.academy-section-nav a[href="/profile/courses/${course}/academy/cohorts"]').click()`)
  await wait(`document.querySelector('.cohort-page-heading h2')?.textContent==='Потоки обучения'`)
  if(!await evaluate(`window.__academyHeader===document.querySelector('.academy-course-header') && window.__siteHeader===document.querySelector('.ft-site-header') && document.querySelectorAll('.academy-course-header').length===1 && document.querySelector('.academy-section-nav a[aria-current="page"]')?.textContent.includes('Потоки')`))throw Error('Academy shell remounted or active cohort tab is missing')
  await evaluate(`document.querySelector('.academy-section-nav a[href="/profile/courses/${course}/academy"]').click()`)
  await wait(`document.querySelector('select[aria-label="Версия программы"]')?.value==='${version}'`)
  if(!await evaluate(`window.__academyHeader===document.querySelector('.academy-course-header') && document.querySelector('.academy-section-nav a[aria-current="page"]')?.textContent.includes('Программа и уроки')`))throw Error('Program tab or persistent header regressed')
  // Selecting a closed module header opens it and selects its editor.
  await wait(`document.querySelector('.academy-tree-module > summary')`)
  await evaluate(`(()=>{const module=document.querySelector('.academy-tree-module');module.querySelector('summary').click()})()`)
  await wait(`document.querySelector('.academy-module-editor') && !document.querySelector('.academy-tree-module.current')?.open`)
  await evaluate(`document.querySelector('.academy-tree-module > summary').click()`)
  await wait(`document.querySelector('.academy-module-editor') && document.querySelector('.academy-tree-module.current')?.open`)
  // Open its lesson through the actual tree.
  await wait(`document.querySelector('.academy-tree-column')?.innerText.includes('Lesson')`)
  await evaluate(`Array.from(document.querySelectorAll('.academy-tree-column button')).find(b=>b.textContent.includes('Lesson')).click()`)
  await wait(`document.querySelector('.academy-lesson-heading input')`)
  await evaluate(`(()=>{const input=document.querySelector('.academy-lesson-heading input');Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,'value').set.call(input,'Browser published lesson');input.dispatchEvent(new Event('input',{bubbles:true}))})()`)
  await click('Сохранить')
  await wait(`document.querySelector('.academy-save-status')?.textContent==='Сохранено'`)
  // A direct room read must still expose the old published content.
  const old=await evaluate(`fetch('/api/academy/enrollments/${enrollment}').then(r=>r.json()).then(x=>x.program.modules[0].lessons[0].title)`)
  if(old==='Browser published lesson')throw Error('Draft leaked to learner')
  await click('Опубликовать изменения')
  await wait(`document.querySelector('dialog[open]')?.textContent.includes('Зачислений')`)
  await click('Опубликовать')
  await wait(`!document.querySelector('dialog[open]')`)
  const fresh=await evaluate(`fetch('/api/academy/enrollments/${enrollment}').then(r=>r.json()).then(x=>x.program.modules[0].lessons[0].title)`)
  if(fresh!=='Browser published lesson')throw Error('Published program did not reach learner')
  await click('Создать новую версию');await click('Создать и открыть')
  await wait(`document.querySelector('select[aria-label="Версия программы"]')?.value!=='${version}'`)
  const copied=await evaluate(`document.querySelector('select[aria-label="Версия программы"]').value`)
  if(!copied||copied===version)throw Error('Independent version not created')
  await evaluate(`(()=>{const select=document.querySelector('select[aria-label="Версия программы"]');select.value='${version}';select.dispatchEvent(new Event('change',{bubbles:true}))})()`)
  await wait(`document.querySelector('.academy-version-panel h2')?.textContent==='Версия 1'`)
  for(const width of [360,900,1440]){
   await cmd('Emulation.setDeviceMetricsOverride',{width,height:950,deviceScaleFactor:1,mobile:false});await sleep(200)
   const overflow=await evaluate(`document.documentElement.scrollWidth>innerWidth+1`);if(overflow)throw Error('Horizontal overflow at '+width)
   const shot=await cmd('Page.captureScreenshot',{format:'png',captureBeyondViewport:false});fs.writeFileSync(path.join(temp,`editor-${width}.png`),Buffer.from(shot.data,'base64'))
  }
  await navigate(`/profile/learning/${enrollment}`);await wait(`document.querySelector('.learning-room-material')?.textContent.includes('Browser published lesson')||document.querySelector('.learning-room-tree')?.textContent.includes('Browser published lesson')`)
  for(const width of [360,900,1440]){await cmd('Emulation.setDeviceMetricsOverride',{width,height:950,deviceScaleFactor:1,mobile:false});await sleep(150);if(await evaluate(`document.documentElement.scrollWidth>innerWidth+1`))throw Error('Room overflow at '+width)}
  await navigate(`/profile/courses/${course}/academy/crm`)
  await wait(`document.querySelector('.cohort-crm-table tbody tr')`)
  await evaluate(`(()=>{const input=document.querySelector('input[type="search"]');Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,'value').set.call(input,'no-such-synthetic-student');input.dispatchEvent(new Event('input',{bubbles:true}))})()`)
  await wait(`!document.querySelector('.cohort-crm-table') && document.body.textContent.includes('Ученики не найдены')`)
  await evaluate(`(()=>{const input=document.querySelector('input[type="search"]');Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,'value').set.call(input,'');input.dispatchEvent(new Event('input',{bubbles:true}))})()`)
  await wait(`document.querySelector('.cohort-crm-table tbody tr')`)
  for(const width of [360,900,1440]){
   await cmd('Emulation.setDeviceMetricsOverride',{width,height:950,deviceScaleFactor:1,mobile:false});await sleep(150)
   if(await evaluate(`document.documentElement.scrollWidth>innerWidth+1`))throw Error('CRM overflow at '+width)
   const shot=await cmd('Page.captureScreenshot',{format:'png',captureBeyondViewport:false});fs.writeFileSync(path.join(temp,`crm-${width}.png`),Buffer.from(shot.data,'base64'))
  }
  await wait(`document.querySelector('.academy-section-nav .academy-menu-counter')?.textContent==='1'`)
  await evaluate(`document.querySelector('.academy-section-nav a[href="/profile/courses/${course}/academy/analytics"]').click()`)
  await wait(`document.querySelector('.analytics-tabs .academy-menu-counter')?.textContent==='1'`)
  await evaluate(`Array.from(document.querySelectorAll('.analytics-tabs button')).find(b=>b.textContent.startsWith('Ученики')).click()`)
  await wait(`new URLSearchParams(location.search).get('interest_payment')==='unpaid' && document.querySelector('.acquisition-paid input')`)
  await evaluate(`document.querySelector('.acquisition-paid input').click()`)
  await wait(`document.querySelector('.analytics-payment-dialog[open]')`)
  await click('Оплата получена')
  await wait(`!document.querySelector('.analytics-payment-dialog[open]') && !document.querySelector('.academy-menu-counter')`)
  await navigate('/profile/courses')
  await wait(`document.querySelector('.course-card-stats')`)
  if(!await evaluate(`document.querySelector('.course-card-stats').textContent.includes('Всего учеников: 1') && document.querySelector('.course-card-stats').textContent.includes('Новые заявки: 0')`))throw Error('Dashboard counters missing')
  for(const width of [360,900,1440]){
   await cmd('Emulation.setDeviceMetricsOverride',{width,height:950,deviceScaleFactor:1,mobile:false});await sleep(150)
   if(await evaluate(`document.documentElement.scrollWidth>innerWidth+1`))throw Error('Dashboard overflow at '+width)
   const heights=await evaluate(`(()=>{const card=document.querySelector('.course-list-card'),body=card.querySelector('.course-card-body'),academy=card.querySelector('.course-card-academy'),landing=card.querySelector('.course-card-landing'),stats=card.querySelector('.course-card-stats');const compact=card.getBoundingClientRect().height;const previous=[body,academy,landing,stats].map(el=>el.getAttribute('style'));body.style.gap='20px';academy.style.padding='18px';academy.style.gap='16px';landing.style.paddingBottom='20px';stats.style.display='none';const old=card.getBoundingClientRect().height;[body,academy,landing,stats].forEach((el,i)=>previous[i]===null?el.removeAttribute('style'):el.setAttribute('style',previous[i]));return {compact,old}})()`)
   if(heights.compact>heights.old+1)throw Error('Card became taller at '+width+': '+JSON.stringify(heights))
   const shot=await cmd('Page.captureScreenshot',{format:'png',captureBeyondViewport:false});fs.writeFileSync(path.join(temp,`dashboard-${width}.png`),Buffer.from(shot.data,'base64'))
  }
  await navigate(`/courses/${publicSlug}`)
  await wait(`document.querySelector('.course-purchase button.course-primary')`)
  await evaluate(`document.querySelector('.course-purchase button.course-primary').click()`)
  await wait(`document.querySelector('.course-contact-dialog[open]')`)
  if(!await evaluate(`document.querySelector('.course-contact-dialog input[autocomplete="name"]').disabled && document.querySelector('.course-contact-dialog input[autocomplete="name"]').value.length>0`))throw Error('Application identity is editable or missing')
  const beforeSubmit=await evaluate(`fetch('/api/courses/my').then(r=>r.json()).then(rows=>rows.find(c=>c.id===Number('${course}')).unpaid_interest_count)`)
  if(beforeSubmit!==0)throw Error('Opening contact form created an application')
  await evaluate(`(()=>{for(const [type,value] of [['tel','+7 (999) 123-45-67'],['email','browser-contact@example.invalid']]){const input=document.querySelector('.course-contact-dialog input[type="'+type+'"]');Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,'value').set.call(input,value);input.dispatchEvent(new Event('input',{bubbles:true}))}})()`)
  for(const width of [360,900,1440]){
   await cmd('Emulation.setDeviceMetricsOverride',{width,height:950,deviceScaleFactor:1,mobile:false});await sleep(150)
   const fits=await evaluate(`(()=>{const r=document.querySelector('.course-contact-dialog').getBoundingClientRect();return r.left>=0&&r.right<=innerWidth&&r.top>=0&&r.bottom<=innerHeight})()`)
   if(!fits)throw Error('Contact modal exceeds viewport at '+width)
   const shot=await cmd('Page.captureScreenshot',{format:'png',captureBeyondViewport:false});fs.writeFileSync(path.join(temp,`contact-${width}.png`),Buffer.from(shot.data,'base64'))
  }
  await evaluate(`document.querySelector('.course-contact-dialog button[type="submit"]').click()`)
  await wait(`!document.querySelector('.course-contact-dialog') && document.querySelector('.course-interest-success')`)
  const afterSubmit=await evaluate(`fetch('/api/courses/my').then(r=>r.json()).then(rows=>rows.find(c=>c.id===Number('${course}')).unpaid_interest_count)`)
  if(afterSubmit!==1)throw Error('Contact submission did not create the application')
  if(errors.length)throw Error(errors.join('\n'))
  console.log(JSON.stringify({browser:'Edge headless',api:'real isolated PostgreSQL',draft_hidden:true,publish_visible:true,independent_copy:true,old_version_editable:true,widths:[360,900,1440],screenshots:temp}))
 }finally{ws?.close();browser.kill()}
})().catch(e=>{console.error(e.stack);process.exitCode=1})
