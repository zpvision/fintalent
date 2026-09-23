import { useEffect, useState } from 'react'

const parse = (value) => String(value || '').split(',').map((item) => item.trim()).filter(Boolean)

export default function CommaSeparatedField({ label, value, onChange }) {
  const serialized = (value || []).join(', ')
  const [draft, setDraft] = useState(serialized)

  useEffect(() => {
    if (parse(draft).join(', ') !== serialized) setDraft(serialized)
  }, [draft, serialized])

  return <label className="pm-field">{label}<input value={draft} onChange={(event) => {
    const next = event.target.value
    setDraft(next)
    onChange(parse(next))
  }} /></label>
}
