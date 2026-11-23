// Real admin course module, isolated HTTP fixtures only; no account or production data.
const fs=require('fs'),path=require('path'),http=require('http'),os=require('os'),{spawn}=require('child_process')
const root=path.resolve(__dirname,'..'),output=path.join(root,'.codex-run','course-students-admin'),sleep=ms=>new Promise(r=>setTimeout(r,ms))
fs.mkdirSync(output,{recursive:true})
const names=['Ирина Синтетическая','Алексей Тестовый','Ольга Синтетическая'],emails=['irina@example.invalid','alex@example.invalid','olga@example.invalid']
let items=Array.from({length:52},(_,n)=>({id:n+1,user_id:n%3+10,course_id:n%2+100,author_id:n%2+1,course_title:'Курс '+(n%2+1),tariff_name:'Выбранный тариф <img src=x onerror="window.xss=true">',selected_price:n===2?0:n===3?null:150000,price_is_current:n===4,author_name:'Автор '+(n%2+1),student_name:names[n%3],contact_email:emails[n%3],contact_phone:'+79990000000',created_at:'2026-11-23T10:00:00Z',can_delete:n!==1}))
const stats=rows=>({applications:rows.length,students:new Set(rows.map(r=>r.user_id)).size,total_price:rows.reduce((sum,r)=>sum+(r.selected_price||0),0),unknown_prices:rows.filter(r=>r.selected_price==null).length,current_prices:rows.filter(r=>r.price_is_current&&r.selected_price!=null).length})
let deletions=0,confirmDelete=true,failDelete=false
const html=`<!doctype html><html lang="ru"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><link rel="stylesheet" href="/static/admin.css"><link rel="stylesheet" href="/static/admin-users.css"><link rel="stylesheet" href="/static/admin-profimarket.css"></head><body><div class="admin-app"><aside class="sidebar"><nav><button id="dictionary-nav">Справочники</button></nav></aside><main class="workspace"><header><button class="mobile-menu">☰</button><div><h1>Админка</h1><p>Проверка</p></div></header><section id="dictionary-list"></section><section id="dictionary-editor" class="hidden"><button id="back">Назад</button></section></main></div><script>const list=document.querySelector('#dictionary-list'),editor=document.querySelector('#dictionary-editor');let dictionaries=[],current=null;function renderList(){};function openDictionary(){};async function api(url,options={}){const r=await fetch(url,options),data=await r.json();if(!r.ok)throw Error(data.error);return data};function notify(message){window.lastNotice=message};document.querySelector('.mobile-menu').onclick=()=>document.querySelector('.sidebar').classList.toggle('open')</script><script src="/static/admin-courses.js?v=7"></script></body></html>`
const server=http.createServer((req,res)=>{
  const u=new URL(req.url,'http://localhost'),json=(data,status=200)=>{res.writeHead(status,{'Content-Type':'application/json'});res.end(JSON.stringify(data))}
  if(u.pathname==='/api/admin/course-access')return json({public_access:true})
  if(u.pathname==='/api/admin/courses/students'){
    const filtered=items.filter(r=>(!u.searchParams.get('author_id')||String(r.author_id)===u.searchParams.get('author_id'))&&r.student_name.toLowerCase().includes((u.searchParams.get('student_name')||'').toLowerCase())&&r.contact_email.toLowerCase().includes((u.searchParams.get('email')||'').toLowerCase()))
    const page=Number(u.searchParams.get('page')||1)
    return json({items:filtered.slice((page-1)*50,page*50),total:filtered.length,page,page_size:50,overall:stats(items),summary:stats(filtered),authors:[{id:1,name:'Автор 1'},{id:2,name:'Автор 2'}]})
  }
  if(u.pathname.startsWith('/api/admin/courses/students/')&&req.method==='DELETE'){
    if(failDelete)return json({error:'Не удалось удалить заявку'},500)
    const id=Number(u.pathname.split('/').at(-1));items=items.filter(r=>r.id!==id);deletions++;return json({ok:true})
  }
  if(u.pathname.startsWith('/static/')){const file=path.join(root,u.pathname);if(file.startsWith(path.join(root,'static')+path.sep)&&fs.existsSync(file)){res.setHeader('Content-Type',file.endsWith('.js')?'text/javascript':'text/css');return fs.createReadStream(file).pipe(res)}}
  if(u.pathname==='/admin/courses'){res.setHeader('Content-Type','text/html; charset=utf-8');return res.end(html)}
  res.writeHead(404).end()
})
;(async()=>{
  await new Promise(r=>server.listen(0,'127.0.0.1',r));const base=`http://127.0.0.1:${server.address().port}`,profile=fs.mkdtempSync(path.join(os.tmpdir(),'ft-course-students-'))
  const browser=spawn(process.env.EDGE_PATH||'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe',['--headless=new','--disable-gpu','--no-first-run','--remote-debugging-port=0','--user-data-dir='+profile,'about:blank'],{windowsHide:true,stdio:'ignore'})
  let ws
  try{
    let port;for(let n=0;n<100;n++){try{port=fs.readFileSync(path.join(profile,'DevToolsActivePort'),'utf8').split('\n')[0];break}catch{await sleep(100)}}if(!port)throw Error('Headless Edge unavailable')
    const tabs=await(await fetch(`http://127.0.0.1:${port}/json/list`)).json();ws=new WebSocket(tabs.find(t=>t.type==='page').webSocketDebuggerUrl);await new Promise(r=>ws.addEventListener('open',r,{once:true}))
    let seq=0;const pending=new Map(),errors=[]
    ws.addEventListener('message',event=>{const m=JSON.parse(event.data);if(m.id){pending.get(m.id)?.(m);pending.delete(m.id)}if(m.method==='Runtime.exceptionThrown')errors.push(m.params.exceptionDetails.exception?.description||m.params.exceptionDetails.text);if(m.method==='Page.javascriptDialogOpening')void cmd('Page.handleJavaScriptDialog',{accept:confirmDelete})})
    const cmd=(method,params={})=>new Promise((resolve,reject)=>{const id=++seq;pending.set(id,m=>m.error?reject(Error(JSON.stringify(m.error))):resolve(m.result));ws.send(JSON.stringify({id,method,params}))})
    const evaluate=async expression=>{const r=await cmd('Runtime.evaluate',{expression,returnByValue:true,awaitPromise:true});if(r.exceptionDetails)throw Error(r.exceptionDetails.exception?.description||r.exceptionDetails.text);return r.result.value}
    const wait=async expression=>{for(let n=0;n<100;n++){if(await evaluate(expression))return;await sleep(50)}throw Error('Timeout '+expression+' '+await evaluate('document.body.innerText'))}
    const assert=async(expression,label)=>{if(!await evaluate(expression))throw Error(label);console.log('PASS '+label)}
    await cmd('Page.enable');await cmd('Runtime.enable');await cmd('Page.navigate',{url:base+'/admin/courses'});await wait(`!!document.querySelector('#courses-admin-nav')`)
    await evaluate(`document.querySelector('#courses-admin-nav').click()`);await wait(`[...document.querySelectorAll('#admin-courses button')].some(b=>b.textContent==='Ученики')`)
    await evaluate(`[...document.querySelectorAll('#admin-courses button')].find(b=>b.textContent==='Ученики').click()`);await wait(`document.querySelectorAll('[data-application-id]').length===50`)
    await assert(`!window.xss&&!document.querySelector('.course-students-table img')&&document.querySelector('[data-overall]').textContent.includes('52')`,'global totals, page size and safe tariff display')
    await assert(`!document.querySelector('[data-application-id="2"] button')&&document.querySelector('[data-application-id="3"]').textContent.includes('0')&&document.querySelector('[data-application-id="5"]').textContent.includes('Текущая цена')`,'paid deletion protected; free and historical prices displayed')
    await evaluate(`[...document.querySelectorAll('#admin-courses button')].find(b=>b.textContent==='Далее →').click()`);await wait(`document.querySelectorAll('[data-application-id]').length===2`)
    await assert(`document.querySelector('[data-overall]').textContent.includes('52')`,'global totals include all pages')
    await evaluate(`document.querySelector('[name=author_id]').value='1';document.querySelector('[name=student_name]').value='ирина';document.querySelector('[name=email]').value='IRINA@';document.querySelector('.course-students-filters').requestSubmit()`)
    await wait(`document.querySelectorAll('[data-application-id]').length===9`)
    await assert(`document.querySelector('[data-filtered]').textContent.includes('9')&&document.querySelector('[data-overall]').textContent.includes('52')&&document.querySelector('[name=email]').value==='IRINA@'`,'combined filters reset page, retain inputs and show matched count')
    for(const width of [1440,900,360]){await cmd('Emulation.setDeviceMetricsOverride',{width,height:1000,deviceScaleFactor:1,mobile:false});await sleep(100);await assert(`document.documentElement.scrollWidth<=innerWidth`,'no page overflow '+width);const shot=await cmd('Page.captureScreenshot',{format:'png'});fs.writeFileSync(path.join(output,'students-'+width+'.png'),Buffer.from(shot.data,'base64'))}
    confirmDelete=false;await evaluate(`document.querySelector('[data-delete-application]').click()`);await sleep(100);if(deletions!==0)throw Error('Cancelled deletion executed')
    confirmDelete=true;failDelete=true;await evaluate(`document.querySelector('[data-delete-application]').click()`);await wait(`window.lastNotice==='Не удалось удалить заявку'`)
    await assert(`!document.querySelector('[data-delete-application]').disabled`,'failed deletion can be retried')
    failDelete=false;await evaluate(`document.querySelector('[data-delete-application]').click()`);await wait(`document.querySelectorAll('[data-application-id]').length===8`)
    await assert(`document.querySelector('[data-overall]').textContent.includes('51')&&document.querySelector('[data-filtered]').textContent.includes('8')&&document.querySelector('[name=author_id]').value==='1'`,'delete refreshes totals and preserves active filters')
    await evaluate(`document.querySelector('[name=student_name]').value='НЕТ СОВПАДЕНИЙ';document.querySelector('.course-students-filters').requestSubmit()`);await wait(`document.querySelector('[data-filtered]')?.textContent.includes('Найдено заявок: 0')`)
    await assert(`document.querySelector('[data-overall]').textContent.includes('51')`,'empty result retains global totals')
    await evaluate(`[...document.querySelector('.course-students-filters').querySelectorAll('button')].find(b=>b.textContent==='Сбросить').click()`);await wait(`document.querySelectorAll('[data-application-id]').length===50`)
    if(errors.length)throw Error(errors.join('\n'))
    console.log('PASS filters, totals, tariff prices, pagination, empty state, cancel/retry/delete; screenshots: '+output)
  }finally{ws?.close();browser.kill();server.close()}
})().catch(error=>{console.error(error);process.exitCode=1;server.close()})
