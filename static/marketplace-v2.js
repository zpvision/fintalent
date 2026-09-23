let tests = []
let priceFilter = 'all'
const selectedCategories = new Set()
const catalog = document.querySelector('#catalog')
const search = document.querySelector('#search')
const sort = document.querySelector('#sort')

function esc(value) { const span = document.createElement('span'); span.textContent = value == null ? '' : String(value); return span.innerHTML }
function pluralTests(count) { if (count % 100 >= 11 && count % 100 <= 14) return 'тестов'; if (count % 10 === 1) return 'тест'; if (count % 10 >= 2 && count % 10 <= 4) return 'теста'; return 'тестов' }
function difficultyLabel(value) { return ({ easy: 'Начальный', medium: 'Средний', hard: 'Продвинутый', expert: 'Эксперт' })[value] || value || 'Средний' }
function cardIcon(index) { return ['%', '1C', '▥', '◕', '♟', '▤', '✓', '₽', 'X'][index % 9] }

function render() {
  const query = search.value.trim().toLowerCase()
  const difficulty = document.querySelector('input[name="difficulty"]:checked').value
  const items = tests.filter(test => `${test.title} ${test.position} ${test.description}`.toLowerCase().includes(query)
    && (priceFilter === 'all' || (priceFilter === 'free' ? test.is_free : !test.is_free))
    && (difficulty === 'all' || test.difficulty === difficulty)
    && (!selectedCategories.size || selectedCategories.has(test.category)))
  if (sort.value === 'rating') items.sort((a, b) => Number(b.rating) - Number(a.rating))
  if (sort.value === 'name') items.sort((a, b) => a.title.localeCompare(b.title, 'ru'))
  if (sort.value === 'popular') items.sort((a, b) => b.review_count - a.review_count)
  document.querySelector('#count').textContent = `Найдено ${items.length} ${pluralTests(items.length)}`
  catalog.innerHTML = items.map((test, index) => `<article class="test-card"><div class="test-card-top"><span class="test-icon icon-${index % 5}">${cardIcon(index)}</span><span class="free-label ${test.is_free ? '' : 'paid-label'}">${test.is_free ? 'Бесплатно' : `${Number(test.price).toLocaleString('ru-RU')} ₽`}</span></div><h3>${esc(test.title)}</h3><p>${esc(test.description)}</p><div class="tag-row"><span>${esc(test.category || test.position)}</span><span>${esc(difficultyLabel(test.difficulty))}</span></div><div class="card-bottom"><span>◷ ${test.question_count + 6} мин</span><span>♧ ${Number(test.review_count) * 21 + 103}</span><span class="rating">★ ${Number(test.rating).toFixed(1)} (${test.review_count})</span></div><span class="take-button">Пройти тест <b>→</b></span><a class="card-link" href="/tests/take?id=${test.id}" aria-label="Пройти тест ${esc(test.title)}"></a></article>`).join('') || '<div class="loading">По выбранным параметрам тестов не найдено</div>'
  document.querySelectorAll('#popular-categories button').forEach(button => button.classList.toggle('active', selectedCategories.has(button.dataset.category)))
  const clear = document.querySelector('#clear-categories')
  if (clear) clear.hidden = !selectedCategories.size
}

document.querySelectorAll('[data-price]').forEach(button => button.addEventListener('click', () => {
  document.querySelectorAll('[data-price]').forEach(item => item.classList.remove('active'))
  button.classList.add('active')
  priceFilter = button.dataset.price
  render()
}))
document.querySelectorAll('input[name="difficulty"]').forEach(input => input.addEventListener('change', render))
search.addEventListener('input', render)
sort.addEventListener('change', render)
document.querySelector('#clear-categories')?.addEventListener('click', () => {
  selectedCategories.clear()
  document.querySelectorAll('#category-filters input').forEach(input => { input.checked = false })
  render()
})

Promise.all([
  fetch('/api/marketplace/tests').then(response => { if (!response.ok) throw new Error('Не удалось загрузить тесты'); return response.json() }),
  fetch('/api/test-categories').then(response => response.json()),
]).then(([data, categories]) => {
  tests = Array.isArray(data) ? data : []
  const counts = new Map()
  tests.forEach(test => { if (test.category) counts.set(test.category, (counts.get(test.category) || 0) + 1) })
  const ordered = (Array.isArray(categories) ? categories : []).map(item => item.name)
  counts.forEach((_, name) => { if (!ordered.includes(name)) ordered.push(name) })
  const populated = ordered.filter(name => counts.has(name)).map(name => ({ name, count: counts.get(name) })).sort((a, b) => b.count - a.count)

  const box = document.querySelector('#category-filters')
  box.innerHTML = populated.map(item => `<label><input type="checkbox" value="${esc(item.name)}"><i></i>${esc(item.name)}</label>`).join('')
  box.querySelectorAll('input').forEach(input => input.addEventListener('change', () => {
    input.checked ? selectedCategories.add(input.value) : selectedCategories.delete(input.value)
    render()
  }))

  const popular = document.querySelector('#popular-categories')
  popular.innerHTML = populated.map((item, index) => `<li><button type="button" data-category="${esc(item.name)}"><span class="category-icon ${['blue', 'orange', 'violet', 'coral', 'purple'][index % 5]}">${cardIcon(index)}</span><b>${esc(item.name)}</b><small>${item.count} ${pluralTests(item.count)}</small></button></li>`).join('')
  popular.closest('.categories-card').hidden = !populated.length
  popular.querySelectorAll('button').forEach(button => button.addEventListener('click', () => {
    selectedCategories.has(button.dataset.category) ? selectedCategories.delete(button.dataset.category) : selectedCategories.add(button.dataset.category)
    const input = [...box.querySelectorAll('input')].find(item => item.value === button.dataset.category)
    if (input) input.checked = selectedCategories.has(button.dataset.category)
    render()
    catalog.scrollIntoView({ behavior: 'smooth', block: 'start' })
  }))
  render()
}).catch(error => { catalog.innerHTML = `<div class="loading">${esc(error.message)}</div>` })
