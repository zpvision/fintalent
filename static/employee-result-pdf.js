// Both React and legacy screens download the same server-rendered HTML report.
export async function downloadEmployeeResultPDF(employee) {
  const response = await fetch(`/api/employee-testing/invitations/${encodeURIComponent(employee.id)}/results.pdf`, {
    credentials: 'include', cache: 'no-store', headers: { Accept: 'application/pdf' },
  })
  if (!response.ok) {
    const body = await response.json().catch(() => ({}))
    throw new Error(body.error || 'Не удалось скачать PDF')
  }
  if (!(response.headers.get('Content-Type') || '').includes('application/pdf')) throw new Error('Сервер вернул некорректный PDF')
  const blob = await response.blob(), url = URL.createObjectURL(blob), link = document.createElement('a')
  link.href = url
  link.download = `FinTalent-${String(employee.full_name || 'result').replace(/[<>:"/\\|?*\x00-\x1f]/g, '_')}.pdf`
  link.click()
  setTimeout(() => URL.revokeObjectURL(url), 60000)
}
