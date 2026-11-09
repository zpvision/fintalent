// Actual built React + real Academy API/PostgreSQL provided by the Go test.
// Auth is synthetic. All traffic is restricted to loopback.
const fs=require('fs'),path=require('path'),os=require('os'),{spawn}=require('child_process')
const [base,course,version,enrollment]=process.argv.slice(2)
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
  await navigate(`/profile/courses/${course}/academy?version=${version}`)
  await wait(`document.querySelector('select[aria-label="Версия программы"]')?.value==='${version}'`)
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
  if(errors.length)throw Error(errors.join('\n'))
  console.log(JSON.stringify({browser:'Edge headless',api:'real isolated PostgreSQL',draft_hidden:true,publish_visible:true,independent_copy:true,old_version_editable:true,widths:[360,900,1440],screenshots:temp}))
 }finally{ws?.close();browser.kill()}
})().catch(e=>{console.error(e.stack);process.exitCode=1})
