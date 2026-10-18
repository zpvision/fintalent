// Headless verification against the actual built React app and isolated in-memory fixtures.
// Run after npm run build. No application DB, credentials or production requests are used.
const fs = require('fs'), http = require('http'), path = require('path'), os = require('os'), { spawn } = require('child_process')
const root = path.resolve(__dirname, '..'), temp = fs.mkdtempSync(path.join(os.tmpdir(), 'ft-academy-check-')), sleep = ms => new Promise(r => setTimeout(r, ms))
const png = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jN0sAAAAASUVORK5CYII=', 'base64')
fs.writeFileSync(path.join(temp, 'image.png'), png); fs.writeFileSync(path.join(temp, 'material.pdf'), '%PDF-1.7\nFixture')
const course = { id: 42, owner_id: 1, title: 'Практический курс для финансовых специалистов', status: 'draft', slug: 'fixture', cover_image: '/static/academy/program-start.png', landing_modules: 2, landing_lessons: 2 }
let program = null, nextID = 10, failNext = false, delaySave = 0, saving = false, media = [], versions = []
const key = () => 'fixture-' + nextID++, lesson = () => ({ id: nextID++, key: key(), title: 'Новый урок', description: '', status: 'draft', duration_minutes: 15, required: true, free_preview: false, self_paced: true, completion_rule: 'manual', cover_asset_id: 0, blocks: [] })
const moduleFixture = () => ({ id: nextID++, key: key(), title: 'Модуль обучения', description: '', status: 'draft', cover_asset_id: 0, lessons: [] })
function assignIDs(p) { p.modules.forEach(m => { m.id ||= nextID++; m.lessons.forEach(l => { l.id ||= nextID++; l.blocks.forEach(b => { b.id ||= nextID++ }) }) }); return p }
function action(a) {
  let m = program.modules.find(m => m.key === a.key), target = program.modules.find(m => m.key === a.target), l, b
  for (const mod of program.modules) for (const les of mod.lessons) { if (les.key === a.key) { m = mod; l = les } for (const block of les.blocks) if (block.key === a.key) { m = mod; l = les; b = block } }
  if (a.type === 'module.create') program.modules.push(moduleFixture())
  if (a.type === 'lesson.create') target.lessons.push(lesson())
  if (a.type === 'lesson.move') { m.lessons = m.lessons.filter(x => x !== l); target.lessons.splice(a.index, 0, l) }
  if (a.type === 'lesson.delete') m.lessons = m.lessons.filter(x => x !== l)
  if (a.type === 'module.delete') program.modules = program.modules.filter(x => x !== m)
  if (a.type === 'block.delete') l.blocks = l.blocks.filter(x => x !== b)
  if (a.type === 'block.duplicate') { const copy = structuredClone(b); copy.id = nextID++; copy.key = key(); l.blocks.splice(l.blocks.indexOf(b) + 1, 0, copy) }
  if (a.type === 'module.move') { program.modules = program.modules.filter(x => x !== m); program.modules.splice(a.index, 0, m) }
  program.revision++; return program
}
const server = http.createServer(async (req, res) => {
  const url = new URL(req.url, 'http://localhost'), send = (v, status = 200) => { res.writeHead(status, { 'Content-Type': 'application/json' }); res.end(JSON.stringify(v)) }
  if (url.pathname.startsWith('/api/')) {
    if (url.pathname === '/api/me') return send({ id: 1, full_name: 'Автор курса' })
    if (url.pathname === '/api/courses/access') return send({ allowed: true })
    if (!url.pathname.startsWith('/api/academy/courses/42')) return send([])
    if (req.method === 'GET') {
      if (url.pathname.endsWith('/program')) return send(program)
      if (url.pathname.endsWith('/assets')) return send(media)
      if (url.pathname.includes('/assets/')) { const asset = media.find(a => a.id === Number(url.pathname.split('/').at(-1))); res.setHeader('Content-Type', asset?.mime || 'application/pdf'); res.end(asset?.mime === 'image/png' ? png : '%PDF-1.7\nfixture'); return }
      return send({ course, program, tests: [{ id: 2, title: 'Проверка знаний', version: 1 }], versions })
    }
    let body = ''; for await (const chunk of req) body += chunk
    if (url.pathname.endsWith('/assets')) { const image = body.includes('filename="image.png"'); const asset = { id: nextID++, name: image ? 'Иллюстрация.png' : 'Учебный материал.pdf', mime: image ? 'image/png' : 'application/pdf', size: body.length }; media.push(asset); return send(asset, 201) }
    const data = JSON.parse(body)
    if (url.pathname.endsWith('/program')) {
      if (req.method === 'POST') { if (!program) { program = { id: 1, course_id: 42, revision: 1, source: data.source, modules: [] }; if (data.source === 'landing') for (let i = 0; i < 2; i++) { const m = moduleFixture(); m.lessons.push(lesson()); program.modules.push(m) } }; return send(program) }
      saving = true; await sleep(delaySave); saving = false
      if (failNext) { failNext = false; return send({ error: 'Проверка ошибки сети: правки не потеряны' }, 503) }
      if (program.revision !== data.revision) return send({ error: 'Конфликт ревизии' }, 409)
      program = assignIDs({ ...data, revision: data.revision + 1 }); return send(program)
    }
    if (url.pathname.endsWith('/actions')) return send(action(data))
    if (url.pathname.endsWith('/versions')) { versions.push({ id: 1, number: versions.length + 1 }); return send(versions.at(-1)) }
    return send({ error: 'Unknown fixture route' }, 404)
  }
  const file = url.pathname.startsWith('/static/') ? path.join(root, url.pathname) : path.join(root, 'static/react/index.html')
  try { res.setHeader('Content-Type', ({ '.html': 'text/html', '.js': 'application/javascript', '.css': 'text/css', '.png': 'image/png' })[path.extname(file)] || 'application/octet-stream'); res.end(fs.readFileSync(file)) } catch { res.writeHead(404); res.end() }
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
    const widths = async (label, target) => { for (const width of [1440, 900, 360]) { await cmd('Emulation.setDeviceMetricsOverride', { width, height: 1100, deviceScaleFactor: 1, mobile: false }); await evaluate(`document.querySelector(${JSON.stringify(target)})?.scrollIntoView({block:'start'})`); await sleep(100); await assert('document.documentElement.scrollWidth<=innerWidth', label + ' no overflow ' + width); if(label==='settings') await assert('(()=>{const r=document.querySelector(".academy-dialog").getBoundingClientRect();return r.left>=0 && r.right<=document.documentElement.clientWidth+1})()', 'settings within viewport '+width); const shot = await cmd('Page.captureScreenshot', { format: 'png' }); fs.writeFileSync(path.join(temp, `${label}-${width}.png`), Buffer.from(shot.result.data, 'base64')) } }
    await cmd('Page.enable'); await cmd('Runtime.enable'); await cmd('Page.navigate', { url: base + '/profile/courses/42/academy' })
    await wait('!!document.querySelector(".academy-start-options")'); await widths('start', '.academy-start')
    await click('Скопировать программу'); await wait('document.querySelectorAll(".academy-tree-column .academy-tree-module").length===2')
    await reload(); await wait('document.querySelectorAll(".academy-tree-column .academy-tree-module").length===2'); console.log('PASS copied structure persists')
    await cmd('Emulation.setDeviceMetricsOverride', { width: 1440, height: 1100, deviceScaleFactor: 1, mobile: false }); await evaluate('document.querySelector(".academy-lesson-select").click()'); await wait('!!document.querySelector(".academy-lesson-heading")')
    delaySave = 500; await input('.academy-lesson-heading input', 'Первая правка'); for (let i = 0; !saving && i < 50; i++) await sleep(50); await input('.academy-lesson-heading input', 'Новая правка во время сохранения'); await wait('document.querySelector(".academy-save-status").textContent==="Сохранено"'); if (program.modules[0].lessons[0].title !== 'Новая правка во время сохранения') throw Error('late response lost edits'); console.log('PASS serialized autosave preserves later edits'); delaySave = 0
    failNext = true; await input('.academy-lesson-heading input', 'Правка после ошибки сети'); await wait('document.querySelector(".academy-save-status").textContent==="Не сохранено"'); await assert('document.querySelector(".academy-lesson-heading input").value==="Правка после ошибки сети"', 'network error retains input'); await click('Повторить сохранение'); await wait('document.querySelector(".academy-save-status").textContent==="Сохранено"')
    failNext = true; await input('.academy-lesson-heading input','Восстановленная правка'); await wait('document.querySelector(".academy-save-status").textContent==="Не сохранено"');await reload();await wait('document.querySelector(".academy-save-status")?.textContent==="Не сохранено"');await evaluate('document.querySelector(".academy-lesson-select").click()');await wait('!!document.querySelector(".academy-lesson-heading")');await assert('document.querySelector(".academy-lesson-heading input").value==="Восстановленная правка"','draft restored after reload')
    await evaluate('document.querySelector(".academy-insert").click()'); await wait('!!document.querySelector(".academy-block-picker")'); await evaluate('[...document.querySelectorAll(".academy-block-picker button")].find(b=>b.querySelector("b").textContent==="Текст").click()'); await wait('!!document.querySelector(".ce-paragraph")')
    await evaluate('const el=document.querySelector(".ce-paragraph");el.focus();el.textContent="Текст урока с автосохранением";el.dispatchEvent(new InputEvent("input",{bubbles:true,inputType:"insertText",data:"Текст урока с автосохранением"}))'); await sleep(400)
    await click('Предпросмотр'); await wait('!!document.querySelector(".academy-lesson-renderer")'); await assert('document.querySelector(".academy-lesson-renderer").textContent.includes("Текст урока с автосохранением")','shared lesson renderer'); await evaluate('document.querySelector(".academy-dialog button[aria-label=Закрыть]").click()')
    await click('Настройки'); await wait('!!document.querySelector(".academy-settings")'); await widths('settings', '.academy-dialog'); await evaluate('document.querySelector(".academy-dialog button[aria-label=Закрыть]").click()')
    await widths('editor', '.academy-lesson-heading'); await click('Программа курса'); await wait('!!document.querySelector("dialog .academy-tree")'); await assert('document.querySelectorAll("dialog .academy-lesson-select").length===2','mobile tree available'); await evaluate('document.querySelector(".academy-dialog button[aria-label=Закрыть]").click()')
    await cmd('Emulation.setDeviceMetricsOverride', { width: 1440, height: 1100, deviceScaleFactor: 1, mobile: false })
    await evaluate('document.querySelectorAll(".academy-lesson-select")[1].click()'); await wait('!!document.querySelector(".academy-empty h2")'); await evaluate('document.querySelector(".academy-lesson-select").click()'); await wait('!!document.querySelector(".ce-paragraph")'); await assert('document.querySelector(".ce-paragraph").textContent.includes("Текст урока")','switching lessons preserves content')
    const addBlock = async (type, label) => { await evaluate('[...document.querySelectorAll(".academy-insert")].at(-1).click()'); await wait('!!document.querySelector(".academy-block-picker")'); await evaluate(`[...document.querySelectorAll('.academy-block-picker button')].find(b=>b.querySelector('b').textContent===${JSON.stringify(label)}).click()`); await wait(`!!document.querySelector('.academy-block-editor[data-block-type=${type}]')`) }
    const upload = async (selector, file) => { const result = await cmd('Runtime.evaluate', { expression: `document.querySelector(${JSON.stringify(selector)})` }); await cmd('DOM.setFileInputFiles', { objectId: result.result.result.objectId, files: [path.join(temp, file)] }) }
    await addBlock('video', 'Видео'); await input('[data-block-type=video] input[type=url]', 'https://cdn.example.invalid/lesson.mp4'); await wait('!!document.querySelector(".academy-video video")'); console.log('PASS supported video preview')
    await addBlock('image', 'Изображение'); await upload('[data-block-type=image] input[type=file]', 'image.png'); await wait('!!document.querySelector("[data-block-type=image] .academy-upload img")'); await assert('document.querySelector("[data-block-type=image] .academy-upload img").src.includes("/api/academy/")','private image reference')
    await addBlock('file', 'Файл'); await upload('[data-block-type=file] input[type=file]', 'material.pdf'); await wait('!!document.querySelector("[data-block-type=file] .academy-asset")'); console.log('PASS file upload')
    await addBlock('link', 'Ссылка'); await input('[data-block-type=link] input[type=url]', 'https://example.invalid/resource')
    await addBlock('note', 'Примечание'); await input('[data-block-type=note] textarea', 'Важная подсказка участнику')
    await addBlock('test', 'Тест FinTalent'); await evaluate('const select=document.querySelector("[data-block-type=test] select");select.value="2";select.dispatchEvent(new Event("change",{bubbles:true}))')
    await addBlock('assignment', 'Практическое задание'); await wait('document.querySelectorAll(".ce-paragraph").length>=2'); await input('[data-block-type=assignment] input', 'Примените знания на практике'); await input('[data-block-type=assignment] textarea', 'Выполните задание по инструкции')
    await widths('materials', '[data-block-type=assignment]')
    await cmd('Emulation.setDeviceMetricsOverride', { width: 1440, height: 1100, deviceScaleFactor: 1, mobile: false }); await click('Предпросмотр'); await wait('document.querySelectorAll(".academy-render-block").length===8'); await assert('!!document.querySelector(".academy-render-block.type-test") && !!document.querySelector(".academy-callout") && document.querySelector(".academy-assignment").textContent.includes("Примените знания")','all eight block types render'); await widths('preview', '.academy-lesson-renderer'); await evaluate('document.querySelector(".academy-dialog button[aria-label=Закрыть]").click()')
    await cmd('Emulation.setDeviceMetricsOverride', { width: 1440, height: 1100, deviceScaleFactor: 1, mobile: false })
    const firstModule = program.modules[0].key, revisionBeforeDrag = program.revision
    await evaluate('(()=>{const modules=document.querySelectorAll(".academy-tree-column .academy-tree-module"),dataTransfer=new DataTransfer();modules[0].querySelector(".academy-grip").dispatchEvent(new DragEvent("dragstart",{bubbles:true,dataTransfer}));modules[1].dispatchEvent(new DragEvent("drop",{bubbles:true,dataTransfer}))})()');for(let i=0;program.revision===revisionBeforeDrag&&i<100;i++)await sleep(50);if(program.modules[1].key!==firstModule)throw Error('module drag order not saved');console.log('PASS module drag and drop persisted')
    await wait('[...document.querySelectorAll("button")].some(b=>b.textContent.trim()==="Зафиксировать версию" && !b.disabled)'); await click('Зафиксировать версию'); await wait('!!document.querySelector(".academy-dialog")'); await evaluate('[...document.querySelectorAll("dialog button")].find(b=>b.textContent.trim()==="Зафиксировать версию").click()'); await wait('!!document.querySelector(".academy-notice")'); console.log('PASS version preparation')
    await reload(); await wait('!!document.querySelector(".academy-tree")'); await assert('!document.querySelector(".academy-start")','repeat opening does not initialize')
    program = null; course.landing_modules = 0; course.landing_lessons = 0; await reload(); await wait('!!document.querySelector(".academy-start")'); await assert('[...document.querySelectorAll("button")].find(b=>b.textContent.trim()==="Скопировать программу").disabled','empty landing disables copy'); await click('Создать с нуля'); await wait('!!document.querySelector(".academy-workspace")'); if (program.modules.length) throw Error('empty program not empty'); console.log('PASS create from scratch')
    await click('＋ Добавить модуль'); await wait('!!document.querySelector(".academy-module-editor")'); await click('＋ Добавить первый урок'); await wait('!!document.querySelector(".academy-lesson-heading")'); console.log('PASS module and first lesson creation')
    await addBlock('text','Текст');await wait('!!window.FinTalentEditorJS');await evaluate('(async()=>{const holder=document.createElement("div");document.body.append(holder);const legacy=await window.FinTalentEditorJS.mount({holder,data:[{type:"paragraph",text:"Существующая публикация"},{type:"note",text:"Примечание публикации"},{type:"image",url:"",caption:""}]});const output=await legacy.save();if(!output.some(b=>b.type==="note"))throw Error("Publication tools changed");legacy.editor.destroy();holder.remove()})()');console.log('PASS existing publication editor tools')
    if (errors.length) throw Error('Frontend console: ' + errors.join('\n'))
    console.log('PASS no frontend console errors\nScreenshots: ' + temp)
  } finally { ws?.close(); browser.kill(); server.close() }
})().catch(e => { console.error(e); server.close(); process.exitCode = 1 })
