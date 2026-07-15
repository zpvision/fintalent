document.head.insertAdjacentHTML('beforeend','<link rel="stylesheet" href="/static/searchable-select.css">');

class SearchableSelect {
  constructor(select) {
    this.select = select;
    this.root = document.createElement('span');
    this.root.className = 'searchable-select';
    this.input = document.createElement('input');
    this.input.type = 'text';
    this.input.autocomplete = 'off';
    this.input.setAttribute('role', 'combobox');
    this.input.setAttribute('aria-expanded', 'false');
    this.input.placeholder = select.options[0]?.text || 'Выберите значение';
    this.dropdown = document.createElement('span');
    this.dropdown.className = 'searchable-options';
    this.root.append(this.input, this.dropdown);
    select.after(this.root);
    select.classList.add('searchable-native');
    this.syncValue();
    this.input.addEventListener('focus', () => this.open());
    this.input.addEventListener('input', () => this.render(this.input.value));
    this.input.addEventListener('keydown', event => {
      if (event.key === 'Escape') this.close();
      if (event.key === 'Enter') {
        const first = this.dropdown.querySelector('button');
        if (first) { event.preventDefault(); first.click(); }
      }
    });
    select.addEventListener('change', () => this.syncValue());
    new MutationObserver(() => { this.syncValue(); if (this.dropdown.classList.contains('open')) this.render(this.input.value); }).observe(select, {childList:true, subtree:true});
    document.addEventListener('click', event => { if (!this.root.contains(event.target)) this.close(); });
  }
  options() { return [...this.select.options]; }
  syncValue() {
    const selected = this.select.options[this.select.selectedIndex];
    this.input.value = selected && selected.value ? selected.text : '';
    this.input.placeholder = this.select.options[0]?.text || 'Выберите значение';
  }
  open() { this.render(''); this.input.select(); }
  close() { this.dropdown.classList.remove('open'); this.input.setAttribute('aria-expanded','false'); if (!this.input.value) this.select.value=''; }
  render(query='') {
    const normalized = query.trim().toLocaleLowerCase('ru');
    const options = this.options().filter(option => !normalized || option.text.toLocaleLowerCase('ru').includes(normalized));
    this.dropdown.innerHTML = options.length ? options.map(option => `<button type="button" data-value="${this.escape(option.value)}" class="${option.value===this.select.value?'active':''}">${this.escape(option.text)}</button>`).join('') : '<em>Ничего не найдено</em>';
    this.dropdown.classList.add('open');
    this.input.setAttribute('aria-expanded','true');
    this.dropdown.querySelectorAll('button').forEach(button => button.addEventListener('click', () => {
      this.select.value = button.dataset.value;
      this.select.dispatchEvent(new Event('change', {bubbles:true}));
      this.close();
    }));
  }
  escape(value) { const node=document.createElement('span'); node.textContent=value; return node.innerHTML; }
}

document.querySelectorAll('select').forEach(select => new SearchableSelect(select));
