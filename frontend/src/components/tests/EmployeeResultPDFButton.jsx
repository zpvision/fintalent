import { useState } from 'react'
import { downloadEmployeeResultPDF } from '../../../../static/employee-result-pdf.js'

export default function EmployeeResultPDFButton({ employee, onNotice }) {
  const [busy, setBusy] = useState(false)

  async function download() {
    if (busy) return
    setBusy(true)
    try {
      await downloadEmployeeResultPDF(employee)
      onNotice({ type: 'success', text: 'PDF готов, скачивание началось' })
    } catch (error) {
      onNotice({ type: 'error', text: error.message || 'Не удалось скачать PDF' })
    } finally {
      setBusy(false)
    }
  }

  return <button type="button" className="et-download-pdf" disabled={busy} aria-busy={busy}
    title={busy ? 'Готовим PDF…' : 'Скачать результаты в PDF'}
    aria-label={busy ? 'Готовим PDF…' : `Скачать результаты ${employee.full_name} в PDF`} onClick={download}>
    {busy ? <><span className="et-pdf-spinner" aria-hidden="true"/><span role="status">Готовим PDF…</span></> :
      <svg width="24" height="28" viewBox="0 0 24 28" aria-hidden="true"><path d="M4 1h10l6 6v19H4Z M14 1v7h6" fill="none" stroke="currentColor" strokeWidth="1.5"/><text x="5" y="20" fontSize="8" fontWeight="bold" fill="currentColor">PDF</text></svg>}
  </button>
}
