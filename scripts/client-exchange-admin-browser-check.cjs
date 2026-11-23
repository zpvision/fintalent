// Real admin assets, synthetic HTTP fixtures only; no database or user sessions.
const fs = require('fs'), path = require('path'), http = require('http'), os = require('os'), {spawn} = require('child_process')
const root = path.resolve(__dirname, '..'), output = path.join(root, '.codex-run', 'client-exchange-admin')
fs.mkdirSync(output, {recursive:true})
const sleep = ms => new Promise(resolve => setTimeout(resolve, ms)), mutations = []
const stamp = '2026-11-23T10:00:00.123456Z'
let state = 'active', title = 'Синтетическая карточка <img src=x onerror="window.xss=true">', failSave = false
const summary = () => ({id:42,public_id:'synthetic-public-id',title,seller_user_id:5,seller_name:'Синтетический пользователь',seller_email:'synthetic@example.invalid',status:state,city:'Москва',transfer_price:12500,views_count:9,responses_count:2,created_at:stamp,updated_at:stamp,published_at:stamp})
const input = () => ({title,client_inn:'7707083893',client_legal_name:'Синтетическая компания',industry_id:1,industry_ids:[1],employee_range_id:2,tax_system_id:3,revenue_range_id:4,accounting_state_id:5,transfer_reason_id:6,transfer_reason_ids:[6],transfer_type_id:7,marketplace_ids:[8],edo_provider_ids:[9],accounting_program_ids:[10],transfer_reason_comment:'Причина передачи',comment:'Комментарий <script>window.xss=true</script>',current_step:6,city:'Москва',region:'Московская область',has_vat:true,foreign_trade:false,bargain_allowed:true,transfer_price:12500,monthly_commission_percent:5,commission_months:6,current_monthly_fee:1000,operations_per_month:10,banks_count:2,client_since:'2023-01-01',desired_transfer_date:null})
const dictionaries = ['industry','employee_range','tax_system','revenue_range','accounting_state','transfer_reason','transfer_type','marketplace','edo_provider','accounting_program'].map((kind,i)=>({id:i+1,kind,name:'Вариант '+(i+1),active:true}))
// Match the legacy/React shell navigation and workspace used by these modules.
const html = `<!doctype html><html lang="ru" data-admin-ready="true"><head><meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1"><link rel="stylesheet" href="/static/admin.css"><link rel="stylesheet" href="/static/admin-sidebar-scroll.css"></head><body><div class="admin-app"><aside class="sidebar"><nav><button id="dictionary-nav">Справочники</button></nav></aside><main class="workspace"><header><button class="mobile-menu">☰</button><div><h1>Админка</h1><p>Проверка</p></div></header><section id="dictionary-list"></section></main></div><script>document.querySelector('#dictionary-nav').onclick=()=>{document.querySelectorAll('.workspace>section').forEach(n=>n.classList.add('hidden'));document.querySelector('#dictionary-list').classList.remove('hidden')};document.querySelector('.mobile-menu').onclick=()=>document.querySelector('.sidebar').classList.toggle('open')</script><script src="/static/admin-client-exchange.js?v=3"></script><script src="/static/admin-client-exchange-listings.js?v=1"></script><script src="/static/admin-routing.js?v=5"></script></body></html>`
const server = http.createServer((req,res) => {
  const url = new URL(req.url,'http://localhost')
  const json = (status,data) => {res.writeHead(status,{'Content-Type':'application/json'});res.end(JSON.stringify(data))}
  if (url.pathname === '/api/admin/client-exchange/dictionaries') return json(200,{items:dictionaries})
  if (url.pathname === '/api/admin/client-exchange/listings') {
    let items = state === 'deleted' ? [] : [summary()]
    if (url.searchParams.get('q') === 'нет совпадений') items = []
    return json(200,{items,total:items.length,page:1,page_size:30})
  }
  if (url.pathname.startsWith('/api/admin/client-exchange/listings/42')) {
    if (req.method === 'GET') return json(200,{listing:summary(),input:input()})
    let body='';req.on('data',chunk=>body+=chunk);req.on('end',()=>{
      const data=JSON.parse(body);mutations.push({method:req.method,path:url.pathname,data})
      if (failSave) return json(409,{error:'Карточка уже изменилась. Откройте её заново'})
      if (data.updated_at !== stamp) return json(409,{error:'Некорректная версия'})
      if (req.method==='PUT') title=data.input.title
      if (url.pathname.endsWith('/archive')) state='archived'
      if (req.method==='DELETE') state='deleted'
      json(200,{ok:true})
    });return
  }
  if (url.pathname.startsWith('/static/')) {
    const file=path.join(root,url.pathname)
    if (file.startsWith(path.join(root,'static')+path.sep) && fs.existsSync(file)) {res.setHeader('Content-Type',file.endsWith('.js')?'text/javascript':'text/css');return fs.createReadStream(file).pipe(res)}
  }
  if (url.pathname.startsWith('/admin')) {res.setHeader('Content-Type','text/html; charset=utf-8');return res.end(html)}
  res.writeHead(404).end()
})
;(async()=>{
  await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
  const base=`http://127.0.0.1:${server.address().port}`, profile=fs.mkdtempSync(path.join(os.tmpdir(),'ft-ce-admin-'))
  const browser=spawn(process.env.EDGE_PATH || 'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe',['--headless=new','--disable-gpu','--no-first-run','--remote-debugging-port=0','--user-data-dir='+profile,'about:blank'],{windowsHide:true,stdio:'ignore'})
  let ws
  try {
    let port;for(let n=0;n<100;n++){try{port=fs.readFileSync(path.join(profile,'DevToolsActivePort'),'utf8').split('\n')[0];break}catch{await sleep(100)}}
    if(!port)throw Error('Headless Edge unavailable')
    const tabs=await(await fetch(`http://127.0.0.1:${port}/json/list`)).json()
    ws=new WebSocket(tabs.find(t=>t.type==='page').webSocketDebuggerUrl);await new Promise(resolve=>ws.addEventListener('open',resolve,{once:true}))
    let seq=0;const pending=new Map(),errors=[]
    ws.addEventListener('message',event=>{const m=JSON.parse(event.data);if(m.id){pending.get(m.id)?.(m);pending.delete(m.id)}if(m.method==='Runtime.exceptionThrown')errors.push(m.params.exceptionDetails.text);if(m.method==='Page.javascriptDialogOpening')void cmd('Page.handleJavaScriptDialog',{accept:true})})
    const cmd=(method,params={})=>new Promise((resolve,reject)=>{const id=++seq;pending.set(id,m=>m.error?reject(Error(JSON.stringify(m.error))):resolve(m.result));ws.send(JSON.stringify({id,method,params}))})
    const evaluate=async expression=>{const r=await cmd('Runtime.evaluate',{expression,returnByValue:true,awaitPromise:true});if(r.exceptionDetails)throw Error(r.exceptionDetails.exception?.description||r.exceptionDetails.text);return r.result.value}
    const wait=async expression=>{for(let n=0;n<100;n++){if(await evaluate(expression))return;await sleep(50)}throw Error('Timeout: '+expression+' '+await evaluate('document.body.innerText'))}
    const assert=async(expression,label)=>{if(!await evaluate(expression))throw Error(label);console.log('PASS '+label)}
    await cmd('Page.enable');await cmd('Runtime.enable');await cmd('Page.navigate',{url:base+'/admin/client-exchange/listings'})
    await wait(`!!document.querySelector('[data-card="42"]')`)
    await assert(`!document.querySelector('#client-exchange-listings-admin').classList.contains('hidden') && document.querySelector('#client-exchange-listings-nav').classList.contains('active')`,'direct admin route opens cards')
    await evaluate(`document.querySelector('[data-card="42"]').click()`);await wait(`!!document.querySelector('dialog[open]')`)
    await assert(`!window.xss && !document.querySelector('dialog img') && document.querySelector('dialog [name=title]').value.includes('<img')`,'untrusted text is escaped')
    await assert(`document.querySelector('dialog fieldset').disabled && document.querySelector('dialog [name=marketplace_ids]').selectedOptions[0].value==='8'`,'details are read-only and retain options')
    for(const width of [1440,900,360]){
      await cmd('Emulation.setDeviceMetricsOverride',{width,height:1000,deviceScaleFactor:1,mobile:false});await sleep(100)
      await assert(`document.documentElement.scrollWidth<=innerWidth && (()=>{const d=document.querySelector('dialog'),r=d.getBoundingClientRect();return r.left>=0 && r.right<=innerWidth+1 && d.scrollWidth<=d.clientWidth+1})()`,'modal within viewport '+width)
      const shot=await cmd('Page.captureScreenshot',{format:'png'});fs.writeFileSync(path.join(output,'modal-'+width+'.png'),Buffer.from(shot.data,'base64'))
    }
    await evaluate(`document.querySelector('[data-edit]').click();document.querySelector('dialog [name=title]').value='Изменённая карточка';document.querySelector('dialog form').requestSubmit()`)
    await wait(`!document.querySelector('dialog')`)
    if(mutations.length!==1 || mutations[0].data.input.marketplace_ids[0]!==8 || mutations[0].data.input.edo_provider_ids[0]!==9 || mutations[0].data.input.accounting_program_ids[0]!==10 || title!=='Изменённая карточка')throw Error('Edit lost fields')
    await evaluate(`document.querySelector('[data-card="42"]').click()`);await wait(`!!document.querySelector('dialog[open]')`)
    failSave=true
    await evaluate(`document.querySelector('[data-edit]').click();document.querySelector('dialog form').requestSubmit()`)
    await wait(`document.querySelector('dialog [data-message]').textContent.includes('уже изменилась')`)
    await assert(`!document.querySelector('[data-save]').disabled && document.querySelector('[data-delete]').disabled`,'conflict keeps editor and retry available')
    failSave=false
    await evaluate(`document.querySelector('dialog [data-close]').click()`);await wait(`!document.querySelector('dialog')`)
    await evaluate(`document.querySelector('[data-card="42"]').click()`);await wait(`!!document.querySelector('dialog[open]')`)
    await evaluate(`document.querySelector('[data-archive]').click()`);await wait(`!document.querySelector('dialog') && document.querySelector('[data-list]').textContent.includes('Снята с публикации')`)
    state='transferred'
    await evaluate(`document.querySelector('[data-card="42"]').click()`);await wait(`!!document.querySelector('dialog[open]')`)
    await assert(`!document.querySelector('[data-edit]') && !document.querySelector('[data-delete]') && !document.querySelector('[data-archive]')`,'transferred card is protected')
    await evaluate(`document.querySelector('dialog [data-close]').click()`);state='archived'
    await evaluate(`document.querySelector('[data-card="42"]').click()`);await wait(`!!document.querySelector('dialog[open]')`)
    await evaluate(`document.querySelector('[data-delete]').click()`);await wait(`!document.querySelector('dialog') && document.querySelector('[data-list]').textContent.includes('Карточки не найдены')`)
    await evaluate(`document.querySelector('#client-exchange-nav').click()`);await wait(`document.querySelector('#client-exchange-admin').classList.contains('active')`)
    await assert(`location.pathname==='/admin/client-exchange/employee_range' && document.querySelector('#client-exchange-listings-admin').classList.contains('hidden')`,'dictionary navigation remains available')
    await cmd('Page.navigate',{url:base+'/admin/client-exchange/listings'});await wait(`!!document.querySelector('.ce-listing-empty')`)
    if(errors.length)throw Error('Browser errors: '+errors.join(';'))
    console.log('PASS edit, conflict, archive, delete, empty state, navigation, XSS protection, complete options and layout; screenshots: '+output)
  } finally {ws?.close();browser.kill();server.close()}
})().catch(error=>{console.error(error);process.exitCode=1;server.close()})
