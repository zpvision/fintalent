// Actual built React UI with isolated loopback fixtures. No app/production DB.
// npm run build && node scripts/course-referrals-browser-check.cjs
const fs=require('fs'),http=require('http'),path=require('path'),os=require('os'),{spawn}=require('child_process')
const root=process.cwd(),temp=fs.mkdtempSync(path.join(os.tmpdir(),'ft-referrals-')),sleep=ms=>new Promise(r=>setTimeout(r,ms))
const course={id:42,owner_id:1,slug:'fixture',title:'Курс для финансовых специалистов',description:'Практический курс',status:'published',revision:1,pricing_mode:'single',price:100000,old_price:null,level:'beginner',access_months:6,hours:10,benefits:[],modules:[],instructors:[],faq:[],bonuses:[],custom_features:[],features:[],feature_ids:[],tariffs:[],learning_outcomes:[],cover_image:''}
let failPayment=false,paidRequests=[],applications=[{id:1,user_id:3,name:"Applicant",source_name:"Telegram",profile_url:"/profiles/view/3",commission_bps:1200,first_clicked_at:new Date().toISOString()},{id:2,user_id:4,name:"Another applicant",source_name:"FinTalent",profile_url:"",commission_bps:2000,first_clicked_at:new Date().toISOString()}];
let authenticated=true,failInterest=false,interestRequests=0,links=[],next=1,visits=[],interests=[],analyticsQueries=[]
const server=http.createServer(async(req,res)=>{
 const url=new URL(req.url,'http://localhost'),send=(v,status=200)=>{res.writeHead(status,{'Content-Type':'application/json'});res.end(JSON.stringify(v))}
 if(url.pathname.startsWith('/api/')){
  let body='';for await(const chunk of req)body+=chunk;const input=body?JSON.parse(body):{}
  if(url.pathname==='/api/me'&&!authenticated)return send({error:'Unauthorized'},401)
  if(url.pathname==='/api/me')return send({id:1,full_name:'Автор курса'})
  if(url.pathname==='/api/courses/access')return send({allowed:true})
  if(url.pathname==='/api/courses/meta')return send({categories:[],features:[],lesson_types:[]})
  if(url.pathname==='/api/courses/my/42')return send(course)
  if(url.pathname==='/api/courses/view/fixture')return send(course)
  if(url.pathname==='/api/courses')return send([])
  if(url.pathname.includes('/favorites/'))return send({favorite:false})
  if(url.pathname==='/api/courses/my/42/referrals'){
   if(req.method==='GET')return send(links.filter(l=>!l.deleted))
   if(links.some(l=>l.code===input.code))return send({error:'Этот код уже использовался'},409)
   const link={id:next++,name:input.name,code:input.code};links.push(link);return send(link,201)
  }
  if(/^\/api\/courses\/my\/42\/referrals\/\d+$/.test(url.pathname)){links.find(l=>l.id===Number(url.pathname.split('/').at(-1))).deleted=true;return send({deleted:true})}
  if(url.pathname==='/api/courses/referrals/visit'){await sleep(150);visits.push(input);return send(null,204)}
  if(url.pathname==='/api/courses/interest'){interestRequests++;if(!authenticated)return send({error:'Login required'},401);if(failInterest)return send({error:'Fixture save failure'},503);interests.push(input);return send(null,204)}
  if(url.pathname==='/api/academy/courses/42')return send({course,program:null})
  if(url.pathname.endsWith('/analytics/interests/1/paid')) { if(failPayment)return send({error:'Fixture invitation queue unavailable'},503);if(input.confirm!==true||input.version_id!==1)throw Error('Invalid confirmation');if(!applications[0].paid_at){paidRequests.push(input);applications[0].paid_at=new Date().toISOString();applications[0].invitation_status='pending';applications[0].paid_enrollment_id=1};return send({paid:true}) }
  if(url.pathname.endsWith('/analytics/overview'))return send({section:'overview',meta:{owner:true,courses:[course],cohorts:[],versions:[{id:1,number:1}],curators:[]},data:{manual_payments:{unpaid:applications.filter(r=>!r.paid_at).length,standard_bps:2000,partner_bps:1200},funnel:[0,0,0,0,0,0],activity:[],risk_students:[]},rows:[],total:0,page:1,size:50,notes:[],generated_at:new Date().toISOString()})
  if(url.pathname.includes('/analytics/students')){
   analyticsQueries.push(url.search)
   const source=url.searchParams.get('source'),student={enrollment_id:1,user_id:3,name:'Ученица курса',source_id:1,source_name:'Telegram-канал',cohort_name:'',version_number:1,enrolled_at:new Date().toISOString(),percent:0,summary:{completed:0,required:1},risk:{level:'low',inactive_days:0,reasons:[]}}
   return send({section:"students",meta:{owner:true,courses:[course],cohorts:[],versions:[{id:1,number:1}],curators:[]},data:{manual_payments:{unpaid:applications.filter(r=>!r.paid_at).length,standard_bps:2000,partner_bps:1200},can_mark_paid:true,sources:[{id:0,name:'FinTalent',interests:1},{id:1,name:'Telegram-канал',visits:8,visitors:4,accounts:2,interests:1},{id:2,name:'Старая рассылка',deleted:true,visits:2,visitors:1,accounts:1,interests:0}],interests:{rows:applications.filter(r=>url.searchParams.get('interest_payment')!=='unpaid'||!r.paid_at),total:applications.filter(r=>url.searchParams.get('interest_payment')!=='unpaid'||!r.paid_at).length,page:1,size:50}},rows:source==='platform'?[]:[{student}],total:1,page:1,size:50,notes:[],generated_at:new Date().toISOString()})
  }
  return send([])
 }
 const file=url.pathname.startsWith('/static/')?path.join(root,url.pathname):path.join(root,'static/react/index.html')
 if(!file.startsWith(root+path.sep)){res.writeHead(404);res.end();return}
 try{res.setHeader('Content-Type',({'.html':'text/html','.js':'application/javascript','.css':'text/css','.png':'image/png','.svg':'image/svg+xml'})[path.extname(file)]||'application/octet-stream');res.end(fs.readFileSync(file))}catch{res.writeHead(404);res.end()}
})
;(async()=>{
 await new Promise(r=>server.listen(0,'127.0.0.1',r));const base=`http://127.0.0.1:${server.address().port}`
 const browser=spawn(process.env.EDGE_PATH||'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe',['--headless=new','--disable-gpu','--no-first-run','--remote-debugging-port=0','--user-data-dir='+path.join(temp,'browser'),'about:blank'],{windowsHide:true,stdio:'ignore'})
 let ws
 try{
  let port;for(let i=0;i<100;i++){try{port=fs.readFileSync(path.join(temp,'browser/DevToolsActivePort'),'utf8').split('\n')[0];break}catch{await sleep(100)}}
  if(!port)throw Error('Set EDGE_PATH to a Chromium browser')
  const tabs=await(await fetch(`http://127.0.0.1:${port}/json/list`)).json();ws=new WebSocket(tabs.find(t=>t.type==='page').webSocketDebuggerUrl);await new Promise(r=>ws.addEventListener('open',r,{once:true}))
  let seq=0;const pending=new Map(),errors=[]
  ws.addEventListener('message',event=>{const m=JSON.parse(event.data);if(m.id){pending.get(m.id)?.(m);pending.delete(m.id)}if(m.method==='Runtime.exceptionThrown')errors.push(m.params.exceptionDetails.text)})
  const cmd=(method,params={})=>new Promise(resolve=>{const id=++seq;pending.set(id,resolve);ws.send(JSON.stringify({id,method,params}))})
  const evaluate=async expression=>{const r=await cmd('Runtime.evaluate',{expression,returnByValue:true,awaitPromise:true});if(r.result?.exceptionDetails)throw Error(JSON.stringify(r.result.exceptionDetails));return r.result?.result?.value}
  const wait=async expression=>{for(let i=0;i<120;i++){if(await evaluate(`Boolean(${expression})`))return;await sleep(100)}throw Error('Timeout: '+expression+'; body='+await evaluate('document.body.innerText')+'; errors='+errors.join('; '))}
  const assert=async(expression,label)=>{if(!await evaluate(expression))throw Error(label);console.log('PASS '+label)}
  const click=async text=>{await evaluate(`(()=>{const b=[...document.querySelectorAll('button')].find(b=>b.textContent.trim()===${JSON.stringify(text)}&&!b.disabled);if(!b)throw Error('Button missing: '+${JSON.stringify(text)});b.click()})()`);await sleep(100)}
  const input=async(selector,value)=>{await evaluate(`(()=>{const el=document.querySelector(${JSON.stringify(selector)});Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,'value').set.call(el,${JSON.stringify(value)});el.dispatchEvent(new Event('input',{bubbles:true}))})()`);await sleep(60)}
  const navigate=async url=>{await cmd('Page.navigate',{url:base+url});await sleep(200)}
  const widths=async(label,target)=>{for(const width of [360,900,1440]){await cmd('Emulation.setDeviceMetricsOverride',{width,height:1100,deviceScaleFactor:1,mobile:false});await evaluate(`document.querySelector(${JSON.stringify(target)})?.scrollIntoView({block:'start',behavior:'instant'})`);await sleep(200);await assert('document.documentElement.scrollWidth<=innerWidth','no overflow '+label+' '+width);const shot=await cmd('Page.captureScreenshot',{format:'png'});fs.writeFileSync(path.join(temp,label+'-'+width+'.png'),Buffer.from(shot.result.data,'base64'))}}
  await cmd('Runtime.enable');await cmd('Page.enable')
  await navigate('/profile/courses/42/edit');await wait('document.querySelectorAll(".wizard-step-node").length===6');await evaluate('document.querySelectorAll(".wizard-step-node")[4].click()');await wait('document.querySelector(".course-referrals")')
  await click('＋ Добавить партнёрскую ссылку');await input('.course-referrals input[maxlength="120"]','Telegram-канал');await input('.course-referrals input[maxlength="64"]','TELEGRAM2026')
  await assert('document.querySelector(".course-referral-preview").textContent.endsWith("/courses/fixture?ref=telegram2026")','normalized URL preview')
  await click('Создать ссылку');await wait('document.querySelector(".course-referral-row")');if(links.length!==1||links[0].code!=='telegram2026')throw Error('Wrong creation payload')
  await evaluate('Object.defineProperty(navigator,"clipboard",{value:{writeText:async value=>{window.copied=value}},configurable:true})');await click('Копировать');await assert('window.copied.endsWith("?ref=telegram2026") && document.querySelector(".course-notification").textContent.includes("скопирована")','copy and notification')
  await widths('editor','.course-referrals')
  await evaluate('document.querySelector(".course-referral-actions button[aria-label]").click()');await wait('document.querySelector(".course-referrals [role=group]")');await click('Удалить ссылку');await wait('!document.querySelector(".course-referral-row")');if(!links[0].deleted)throw Error('Soft deletion not requested');console.log('PASS confirmed deletion')
  await navigate('/courses/fixture?ref=telegram2026&utm_source=test');await wait('document.querySelector(".course-purchase .course-primary")');await evaluate('document.querySelector(".course-purchase .course-primary").click()');await wait('document.querySelector(".course-interest-success [role=dialog]")');if(!visits.length||interests.length!==1||interests[0].slug!=='fixture'||'referral_link_id'in interests[0])throw Error('Interest/visit payload');console.log('PASS visit then interest without trusted client source')
  await assert('document.body.style.overflow==="hidden" && Boolean(document.querySelector(".course-interest-success .publish-success-footer"))','modal locks page and includes payment note')
  await widths('success-modal','.course-interest-success [role=dialog]')
  await assert('(()=>{const r=document.querySelector(".course-interest-success [role=dialog]").getBoundingClientRect();return r.left>=0&&r.right<=innerWidth&&r.top>=0&&r.bottom<=innerHeight})()','dialog remains inside viewport')
  await evaluate('document.querySelector(".course-interest-success .publish-success-actions button").focus()');await cmd('Input.dispatchKeyEvent',{type:'keyDown',key:'Tab',code:'Tab',windowsVirtualKeyCode:9});await cmd('Input.dispatchKeyEvent',{type:'keyUp',key:'Tab',code:'Tab',windowsVirtualKeyCode:9});await assert('document.activeElement.classList.contains("publish-success-close")','keyboard focus stays in dialog')
  await cmd('Input.dispatchKeyEvent',{type:'keyDown',key:'Escape',code:'Escape',windowsVirtualKeyCode:27});await wait('!document.querySelector(".course-interest-success")');await assert('document.body.style.overflow!=="hidden"','Escape closes modal and restores page scroll')
  const priorVisits=visits.length;await evaluate('history.pushState({},"","/courses/fixture?ref=email2026&utm_source=test");window.dispatchEvent(new PopStateEvent("popstate"))');for(let n=0;n<30&&visits.length===priorVisits;n++)await sleep(50);if(visits.length!==priorVisits+1||visits.at(-1).code!=='email2026')throw Error('SPA ref navigation not captured');console.log('PASS same-course SPA referral navigation')
  await widths('landing','.course-purchase');await evaluate('document.querySelector(".course-mobile-cta button").click()');await sleep(300);if(interests.length!==2)throw Error('Mobile interest not recorded');console.log('PASS mobile course selection')
  await wait('document.querySelector(".course-interest-success")');await evaluate('document.querySelector(".course-interest-success .publish-success-actions button").click()');await wait('!document.querySelector(".course-interest-success")')
  failInterest=true;await evaluate('document.querySelector(".course-purchase .course-primary").click()');await wait('document.querySelector(".course-notification")');await assert('!document.querySelector(".course-interest-success")','no success dialog when saving fails');failInterest=false

  const before=visits.length;await navigate('/profile/courses/42/preview?ref=telegram2026');await wait('document.querySelector(".course-purchase")');await sleep(200);await evaluate('document.querySelector(".course-purchase .course-primary").click()');await sleep(100);if(visits.length!==before||interests.length!==2)throw Error('Preview polluted statistics');console.log('PASS preview creates no analytics events')
  await navigate('/profile/courses/42/academy/analytics?tab=students');await wait('document.querySelector(".analytics-source-filter select")');await assert('document.body.textContent.includes("Telegram-канал") && document.body.textContent.includes("Выбрали курс")','source and interest analytics')
  await assert('!document.querySelector(".analytics-filter-panel").open','audience filters collapsed by default')
  await assert('document.querySelector(".acquisition-person a").getAttribute("href")==="/profiles/view/3"','application links to actual public profile')
  await assert('document.body.textContent.includes("12%")&&document.body.textContent.includes("20%")','per-application commission rates displayed')
  await cmd('Emulation.setDeviceMetricsOverride',{width:1440,height:1100,deviceScaleFactor:1,mobile:false});await assert('getComputedStyle(document.querySelector(".analytics-acquisition-grid")).gridTemplateColumns.split(" ").length===2','acquisition panels use two desktop columns')
  await evaluate('document.querySelector(".acquisition-paid input").click()');await wait('document.querySelector(".analytics-payment-dialog[open]")');await widths('payment-confirmation','.analytics-payment-dialog');failPayment=true;await click('Оплата получена');await wait('document.querySelector(".analytics-payment-dialog .course-error")');if(paidRequests.length)throw Error('Failed confirmation changed payment');failPayment=false;await click('Оплата получена');await wait('!document.querySelector(".analytics-payment-dialog")');await wait('document.querySelector(".acquisition-paid.is-paid input:disabled")');if(paidRequests.length!==1)throw Error('Repeated confirmation');console.log('PASS confirmed payment, queue error handling and disabled paid checkbox')
  await widths('analytics','.analytics-source-filter');await widths('applications','.acquisition-applications');await evaluate('(()=>{const s=document.querySelector(".analytics-source-filter select");s.value="1";s.dispatchEvent(new Event("change",{bubbles:true}))})()');await sleep(300);if(!analyticsQueries.some(q=>q.includes('source=1')))throw Error('Source filter not sent to backend');console.log('PASS backend source filter')
  await navigate('/profile/courses/42/academy/analytics');await wait('document.querySelector(".analytics-unpaid-card")');await evaluate('document.querySelector(".analytics-unpaid-card").click()');await wait('document.querySelector(".acquisition-controls")');await assert('new URLSearchParams(location.search).get("tab")==="students"&&new URLSearchParams(location.search).get("interest_payment")==="unpaid"','overview navigates to unpaid applications')
  const savedBeforeGuest=interests.length,requestsBeforeGuest=interestRequests;authenticated=false;await navigate('/courses/fixture');await wait('document.querySelector(".course-purchase")');await sleep(300);await evaluate('document.querySelector(".course-purchase .course-primary").click()');await wait('document.querySelector(".course-interest-success")');await assert('document.querySelector(".course-interest-success .publish-success-actions a").getAttribute("href").startsWith("/login?next=")','guest is prompted to log in before application');if(interests.length!==savedBeforeGuest||interestRequests!==requestsBeforeGuest)throw Error('Guest attempted to save an application');console.log('PASS no guest interest API call or saved application');await widths('guest-modal','.course-interest-success [role=dialog]')
  await cmd('Emulation.setEmulatedMedia',{features:[{name:'prefers-reduced-motion',value:'reduce'}]});await assert('getComputedStyle(document.querySelector(".course-interest-success .publish-confetti")).display==="none"','reduced motion disables confetti')
  if(errors.length)throw Error('Browser exceptions: '+errors.join('; '));console.log('Screenshots: '+temp)
 }finally{if(ws)ws.close();browser.kill();server.close()}
})().catch(e=>{console.error(e);server.close();process.exitCode=1})
