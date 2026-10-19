// Render locally so employee results never leave the authenticated browser.
export function downloadEmployeeResultPDF(employee, result) {
  const width = 1240, height = 1754, margin = 90, lineHeight = 38
  const pages = []
  let canvas, ctx, y
  function page() {
    canvas = document.createElement('canvas')
    canvas.width = width; canvas.height = height
    ctx = canvas.getContext('2d')
    ctx.fillStyle = '#fff'; ctx.fillRect(0, 0, width, height)
    ctx.fillStyle = '#162340'; ctx.font = '26px Arial'
    y = margin
  }
  function finish() {
    ctx.font = '20px Arial'
    ctx.fillText(`FinTalent · ${pages.length + 1}`, margin, height - 40)
    pages.push(Uint8Array.from(atob(canvas.toDataURL('image/jpeg', 0.92).split(',')[1]), c => c.charCodeAt(0)))
  }
  function line(text) {
    if (y > height - margin) { finish(); page() }
    ctx.fillText(text, margin, y); y += lineHeight
  }
  function text(value) {
    for (const paragraph of String(value ?? '').split(/\r?\n/)) {
      let current = ''
      for (const char of paragraph) {
        if (ctx.measureText(current + char).width > width - margin * 2) { line(current); current = '' }
        current += char
      }
      line(current)
    }
  }
  page()
  text('FinTalent — Результаты тестирования сотрудника')
  text(employee.full_name); text(employee.email)
  text(result.test_title || employee.test_title)
  text(`Результат: ${Math.round(result.percent)}% · ${result.score} из ${result.max_score} баллов`)
  text(`Время прохождения: ${Math.floor((result.duration_seconds || 0) / 60)} мин. ${(result.duration_seconds || 0) % 60} сек.`)
  const date = employee.finished_at || result.finished_at
  if (date) text(`Дата: ${new Date(date).toLocaleString('ru-RU')}`)
  const answers = new Map()
  for (const answer of result.answers || []) {
    const group = answers.get(answer.question_id) || { ...answer, ids: [] }
    if (answer.selected_answer_id) group.ids.push(answer.selected_answer_id)
    answers.set(answer.question_id, group)
  }
  for (const [index, question] of (result.questions || []).entries()) {
    const answer = answers.get(question.id)
    text(''); text(`Вопрос ${index + 1}. ${question.question}`)
    text(answer?.is_correct ? 'Верно' : 'Есть ошибка / ответ не дан')
    if (question.question_type === 'text') text(`Ответ сотрудника: ${answer?.text_answer || 'Ответ не дан'}`)
    else for (const option of question.answers || []) {
      text(`${answer?.ids.includes(option.id) ? '[Выбран] ' : ''}${option.is_correct ? '[Верный] ' : ''}${option.answer}`)
    }
    if (question.explanation) text(`Пояснение: ${question.explanation}`)
  }
  finish()
  const blob = createImagePDF(pages, width, height)
  const url = URL.createObjectURL(blob), link = document.createElement('a')
  link.href = url
  link.download = `FinTalent-${String(employee.full_name || 'result').replace(/[<>:"/\\|?*\x00-\x1f]/g, '_')}.pdf`
  link.click()
  setTimeout(() => URL.revokeObjectURL(url), 60000)
}

export function createImagePDF(pages, width, height) {
  const encoder = new TextEncoder(), chunks = [], offsets = [0]
  let length = 0
  const append = value => { const bytes = typeof value === 'string' ? encoder.encode(value) : value; chunks.push(bytes); length += bytes.length }
  const object = (id, body) => { offsets[id] = length; append(`${id} 0 obj\n`); append(body); append('\nendobj\n') }
  append('%PDF-1.4\n')
  object(1, '<< /Type /Catalog /Pages 2 0 R >>')
  object(2, `<< /Type /Pages /Count ${pages.length} /Kids [${pages.map((_, i) => `${3 + i * 3} 0 R`).join(' ')}] >>`)
  pages.forEach((jpeg, i) => {
    const id = 3 + i * 3, stream = 'q 595.28 0 0 841.89 0 0 cm /Image Do Q'
    object(id, `<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595.28 841.89] /Resources << /XObject << /Image ${id + 1} 0 R >> >> /Contents ${id + 2} 0 R >>`)
    offsets[id + 1] = length
    append(`${id + 1} 0 obj\n<< /Type /XObject /Subtype /Image /Width ${width} /Height ${height} /ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /DCTDecode /Length ${jpeg.length} >>\nstream\n`)
    append(jpeg); append('\nendstream\nendobj\n')
    object(id + 2, `<< /Length ${encoder.encode(stream).length} >>\nstream\n${stream}\nendstream`)
  })
  const start = length, count = 3 + pages.length * 3
  append(`xref\n0 ${count}\n0000000000 65535 f \n`)
  for (let id = 1; id < count; id++) append(`${String(offsets[id]).padStart(10, '0')} 00000 n \n`)
  append(`trailer\n<< /Size ${count} /Root 1 0 R >>\nstartxref\n${start}\n%%EOF\n`)
  return new Blob(chunks, { type: 'application/pdf' })
}
