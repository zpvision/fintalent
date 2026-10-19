// Run after TestEmployeePDFChromium with PDF_PREVIEW_DIR set. Uses synthetic data only.
const {chromium} = require('playwright-core')
const fs = require('node:fs')
const path = require('node:path')
const assert = require('node:assert/strict')

;(async () => {
  const preview = process.env.PDF_PREVIEW_DIR
  assert(preview, 'Set PDF_PREVIEW_DIR to the Go-generated report fixture directory')
  const browser = await chromium.launch({executablePath: process.env.CHROME_PATH || undefined, headless: true})
  try {
    const page = await browser.newPage({viewport: {width: 1240, height: 1754}, acceptDownloads: true})
    const errors = []
    page.on('pageerror', error => errors.push(error.message))
    let apiStatus = 200
    await page.route('**/*', route => {
      const file = new URL(route.request().url()).pathname
      if (file === '/') return route.fulfill({contentType: 'text/html', body: '<html><body style="margin:0"></body></html>'})
      if (file === '/report.html') return route.fulfill({path: path.join(preview, 'employee-result.html'), contentType: 'text/html'})
      if (file === '/pdfjs.mjs') return route.fulfill({path: require.resolve('pdfjs-dist/build/pdf.mjs'), contentType: 'text/javascript'})
      if (file === '/pdf.worker.mjs') return route.fulfill({path: require.resolve('pdfjs-dist/build/pdf.worker.mjs'), contentType: 'text/javascript'})
      if (file === '/static/employee-result-pdf.js') return route.fulfill({path: path.join(process.cwd(), file.slice(1)), contentType: 'text/javascript'})
      if (file === '/report.pdf') return route.fulfill({path: path.join(preview, 'FinTalent-result-example.pdf'), contentType: 'application/pdf'})
      if (file === '/long.pdf') return route.fulfill({path: path.join(preview, 'FinTalent-result-long.pdf'), contentType: 'application/pdf'})
      if (file === '/api/employee-testing/invitations/42/results.pdf') {
        if (apiStatus !== 200) return route.fulfill({status: apiStatus, contentType: 'application/json', body: JSON.stringify({error: 'Результат недоступен'})})
        return route.fulfill({path: path.join(preview, 'FinTalent-result-example.pdf'), contentType: 'application/pdf'})
      }
      return route.abort()
    })
    await page.goto('http://pdf.invalid/')
    const report = await page.evaluate(async () => {
      const pdfjs = await import('/pdfjs.mjs')
      pdfjs.GlobalWorkerOptions.workerSrc = '/pdf.worker.mjs'
      const bytes = new Uint8Array(await (await fetch('/report.pdf')).arrayBuffer())
      const pdf = await pdfjs.getDocument({data: bytes, useSystemFonts: true}).promise
      const texts = []
      for (let i = 1; i <= pdf.numPages; i++) {
        const pdfPage = await pdf.getPage(i)
        texts.push((await pdfPage.getTextContent()).items.map(x => x.str).join(' '))
        const viewport = pdfPage.getViewport({scale: 2})
        const canvas = document.createElement('canvas')
        canvas.width = viewport.width; canvas.height = viewport.height; canvas.style.display = 'block'
        document.body.append(canvas)
        await pdfPage.render({canvasContext: canvas.getContext('2d'), viewport}).promise
      }
      const longBytes = new Uint8Array(await (await fetch('/long.pdf')).arrayBuffer())
      const longPDF = await pdfjs.getDocument({data: longBytes, useSystemFonts: true}).promise
      const lastPage = await longPDF.getPage(longPDF.numPages)
      const lastText = (await lastPage.getTextContent()).items.map(x => x.str).join(' ')
      return {pages: pdf.numPages, text: texts.join(' '), longPages: longPDF.numPages, lastText}
    })
    for (const expected of ['Анна Смирнова', 'Проверьте свои ответы', 'Расходы на рекламу', 'Амортизация начисляется', 'fintalent.ru']) assert(report.text.includes(expected), 'PDF text missing: '+expected)
    assert(report.pages >= 1)
    assert(report.longPages > report.pages)
    assert(report.lastText.includes('КОНЕЦ ДЛИННОГО ПОЯСНЕНИЯ'), 'Long content must not be cut off')
    assert(report.lastText.includes('fintalent.ru'), 'Brand header must repeat on continuation pages')
    await page.locator('canvas').first().screenshot({path: path.join(preview, 'employee-result-page-1.png')})
    if (report.pages > 1) await page.locator('canvas').nth(1).screenshot({path: path.join(preview, 'employee-result-page-2.png')})
    const downloadPromise = page.waitForEvent('download')
    await page.evaluate(async () => {const {downloadEmployeeResultPDF} = await import('/static/employee-result-pdf.js'); await downloadEmployeeResultPDF({id:42, full_name:'Анна Смирнова'})})
    const download = await downloadPromise
    assert.match(download.suggestedFilename(), /FinTalent-.*\.pdf$/)
    assert.deepEqual(fs.readFileSync(await download.path()), fs.readFileSync(path.join(preview, 'FinTalent-result-example.pdf')))
    apiStatus = 404
    const error = await page.evaluate(async () => {
      try { const {downloadEmployeeResultPDF} = await import('/static/employee-result-pdf.js'); await downloadEmployeeResultPDF({id:42}) }
      catch (error) { return error.message }
    })
    assert.equal(error, 'Результат недоступен')
    assert.deepEqual(errors, [])
    console.log(`PASS: HTML-generated PDF, ${report.pages} pages, searchable Cyrillic text, download and error handling`)
  } finally {await browser.close()}
})().catch(error => {console.error(error); process.exitCode = 1})
