// Resolve a wall-clock input against an IANA zone, never the browser's implicit zone.
// Gaps and repeated DST times must be corrected before scheduling.
export function cohortTimeInput(value, zone) {
 if (!value) throw Error('Укажите дату и время')
 const [date, clock] = value.split('T'), [y,m,d] = date.split('-').map(Number), [h,min] = clock.split(':').map(Number), naive = Date.UTC(y,m-1,d,h,min)
 const formatter = new Intl.DateTimeFormat('sv-SE', { year:'numeric', month:'2-digit', day:'2-digit', hour:'2-digit', minute:'2-digit', hourCycle:'h23', timeZone:zone }), offsets = new Set()
 for (const delta of [-86400000,0,86400000]) { const at = naive + delta, parts = Object.fromEntries(formatter.formatToParts(new Date(at)).map(p=>[p.type,p.value])); offsets.add(Date.UTC(+parts.year,+parts.month-1,+parts.day,+parts.hour,+parts.minute)-at) }
 const matches = [...offsets].map(offset=>naive-offset).filter(at=>formatter.format(new Date(at)).replace(' ','T')===value)
 if (matches.length !== 1) throw Error('Это местное время не существует или повторяется при переводе часов. Выберите другое время.')
 return new Date(matches[0]).toISOString()
}
