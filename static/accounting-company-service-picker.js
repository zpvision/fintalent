(() => {
  let services = []
  function decorate(select) {
    if (!select || select.dataset.searchable === 'true') return
    select.dataset.searchable = 'true'
    const picker = document.createElement('div')
    picker.className = 'ac-service-search'
    select.before(picker)
    picker.append(select)
    const input = document.createElement('input')
    input.type = 'text'
    input.placeholder = 'Найти и выбрать услугу'
    input.autocomplete = 'off'
    input.setAttribute('role', 'combobox')
    const arrow = document.createElement('span')
    arrow.textContent = '⌄'
    const options = document.createElement('div')
    options.className = 'ac-service-search-options'
    options.hidden = true
    picker.append(input, arrow, options)
    function render(query = '') {
      const normalized = query.trim().toLocaleLowerCase('ru')
      const filtered = services.filter(item => `${item.name} ${item.category || ''}`.toLocaleLowerCase('ru').includes(normalized))
      options.replaceChildren()
      if (!filtered.length) { const empty = document.createElement('em'); empty.textContent = 'Ничего не найдено'; options.append(empty); return }
      filtered.forEach(item => {
        const button = document.createElement('button')
        button.type = 'button'
        if (Number(item.id) === Number(select.value)) button.classList.add('selected')
        if (item.icon) { const image = document.createElement('img'); image.src = item.icon; image.alt = ''; button.append(image) }
        const name = document.createElement('b'); name.textContent = item.name; button.append(name)
        if (item.category) { const category = document.createElement('small'); category.textContent = item.category; button.append(category) }
        button.onclick = () => { select.value = item.id; input.value = item.name; options.hidden = true; picker.classList.remove('open'); input.setAttribute('aria-expanded', 'false'); select.dispatchEvent(new Event('input', { bubbles: true })); select.dispatchEvent(new Event('change', { bubbles: true })) }
        options.append(button)
      })
    }
    function sync() {
      const selected = services.find(item => Number(item.id) === Number(select.value))
      input.value = selected?.name || ''
      let icon = picker.querySelector(':scope > .ac-service-search-icon')
      if (!selected?.icon) { icon?.remove(); return }
      if (!icon) { icon = document.createElement('img'); icon.className = 'ac-service-search-icon'; icon.alt = ''; picker.prepend(icon) }
      icon.src = selected.icon
    }
    input.onfocus = () => { input.value = ''; render(); options.hidden = false; picker.classList.add('open'); input.setAttribute('aria-expanded', 'true') }
    input.oninput = () => { render(input.value); options.hidden = false; picker.classList.add('open') }
    input.onkeydown = event => { if (event.key === 'Escape') { options.hidden = true; picker.classList.remove('open'); sync() } }
    select.addEventListener('change', sync)
    sync()
  }
  function decorateAll() { document.querySelectorAll('.ac-service-row select[name="service_id"]').forEach(decorate) }
  document.addEventListener('pointerdown', event => { document.querySelectorAll('.ac-service-search').forEach(picker => { if (!picker.contains(event.target)) { const options = picker.querySelector('.ac-service-search-options'); if (options) options.hidden = true; picker.classList.remove('open') } }) })
  new MutationObserver(decorateAll).observe(document.body, { childList: true, subtree: true })
  fetch('/api/accounting-companies/meta').then(response => response.ok ? response.json() : Promise.reject()).then(meta => { services = Array.isArray(meta.services) ? meta.services : []; decorateAll() }).catch(() => {})
})()
