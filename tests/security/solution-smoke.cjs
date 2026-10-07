// Built local assets + offline API fixtures. No real site, DB or emails.
const {chromium}=require('playwright-core');
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
(async()=>{const browser=await chromium.launch({executablePath:process.env.CHROME_PATH||'C:/Program Files/Google/Chrome/Application/chrome.exe',headless:true});
try {const page=await browser.newPage();let type='REGULATION';const errors=[];page.on('pageerror',e=>errors.push(e.message));
await page.route('**/*',async route=>{const url=new URL(route.request().url()),p=url.pathname;
if(url.hostname!=='audit.invalid')return route.abort();
if(p.startsWith('/api/')){
 let data={};let status=200;
 if(p==='/api/me'){status=401;data={error:'Not authenticated'}}
 else if(p.startsWith('/api/profimarket/solution/'))data={id:1,type,title:'Тестовое решение «Кавычки»',slug:'fixture',status:'PUBLISHED',author_name:'Автор',price:100,pricing_type:'ONE_TIME',short_description:'Проверка интерфейса',tags:[],media:[],sections:Array.from({length:4},(_,i)=>({title:'Раздел '+i,items:Array.from({length:10},(_,j)=>({title:'Регламент '+j}))})),key_metrics:[],access_features:[],ai_features:[],how_it_works:[],bonuses:[],product_data:{items:['Пункт 1','Пункт 2'],preview_count:1,features:['Возможность'],steps:['Шаг'],video_url:'javascript:alert(1)'}};
 else if(p.includes('/reviews'))data={reviews:[],can_review:false};
 else if(p.includes('/questions'))data={questions:[]};
 return route.fulfill({status,contentType:'application/json',body:JSON.stringify(data)});
}
let file=p.startsWith('/static/')?path.resolve('.'+p):path.resolve('static/react/index.html');
if(!file.startsWith(path.resolve('static')+path.sep))return route.abort();
if(!fs.existsSync(file))return route.fulfill({status:404,body:''});
const ext=path.extname(file),mime={'.js':'text/javascript','.css':'text/css','.html':'text/html','.svg':'image/svg+xml','.png':'image/png','.webp':'image/webp','.woff2':'font/woff2'}[ext]||'application/octet-stream';
return route.fulfill({contentType:mime,body:fs.readFileSync(file)});
});
for(const width of [360,900,1440]){await page.setViewportSize({width,height:900});for(type of ['REGULATION','AI_ASSISTANT','AUTOMATION','ONEC_INTEGRATION','INSTRUCTION','TEMPLATE','CHECKLIST']){
 await page.goto('http://audit.invalid/profimarket/solution/fixture');await page.locator('#pm-detail h1').waitFor();await page.waitForTimeout(200);
 assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1),false,`${type} ${width} overflow`);
 assert.equal(await page.locator('a[href^="javascript:"]').count(),0);
 const copy=await page.locator('#pm-detail').innerText();
 assert.match(copy, /(?:без оплаты на сайте|оплата на сайте не производится)/i, `${type}: lead disclosure`);
 assert.doesNotMatch(copy, /подтверждённые покупатели|доступны после покупки|Оплачено/);
 const links=page.locator('.pmp-question-link:visible');if(await links.count()){for(let i=0;i<2;i++){await page.evaluate(()=>scrollTo(0,0));await links.first().click();await page.waitForTimeout(400);assert.ok(page.url().endsWith('#questions'));}}
 await page.reload();await page.locator('#pm-detail h1').waitFor();
 assert.deepEqual(errors,[],`${type} ${width}`);
}}
console.log('PASS: 7 solution types × 3 widths, reload, repeated question links, unsafe React URL');
}finally{await browser.close()}})().catch(e=>{console.error(e);process.exit(1)});
