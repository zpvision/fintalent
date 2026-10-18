// Built React + isolated API fixtures. Authorization/version/progress SQL is tested separately.
// No credentials, application database or production requests are used by this browser check.
const fs = require('fs'), http = require('http'), path = require('path'), os = require('os'), { spawn } = require('child_process')
const root = path.resolve(__dirname, '..'), temp = fs.mkdtempSync(path.join(os.tmpdir(), 'ft-learning-check-')), sleep = ms => new Promise(r => setTimeout(r, ms))
const png = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jN0sAAAAASUVORK5CYII=', 'base64')
const course = { id: 42, owner_id: 1, title: 'Практический курс для финансовых специалистов', status: 'draft', slug: 'fixture', cover_image: '/static/academy/program-start.png', author_name: 'Автор курса' }
const block = (type, data) => ({ key: type, type, data })
const lesson = (key, title, required = true, completion_rule = 'manual', blocks = [block('text', { rich: [{ type: 'paragraph', text: 'Структурированные учебные материалы' }] })]) => ({ key, title, status: 'published', self_paced: true, required, completion_rule, duration_minutes: 15, description: 'Учебный материал опубликованной версии', blocks })
const program = { id: 1, course_id: 42, revision: 2, modules: [{ key: 'module-one', title: 'Основы и применение', status: 'published', lessons: [lesson('lesson-one', 'Начало обучения', true, 'manual', [block('text', { rich: [{ type: 'h2', text: 'Материалы вашего курса' }, { type: 'paragraph', text: 'Текст <b>с форматированием</b> и безопасным отображением' }] }), block('image', { asset_id: 10, caption: 'Схема обучения' }), block('file', { asset_id: 11 }), block('link', { title: 'Дополнительный ресурс', url: 'https://example.invalid/resource' }), block('note', { title: 'Совет', variant: 'tip', text: 'Применяйте знания на практике' }), block('video', { url: 'https://www.youtube.com/watch?v=fixture', poster_id: 10, title: 'Видеоматериал' })]), lesson('lesson-optional', 'Дополнительная практика', false)] }, { key: 'module-two', title: 'Закрепление материала', status: 'published', lessons: [lesson('lesson-last', 'Итоговое чтение', true, 'materials')] }] }
const assets = [{ id: 10, name: 'Схема.png', mime: 'image/png', size: 1000 }, { id: 11, name: 'Материалы.pdf', mime: 'application/pdf', size: 2000 }]
let actor = 2, enrollments = [], releases = [{ id: 1, number: 1, revision: 1, readiness: { lessons: 0, required: 0, blockers: ['Обязательный урок «Новый урок» не опубликован', 'Опубликуйте хотя бы один модуль и урок в редакторе, затем зафиксируйте новую версию'], warnings: [] } }], failComplete = false, failRoom = false, nextID = 100, opens = 0, paged = false
const now = () => new Date().toISOString()
function summary(e) { const required = e.program.modules.flatMap(m => m.lessons).filter(l => l.required); const completed = required.filter(l => e.progress.some(p => p.lesson_key === l.key && p.status === 'completed')).length; return { required: required.length, completed, total_lessons: e.program.modules.flatMap(m => m.lessons).length, completed_lessons: e.progress.filter(p => p.status === 'completed').length, percent: Math.floor(completed * 100 / required.length), status: completed === required.length ? 'completed' : e.started_at ? 'in_progress' : 'not_started' } }
const dto = e => ({ ...e, program: undefined, summary: summary(e) })
function room(e) { return { enrollment: dto(e), program: e.effective_status === 'active' ? e.program : null, assets, tests: [{ id: 2, title: 'Проверка знаний', description: 'Закрепите изученное' }], completion_blocks: e.id === 101 ? { 'lesson-test': 'Интерактивное прохождение теста пока недоступно', 'lesson-assignment': 'Сдача задания пока недоступна' } : {}, resume_lesson_key: e.program.modules.flatMap(m => m.lessons).find(l => !e.progress.some(p => p.lesson_key === l.key && p.status === 'completed'))?.key || 'lesson-one' } }
function grant(version = 1) { const p = structuredClone(program); if (version === 2) p.modules[1].lessons = [lesson('lesson-test', 'Проверка знаний', true, 'test', [block('test', { test_version_id: 2 })]), lesson('lesson-assignment', 'Практическая работа', true, 'assignment', [block('assignment', { title: 'Задание по материалу', text: 'Описание задания', rich: [{ type: 'paragraph', text: 'Инструкция для самостоятельной работы' }], answer_format: 'file', material_ids: [11] })])]; const e = { id: nextID++, course_id: 42, user_id: 2, program_version_id: version, version_number: version, title: course.title, cover_image: course.cover_image, author_name: course.author_name, user_name: 'Ученик FinTalent', enrolled_at: now(), access_status: 'active', effective_status: 'active', program: p, progress: [] }; enrollments.push(e); return e }
const server = http.createServer(async (req, res) => {
  const u = new URL(req.url, 'http://localhost'), send = (v, status = 200) => { res.writeHead(status, { 'Content-Type': 'application/json' }); res.end(JSON.stringify(v)) }
  if (u.pathname.startsWith('/api/')) {
    if (u.pathname === '/api/me') return send({ id: actor, full_name: actor === 1 ? 'Автор курса' : 'Ученик FinTalent' })
    if (u.pathname === '/api/courses/access') return send({ allowed: actor === 1 })
    if (u.pathname === '/api/academy/learning/access') return send({ allowed: enrollments.length > 0 })
    if (u.pathname === '/api/profimarket/meta') return send({ categories: [{ type: 'TEMPLATE', count: 1 }] })
    if (u.pathname === '/api/profimarket') return send({ items: [{ id: 9, slug: 'fixture-solution', title: 'Рабочий шаблон отчёта', type: 'TEMPLATE', short_description: 'Проверка существующего каталога решений', pricing_type: 'FREE', author_name: 'Автор FinTalent' }] })
    if (u.pathname === '/api/academy/learning') { paged ||= u.searchParams.has('before'); return send(enrollments.filter(e => e.id < Number(u.searchParams.get('before') || Number.MAX_SAFE_INTEGER)).sort((a,b) => b.id-a.id).slice(0,200).map(dto)) }
    if (!u.pathname.startsWith('/api/academy/')) return send([])
    let raw = ''; for await (const chunk of req) raw += chunk
    const data = raw ? JSON.parse(raw) : {}
    if (u.pathname.startsWith('/api/academy/enrollments/')) {
      const parts = u.pathname.split('/'), e = enrollments.find(e => e.id === Number(parts[4]))
      if (!e) return send({ error: 'Зачисление не найдено' }, 404)
      if (parts.length === 5) { if (failRoom) { failRoom = false; return send({ error: 'Проверка ошибки обновления доступа' }, 503) }; return send(room(e)) }
      if (e.effective_status !== 'active') return send({ error: 'Доступ приостановлен' }, 403)
      if (parts[5] === 'assets') { res.setHeader('Content-Type', Number(parts[6]) === 10 ? 'image/png' : 'application/pdf'); res.end(Number(parts[6]) === 10 ? png : '%PDF-1.7\nfixture'); return }
      const key = parts[6]
      if (parts[7] === 'complete' && failComplete) { failComplete = false; return send({ error: 'Не удалось сохранить завершение. Повторите действие.' }, 503) }
      let p = e.progress.find(p => p.lesson_key === key)
      if (!p) { p = { lesson_key: key, status: 'in_progress' }; e.progress.push(p) }
      e.started_at ||= now(); e.last_activity_at = now(); e.last_lesson_key = key
      if (parts[7] === 'open') opens++
      if (parts[7] === 'complete') { p.status = 'completed'; p.completed_at = now(); if (summary(e).status === 'completed') e.completed_at = now() }
      return send(dto(e))
    }
    if (u.pathname.endsWith('/releases/1/preview')) return send({ ...room({ ...enrollments[0], id: 100, program, effective_status: 'active', progress: [] }), enrollment: undefined, course, preview: true })
    if (u.pathname.endsWith('/versions') && req.method === 'POST') {
      if (data.revision !== program.revision) return send({error: 'Конфликт ревизии'}, 409)
      const existing = releases.find(v => v.revision === data.revision)
      if (existing) return send({number: existing.number, revision: existing.revision})
      const created = {id: 3, number: 2, revision: data.revision, readiness: {lessons: 3, required: 2, blockers: [], warnings: []}}
      releases.unshift(created); return send({number: created.number, revision: created.revision})
    }
    if (u.pathname.endsWith('/releases')) { if (req.method === 'POST') releases[0].published_at = now(); return send(req.method === 'POST' ? releases[0] : releases) }
    if (u.pathname.endsWith('/user-search')) return send([{ id: 2, name: 'Ученик FinTalent' }])
    if (u.pathname.endsWith('/students')) { if (req.method === 'POST') return send({ id: grant(data.version_id || 1).id }); return send(enrollments.map(dto)) }
    if (/\/students\/\d+$/.test(u.pathname)) { const e = enrollments.find(e => e.id === Number(u.pathname.split('/').at(-1))); if (req.method === 'PUT') { e.access_status = e.effective_status = data.status; e.expires_at = data.expires_at; return send(dto(e)) }; return send({ enrollment: dto(e), lessons: [{ key: 'lesson-one', title: 'Начало обучения', status: e.progress[0]?.status }], history: [{ type: 'enrolled', created_at: e.enrolled_at }] }) }
    if (u.pathname.endsWith('/workspace')) return send({ course, program, tests: [], versions: releases })
    return send({ course, program, tests: [], versions: releases })
  }
  const file = u.pathname.startsWith('/static/') ? path.join(root, u.pathname) : path.join(root, 'static/react/index.html')
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
    const widths = async (label, target) => { for (const width of [1440, 900, 360]) { await cmd('Emulation.setDeviceMetricsOverride', { width, height: 1100, deviceScaleFactor: 1, mobile: false }); await evaluate(`document.querySelector(${JSON.stringify(target)})?.scrollIntoView({block:'start'})`); await sleep(150); await assert('document.documentElement.scrollWidth<=innerWidth', label + ' no overflow ' + width); if (await evaluate('!!document.querySelector("dialog")')) await assert('(()=>{const r=document.querySelector(".academy-dialog").getBoundingClientRect();return r.left>=0 && r.right<=document.documentElement.clientWidth+1})()', 'dialog within viewport '+width); const shot = await cmd('Page.captureScreenshot', { format: 'png' }); fs.writeFileSync(path.join(temp, `${label}-${width}.png`), Buffer.from(shot.result.data, 'base64')) } }
    const navigate = async url => { const origin = await evaluate('performance.timeOrigin'); await cmd('Page.navigate', { url: base + url }); await wait('performance.timeOrigin!==' + origin) }
    const desktop = () => cmd('Emulation.setDeviceMetricsOverride', { width: 1440, height: 1100, deviceScaleFactor: 1, mobile: false })
    const close = () => evaluate('document.querySelector(".academy-dialog button[aria-label=Закрыть]").click()')
    const select = (selector, value) => evaluate(`(()=>{const el=document.querySelector(${JSON.stringify(selector)});Object.getOwnPropertyDescriptor(HTMLSelectElement.prototype,'value').set.call(el,${JSON.stringify(value)});el.dispatchEvent(new Event('change',{bubbles:true}))})()`)
    await cmd('Page.enable'); await cmd('Runtime.enable'); await navigate('/profile/learning')
    await wait('!!document.querySelector(".learning-empty")'); await assert('document.body.textContent.includes("Здесь пока нет курсов")', 'honest empty learning catalog'); await widths('empty', '.learning-page-heading')
    actor = 1; await desktop(); await navigate('/profile/courses/42/academy/students'); await wait('!!document.querySelector(".teaching-panel")')
    await assert('document.querySelector(".teaching-version-current").textContent.includes("после фиксации версии 1")', 'outdated snapshot is distinguished from current published lessons')
    await assert('[...document.querySelectorAll("button")].find(b=>b.textContent.trim()==="Опубликовать версию").disabled', 'unready immutable snapshot remains blocked')
    const oldSnapshot = JSON.stringify(releases[0])
    await widths('outdated-version', '.teaching-access-grid'); await desktop()
    await click('Зафиксировать текущую программу'); await wait('document.querySelector(".teaching-panel select").value === "3"')
    await assert('!document.querySelector(".teaching-version-current") && !document.querySelector(".teaching-blocker")', 'new current snapshot is selected and validated')
    if (JSON.stringify(releases.find(v => v.id === 1)) !== oldSnapshot) throw Error('Old immutable version changed')
    await reload(); await wait('document.querySelector(".teaching-panel select")?.value === "3"'); console.log('PASS current snapshot replaces stale selection without changing history')
    await assert('document.querySelector("fieldset").disabled', 'grant disabled before publication'); await click('Опубликовать версию'); await wait('!!document.querySelector(".teaching-published")')
    await input('input[placeholder^="Точный email"]', 'student@example.invalid'); await wait('!!document.querySelector(".teaching-user-results button")'); await evaluate('document.querySelector(".teaching-user-results button").click()'); await click('Предоставить доступ'); await wait('document.querySelectorAll(".teaching-students tbody tr").length===1'); console.log('PASS publish and manual enrollment')
    await widths('teacher', '.teaching-access-grid'); await desktop(); await click('Ученик FinTalent'); await wait('!!document.querySelector("dialog")'); await assert('document.querySelector("dialog").textContent.includes("Доступ предоставлен")', 'student card access history'); await widths('student-card', 'dialog'); await close()
    const beforePreview = opens; await navigate('/profile/courses/42/academy/room/1'); await wait('!!document.querySelector(".academy-lesson-renderer")'); await assert('[...document.querySelectorAll("button")].find(b=>b.textContent.trim()==="Завершить урок").disabled', 'author preview cannot complete'); if (opens !== beforePreview || enrollments.length !== 1) throw Error('Preview mutated enrollment'); console.log('PASS author preview creates no progress or enrollment')
    actor = 2; await navigate('/profile/learning'); await wait('!!document.querySelector(".learning-course-card")'); await assert('document.querySelector(".learning-added-notice").textContent.includes("Курс добавлен")', 'new enrollment confirmation'); await assert('[...document.querySelectorAll("a")].some(a=>a.getAttribute("href")==="/profile/learning" && a.textContent.includes("Моё обучение"))', 'learning navigation available to enrolled student'); await widths('catalog', '.learning-page-heading')
    await click('В процессе'); await wait('!!document.querySelector(".learning-filter-empty")'); await click('Не начатые'); await wait('!!document.querySelector(".learning-course-card")'); await select('.learning-catalog-tools select', 'title'); await evaluate('document.querySelector(".learning-course-card a.course-primary").click()'); await wait('!!document.querySelector(".academy-lesson-renderer")'); await assert('document.querySelector(".academy-lesson-renderer h1").textContent==="Начало обучения"', 'opens first enrolled lesson')
    await assert('document.querySelector(".academy-asset").href.includes("/enrollments/100/assets/11?lesson=lesson-one") && document.querySelector(".type-image img").src.includes("lesson=lesson-one")', 'shared renderer uses enrollment-bound files'); await widths('room', '.learning-room-heading')
    await click('Программа курса'); await wait('!!document.querySelector("dialog .learning-room-tree")'); await assert('document.querySelectorAll("dialog .learning-room-tree li").length===3', 'mobile curriculum drawer'); await widths('curriculum-drawer', 'dialog'); await close(); await desktop()
    failComplete = true; await click('Завершить урок'); await wait('!!document.querySelector(".course-error")'); await assert('document.querySelector(".learning-room-heading").textContent.includes("0%")', 'failed save does not claim progress'); await click('Завершить урок'); await wait('document.querySelector(".learning-room-heading").textContent.includes("50%")'); console.log('PASS confirmed completion updates server progress')
    await click('Следующий урок →'); await wait('document.querySelector(".academy-lesson-renderer h1")?.textContent==="Дополнительная практика"'); await reload(); await wait('document.querySelector(".academy-lesson-renderer h1")?.textContent==="Дополнительная практика"'); console.log('PASS reload retains last lesson')
    await click('Завершить урок'); await wait('!!document.querySelector(".academy-notice")'); await assert('document.querySelector(".learning-room-heading").textContent.includes("50%")', 'optional lesson does not distort progress'); await click('Следующий урок →'); await wait('!!document.querySelector(".learning-materials-ack")'); await assert('[...document.querySelectorAll("button")].find(b=>b.textContent.trim()==="Завершить урок").disabled', 'material completion requires acknowledgment'); await evaluate('document.querySelector(".learning-materials-ack input").click()'); await click('Завершить урок'); await wait('!!document.querySelector(".learning-success")'); await assert('document.querySelector(".learning-success").textContent.includes("Поздравляем! Вы завершили курс")', 'course completion dialog'); await widths('completion', 'dialog'); await close()
    await navigate('/profile/learning'); await wait('!!document.querySelector(".learning-course-card")'); await click('Завершённые'); await assert('document.querySelector(".learning-course-card").textContent.includes("100%")', 'persisted completed course in catalog')
    const e = grant(2); await navigate('/profile/learning/101?lesson=lesson-test'); await wait('document.querySelector(".type-test")?.textContent.includes("не подготовлена автором")'); await assert('document.querySelector(".type-test").textContent.includes("не подготовлена автором") && [...document.querySelectorAll("button")].find(b=>b.textContent.trim()==="Завершить урок").disabled', 'unsupported required test cannot be falsely completed'); await click('Следующий урок →'); await wait('!!document.querySelector(".type-assignment")'); await widths('assignment', '.academy-assignment'); await assert('document.querySelector(".academy-assignment").textContent.includes("Инструкция")', 'assignment instructions and material render')
    await navigate('/profile/learning/101?lesson=lesson-one'); await wait('!!document.querySelector(".academy-lesson-renderer")'); e.effective_status = 'suspended'; failRoom = true; await click('Завершить урок'); await wait('!!document.querySelector(".learning-access-notice")'); await assert('!document.querySelector(".academy-lesson-renderer")', '403 clears materials even when access refresh fails')
    for (const status of ['suspended', 'revoked', 'expired']) { e.effective_status = status; await navigate('/profile/learning/101'); await wait('!!document.querySelector(".learning-access-notice")'); await assert('!document.querySelector(".academy-lesson-renderer") && !document.querySelector(".academy-asset")', status + ' access does not display materials') }
    await widths('access-ended', '.learning-access-notice')
    actor = 1; await desktop(); await navigate('/profile/courses/42/academy/students'); await wait('!!document.querySelector(".teaching-students tbody")'); await evaluate('document.querySelector(".teaching-students .course-secondary").click()'); await wait('!!document.querySelector("dialog")'); await select('dialog select', 'suspended'); await click('Сохранить доступ'); await wait('!document.querySelector("dialog")'); await assert('document.querySelector(".teaching-students tbody tr").textContent.includes("приостановлен")', 'author access management'); await widths('teacher-progress', '.teaching-students')
    for (let n = 0; n < 200; n++) grant(); actor = 2; await navigate('/profile/learning'); await wait('document.querySelectorAll(".learning-course-card").length===202'); if (!paged) throw Error('Missing cursor request'); console.log('PASS catalog loads beyond first 200 enrollments')
    await desktop(); await evaluate('[...document.querySelectorAll(".ft-main-nav a")].find(a=>a.pathname==="/profimarket").click()'); await wait('!!document.querySelector(".pmh-card")'); await assert('document.querySelector(".pmh-card").textContent.includes("Рабочий шаблон отчёта")', 'existing ProfiMarket after SPA navigation'); await widths('profimarket', '.pmh-hero'); await input('#pmh-search input', 'отчёт'); await click('Найти'); await wait('!!document.querySelector(".pmh-card")'); console.log('PASS existing ProfiMarket search')
    if (errors.length) throw Error('Frontend console: ' + errors.join('\n'))
    console.log('PASS no frontend console errors\nScreenshots: ' + temp)
  } finally { ws?.close(); browser.kill(); server.close() }
})().catch(e => { console.error(e); server.close(); process.exitCode = 1 })
