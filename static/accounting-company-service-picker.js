(() => {
  let services = []
  function decorate(select) {
    if (!select) return
    let picker = select.closest('.ac-service-select')
    if (!picker) {
      picker = document.createElement('div')
      picker.className = 'ac-service-select'
      select.before(picker)
      picker.append(select)
    }
    const selected = services.find(item => Number(item.id) === Number(select.value))
    let icon = picker.querySelector(':scope > .ac-service-selected-icon')
    picker.classList.toggle('has-icon', Boolean(selected?.icon))
    if (!selected?.icon) { icon?.remove(); return }
    if (!icon) { icon = document.createElement('img'); icon.className = 'ac-service-selected-icon'; icon.alt = ''; picker.prepend(icon) }
    icon.src = selected.icon
  }
  function decorateAll() { document.querySelectorAll('.ac-service-row select[name="service_id"]').forEach(decorate) }
  document.addEventListener('change', event => { if (event.target.matches('.ac-service-row select[name="service_id"]')) decorate(event.target) })
  new MutationObserver(decorateAll).observe(document.body, { childList: true, subtree: true })
  fetch('/api/accounting-companies/meta').then(response => response.ok ? response.json() : Promise.reject()).then(meta => { services = Array.isArray(meta.services) ? meta.services : []; decorateAll() }).catch(() => {})
})()
