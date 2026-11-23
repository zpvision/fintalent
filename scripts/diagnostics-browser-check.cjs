// Isolated browser check: synthetic diagnostics only, no app/database sessions.
const fs = require('fs'), path = require('path'), http = require('http'), os = require('os'), { spawn } = require('child_process')
const root = path.resolve(__dirname, '..'), output = path.join(root, '.codex-run', 'diagnostics-preview')
fs.mkdirSync(output, { recursive: true })
const sleep = ms => new Promise(resolve => setTimeout(resolve, ms)), received = []
let rejectEvents = false
const report = { events: 3, max_bytes: 50 * 1048576, stored_bytes: 12345, retention_days: 14, dropped_since_start: 0, write_errors_since_start: 0, groups: [{ count: 3, first: new Date().toISOString(), last: new Date().toISOString(), sample: { source: 'worker', code: 'permission_changed', operation: 'academy.upload.finalize', user_id: 42, course_id: 10, entity_id: 'synthetic-upload', fingerprint: 'test-fingerprint', release: 'test-release' } }] }
const html = `<!doctype html><html lang="ru"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><link rel="stylesheet" href="/static/admin.css"><link rel="stylesheet" href="/static/admin-users.css"><meta name="fintalent-release" content="test-release"><script src="/static/error-telemetry.js"></script></head><body><div class="admin-app"><aside class="sidebar"><a class="sidebar-brand" href="/"><span>F</span><div><b>FinTalent</b><small>Проверка журнала</small></div></a><nav><button id="dictionary-nav">Справочники</button></nav></aside><main class="workspace"><header><button class="mobile-menu">☰</button><div><h1>Журнал</h1><p>Проверка</p></div></header><section id="dictionary-list"></section></main></div><script>async function api(url){const r=await fetch(url);if(!r.ok)throw Error('Ошибка');return r.json()};document.querySelector('.mobile-menu').onclick=()=>document.querySelector('.sidebar').classList.toggle('open')</script><script src="/static/admin-errors.js"></script></body></html>`
const server = http.createServer((req, res) => {
  if (req.url.startsWith('/api/admin/diagnostics')) { res.setHeader('Content-Type', 'application/json'); return res.end(JSON.stringify(report)) }
  if (req.url === '/api/diagnostics/events') { let data = ''; req.on('data', b => { data += b }); req.on('end', () => { received.push(JSON.parse(data)); res.writeHead(rejectEvents ? 503 : 204).end() }); return }
  if (req.url === '/') { res.setHeader('Content-Type', 'text/html; charset=utf-8'); return res.end(html) }
  if (req.url.startsWith('/static/')) { const file = path.join(root, new URL(req.url, 'http://localhost').pathname); if (file.startsWith(path.join(root, 'static') + path.sep) && fs.existsSync(file)) { res.setHeader('Content-Type', file.endsWith('.js') ? 'text/javascript' : 'text/css'); return fs.createReadStream(file).pipe(res) } }
  res.writeHead(404).end()
})
;(async () => {
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve))
  const base = `http://127.0.0.1:${server.address().port}`, profile = fs.mkdtempSync(path.join(os.tmpdir(), 'ft-diagnostic-ui-'))
  const browser = spawn(process.env.EDGE_PATH || 'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe', ['--headless=new', '--disable-gpu', '--no-first-run', '--remote-debugging-port=0', '--user-data-dir=' + profile, 'about:blank'], { windowsHide: true, stdio: 'ignore' })
  let ws
  try {
    let port
    for (let n = 0; n < 100; n++) { try { port = fs.readFileSync(path.join(profile, 'DevToolsActivePort'), 'utf8').split('\n')[0]; break } catch { await sleep(100) } }
    if (!port) throw Error('Headless Edge unavailable')
    const tabs = await (await fetch(`http://127.0.0.1:${port}/json/list`)).json()
    ws = new WebSocket(tabs.find(t => t.type === 'page').webSocketDebuggerUrl); await new Promise(resolve => ws.addEventListener('open', resolve, { once: true }))
    let seq = 0; const pending = new Map()
    ws.addEventListener('message', event => { const m = JSON.parse(event.data); if (m.id) { pending.get(m.id)?.(m); pending.delete(m.id) } })
    const cmd = (method, params = {}) => new Promise((resolve, reject) => { const id = ++seq; pending.set(id, m => m.error ? reject(Error(JSON.stringify(m.error))) : resolve(m.result)); ws.send(JSON.stringify({ id, method, params })) })
    const evaluate = async expression => { const r = await cmd('Runtime.evaluate', { expression, returnByValue: true, awaitPromise: true }); if (r.exceptionDetails) throw Error(r.exceptionDetails.exception.description); return r.result.value }
    await cmd('Page.enable'); await cmd('Page.navigate', { url: base })
    for (let n = 0; n < 100; n++) { if (await evaluate(`!!document.querySelector('#errors-nav')`)) break; await sleep(100) }
    await evaluate(`document.querySelector('#errors-nav').click()`)
    for (let n = 0; n < 100; n++) { if (await evaluate(`!!document.querySelector('#error-journal-section tbody tr')`)) break; await sleep(100) }
    for (const width of [360, 900, 1440]) {
      await cmd('Emulation.setDeviceMetricsOverride', { width, height: 900, deviceScaleFactor: 1, mobile: width < 500 })
      await sleep(100)
      const check = await evaluate(`({overflow:document.documentElement.scrollWidth>innerWidth,rows:document.querySelectorAll('#error-journal-section tbody tr').length,download:document.querySelector('#error-journal-section a').getAttribute('href')})`)
      if (check.overflow || check.rows !== 1 || !check.download.includes('download=1')) throw Error('Invalid admin layout: ' + JSON.stringify(check))
      const shot = await cmd('Page.captureScreenshot', { format: 'png' }); fs.writeFileSync(path.join(output, `errors-${width}.png`), Buffer.from(shot.data, 'base64'))
    }
    await evaluate(`window.reportFinTalentError({code:'javascript_error',stack:'Error: PRIVATE_PASSWORD\\n/static/react/assets/index.js:12:3'});window.reportFinTalentError({code:'javascript_error',stack:'Error: PRIVATE_PASSWORD\\n/static/react/assets/index.js:12:3'})`)
    for (let n = 0; n < 100 && received.length === 0; n++) await sleep(50)
    if (received.length !== 1 || JSON.stringify(received).includes('PRIVATE_PASSWORD') || received[0].release !== 'test-release') throw Error('Browser reporting privacy or deduplication failed')
    rejectEvents = true
    await evaluate(`window.reportFinTalentError({code:'network_error',route:'/api/courses'})`)
    for (let n = 0; n < 100 && received.length < 2; n++) await sleep(50)
    await sleep(150)
    if (received.length !== 2) throw Error('Telemetry failure caused retries or recursive reports')
    rejectEvents = false
    await evaluate(`history.replaceState(null, '', '/profile?token=PRIVATE_TOKEN#PRIVATE_FRAGMENT');
      for (const name of ['missing-first.js', 'missing-second.js']) {
        const script = document.createElement('script'); script.src = '/static/' + name + '?token=PRIVATE_TOKEN'; document.body.append(script)
      }
      const link = document.createElement('link'); link.rel = 'stylesheet'; Object.defineProperty(link, 'href', {value:'https://fonts.googleapis.com/css2?family=PRIVATE_FONT'}); document.body.append(link);
      link.dispatchEvent(new Event('error', { bubbles: true }));
      window.reportFinTalentError({code:'resource_error',resource_url:'/static/uploads/PRIVATE_UPLOAD.js?token=PRIVATE_TOKEN',resource_type:'script'});
      window.reportFinTalentError({code:'resource_error',resource_url:'https://PRIVATE_TENANT.example.com/PRIVATE_PATH.js',resource_type:'script'})`)
    for (let n = 0; n < 100 && received.length < 7; n++) await sleep(50)
    const resources = received.filter(event => event.code === 'resource_error')
    if (!resources.some(event => event.resource_url === '/static/missing-first.js' && event.resource_type === 'script') ||
        !resources.some(event => event.resource_url === '/static/missing-second.js') ||
        !resources.some(event => event.resource_url === 'https://fonts.googleapis.com/css2' && event.resource_type === 'stylesheet') ||
        !resources.some(event => event.resource_url === '/:redacted') ||
        !resources.some(event => event.resource_url === '/external/:redacted') ||
        resources.some(event => event.page_route !== '/profile' || event.route !== '/profile') ||
        JSON.stringify(received).includes('PRIVATE_')) throw Error('Resource context, privacy or grouping failed: ' + JSON.stringify(resources))
    await evaluate(`document.querySelector('#error-journal-section select').value='168';document.querySelector('#error-journal-section select').dispatchEvent(new Event('change'))`)
    await sleep(100)
    if (!await evaluate(`document.querySelector('#error-journal-section a').href.includes('hours=168')`)) throw Error('Period selection not reflected in export')
    console.log('PASS: admin journal at 360/900/1440px; export period; browser privacy; duplicate suppression; failed telemetry does not recurse; distinct failed resources, page context and safe external URLs')
  } finally { ws?.close(); browser.kill(); server.close() }
})().catch(error => { console.error(error.message); process.exitCode = 1; server.close() })
