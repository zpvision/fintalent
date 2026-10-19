// Synthetic employee data only; no running application, database or external services.
const {chromium} = require('playwright-core')
const fs = require('node:fs')
const path = require('node:path')
const assert = require('node:assert/strict')

;(async () => {
  const browser = await chromium.launch({executablePath: process.env.CHROME_PATH || 'C:/Program Files/Google/Chrome/Application/chrome.exe', headless: true})
  try {
    const page = await browser.newPage({viewport: {width: 1240, height: 1754}, acceptDownloads: true})
    const errors = []
    page.on('pageerror', error => errors.push(error.message))
    await page.route('**/*', route => {
      const file = new URL(route.request().url()).pathname
      if (file === '/') return route.fulfill({contentType: 'text/html', body: '<html><body style="margin:0"></body></html>'})
      if (file === '/static/employee-result-pdf.js' || file === '/static/logo.png') return route.fulfill({path: path.join(process.cwd(), file.slice(1)), contentType: file.endsWith('.js') ? 'text/javascript' : 'image/png'})
      return route.abort()
    })
    await page.goto('http://pdf.invalid/')
    const info = await page.evaluate(async () => {
      const module = await import('/static/employee-result-pdf.js')
      const employee = {full_name: 'Анна Смирнова', email: 'anna@example.test', finished_at: '2026-10-19T10:30:00Z'}
      const result = {test_title: 'Бухгалтерский учёт: основные средства и налоги', percent: 67, score: 2, max_score: 3, duration_seconds: 725, questions: [
        {id: 1, question: 'Какой документ подтверждает принятие основного средства к учёту?', answers: [{id: 11, answer: 'Акт о приёме-передаче основного средства', is_correct: true}, {id: 12, answer: 'Платёжное поручение', is_correct: false}], explanation: 'Принятие объекта к учёту оформляется первичным учётным документом.'},
        {id: 2, question: 'Что относится к первоначальной стоимости основного средства?', answers: [{id: 21, answer: 'Затраты на доставку и монтаж', is_correct: true}, {id: 22, answer: 'Расходы на рекламу', is_correct: false}]},
        {id: 3, question_type: 'text', question: 'Объясните порядок начисления амортизации.', answers: [], explanation: 'Учитывайте срок полезного использования и выбранный способ амортизации.'}
      ], answers: [{question_id: 1, selected_answer_id: 11, is_correct: true}, {question_id: 2, selected_answer_id: 22, is_correct: false}, {question_id: 3, text_answer: 'Амортизация начисляется в течение срока полезного использования.', is_correct: true}]}
      const canvases = await module.renderEmployeeResultPages(employee, result)
      for (const canvas of canvases) {canvas.style.display = 'block'; document.body.append(canvas)}
      const colors = new Set()
      const pixels = canvases[0].getContext('2d').getImageData(0, 0, 1240, 1754).data
      for (let i = 0; i < pixels.length; i += 4) colors.add(`${pixels[i]},${pixels[i+1]},${pixels[i+2]}`)
      window.pdfFixture = {module, employee, result}
      const longResult = {...result, questions: Array.from({length: 12}, (_, i) => ({...result.questions[i % 3], id: i + 100, explanation: 'Длинное пояснение. '.repeat(180)}))}
      const longPages = await module.renderEmployeeResultPages(employee, longResult)
      return {pages: canvases.length, longPages: longPages.length, blue: colors.has('23,98,236'), green: colors.has('231,248,241')}
    })
    assert(info.pages >= 2, 'Questions must paginate')
    assert(info.longPages > info.pages, 'Long explanations must paginate')
    assert(info.blue && info.green, 'Brand and result colors must appear')
    const downloadPromise = page.waitForEvent('download')
    await page.evaluate(async () => {const {module, employee, result} = window.pdfFixture; await module.downloadEmployeeResultPDF(employee, result)})
    const download = await downloadPromise
    assert.match(download.suggestedFilename(), /FinTalent-.*\.pdf$/)
    const pdf = fs.readFileSync(await download.path()).toString('latin1')
    assert(pdf.startsWith('%PDF-1.4'))
    assert.equal(Number(pdf.match(/\/Count (\d+)/)[1]), info.pages)
    const start = Number(pdf.match(/startxref\n(\d+)/)[1]), entries = pdf.slice(start).split('\n')
    assert.equal(entries[0], 'xref')
    for (let id = 1; id < 3 + info.pages * 3; id++) assert(pdf.slice(Number(entries[id + 2].slice(0, 10))).startsWith(`${id} 0 obj`))
    if (process.env.PDF_PREVIEW_DIR) {
      fs.mkdirSync(process.env.PDF_PREVIEW_DIR, {recursive: true})
      await download.saveAs(path.join(process.env.PDF_PREVIEW_DIR, 'FinTalent-result-example.pdf'))
      await page.locator('canvas').first().screenshot({path: path.join(process.env.PDF_PREVIEW_DIR, 'employee-result-page-1.png')})
      await page.locator('canvas').nth(1).screenshot({path: path.join(process.env.PDF_PREVIEW_DIR, 'employee-result-page-2.png')})
    }
    assert.deepEqual(errors, [])
    console.log(`PASS: colored report, ${info.pages} pages, ${info.longPages} long-content pages, PDF download and offsets`)
  } finally {await browser.close()}
})().catch(error => {console.error(error); process.exitCode = 1})
