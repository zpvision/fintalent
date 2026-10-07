// Local built assets and synthetic CSS failures only; no live server or DB.
const {chromium}=require('playwright-core');
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
(async()=>{
 const browser=await chromium.launch({executablePath:process.env.CHROME_PATH||'C:/Program Files/Google/Chrome/Application/chrome.exe',headless:true});
 try{
  const page=await browser.newPage();let mode='slow',attempts=0;const errors=[];page.on('pageerror',e=>errors.push(e.message));
  await page.route('**/*',async route=>{
   const url=new URL(route.request().url()),p=url.pathname;
   if(url.hostname!=='audit.invalid')return route.abort();
   if(p.startsWith('/api/')){
    let status=200,data={};
    if(p==='/api/me'){status=401;data={error:'guest'}}
    else if(p.startsWith('/api/profimarket/solution/'))data={id:1,type:'REGULATION',title:'Styles fixture',slug:'fixture',status:'PUBLISHED',price:100,pricing_type:'ONE_TIME',tags:[],sections:[],access_features:[]};
    else if(p.includes('/reviews'))data={reviews:[],can_review:false};
    else if(p.includes('/questions'))data={questions:[]};
    return route.fulfill({status,contentType:'application/json',body:JSON.stringify(data)});
   }
   if(p==='/static/profimarket.css'){
    attempts++;
    if(mode==='fail'||(mode==='once'&&attempts===1))return route.abort();
    if(mode==='slow')await new Promise(resolve=>setTimeout(resolve,4500));
   }
   const file=p.startsWith('/static/')?path.resolve('.'+p):path.resolve('static/react/index.html');
   if(!file.startsWith(path.resolve('static')+path.sep)||!fs.existsSync(file))return route.fulfill({status:404,body:''});
   const mime={'.js':'text/javascript','.css':'text/css','.html':'text/html','.svg':'image/svg+xml','.png':'image/png','.woff2':'font/woff2'}[path.extname(file)]||'application/octet-stream';
   return route.fulfill({contentType:mime,body:fs.readFileSync(file)});
  });
  const ready=()=>page.waitForFunction(()=>!document.documentElement.classList.contains('react-page-styles-loading'));
  for(const width of [360,900,1440]){
   await page.setViewportSize({width,height:900});mode='slow';attempts=0;
   await page.goto('http://audit.invalid/profimarket/solution/fixture',{waitUntil:'domcontentloaded'});
   await page.waitForFunction(()=>document.documentElement.classList.contains('react-page-styles-loading'));
   await page.waitForTimeout(3200);
   assert.equal(await page.evaluate(()=>document.documentElement.classList.contains('react-page-styles-loading')),true,'slow CSS must not count as loaded after 3 seconds');
   await ready();assert.equal(await page.locator('[data-page-styles-error]').count(),0);
   mode='once';attempts=0;await page.reload({waitUntil:'domcontentloaded'});await ready();assert.ok(attempts>=2,'transient CSS failure must retry');
   mode='fail';attempts=0;await page.reload({waitUntil:'domcontentloaded'});
   await page.locator('[data-page-styles-error]').waitFor();await page.waitForTimeout(1300);
   assert.equal(await page.evaluate(()=>document.documentElement.classList.contains('react-page-styles-loading')),true);
   assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1),false);
   mode='good';await page.getByRole('button',{name:'Повторить загрузку'}).click();await ready();
   assert.equal(await page.locator('[data-page-styles-error]').count(),0);
  }
  mode='fail';await page.reload({waitUntil:'domcontentloaded'});await page.locator('[data-page-styles-error]').waitFor();
  await page.evaluate(()=>{history.pushState({},'', '/login');dispatchEvent(new PopStateEvent('popstate'))});await ready();
  assert.equal(await page.locator('[data-page-styles-error]').count(),0,'navigation must remove stale error');
  assert.deepEqual(errors,[]);
  console.log('PASS: slow CSS, automatic retry, manual recovery, navigation cleanup × 3 widths');
 }finally{await browser.close()}
})().catch(e=>{console.error(e);process.exit(1)});
