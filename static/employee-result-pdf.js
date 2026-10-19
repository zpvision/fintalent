// Render locally so employee results never leave the authenticated browser.
export async function downloadEmployeeResultPDF(employee, result) {
  const pages = await renderEmployeeResultPages(employee, result)
  const blob = createImagePDF(pages.map(canvas => Uint8Array.from(atob(canvas.toDataURL('image/jpeg', 0.96).split(',')[1]), c => c.charCodeAt(0))), 1240, 1754)
  const url = URL.createObjectURL(blob), link = document.createElement('a')
  link.href = url
  link.download = `FinTalent-${String(employee.full_name || 'result').replace(/[<>:"/\\|?*\x00-\x1f]/g, '_')}.pdf`
  link.click()
  setTimeout(() => URL.revokeObjectURL(url), 60000)
}

// Match the palette, cards and answer markers in test-answer-review.css.
export async function renderEmployeeResultPages(employee, result) {
  const width = 1240, height = 1754, margin = 70, bodyWidth = width - margin * 2, bottom = height - 95
  const pages = [], navy = '#142052', blue = '#1762ec', muted = '#7583a4'
  const logo = await new Promise(resolve => {
    const image = new Image()
    image.onload = () => resolve(image)
    image.onerror = () => resolve(null)
    image.src = '/static/logo.png'
  })
  let canvas, ctx, y
  const font = (context, size = 20, bold = false) => { context.font = `${bold ? '700' : '400'} ${size}px Arial`; context.textBaseline = 'top' }
  function box(context, x, top, w, h, fill, stroke, radius = 14) {
    context.beginPath(); context.roundRect(x, top, w, h, radius)
    context.fillStyle = fill; context.fill()
    if (stroke) { context.strokeStyle = stroke; context.lineWidth = 1.5; context.stroke() }
  }
  function lines(context, value, maxWidth, size = 20, bold = false) {
    font(context, size, bold)
    const output = []
    for (const paragraph of String(value ?? '').split(/\r?\n/)) {
      let current = ''
      for (const word of paragraph.split(/(\s+)/)) {
        if (context.measureText(current + word).width <= maxWidth) { current += word; continue }
        if (current.trim()) output.push(current.trimEnd())
        current = ''
        for (const char of word.trimStart()) {
          if (context.measureText(current + char).width > maxWidth && current) { output.push(current); current = '' }
          current += char
        }
      }
      output.push(current.trimEnd())
    }
    return output
  }
  function write(context, value, x, top, maxWidth, size = 20, color = navy, bold = false) {
    const wrapped = lines(context, value, maxWidth, size, bold)
    context.fillStyle = color
    wrapped.forEach((line, index) => context.fillText(line, x, top + index * size * 1.45))
    return wrapped.length * size * 1.45
  }
  function page() {
    canvas = document.createElement('canvas'); canvas.width = width; canvas.height = height
    ctx = canvas.getContext('2d'); ctx.fillStyle = '#f5f8fd'; ctx.fillRect(0, 0, width, height)
    ctx.fillStyle = blue; ctx.fillRect(0, 0, width, 7)
    if (logo) ctx.drawImage(logo, 210, 140, 850, 760, margin, 35, 65, 58)
    write(ctx, 'FinTalent', margin + 78, 42, 500, 28, navy, true)
    write(ctx, 'Профессиональное тестирование', margin + 78, 78, 650, 16, muted)
    write(ctx, 'fintalent.ru', width - margin - 160, 55, 160, 21, blue, true)
    y = 135
  }
  function finish() {
    ctx.fillStyle = '#dfe7f3'; ctx.fillRect(margin, height - 73, bodyWidth, 1)
    write(ctx, 'FinTalent · Результаты тестирования сотрудников', margin, height - 53, 900, 16, muted)
    write(ctx, String(pages.length + 1), width - margin - 45, height - 53, 45, 16, muted)
    pages.push(canvas)
  }
  function room(h) { if (y + h > bottom && y > 135) { finish(); page() } }
  function panel(label, value, accent = false) {
    const h = 46 + lines(ctx, value, bodyWidth - 64, accent ? 30 : 24, true).length * (accent ? 43.5 : 34.8)
    room(h + 16)
    box(ctx, margin, y, bodyWidth, h, accent ? '#edf4ff' : '#fff', '#e0e7f3')
    write(ctx, label, margin + 28, y + 17, bodyWidth - 56, 16, muted)
    write(ctx, value, margin + 28, y + 43, bodyWidth - 56, accent ? 30 : 24, accent ? blue : navy, true)
    y += h + 16
  }
  page()
  y += write(ctx, 'Результаты тестирования', margin, y, bodyWidth, 40, navy, true) + 20
  panel('ТЕСТ', result.test_title || employee.test_title || 'Тест', true)
  panel('СОТРУДНИК', `${employee.full_name || 'Сотрудник'}${employee.email ? ` · ${employee.email}` : ''}`)
  const date = employee.finished_at || result.finished_at
  const duration = Math.max(0, Math.round(result.duration_seconds || 0))
  const metrics = [
    ['РЕЗУЛЬТАТ', `${Math.round(result.percent || 0)}%`],
    ['БАЛЛЫ', `${result.score ?? 0} / ${result.max_score ?? 0}`],
    ['ВРЕМЯ', `${Math.floor(duration / 60)} мин. ${duration % 60} сек.`],
  ]
  room(120)
  metrics.forEach(([label, value], i) => {
    const w = (bodyWidth - 28) / 3, x = margin + i * (w + 14)
    box(ctx, x, y, w, 100, i === 0 ? '#e7f8f1' : '#fff', '#e0e7f3')
    write(ctx, label, x + 22, y + 16, w - 44, 16, muted)
    write(ctx, value, x + 22, y + 47, w - 44, i === 0 ? 32 : 26, i === 0 ? '#087b56' : navy, true)
  })
  y += 120
  if (date) y += write(ctx, `Дата завершения: ${new Date(date).toLocaleString('ru-RU')}`, margin, y, bodyWidth, 18, muted) + 22
  const answers = new Map()
  for (const answer of result.answers || []) {
    const group = answers.get(answer.question_id) || { ...answer, ids: [] }
    if (answer.selected_answer_id) group.ids.push(answer.selected_answer_id)
    answers.set(answer.question_id, group)
  }
  const questions = result.questions || []
  room(150)
  write(ctx, 'РАЗБОР РЕЗУЛЬТАТОВ', margin, y, bodyWidth - 180, 17, blue, true)
  write(ctx, 'Проверьте свои ответы', margin, y + 30, bodyWidth - 180, 34, navy, true)
  box(ctx, width - margin - 140, y, 140, 80, '#edf4ff')
  write(ctx, `${questions.filter(q => answers.get(q.id)?.is_correct).length} / ${questions.length}`, width - margin - 120, y + 23, 110, 28, blue, true)
  y += 110
  for (const [index, question] of questions.entries()) {
    const answer = answers.get(question.id), correct = answer?.is_correct === true
    const green = '#087b56', red = '#b92f3b', accent = correct ? '#16aa78' : '#ed5964'
    const titleLines = lines(ctx, question.question, bodyWidth - 300, 23, true)
    const title = titleLines.slice(0, 12).join('\n'), titleHeight = Math.min(12, titleLines.length) * 33.35
    const blocks = []
    if (titleLines.length > 12) blocks.push({value: titleLines.slice(12).join('\n'), label: 'ПРОДОЛЖЕНИЕ ВОПРОСА', fill: '#fff', border: '#e4e9f2', color: navy})
    if (question.question_type === 'text') blocks.push({value: answer?.text_answer || 'Ответ не дан', label: 'ОТВЕТ СОТРУДНИКА', fill: correct ? '#fafcff' : '#fff3f4', border: correct ? '#e4e9f2' : '#f1bec3', color: navy})
    else for (const option of question.answers || []) {
      const selected = answer?.ids.includes(option.id), right = option.is_correct
      blocks.push({value: option.answer, label: selected ? 'ВЫБРАН СОТРУДНИКОМ' : '', marker: right ? '✓' : selected ? '×' : '', fill: right ? '#effbf6' : selected ? '#fff1f2' : '#fff', border: right ? '#a9e2cc' : selected ? '#f1b7bd' : '#e4e9f2', color: right ? green : selected ? red : '#667496'})
    }
    if (question.explanation) blocks.push({value: question.explanation, label: 'ПОЯСНЕНИЕ', fill: '#f1f5fb', border: '#f1f5fb', color: '#627194'})
    for (const block of blocks) block.height = 30 + (block.label ? 25 : 0) + lines(ctx, block.value, bodyWidth - 140, 20).length * 29
    const fullHeight = 80 + titleHeight + blocks.reduce((sum, b) => sum + b.height + 12, 0)
    // Keep ordinary question cards together; long answers continue on another page.
    room(Math.min(fullHeight + 22, bottom - 135))
    let segmentTop
    function closeCard() {
      ctx.beginPath(); ctx.roundRect(margin, segmentTop, bodyWidth, y - segmentTop, 14)
      ctx.strokeStyle = '#dde6f3'; ctx.lineWidth = 1.5; ctx.stroke()
      ctx.fillStyle = accent; ctx.fillRect(margin, segmentTop + 12, 5, y - segmentTop - 24)
    }
    function cardHeader(continued = false) {
      segmentTop = y
      const h = continued ? 77 : 80 + titleHeight
      box(ctx, margin, y, bodyWidth, h, correct ? '#fff' : '#fffdfd')
      box(ctx, margin + 22, y + 22, 44, 44, correct ? '#e3f8ef' : '#ffeaec', null, 22)
      write(ctx, correct ? '✓' : '×', margin + 34, y + 27, 30, 28, correct ? green : red, true)
      write(ctx, `ВОПРОС ${index + 1}${continued ? ' · ПРОДОЛЖЕНИЕ' : ''}`, margin + 82, y + 22, bodyWidth - 300, 15, muted)
      if (!continued) write(ctx, title, margin + 82, y + 48, bodyWidth - 300, 23, navy, true)
      const badge = correct ? 'Верно' : 'Есть ошибка'
      box(ctx, width - margin - 184, y + 22, 160, 38, correct ? '#e7f8f1' : '#ffebed', null, 8)
      write(ctx, badge, width - margin - 169, y + 31, 136, 18, correct ? green : red, true)
      y += h - 2
    }
    cardHeader()
    for (const block of blocks) {
      let remaining = lines(ctx, block.value, bodyWidth - 140, 20)
      while (remaining.length) {
        if (bottom - y < 110) { closeCard(); finish(); page(); cardHeader(true) }
        const count = Math.max(1, Math.floor((bottom - y - 42 - (block.label ? 25 : 0)) / 29))
        const part = remaining.splice(0, count), h = 30 + (block.label ? 25 : 0) + part.length * 29
        box(ctx, margin, y, bodyWidth, h + 12, correct ? '#fff' : '#fffdfd', null, 0)
        box(ctx, margin + 24, y + 6, bodyWidth - 48, h, block.fill, block.border, 12)
        if (block.marker !== undefined) {
          box(ctx, margin + 40, y + 21, 30, 30, block.marker ? block.color : '#fff', block.border, 15)
          if (block.marker) write(ctx, block.marker, margin + 47, y + 23, 25, 22, '#fff', true)
        }
        let top = y + 20
        if (block.label) { write(ctx, block.label, margin + 90, top, bodyWidth - 140, 14, block.color, true); top += 25 }
        write(ctx, part.join('\n'), margin + 90, top, bodyWidth - 140, 20, block.color)
        y += h + 12
        if (remaining.length) { closeCard(); finish(); page(); cardHeader(true) }
      }
    }
    closeCard()
    y += 24
  }
  finish()
  return pages
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
