(function () {
  if (!document.querySelector('link[data-duty-picker]')) {
    const styles = document.createElement('link');
    styles.rel = 'stylesheet';
    styles.href = '/static/duty-picker.css?v=4';
    styles.dataset.dutyPicker = 'true';
    document.head.append(styles);
  }

  function esc(value) {
    const node = document.createElement('span');
    node.textContent = value ?? '';
    return node.innerHTML;
  }

  function plural(count) {
    const last = count % 10;
    const lastTwo = count % 100;
    if (last === 1 && lastTwo !== 11) return 'обязанность';
    if (last >= 2 && last <= 4 && (lastTwo < 12 || lastTwo > 14)) return 'обязанности';
    return 'обязанностей';
  }

  window.DutyPicker = {
    mount(container, {
      categories,
      selected,
      onChange,
      title = 'Обязанности',
      subtitle = 'Выберите обязанности, которые относятся к этой позиции'
    }) {
      const chosen = selected instanceof Set ? selected : new Set(selected || []);
      let categoryID = categories[0]?.id || 0;
      let query = '';

      const allDuties = () => categories.flatMap(category =>
        (category.duties || []).map(duty => ({
          ...duty,
          category_icon: category.icon,
          category_name: category.name
        }))
      );
      const categoryDuties = id => id
        ? categories.find(category => category.id === id)?.duties || []
        : allDuties();
      const total = () => allDuties().length;
      const allSelected = duties => duties.length > 0 && duties.every(duty => chosen.has(duty.id));
      const someSelected = duties => duties.some(duty => chosen.has(duty.id));

      function visibleDuties() {
        const source = categoryDuties(categoryID);
        if (!query) return source;
        return source.filter(duty =>
          `${duty.name} ${duty.description || ''}`.toLowerCase().includes(query)
        );
      }

      function notify() {
        onChange?.([...chosen]);
      }

      function categoryRow(category) {
        const id = category?.id || 0;
        const duties = categoryDuties(id);
        const name = category?.name || 'Все категории';
        const count = category?.duty_count || duties.length;
        const state = allSelected(duties) ? 'checked' : someSelected(duties) ? 'partial' : '';
        const checked = state === 'checked' ? 'true' : state === 'partial' ? 'mixed' : 'false';
        return `
          <div class="duty-category-row ${id === categoryID ? 'active' : ''}">
            <button type="button" class="duty-category-check ${state}"
              data-category-check="${id}" aria-label="Выбрать все: ${esc(name)}"
              aria-checked="${checked}" role="checkbox"></button>
            <button type="button" class="duty-category-open" data-category="${id}">
              ${category ? `<i>${esc(category.icon || '◇')}</i>` : ''}
              <span>${esc(name)}</span><em>${count}</em>
            </button>
          </div>`;
      }

      function render() {
        const duties = visibleDuties();
        const activeCategory = categories.find(category => category.id === categoryID);
        container.innerHTML = `
          <section class="duty-picker-shell">
            <header class="duty-picker-head">
              <div><h1>${esc(title)}</h1><p>${esc(subtitle)}</p></div>
            </header>

            <div class="duty-toolbar">
              <label class="duty-search">
                <span>⌕</span>
                <input type="search" value="${esc(query)}"
                  placeholder="Поиск обязанностей" aria-label="Поиск обязанностей">
              </label>
            </div>

            <div class="duty-browser">
              <nav class="duty-categories" aria-label="Категории обязанностей">
                ${categories.map(categoryRow).join('')}
              </nav>

              <section class="duty-options">
                <header>
                  <div><span>✓</span> Выбрано <strong class="duty-selected-count">${chosen.size}</strong> из ${total()}</div>
                  <div class="duty-bulk-actions">
                    <button type="button" data-select-visible>Выбрать все</button>
                    <button type="button" data-clear-visible>Снять все</button>
                  </div>
                </header>
                <div class="duty-options-list">
                  ${activeCategory && !query ? `<h2>${esc(activeCategory.name)}</h2>` : ''}
                  ${duties.map(duty => `
                    <label class="duty-option ${chosen.has(duty.id) ? 'selected' : ''}">
                      <input type="checkbox" data-duty="${duty.id}" ${chosen.has(duty.id) ? 'checked' : ''}>
                      <span class="duty-checkbox">✓</span>
                      <span class="duty-copy">
                        <b>${esc(duty.name)}</b>
                        ${duty.description ? `<small>${esc(duty.description)}</small>` : ''}
                      </span>
                      <span class="duty-info" title="${esc(duty.description || duty.name)}">i</span>
                    </label>`).join('') || '<p class="duty-empty">По вашему запросу ничего не найдено</p>'}
                </div>
              </section>
            </div>

          </section>`;
        bind();
      }

      function updateCategory(id) {
        const duties = categoryDuties(id);
        const shouldSelect = !allSelected(duties);
        duties.forEach(duty => shouldSelect ? chosen.add(duty.id) : chosen.delete(duty.id));
        notify();
        render();
      }

      function bind() {
        const search = container.querySelector('.duty-search input');
        search.oninput = event => {
          query = event.target.value.trim().toLowerCase();
          if (query) categoryID = 0;
          render();
          requestAnimationFrame(() => {
            const next = container.querySelector('.duty-search input');
            next?.focus();
            next?.setSelectionRange(next.value.length, next.value.length);
          });
        };
        container.querySelectorAll('[data-category]').forEach(button => {
          button.onclick = () => {
            categoryID = Number(button.dataset.category);
            query = '';
            render();
          };
        });
        container.querySelectorAll('[data-category-check]').forEach(button => {
          button.onclick = event => {
            event.stopPropagation();
            updateCategory(Number(button.dataset.categoryCheck));
          };
        });
        container.querySelectorAll('[data-duty]').forEach(input => {
          input.onchange = () => {
            const id = Number(input.dataset.duty);
            input.checked ? chosen.add(id) : chosen.delete(id);
            notify();
            render();
          };
        });
        container.querySelector('[data-select-visible]')?.addEventListener('click', () => {
          visibleDuties().forEach(duty => chosen.add(duty.id));
          notify();
          render();
        });
        container.querySelector('[data-clear-visible]')?.addEventListener('click', () => {
          visibleDuties().forEach(duty => chosen.delete(duty.id));
          notify();
          render();
        });
      }

      render();
      return {
        selected: chosen,
        getSelected: () => [...chosen],
        validate: () => chosen.size > 0,
        rerender: render
      };
    }
  };
})();
