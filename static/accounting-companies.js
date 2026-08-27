(() => {
  const API = "/api/accounting-companies";
  const form = document.querySelector("#ac-filters");
  const list = document.querySelector("#ac-company-list");
  const found = document.querySelector("#ac-found");
  const pager = document.querySelector("#ac-pagination");
  const quick = document.querySelector("#ac-quick-filters");
  const sort = document.querySelector("#ac-sort");
  const limit = document.querySelector("#ac-limit");
  let page = 1;
  let timer;

  const esc = (value) => {
    const element = document.createElement("span");
    element.textContent = value ?? "";
    return element.innerHTML;
  };
  const money = (value) => value == null ? "по запросу" : new Intl.NumberFormat("ru-RU").format(value) + " ₽";
  const priceType = {
    from_month: "от {p} / мес.",
    month: "{p} / мес.",
    from_hour: "от {p} / час",
    hour: "{p} / час",
    from_once: "от {p}",
    request: "по запросу",
  };
  const chipIcons = ["▤", "⌑", "▣", "◉", "▧", "◫", "◆", "◇", "✦"];

  async function api(url) {
    const response = await fetch(url, { cache: "no-store" });
    const data = await response.json().catch(() => ({}));
    if (!response.ok) throw Error(data.error || "Ошибка загрузки");
    return data;
  }

  function price(service) {
    return (priceType[service.price_type] || "{p}").replace("{p}", money(service.price_from));
  }

  function logo(company) {
    if (company.logo) return `<img src="${esc(company.logo)}" alt="">`;
    return `<span>${esc((company.name || "Б").trim().charAt(0).toUpperCase())}</span>`;
  }

  function founded(company) {
    return company.founded_year ? `Работает с ${esc(company.founded_year)} года` : "Опыт работы указан в профиле";
  }

  function passport(company) {
    const value = company.passport_summary ? Math.round(company.passport_summary.overall_index) : 0;
    if (!value) {
      return `<div class="ac-directory-passport ghost">
        <div class="ac-directory-ring" style="--score:62;--ring:#b9c3de"><b>?</b></div>
        <span><small>Паспорт компетенций</small><b>Формируется</b><em>Примерный график появится после тестов</em></span>
      </div>`;
    }
    const tone = value >= 90 ? "green" : value >= 80 ? "blue" : value >= 70 ? "orange" : "red";
    const label = value >= 90 ? "Отличный уровень" : value >= 80 ? "Хороший уровень" : value >= 70 ? "Средний уровень" : "Требует проверки";
    const color = { green: "#10a970", blue: "#2a7bf6", orange: "#ff8a00", red: "#ef496f" }[tone];
    return `<div class="ac-directory-passport ${tone}">
      <div class="ac-directory-ring ${tone}" style="--score:${value};--ring:${color}"><b>${value}%</b></div>
      <span><small>Паспорт компетенций</small><b>${label}</b><em>${company.passport_summary.tests_count || 0} направлений<br>${company.passport_summary.specialists_count || 0} специалистов</em></span>
    </div>`;
  }

  function services(company) {
    const items = (company.services || []).slice(0, 3);
    if (!items.length) return '<div><small>Услуги</small><b>по запросу</b></div>';
    return items.map((service) => `<div><small>${esc(service.name)}</small><b>${esc(price(service))}</b></div>`).join("");
  }

  function row(company) {
    const href = `/accounting-companies/view?slug=${encodeURIComponent(company.slug)}`;
    const tags = (company.directions || []).slice(0, 3).map((item) => `<span>${esc(item.name)}</span>`).join("");
    const more = Math.max(0, (company.directions || []).length - 3);
    return `<article class="ac-directory-row">
      <a class="ac-directory-logo" href="${href}">${logo(company)}</a>
      <section class="ac-directory-company">
        <h3><a href="${href}">${esc(company.name)}</a>${company.verified ? "<i>✓</i>" : ""}</h3>
        <p>${esc(company.short_description || "Комплексное бухгалтерское сопровождение бизнеса")}</p>
        <div><span>⌖ ${esc(company.city || "Онлайн")}</span><span>${founded(company)}</span>${company.employee_count ? `<span>${esc(company.employee_count)} сотрудников</span>` : ""}</div>
      </section>
      <section class="ac-directory-tags">${tags}${more ? `<span>+${more}</span>` : ""}</section>
      <section class="ac-directory-services">${services(company)}</section>
      ${passport(company)}
      <section class="ac-directory-actions">
        <a href="${href}">Посмотреть</a>
        <button type="button" aria-label="Сохранить компанию">♡ <span>Сохранить</span></button>
      </section>
    </article>`;
  }

  function renderQuick(meta) {
    const serviceItems = (meta.services || []).slice(0, 6).map((item) => ({ ...item, kind: "service_id" }));
    const directionItems = (meta.directions || []).slice(0, 3).map((item) => ({ ...item, kind: "direction_id" }));
    quick.innerHTML = [...serviceItems, ...directionItems].map((item, index) => `<button type="button" data-kind="${item.kind}" data-value="${item.id}"><i>${chipIcons[index % chipIcons.length]}</i>${esc(item.name)}</button>`).join("") + '<button type="button" data-more>Ещё⌄</button>';
    quick.querySelectorAll("[data-kind]").forEach((button) => button.onclick = () => {
      form.elements[button.dataset.kind].value = button.dataset.value;
      page = 1;
      load();
    });
  }

  function sortedItems(items) {
    if (sort.value !== "passport") return items;
    return [...items].sort((a, b) => (b.passport_summary?.overall_index || 0) - (a.passport_summary?.overall_index || 0));
  }

  async function load() {
    list.innerHTML = '<div class="ac-skeleton"></div><div class="ac-skeleton"></div><div class="ac-skeleton"></div>';
    try {
      const params = new URLSearchParams(new FormData(form));
      [...params].forEach(([key, value]) => { if (!value) params.delete(key); });
      params.set("page", page);
      params.set("limit", limit?.value || "10");
      const data = await api(API + "?" + params);
      found.textContent = `Найдено компаний: ${data.total}`;
      list.innerHTML = sortedItems(data.items).map(row).join("") || '<div class="ac-empty"><i>⌕</i><h3>Компании не найдены</h3><p>Измените параметры или сбросьте фильтры.</p></div>';
      pager.innerHTML = Array.from({ length: data.pages }, (_, index) => index + 1)
        .filter((number) => number === 1 || number === data.pages || Math.abs(number - data.page) < 3)
        .map((number) => `<button class="${number === data.page ? "active" : ""}" data-page="${number}">${number}</button>`)
        .join("");
      pager.querySelectorAll("button").forEach((button) => button.onclick = () => {
        page = +button.dataset.page;
        load();
        scrollTo({ top: 260, behavior: "smooth" });
      });
    } catch (error) {
      found.textContent = "Не удалось загрузить";
      list.innerHTML = `<div class="ac-empty"><i>!</i><h3>Ошибка загрузки</h3><p>${esc(error.message)}</p><button class="ac-button" id="ac-retry">Повторить</button></div>`;
      document.querySelector("#ac-retry")?.addEventListener("click", load);
    }
  }

  async function init() {
    try {
      const meta = await api(API + "/meta");
      for (const [name, items] of [["direction_id", meta.directions], ["service_id", meta.services], ["tax_system_id", meta.tax_systems]]) {
        const select = form.elements[name];
        items.forEach((item) => select.insertAdjacentHTML("beforeend", `<option value="${item.id}">${esc(item.name)}</option>`));
      }
      renderQuick(meta);
    } catch {}
    load();
  }

  form.addEventListener("input", () => {
    clearTimeout(timer);
    timer = setTimeout(() => {
      page = 1;
      load();
    }, 300);
  });
  sort.addEventListener("change", load);
  limit?.addEventListener("change", () => {
    page = 1;
    load();
  });
  document.querySelector("#ac-reset").onclick = () => {
    form.reset();
    page = 1;
    load();
  };
  init();
})();
