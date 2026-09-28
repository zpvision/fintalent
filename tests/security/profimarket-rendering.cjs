// Run with NODE_PATH pointing to an installed playwright-core; CHROME_PATH is optional.
const {chromium}=require('playwright-core');
const fs=require('node:fs'),assert=require('node:assert/strict');
(async()=>{
 const browser=await chromium.launch({executablePath:process.env.CHROME_PATH||'C:/Program Files/Google/Chrome/Application/chrome.exe',headless:true});
 try {
 const page=await browser.newPage();await page.route('**/*',r=>r.request().isNavigationRequest()&&r.request().url()==='http://localhost.invalid/'?r.fulfill({contentType:'text/html',body:'<html><head></head><body></body></html>'}):r.abort());
 await page.goto('http://localhost.invalid/');
 await page.addScriptTag({content:fs.readFileSync('static/profimarket-components.js','utf8')});
 for(const payload of ['invalid" onerror="window.auditXSS=1',"<img src=x onerror=window.auditXSS=1>",'javascript:window.auditXSS=1','data:text/html,<script>parent.auditXSS=1</script>','https://evil.example/embed']){
 const result=await page.evaluate(async payload=>{
 const api=window.ProfiMarketUI;
 const solution={id:1,type:'REGULATION',title:payload,slug:'fixture',short_description:payload,cover_image:payload,author_name:payload,author_avatar:payload,sections:[{title:payload,description:payload,image_url:payload,icon_image_url:payload,numbering_color:'red;position:fixed',items:[{title:payload}]}],tags:[payload],media:[{type:'VIDEO',url:payload,is_preview:true}],price:0};
 document.body.innerHTML=api.solutionCard(solution)+api.solutionView(solution)+api.solutionView({...solution,type:'AI_ASSISTANT'});
 await new Promise(r=>setTimeout(r,30));
 return {executed:window.auditXSS===1,events:[...document.querySelectorAll('*')].flatMap(n=>[...n.attributes].filter(a=>/^on/i.test(a.name)).map(a=>a.name)),iframes:document.querySelectorAll('iframe').length,css:[...document.querySelectorAll('.pmr-group')].some(n=>n.style.position==='fixed')};
 },payload);
 assert.equal(result.executed,false);assert.deepEqual(result.events,[]);assert.equal(result.iframes,0);assert.equal(result.css,false);
 }
 const valid=await page.evaluate(()=>{const api=window.ProfiMarketUI;return {quoted:api.esc('" & < >'),video:api.videoEmbedURL('https://rutube.ru/video/0123456789abcdef0123456789abcdef/'),image:api.safeURL('/static/uploads/a.png')}});
 assert.equal(valid.quoted,'&quot; &amp; &lt; &gt;');assert.equal(valid.image,'/static/uploads/a.png');assert.match(valid.video,/^https:\/\/rutube.ru\/play\/embed\//);
 const companySource=fs.readFileSync('static/accounting-company-view.js','utf8');
 const heroSource=companySource.slice(companySource.indexOf('  function heroImage()'),companySource.indexOf('  function breadcrumbs()'));
 for(const payload of ["x');position:fixed;background-image:url('x",'javascript:alert(1)','data:text/html,<script>alert(1)</script>','x\" onmouseover=\"window.auditXSS=1']){
  const result=await page.evaluate(({source,payload})=>{const render=new Function('company','esc',source+'; return heroImage()');document.body.innerHTML=render({header_image:payload},window.ProfiMarketUI.esc);const el=document.querySelector('.ac-profile-visual');return {position:el.style.position,image:el.style.backgroundImage,events:[...el.attributes].filter(a=>/^on/i.test(a.name)).length}}, {source:heroSource,payload});
  assert.equal(result.position,'');assert.equal(result.events,0);assert.match(result.image,/header-01\.jpg/);
 }
 console.log('PASS: stored XSS, unsafe iframe and CSS injection regression cases; valid resources preserved');
 }finally{await browser.close()}
})().catch(e=>{console.error(e);process.exitCode=1});
