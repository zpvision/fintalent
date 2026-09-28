// Browser UI contract using built assets and an in-memory API fixture only.
const {chromium}=require('playwright-core');
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
(async()=>{
 const browser=await chromium.launch({executablePath:process.env.CHROME_PATH||'C:/Program Files/Google/Chrome/Application/chrome.exe',headless:true});
 try {
  const page=await browser.newPage();const errors=[],leakedReferrers=[];page.on('pageerror',e=>errors.push(e.message));
  const questions=['single_choice','multiple_choice','boolean','text'].map((kind,i)=>({id:i+1,question:`Fixture question ${i+1}`,question_type:kind,points:1,answers:kind==='text'?[]:[{id:i*10+1,answer:'Option A'},{id:i*10+2,answer:'Option B'}]}));
  let attempt,starts=0,finishCalls=0,failFinish=true;
  const snapshot=()=>({...attempt,remaining_seconds:Math.max(0,300-Math.floor((Date.now()-Date.parse(attempt.started_at))/1000))});
  await page.route('**/*',async route=>{
   const req=route.request(),url=new URL(req.url()),p=url.pathname;
   if((req.headers().referer||'').includes('PRIVATE_INVITATION_FIXTURE'))leakedReferrers.push(req.url());
   if(url.hostname!=='audit.invalid')return route.abort();
   if(p.startsWith('/api/')){
    let data={},status=200;
    if(p==='/api/me')data={id:10,full_name:'Fixture user',email:'fixture@example.invalid'};
    else if(p.startsWith('/api/employee-test/'))data={test_title:'Employee fixture',employee_name:'Fixture',status:'pending',question_count:4,time_limit_seconds:300,questions:[],answered_question_ids:[]};
    else if(p==='/api/v1/contact-threads'||p.includes('/test-reviews'))data=[];
    else if(p==='/api/tests/1')data={id:1,title:'Reload fixture',questions,time_limit_seconds:300};
    else if(p==='/api/tests/1/attempts'){starts++;attempt={id:17,test_id:1,test_title:'Reload fixture',status:'started',started_at:new Date(Date.now()-40000).toISOString(),time_limit_seconds:300,questions,answers:[]};data=snapshot()}
    else if(p==='/api/attempts/17')data=snapshot();
    else if(p==='/api/attempts/17/answers'){
     const input=req.postDataJSON();attempt.answers=attempt.answers.filter(a=>a.question_id!==input.question_id);
     if(input.selected_answer_ids.length)for(const id of input.selected_answer_ids)attempt.answers.push({question_id:input.question_id,selected_answer_id:id});
     else attempt.answers.push({question_id:input.question_id,text_answer:input.text_answer});
    }else if(p==='/api/attempts/17/finish'){
     finishCalls++;if(failFinish){failFinish=false;status=503;data={error:'Fixture temporary finish failure'}}
     else {attempt={...attempt,status:'finished',percent:100,score:4,max_score:4,passed:true,duration_seconds:50};data=snapshot()}
    }
    return route.fulfill({status,contentType:'application/json',body:JSON.stringify(data)});
   }
   const file=p.startsWith('/static/')?path.resolve('.'+p):path.resolve('static/react/index.html');
   if(!file.startsWith(path.resolve('static')+path.sep))return route.abort();
   if(!fs.existsSync(file))return route.fulfill({status:404,body:''});
   const mime={'.js':'text/javascript','.css':'text/css','.html':'text/html','.svg':'image/svg+xml','.png':'image/png','.webp':'image/webp','.woff2':'font/woff2'}[path.extname(file)]||'application/octet-stream';
   return route.fulfill({contentType:mime,body:fs.readFileSync(file)});
  });
  const question=async n=>{await page.locator('.question-label').filter({hasText:`Вопрос ${n}`}).waitFor()};
  for(const width of [360,900,1440]){
   starts=0;finishCalls=0;failFinish=true;
   await page.setViewportSize({width,height:900});await page.goto('http://audit.invalid/tests/take?id=1');
   await page.getByRole('button',{name:'Начать тест'}).click();await question(1);
   await page.waitForURL('**attempt=17');
   const started=attempt.started_at;
   await page.locator('.options-message li').first().click();await question(2);
   await page.reload();await question(2);
   assert.equal(await page.locator('.user-message p').first().textContent(),'Option A','saved conversation answer lost on reload');
   assert.equal(starts,1,'reload created another attempt');assert.equal(attempt.started_at,started);
   const timer=await page.locator('.chat-progress span').textContent();assert.match(timer,/^04:/,'server remaining time lost');
   await page.locator('.options-message li').nth(0).click();await page.locator('.options-message li').nth(1).click();
   await page.getByRole('button',{name:'Ответить',exact:true}).click();await question(3);
   await page.locator('.options-message li').first().press('Enter');await question(4);
   await page.locator('.chat-compose input').fill('Fixture text answer');await page.locator('.chat-compose input').press('Enter');
   await page.locator('.bad').waitFor();assert.equal(finishCalls,1);
   await page.reload();await page.getByRole('button',{name:'Завершить тест',exact:true}).click();
   await page.locator('.result-message').waitFor();assert.equal(starts,1);assert.equal(finishCalls,2);assert.equal(attempt.answers.length,5);
   assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1),false,`overflow ${width}`);
   await page.reload();await page.locator('.history-review').waitFor();assert.equal(starts,1);
   assert.deepEqual(errors,[]);
  }
  // SPA entry must protect the token before effects request data or styles.
  await page.evaluate(()=>{history.pushState({},'', '/employee-test?token=PRIVATE_INVITATION_FIXTURE');dispatchEvent(new PopStateEvent('popstate'))});
  await page.locator('.et-public-intro').waitFor();
  assert.match(await page.locator('.et-public-intro').innerText(), /имя и результаты тестирования будут доступны всем посетителям/);
  assert.equal(await page.locator('meta[name="referrer"]').getAttribute('content'),'no-referrer');
  await page.evaluate(()=>fetch('/static/logo.png?privacy-probe'));
  assert.deepEqual(leakedReferrers,[],'invitation token escaped via Referer');
  await page.goBack();await page.locator('.history-review').waitFor();
  assert.equal(await page.locator('meta[name="referrer"]').count(),0,'referrer policy not cleaned up');
  assert.deepEqual(errors,[]);
  console.log('PASS: four question types, keyboard, reload preserves attempt/timer/index, finish retry, history × 3 widths; SPA invitation referrer privacy/cleanup');
 } finally {await browser.close()}
})().catch(e=>{console.error(e);process.exit(1)});
