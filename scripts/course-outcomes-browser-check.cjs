// Built React with isolated HTTP fixtures; no production data or credentials.
const fs=require('fs'),http=require('http'),path=require('path'),os=require('os'),{spawn}=require('child_process')
const root=path.resolve(__dirname,'..'),temp=fs.mkdtempSync(path.join(os.tmpdir(),'ft-outcomes-')),sleep=ms=>new Promise(r=>setTimeout(r,ms))
let course={id:42,owner_id:1,revision:1,title:'Финансовая модель: от данных к решениям',description:'Практический курс для финансовых специалистов и бухгалтеров',slug:'fixture-course',status:'draft',level:'beginner',cover_image:'',category:'Финансы',benefits:[],bonuses:[],custom_features:[],feature_ids:[],tariffs:[],pricing_mode:'single',price:150000,hours:6,access_months:12,modules:[{id:1,title:'Основы финансового моделирования',lessons:[{id:1,title:'Первые расчёты',type:'video',duration_minutes:20}]}],instructors:[],reviews:[],faq:[],learning_outcomes:[]}
const server=http.createServer(async(req,res)=>{
 const u=new URL(req.url,'http://localhost'),send=(data,status=200)=>{res.writeHead(status,{'Content-Type':'application/json'});res.end(JSON.stringify(data))}
 if(u.pathname.startsWith('/api/')){
  if(u.pathname==='/api/me')return send({id:1,full_name:'Автор курса'})
  if(u.pathname==='/api/courses/access')return send({allowed:true})
  if(u.pathname==='/api/courses/meta')return send({categories:[],features:[],lesson_types:[]})
  if(u.pathname==='/api/courses/my/42'&&req.method==='PUT'){let raw='';for await(const c of req)raw+=c;const value=JSON.parse(raw);if(value.revision!==course.revision)return send({error:'Конфликт'},409);course={...course,...value,revision:course.revision+1};return send(course)}
  if(u.pathname==='/api/courses/my/42'||u.pathname==='/api/courses/view/fixture-course')return send(course)
  return send([])
 }
 const file=u.pathname.startsWith('/static/')?path.join(root,u.pathname):path.join(root,'static/react/index.html')
 try{res.setHeader('Content-Type',({'.html':'text/html','.js':'application/javascript','.css':'text/css','.svg':'image/svg+xml','.png':'image/png','.webp':'image/webp'})[path.extname(file)]||'application/octet-stream');res.end(fs.readFileSync(file))}catch{res.writeHead(404);res.end()}
})
;(async () => {
  await new Promise(r => server.listen(0, '127.0.0.1', r)); const base = `http://127.0.0.1:${server.address().port}`
  const browserPath = process.env.EDGE_PATH || 'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe'
  const browser = spawn(browserPath, ['--headless=new', '--disable-gpu', '--no-first-run', '--remote-debugging-port=0', '--user-data-dir=' + path.join(temp, 'browser'), 'about:blank'], { windowsHide: true, stdio: 'ignore' })
  let ws
  try {
    let debugPort
    for (let n = 0; n < 100; n++) { try { debugPort = fs.readFileSync(path.join(temp, 'browser/DevToolsActivePort'), 'utf8').split('\n')[0]; break } catch { await sleep(100) } }
    if (!debugPort) throw Error('Headless Edge did not start; set EDGE_PATH')
    const tabs = await (await fetch(`http://127.0.0.1:${debugPort}/json/list`)).json()
    ws = new WebSocket(tabs.find(t => t.type === 'page').webSocketDebuggerUrl); await new Promise(r => ws.addEventListener('open', r, { once: true }))
    let id = 0; const pending = new Map(), errors = []
    ws.addEventListener('message', e => { const m = JSON.parse(e.data); if (m.id) { pending.get(m.id)?.(m); pending.delete(m.id) } if (m.method === 'Runtime.exceptionThrown') errors.push(m.params.exceptionDetails.text); if (m.method === 'Runtime.consoleAPICalled' && m.params.type === 'error') errors.push(m.params.args.map(a => a.value || a.description).join(' ')) })
    const cmd = (method, params = {}) => new Promise(r => { const n = ++id; pending.set(n, r); ws.send(JSON.stringify({ id: n, method, params })) })
    ws.addEventListener('message', e => { const m=JSON.parse(e.data);if(m.method==='Page.javascriptDialogOpening') cmd('Page.handleJavaScriptDialog',{accept:true}) })
    const evaluate = async expression => { const r = await cmd('Runtime.evaluate', { expression, returnByValue: true, awaitPromise: true }); if (r.result?.exceptionDetails) throw Error(JSON.stringify(r.result.exceptionDetails)); return r.result?.result?.value }
    const assert = async (expression, label) => { if (!await evaluate(expression)) throw Error(label); console.log('PASS ' + label) }
    const wait = async expression => { for (let n = 0; n < 150; n++) { if (await evaluate(expression)) return; await sleep(100) } throw Error('Timeout: ' + expression) }
    const reload = async () => { const origin = await evaluate('performance.timeOrigin'); await cmd('Page.reload'); await wait('performance.timeOrigin!==' + origin) }
    const click = text => evaluate(`(()=>{const b=[...document.querySelectorAll('button')].find(b=>b.textContent.trim()===${JSON.stringify(text)} && !b.disabled);if(!b)throw Error('Button missing: '+${JSON.stringify(text)});b.click()})()`)
    const input = async (selector, value) => { await evaluate(`(()=>{const el=document.querySelector(${JSON.stringify(selector)});Object.getOwnPropertyDescriptor(el.tagName==='TEXTAREA'?HTMLTextAreaElement.prototype:HTMLInputElement.prototype,'value').set.call(el,${JSON.stringify(value)});el.dispatchEvent(new Event('input',{bubbles:true}))})()`); await sleep(60) }
    const widths = async (label, target) => { for (const width of [1440, 900, 360]) { await cmd('Emulation.setDeviceMetricsOverride', { width, height: 1100, deviceScaleFactor: 1, mobile: false }); await evaluate(`document.querySelector(${JSON.stringify(target)})?.scrollIntoView({block:'start'})`); await sleep(150); await assert('document.documentElement.scrollWidth<=innerWidth', label + ' no overflow ' + width); if (await evaluate('!!document.querySelector("dialog")')) await assert('(()=>{const r=document.querySelector(".academy-dialog").getBoundingClientRect();return r.left>=0 && r.right<=document.documentElement.clientWidth+1})()', 'dialog within viewport '+width); const shot = await cmd('Page.captureScreenshot', { format: 'png' }); fs.writeFileSync(path.join(temp, `${label}-${width}.png`), Buffer.from(shot.result.data, 'base64')) } }
    const navigate = async url => { const origin = await evaluate('performance.timeOrigin'); await cmd('Page.navigate', { url: base + url }); await wait('performance.timeOrigin!==' + origin) }
    const desktop = () => cmd('Emulation.setDeviceMetricsOverride', { width: 1440, height: 1100, deviceScaleFactor: 1, mobile: false })
    const close = () => evaluate('document.querySelector(".academy-dialog button[aria-label=Закрыть]").click()')
    const select = (selector, value) => evaluate(`(()=>{const el=document.querySelector(${JSON.stringify(selector)});Object.getOwnPropertyDescriptor(HTMLSelectElement.prototype,'value').set.call(el,${JSON.stringify(value)});el.dispatchEvent(new Event('change',{bubbles:true}))})()`)
    await cmd('Page.enable');await cmd('Runtime.enable');await navigate('/profile/courses/42/edit');await wait('!!document.querySelector(".course-editor-fields")')
    await assert('document.querySelectorAll(".wizard-step-node").length===6','six editor steps')
    await evaluate('document.querySelectorAll(".wizard-step-node")[3].click()');await wait('!!document.querySelector(".course-outcomes-editor")')
    const texts=['Строить финансовую модель бизнеса','Оценивать рентабельность проектов','Планировать денежные потоки','Анализировать <img src=x onerror=alert(1)> как обычный текст','Подготавливать отчёт для руководителя']
    for(let i=0;i<texts.length;i++){await click('＋ Добавить результат');await input('.course-outcome-field:nth-child('+(i+1)+') textarea',texts[i])}
    await evaluate('document.querySelectorAll(".course-outcome-field .course-remove")[1].click()');await wait('document.querySelectorAll(".course-outcome-field").length===4')
    await evaluate('document.querySelectorAll(".course-outcome-field")[3].querySelector("button").click()');await click('＋ Добавить результат')
    await widths('editor','.course-outcomes-editor');await desktop();await click('Сохранить черновик');await wait('document.querySelector(".course-editor-footer").textContent.includes("Сохранено")')
    if(course.learning_outcomes.length!==5||course.learning_outcomes[1]!==texts[2]||course.learning_outcomes[2]!==texts[4])throw Error('Reorder/delete/save failed')
    await reload();await wait('!!document.querySelector(".course-editor-fields")');await evaluate('document.querySelectorAll(".wizard-step-node")[3].click()');await wait('document.querySelectorAll(".course-outcome-field").length===5');console.log('PASS add/remove/reorder and saved reload')
    await click('Далее →');await wait('document.querySelector(".course-editor-fields h2").textContent==="Стоимость и доступ"');await click('Далее →');await wait('document.querySelector(".course-editor-fields h2").textContent==="Частые вопросы"');console.log('PASS pricing and FAQ remain after outcomes')
    await navigate('/profile/courses/42/preview');await wait('!!document.querySelector("#outcomes")');await assert('document.querySelectorAll("#outcomes li").length===4 && !document.querySelector("#outcomes img")','blank outcomes hidden and HTML rendered safely');await widths('preview','#outcomes')
    await desktop();await assert('getComputedStyle(document.querySelector("#outcomes ul")).gridTemplateColumns.split(" ").length===2','two columns on desktop')
    await cmd('Emulation.setDeviceMetricsOverride',{width:360,height:1100,deviceScaleFactor:1,mobile:false});await assert('getComputedStyle(document.querySelector("#outcomes ul")).gridTemplateColumns.split(" ").length===1','single column on mobile')
    await navigate('/courses/fixture-course');await wait('!!document.querySelector("#outcomes")');await assert('[...document.querySelectorAll("a")].some(a=>a.hash==="#outcomes")','results visible on public landing')
    course.learning_outcomes=[];await reload();await wait('!!document.querySelector(".course-public")');await assert('!document.querySelector("#outcomes") && ![...document.querySelectorAll(".course-tabs a")].some(a=>a.hash==="#outcomes")','old courses have no empty section')
    if(errors.length)throw Error('Frontend console: '+errors.join('\n'))
    console.log('PASS no console errors\nScreenshots: '+temp)
  }finally{ws?.close();browser.kill();server.close()}
})().catch(e=>{console.error(e);server.close();process.exitCode=1})
