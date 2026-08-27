(() => {
  const API = "/api/client-exchange";
  const card = document.querySelector("#wizard-card");
  const next = document.querySelector("#wizard-next");
  const back = document.querySelector("#wizard-back");
  const draft = document.querySelector("#save-draft");
  const stepTrack = document.querySelector("#client-exchange-step-track");
  const stepNames = ["Основное", "Размер бизнеса", "Особенности", "Причина", "Передача", "Проверка"];
  let meta = {};
  let step = 1;
  let id = Number(new URLSearchParams(location.search).get("id")) || 0;
  let saving = false;
  let cityTimer;
  let cityRequest = 0;

  let data = {
    title: "",
    client_inn: "",
    client_legal_name: "",
    industry_id: null,
    industry_ids: [],
    employee_range_id: null,
    tax_system_id: null,
    revenue_range_id: null,
    accounting_state_id: null,
    transfer_reason_id: null,
    transfer_reason_ids: [],
    transfer_type_id: null,
    transfer_reason_comment: "",
    transfer_price: null,
    monthly_commission_percent: null,
    commission_months: null,
    current_monthly_fee: null,
    operations_per_month: null,
    banks_count: null,
    has_vat: false,
    foreign_trade: false,
    bargain_allowed: false,
    region: "",
    city: "",
    client_since: null,
    desired_transfer_date: null,
    comment: "",
    current_step: 1,
    marketplace_ids: [],
    edo_provider_ids: [],
    accounting_program_ids: [],
  };

  const esc = (value) => {
    const element = document.createElement("span");
    element.textContent = value ?? "";
    return element.innerHTML;
  };
  const money = (value) => value == null ? "—" : new Intl.NumberFormat("ru-RU").format(value) + " ₽";

  async function api(url, options = {}) {
    const response = await fetch(url, { cache: "no-store", ...options });
    let body = {};
    try { body = await response.json(); } catch {}
    if (!response.ok) throw Error(body.error || "Не удалось выполнить запрос");
    return body;
  }

  function toast(message, error = false) {
    const toastNode = document.querySelector("#ce-toast");
    toastNode.textContent = message;
    toastNode.className = "ce-toast show" + (error ? " error" : "");
    setTimeout(() => toastNode.className = "ce-toast", 2600);
  }

  function dictionary(kind) {
    const items = meta[kind] || [];
    if (kind === "revenue_range") return items.filter((item) => item.code !== "over_1b");
    return items;
  }

  function iconFor(item, fallback = "▣") {
    const icon = String(item?.icon || "").trim();
    if (!icon) return fallback;
    if (icon.startsWith("/")) return `<img src="${esc(icon)}" alt="">`;
    if (/^(?:static\/)?[a-z0-9][a-z0-9._/-]*\.(?:svg|png|jpe?g|webp|gif)$/i.test(icon) && !icon.includes("..")) {
      const source = icon.toLowerCase().startsWith("static/") ? `/${icon}` : `/static/${icon}`;
      return `<img src="${esc(source)}" alt="">`;
    }
    return esc(icon);
  }

  function renderStepTrack() {
    if (!stepTrack) return;
    stepTrack.innerHTML = stepNames.map((label, index) => {
      const number = index + 1;
      const done = number < step;
      const current = number === step;
      return `${index ? `<span class="wizard-step-connector ${number <= step ? "completed" : ""}"></span>` : ""}
        <span class="wizard-step-item">
          <button type="button" class="wizard-step-node ${done ? "completed" : current ? "current" : ""}" data-step-jump="${number}" ${current ? 'aria-current="step"' : ""}>${done ? "✓" : number}</button>
          <small class="${isStepFilled(number) ? "filled" : ""}">${esc(label)}</small>
        </span>`;
    }).join("");
    stepTrack.querySelectorAll("[data-step-jump]").forEach((button) => button.onclick = () => {
      capture();
      step = Number(button.dataset.stepJump);
      render();
      save(true);
    });
  }

  function isStepFilled(number) {
    if (number === 1) return data.industry_ids.length && data.tax_system_id && String(data.city || "").trim();
    if (number === 2) return data.revenue_range_id && data.employee_range_id;
    if (number === 3) return data.accounting_state_id;
    if (number === 4) return data.transfer_reason_ids.length;
    if (number === 5) return data.transfer_type_id;
    return data.industry_ids.length && data.revenue_range_id && data.employee_range_id && data.transfer_type_id;
  }

  function stepHead(number, icon, title, text) {
    return `<header class="step-head"><i>${icon}</i><div><small>ШАГ ${number}</small><h2>${esc(title)}</h2><p>${esc(text)}</p></div></header>`;
  }

  function field(label, name, type = "text", wide = false, attrs = "") {
    return `<label class="field ${wide ? "wide" : ""}"><span>${label}</span><input type="${type}" name="${name}" value="${esc(data[name] ?? "")}" ${attrs}></label>`;
  }

  function selectField(label, name, kind, wide = false, extraClass = "") {
    return `<label class="field ${wide ? "wide" : ""} ${extraClass}"><span>${label}</span><select name="${name}"><option value="">Выберите вариант</option>${dictionary(kind).map((item) => `<option value="${item.id}" ${Number(data[name]) === item.id ? "selected" : ""}>${esc(item.name)}</option>`).join("")}</select></label>`;
  }

  function choices(kind, name, multiple = false, options = {}) {
    const selected = multiple ? new Set((data[name] || []).map(Number)) : new Set([Number(data[name])]);
    const classes = ["choice-grid"];
    if (options.four) classes.push("four");
    if (options.compact) classes.push("compact");
    if (options.eight) classes.push("eight");
    if (options.plain) classes.push("plain");
    const items = dictionary(kind);
    const visible = options.limit && !options.showAll ? items.slice(0, options.limit) : items;
    const grid = `<div class="${classes.join(" ")}">${visible.map((item, index) => `
      <label class="choice-card ${options.withIcons ? "with-icon" : ""} ${options.plain ? "plain" : ""}">
        <input type="${multiple ? "checkbox" : "radio"}" name="${name}" value="${item.id}" ${selected.has(item.id) ? "checked" : ""}>
        <span>${options.withIcons ? `<i>${iconFor(item, options.fallbackIcon || fallbackIcon(kind, index))}</i>` : ""}<b>${esc(item.name)}</b>${item.description && !options.plain ? `<small>${esc(item.description)}</small>` : ""}</span>
      </label>`).join("")}</div>`;
    if (options.limit && items.length > options.limit && !options.showAll) {
      return `${grid}<button type="button" class="show-more-choices" data-show-industries>Показать остальные</button>`;
    }
    return grid;
  }

  function multiSearchSelect(kind, name, placeholder = "Поиск") {
    const selected = new Set((data[name] || []).map(Number));
    const items = dictionary(kind);
    const selectedItems = items.filter((item) => selected.has(item.id));
    return `<div class="multi-search" data-multi-search="${name}" data-kind="${kind}">
      <div class="multi-search-control">
        <div class="multi-search-tags">${selectedItems.map((item) => `<button type="button" class="multi-search-tag" data-remove-value="${item.id}">${esc(item.name)} <span>×</span></button>`).join("") || `<span class="multi-search-placeholder">Выберите направления</span>`}</div>
        <input type="search" class="multi-search-input" placeholder="${esc(placeholder)}" autocomplete="off">
      </div>
      <div class="multi-search-options">
        ${items.map((item) => `<label data-option-text="${esc(item.name).toLocaleLowerCase("ru")}">
          <input type="checkbox" name="${name}" value="${item.id}" ${selected.has(item.id) ? "checked" : ""}>
          <span>${esc(item.name)}</span>
        </label>`).join("")}
      </div>
    </div>`;
  }

  function fallbackIcon(kind, index) {
    const sets = {
      industry: ["🛒", "🏪", "🚚", "🏭", "🏗", "💻", "📣", "⚖"],
      revenue_range: ["₽", "₽", "₽", "₽", "₽", "₽", "₽", "₽", "₽"],
      employee_range: ["👤", "👥", "👥", "👥", "🏢", "🏢", "🏢", "🏢", "🏢", "🏢"],
      marketplace: ["🛍"],
      edo_provider: ["⇄"],
      accounting_program: ["▦"],
      accounting_state: ["✓"],
      transfer_reason: ["!"],
      transfer_type: ["₽"],
    };
    return (sets[kind] || ["▣"])[index % (sets[kind] || ["▣"]).length];
  }

  function render() {
    renderStepTrack();
    document.querySelector("#step-label").textContent = `Шаг ${step} из 6`;
    document.querySelector("#step-name").textContent = stepNames[step - 1];
    back.disabled = step === 1;
    next.textContent = step === 6 ? "Опубликовать" : "Продолжить";

    if (step === 1) renderBasics();
    if (step === 2) renderBusinessSize();
    if (step === 3) renderFeatures();
    if (step === 4) renderReason();
    if (step === 5) renderTransfer();
    if (step === 6) renderPreview();
    bind();
  }

  function renderBasics() {
    card.innerHTML = `${stepHead(1, "◇", "Кого вы передаёте?", "Укажите город, налоговый режим и направления клиента. Приватные данные нужны только для проверки.")}
      <div class="fields">
        <label class="field wide inn-field"><span>ИНН клиента</span><input type="text" name="client_inn" value="${esc(data.client_inn)}" inputmode="numeric" maxlength="12" placeholder="10 или 12 цифр"><small>Нужен для проверки карточки и защиты от дублей. Покупатели и каталог ИНН не увидят.</small></label>
        ${field("Название клиента", "client_legal_name", "text", false, 'placeholder="Не показывается в каталоге"')}
        <label class="field city-picker"><span>Город *</span><input name="city" value="${esc(data.city || "")}" placeholder="Начните вводить город" autocomplete="off" aria-autocomplete="list"><div class="city-suggestions" role="listbox"></div></label>
        ${selectField("Система налогообложения *", "tax_system_id", "tax_system")}
        <label class="field"><span>НДС</span><select name="has_vat"><option value="false" ${!data.has_vat ? "selected" : ""}>Нет</option><option value="true" ${data.has_vat ? "selected" : ""}>Да</option></select></label>
        <div class="field wide industry-field"><span>Направления *</span>${multiSearchSelect("industry", "industry_ids", "Найти направление")}</div>
      </div>`;
    bindCityPicker();
  }

  function renderBusinessSize() {
    card.innerHTML = `${stepHead(2, "₽", "Размер бизнеса", "Разложите объём обслуживания по понятным параметрам: оборот, команда, банки и операции.")}
      <div class="fields business-size-fields">
        <section class="number-panel third"><i>🏦</i>${field("Количество банков", "banks_count", "number", false, 'min="0"')}</section>
        <section class="number-panel third"><i>⇄</i>${field("Операций в месяц", "operations_per_month", "number", false, 'min="0"')}</section>
        <section class="number-panel third fee-panel"><i>₽</i>${field("Текущая стоимость обслуживания, ₽ / мес.", "current_monthly_fee", "number", false, 'min="0" step="100"')}</section>
        <section class="choice-section half slim-choice"><header><i>₽</i><div><b>Размер бизнеса *</b><small>Диапазон годовой выручки</small></div></header>${choices("revenue_range", "revenue_range_id", false, { four: true })}</section>
        <section class="choice-section half slim-choice"><header><i>👥</i><div><b>Количество сотрудников *</b><small>Команда клиента, которую нужно учитывать</small></div></header>${choices("employee_range", "employee_range_id", false, { compact: true })}</section>
      </div>`;
  }

  function renderFeatures() {
    card.innerHTML = `${stepHead(3, "▦", "Особенности клиента", "Выберите площадки, ЭДО, программы и состояние учёта.")}
      <div class="fields feature-fields">
        <section class="choice-section half slim-choice brand-choice"><header><i>🛍</i><div><b>Маркетплейсы</b><small>Можно выбрать несколько</small></div></header>${choices("marketplace", "marketplace_ids", true, { withIcons: true, compact: true })}</section>
        <section class="choice-section half slim-choice brand-choice"><header><i>⇄</i><div><b>Операторы ЭДО</b><small>Можно выбрать несколько</small></div></header>${choices("edo_provider", "edo_provider_ids", true, { withIcons: true, compact: true })}</section>
        <div class="field wide"><span>Есть внешнеэкономическая деятельность?</span><div class="boolean-grid"><label class="choice-card"><input type="radio" name="foreign_trade" value="true" ${data.foreign_trade ? "checked" : ""}><span><b>Да, есть ВЭД</b></span></label><label class="choice-card"><input type="radio" name="foreign_trade" value="false" ${!data.foreign_trade ? "checked" : ""}><span><b>Нет</b></span></label></div></div>
        <section class="choice-section wide slim-choice brand-choice program-choice"><header><i>▦</i><div><b>Бухгалтерские программы</b><small>Можно выбрать несколько</small></div></header>${choices("accounting_program", "accounting_program_ids", true, { withIcons: true, compact: true })}</section>
        ${selectField("Состояние учёта *", "accounting_state_id", "accounting_state", true)}
      </div>`;
  }

  function renderReason() {
    card.innerHTML = `${stepHead(4, "!", "Почему вы передаёте клиента?", "Причина будет заметно показана покупателю перед ключевыми показателями.")}
      <section class="choice-section wide slim-choice">${choices("transfer_reason", "transfer_reason_ids", true, { four: true })}</section>
      <label class="field wide reason-comment"><span>Дополнительный комментарий</span><textarea name="transfer_reason_comment" maxlength="2000" placeholder="Необязательно. Уточните причину, не раскрывая клиента.">${esc(data.transfer_reason_comment)}</textarea></label>`;
  }

  function renderTransfer() {
    const selected = find("transfer_type", data.transfer_type_id);
    card.innerHTML = `${stepHead(5, "₽", "Условия передачи", "Выберите модель передачи и укажите желаемые условия.")}
      <section class="choice-section wide">${choices("transfer_type", "transfer_type_id", false, { withIcons: true, compact: true, fallbackIcon: "₽" })}</section>
      <div class="transfer-fields">
        ${selected?.code === "fixed" ? field("Цена передачи, ₽", "transfer_price", "number", true, 'min="0" step="1000"') : ""}
        ${["monthly_commission", "term_commission"].includes(selected?.code) ? field("Комиссия, %", "monthly_commission_percent", "number", false, 'min="0" max="100" step="0.1"') : ""}
        ${selected?.code === "term_commission" ? field("Срок комиссии, месяцев", "commission_months", "number", false, 'min="1"') : ""}
        <label class="switch"><input type="checkbox" name="bargain_allowed" ${data.bargain_allowed ? "checked" : ""}><i></i><span>Возможен торг</span></label>
      </div>
      <div class="fields transfer-extra">${field("Желаемая дата передачи", "desired_transfer_date", "date")}${field("Клиент обслуживается с", "client_since", "date")}<label class="field wide"><span>Дополнительная информация</span><textarea name="comment" maxlength="5000" placeholder="Особенности обслуживания без персональных данных">${esc(data.comment)}</textarea></label></div>`;
  }

  function renderPreview() {
    const industries = data.industry_ids.map((industryID) => find("industry", industryID)).filter(Boolean);
    const reasons = (data.transfer_reason_ids || []).map((reasonID) => find("transfer_reason", reasonID)).filter(Boolean);
    const transfer = find("transfer_type", data.transfer_type_id);
    card.innerHTML = `${stepHead(6, "✓", "Проверьте объявление", "Так карточка будет выглядеть для пользователей каталога.")}
      <article class="preview-card">
        <header><div><small>${esc(data.city || "Город не указан")}</small><h3>${esc(industries[0]?.name || "Клиент")}</h3></div><b>${esc(find("tax_system", data.tax_system_id)?.name || "—")} · ${data.has_vat ? "НДС" : "Без НДС"}</b></header>
        <div class="preview-tags">${industries.map((item) => `<span>${iconFor(item)} ${esc(item.name)}</span>`).join("")}</div>
        <section class="reason-big"><h3>ПРИЧИНА ПЕРЕДАЧИ</h3><p><b>${esc(reasons.map((item) => item.name).join(", ") || "—")}</b>${data.transfer_reason_comment ? "<br>" + esc(data.transfer_reason_comment) : ""}</p></section>
        <div class="key-grid">${previewMetric("Выручка", find("revenue_range", data.revenue_range_id)?.name)}${previewMetric("Сотрудники", find("employee_range", data.employee_range_id)?.name)}${previewMetric("Абонплата", money(data.current_monthly_fee))}${previewMetric("Операций", data.operations_per_month)}${previewMetric("Банков", data.banks_count)}${previewMetric("Состояние", find("accounting_state", data.accounting_state_id)?.name)}</div>
        <footer><div><small>УСЛОВИЯ ПЕРЕДАЧИ</small><h3>${esc(transferLabel(transfer))}</h3></div></footer>
      </article>
      <div class="privacy-note">✓ ИНН, юридическое название и контакты клиента скрыты от публичного просмотра.</div>`;
  }

  function previewMetric(label, value) {
    return `<div><small>${esc(label)}</small><b>${esc(value ?? "—")}</b></div>`;
  }

  function find(kind, itemID) {
    return dictionary(kind).find((item) => item.id === Number(itemID));
  }

  function transferLabel(item) {
    if (item?.code === "fixed") return money(data.transfer_price);
    if (item?.code === "monthly_commission") return (data.monthly_commission_percent || 0) + "% ежемесячно";
    if (item?.code === "term_commission") return (data.monthly_commission_percent || 0) + "% · " + (data.commission_months || 0) + " мес.";
    return item?.name || "—";
  }

  function bind() {
    bindMultiSearch();
    card.querySelectorAll("input,select,textarea").forEach((input) => {
      if (!input.name) return;
      if (input.closest("[data-multi-search]")) return;
      input.onchange = () => {
        capture();
        if (step === 5 && input.name === "transfer_type_id") render();
        else renderStepTrack();
      };
      if (input.tagName === "TEXTAREA" || input.type === "text" || input.type === "number" || input.type === "date") {
        input.oninput = () => {
          capture();
          renderStepTrack();
        };
      }
    });
  }

  function bindMultiSearch() {
    card.querySelectorAll("[data-multi-search]").forEach((root) => {
      const input = root.querySelector(".multi-search-input");
      const options = root.querySelector(".multi-search-options");
      const filter = () => {
        const query = input.value.trim().toLocaleLowerCase("ru");
        root.querySelectorAll(".multi-search-options label").forEach((label) => {
          label.hidden = query && !label.dataset.optionText.includes(query);
        });
      };
      const open = () => root.classList.add("open");
      input.addEventListener("focus", open);
      input.addEventListener("input", () => { open(); filter(); });
      input.addEventListener("keydown", (event) => {
        if (event.key === "Escape") root.classList.remove("open");
      });
      options.querySelectorAll("input").forEach((checkbox) => {
        checkbox.addEventListener("change", () => {
          capture();
          syncMultiSearchTags(root);
          renderStepTrack();
        });
      });
      root.querySelectorAll("[data-remove-value]").forEach((button) => {
        button.addEventListener("click", () => {
          const checkbox = root.querySelector(`input[value="${button.dataset.removeValue}"]`);
          if (checkbox) checkbox.checked = false;
          capture();
          syncMultiSearchTags(root);
          renderStepTrack();
        });
      });
      document.addEventListener("click", (event) => {
        if (!root.contains(event.target)) root.classList.remove("open");
      });
    });
  }

  function syncMultiSearchTags(root) {
    const name = root.dataset.multiSearch;
    const kind = root.dataset.kind;
    const selected = new Set((data[name] || []).map(Number));
    const selectedItems = dictionary(kind).filter((item) => selected.has(item.id));
    root.querySelector(".multi-search-tags").innerHTML = selectedItems.map((item) => `<button type="button" class="multi-search-tag" data-remove-value="${item.id}">${esc(item.name)} <span>×</span></button>`).join("") || `<span class="multi-search-placeholder">Выберите направления</span>`;
    root.querySelectorAll("[data-remove-value]").forEach((button) => {
      button.addEventListener("click", () => {
        const checkbox = root.querySelector(`input[value="${button.dataset.removeValue}"]`);
        if (checkbox) checkbox.checked = false;
        capture();
        syncMultiSearchTags(root);
        renderStepTrack();
      });
    });
  }

  function bindCityPicker() {
    const input = card.querySelector("[name=city]");
    const box = card.querySelector(".city-suggestions");
    if (!input || !box) return;
    const close = () => box.classList.remove("open");
    const loadCities = () => {
      clearTimeout(cityTimer);
      cityTimer = setTimeout(async () => {
        const current = ++cityRequest;
        const query = input.value.trim();
        box.innerHTML = "<em>Ищем города...</em>";
        box.classList.add("open");
        try {
          const cities = await api("/api/public/cities?country=RU&q=" + encodeURIComponent(query));
          if (current !== cityRequest) return;
          box.innerHTML = cities.length ? cities.map((city) => `<button type="button" data-city="${esc(city.name)}" data-region="${esc(city.region || "")}"><b>${esc(city.name)}</b>${city.region ? `<small>${esc(city.region)}</small>` : ""}</button>`).join("") : "<em>Города не найдены</em>";
          box.querySelectorAll("button").forEach((button) => button.onclick = () => {
            input.value = button.dataset.city;
            data.city = button.dataset.city;
            data.region = button.dataset.region || button.dataset.city;
            close();
            renderStepTrack();
          });
        } catch {
          if (current === cityRequest) box.innerHTML = "<em>Не удалось загрузить города</em>";
        }
      }, 180);
    };
    input.addEventListener("focus", loadCities);
    input.addEventListener("input", () => {
      data.city = input.value;
      data.region = "";
      loadCities();
    });
    input.addEventListener("keydown", (event) => { if (event.key === "Escape") close(); });
    document.addEventListener("click", (event) => {
      if (!event.target.closest(".city-picker")) close();
    });
  }

  function capture() {
    ["industry_ids", "marketplace_ids", "edo_provider_ids", "accounting_program_ids", "transfer_reason_ids"].forEach((name) => {
      if (card.querySelector(`[name="${name}"]`)) data[name] = [];
    });
    card.querySelectorAll("input,select,textarea").forEach((input) => {
      if (!input.name) return;
      if (input.type === "radio" && !input.checked) return;
      if (input.type === "checkbox" && !["bargain_allowed"].includes(input.name) && !input.checked) return;
      if (["industry_ids", "marketplace_ids", "edo_provider_ids", "accounting_program_ids", "transfer_reason_ids"].includes(input.name)) {
        data[input.name] = [...card.querySelectorAll(`[name="${input.name}"]:checked`)].map((item) => Number(item.value));
        if (input.name === "industry_ids") data.industry_id = data.industry_ids[0] || null;
        if (input.name === "transfer_reason_ids") data.transfer_reason_id = data.transfer_reason_ids[0] || null;
        return;
      }
      if (["employee_range_id", "tax_system_id", "revenue_range_id", "accounting_state_id", "transfer_reason_id", "transfer_type_id"].includes(input.name)) {
        data[input.name] = input.value ? Number(input.value) : null;
      } else if (["current_monthly_fee", "operations_per_month", "banks_count", "transfer_price", "monthly_commission_percent", "commission_months"].includes(input.name)) {
        data[input.name] = input.value === "" ? null : Number(input.value);
      } else if (["has_vat", "foreign_trade"].includes(input.name)) {
        data[input.name] = input.value === "true";
      } else if (input.name === "bargain_allowed") {
        data[input.name] = input.checked;
      } else {
        data[input.name] = input.value || null;
      }
    });
  }

  function validate() {
    const missing = (condition, message) => {
      if (!condition) {
        toast(message, true);
        return false;
      }
      return true;
    };
    if (step === 1) return missing(data.industry_ids.length && data.tax_system_id && String(data.city || "").trim(), "Выберите направление, город и налоговый режим");
    if (step === 2) return missing(data.revenue_range_id && data.employee_range_id, "Выберите размер бизнеса и количество сотрудников");
    if (step === 3) return missing(data.accounting_state_id, "Выберите состояние учёта");
    if (step === 4) return missing(data.transfer_reason_ids.length, "Выберите причину передачи");
    if (step === 5) {
      const transfer = find("transfer_type", data.transfer_type_id);
      if (!missing(transfer, "Выберите условия передачи")) return false;
      if (transfer.code === "fixed" && !missing(data.transfer_price != null, "Укажите цену передачи")) return false;
      if (["monthly_commission", "term_commission"].includes(transfer.code) && !missing(data.monthly_commission_percent != null, "Укажите процент комиссии")) return false;
      if (transfer.code === "term_commission" && !missing(data.commission_months, "Укажите срок комиссии")) return false;
    }
    return true;
  }

  async function save(silent = false) {
    if (saving) return false;
    saving = true;
    capture();
    data.current_step = step;
    data.industry_id = data.industry_ids[0] || null;
    data.transfer_reason_id = data.transfer_reason_ids[0] || null;
    try {
      const saved = await api(id ? `${API}/listings/${id}` : API + "/listings", {
        method: id ? "PUT" : "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(data),
      });
      id = saved.id;
      history.replaceState(null, "", `?id=${id}`);
      if (!silent) toast("Черновик сохранён");
      return true;
    } catch (error) {
      toast(error.message, true);
      return false;
    } finally {
      saving = false;
    }
  }

  next.onclick = async () => {
    capture();
    if (!validate()) return;
    if (step < 6) {
      if (await save(true)) {
        step++;
        render();
        scrollTo({ top: 0, behavior: "smooth" });
      }
      return;
    }
    if (!await save(true)) return;
    next.disabled = true;
    try {
      await api(`${API}/listings/${id}/publish`, { method: "POST" });
      document.querySelector(".wizard-progress-head").remove();
      document.querySelector(".wizard-actions").remove();
      card.innerHTML = '<div class="success-screen"><div class="success-mark">✓</div><h2>Объявление опубликовано!</h2><p>Карточка уже появилась в каталоге Клиентской биржи.</p><div class="success-actions"><a class="ce-secondary" href="/profile?section=client-exchange">Мои объявления</a><a class="ce-primary" href="/client-exchange">Открыть каталог</a></div></div>';
    } catch (error) {
      toast(error.message, true);
      next.disabled = false;
    }
  };

  back.onclick = () => {
    capture();
    if (step > 1) {
      step--;
      render();
    }
  };
  draft.onclick = async () => { await save(); };

  function fromListing(listing) {
    const industryIDs = (listing.industries?.length ? listing.industries : listing.industry ? [listing.industry] : []).map((item) => item.id).filter(Boolean);
    const transferReasonIDs = (listing.transfer_reasons?.length ? listing.transfer_reasons : listing.transfer_reason ? [listing.transfer_reason] : []).map((item) => item.id).filter(Boolean);
    return {
      ...data,
      title: listing.title || "",
      industry_id: industryIDs[0] || listing.industry?.id || null,
      industry_ids: industryIDs.length ? industryIDs : (listing.industry?.id ? [listing.industry.id] : []),
      employee_range_id: listing.employees?.id || null,
      tax_system_id: listing.tax_system?.id || null,
      revenue_range_id: listing.revenue?.id || null,
      accounting_state_id: listing.accounting_state?.id || null,
      transfer_reason_id: transferReasonIDs[0] || listing.transfer_reason?.id || null,
      transfer_reason_ids: transferReasonIDs.length ? transferReasonIDs : (listing.transfer_reason?.id ? [listing.transfer_reason.id] : []),
      transfer_type_id: listing.transfer_type?.id || null,
      transfer_reason_comment: listing.transfer_reason_comment || "",
      transfer_price: listing.transfer_price,
      monthly_commission_percent: listing.monthly_commission_percent,
      commission_months: listing.commission_months,
      current_monthly_fee: listing.current_monthly_fee,
      operations_per_month: listing.operations_per_month,
      banks_count: listing.banks_count,
      has_vat: listing.has_vat,
      foreign_trade: listing.foreign_trade,
      bargain_allowed: listing.bargain_allowed,
      region: listing.region || "",
      city: listing.city || "",
      client_since: listing.client_since,
      desired_transfer_date: listing.desired_transfer_date,
      comment: listing.comment || "",
      current_step: listing.current_step || 1,
      marketplace_ids: (listing.marketplaces || []).map((item) => item.id),
      edo_provider_ids: (listing.edo_providers || []).map((item) => item.id),
      accounting_program_ids: (listing.accounting_programs || []).map((item) => item.id),
      client_inn: listing.private?.client_inn || "",
      client_legal_name: listing.private?.client_legal_name || "",
    };
  }

  (async () => {
    try {
      meta = (await api(API + "/meta")).dictionaries;
      if (id) {
        const listing = await api(`${API}/listings/${id}`);
        if (!listing.is_owner) throw Error("Редактирование доступно только владельцу");
        data = fromListing(listing);
        step = Math.min(6, Math.max(1, data.current_step || 1));
      }
      render();
    } catch (error) {
      if (error.message.includes("авторизац")) location.href = "/login?next=" + encodeURIComponent(location.pathname + location.search);
      else {
        toast(error.message, true);
        setTimeout(() => location.href = "/client-exchange", 1400);
      }
    }
  })();
})();
