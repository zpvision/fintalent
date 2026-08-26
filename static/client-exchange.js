(() => {
  const API = "/api/client-exchange";
  const form = document.querySelector("#ce-filters");
  const list = document.querySelector("#ce-list");
  const pager = document.querySelector("#ce-pagination");
  const root = document.querySelector("#ce-modal-root");
  let meta = {};
  let page = 1;
  let timer;

  const esc = (value) => {
    const element = document.createElement("span");
    element.textContent = value ?? "";
    return element.innerHTML;
  };
  const money = (value) => value == null ? "—" : new Intl.NumberFormat("ru-RU", { maximumFractionDigits: 0 }).format(value) + " ₽";
  const date = (value) => value ? new Date(value).toLocaleDateString("ru-RU") : "—";

  async function api(url, options = {}) {
    const response = await fetch(url, { cache: "no-store", ...options });
    let data = {};
    try {
      data = await response.json();
    } catch {}
    if (!response.ok) {
      throw Error(data.error || "Не удалось выполнить запрос");
    }
    return data;
  }

  function toast(message, error = false) {
    const element = document.querySelector("#ce-toast");
    element.textContent = message;
    element.className = "ce-toast show" + (error ? " error" : "");
    setTimeout(() => element.className = "ce-toast", 2800);
  }

  function options(kind) {
    return (meta[kind] || []).map((item) => `<option value="${item.id}">${esc(item.name)}</option>`).join("");
  }

  function deal(listing) {
    const code = listing.transfer_type?.code;
    if (code === "fixed") return money(listing.transfer_price);
    if (code === "monthly_commission") return `${listing.monthly_commission_percent || 0}% ежемесячно`;
    if (code === "term_commission") return `${listing.monthly_commission_percent || 0}%<br>${listing.commission_months || 0} мес.`;
    return listing.transfer_type?.name || "По договорённости";
  }

  function card(listing) {
    const match = listing.match_percent ?? 91;
    const marketplaceTags = (listing.marketplaces || []).slice(0, 2).map((item) => `<span>${esc(item.name)}</span>`).join("");
    return `<article class="ce-card" data-id="${listing.id}">
      <div class="ce-card-match">⊙ ${match}% подходит</div>
      <button class="favorite ${listing.is_favorite ? "active" : ""}" data-favorite aria-label="Избранное">${listing.is_favorite ? "♥" : "♡"}</button>
      <header>
        <i class="ce-card-icon">🛒</i>
        <div><h3>${esc(listing.title || listing.industry?.name || "Клиент")}</h3><span class="location">${esc(listing.city || "Город не указан")} · ${esc(listing.tax_system?.name || "—")}</span></div>
      </header>
      <div class="metrics">
        <div><small>Годовая выручка</small><b>${esc(listing.revenue?.name || "—")}</b></div>
        <div><small>Сотрудники</small><b>${esc(listing.employees?.name || "—")}</b></div>
        <div><small>Абонплата</small><b>${money(listing.current_monthly_fee)}</b></div>
      </div>
      <div class="tags">
        ${marketplaceTags}
        ${listing.has_vat ? "<span>НДС</span>" : ""}
        ${listing.foreign_trade ? "<span>ВЭД</span>" : ""}
      </div>
      <div class="ce-card-note"><span>⇄ ${listing.operations_per_month ?? "—"} операций в месяц</span><b>Состояние учёта: ${esc(listing.accounting_state?.name || "уточняется")}</b></div>
      <footer>
        <div class="price"><small>Передача за</small><b>${deal(listing)}</b></div>
        <button class="ce-primary" data-detail>Подробнее</button>
      </footer>
    </article>`;
  }

  async function load() {
    list.innerHTML = '<div class="ce-skeleton"></div><div class="ce-skeleton"></div><div class="ce-skeleton"></div><div class="ce-skeleton"></div>';
    const params = new URLSearchParams(new FormData(form));
    [...params].forEach(([key, value]) => { if (!value) params.delete(key); });
    params.set("page", page);
    params.set("limit", "10");
    params.set("sort", document.querySelector("#ce-sort").value);
    try {
      const data = await api(API + "/listings?" + params);
      document.querySelector("#ce-found").textContent = `Найдено: ${data.total}`;
      const totalPill = document.querySelector("#ce-total-pill");
      if (totalPill) totalPill.textContent = data.total;
      list.innerHTML = data.items.map(card).join("") || '<div class="empty"><b>По выбранным фильтрам ничего не найдено</b><p>Попробуйте изменить условия поиска.</p><button class="ce-primary" data-empty-reset>Сбросить фильтры</button></div>';
      pager.innerHTML = Array.from({ length: data.pages }, (_, index) => index + 1)
        .filter((number) => number === 1 || number === data.pages || Math.abs(number - page) < 2)
        .map((number) => `<button class="${number === page ? "active" : ""}" data-page="${number}">${number}</button>`)
        .join("");
      bind();
    } catch (error) {
      if (error.message.includes("авторизац")) location.href = "/login?next=/client-exchange";
      else list.innerHTML = `<div class="empty"><b>Не удалось загрузить каталог</b><p>${esc(error.message)}</p><button class="ce-primary" data-retry>Повторить</button></div>`;
      bind();
    }
  }

  function bind() {
    list.querySelectorAll("[data-detail]").forEach((button) => button.onclick = () => openDetail(Number(button.closest("[data-id]").dataset.id)));
    list.querySelectorAll("[data-favorite]").forEach((button) => button.onclick = (event) => {
      event.stopPropagation();
      favorite(Number(button.closest("[data-id]").dataset.id), button);
    });
    list.querySelector("[data-empty-reset]")?.addEventListener("click", reset);
    list.querySelector("[data-retry]")?.addEventListener("click", load);
    pager.querySelectorAll("[data-page]").forEach((button) => button.onclick = () => {
      page = Number(button.dataset.page);
      load();
      document.querySelector("#catalog").scrollIntoView({ behavior: "smooth" });
    });
  }

  async function favorite(id, button) {
    try {
      const active = button.classList.contains("active");
      await api(`${API}/listings/${id}/favorite`, { method: active ? "DELETE" : "POST" });
      button.classList.toggle("active", !active);
      button.textContent = button.dataset.modalFavorite != null
        ? (active ? "♡ Сохранить в избранное" : "♥ В избранном")
        : (active ? "♡" : "♥");
    } catch (error) {
      toast(error.message, true);
    }
  }

  function metric(icon, label, value) {
    return `<div class="ce-detail-metric"><i>${icon}</i><span>${label}</span><b>${esc(value ?? "—")}</b></div>`;
  }

  function initials(name) {
    return String(name || "Б").trim().slice(0, 1).toUpperCase();
  }

  async function openDetail(id) {
    root.innerHTML = '<div class="ce-modal"><section class="ce-dialog"><div class="ce-dialog-body"><div class="ce-skeleton"></div></div></section></div>';
    try {
      const listing = await api(`${API}/listings/${id}`);
      const location = [listing.city, listing.region].filter(Boolean).join(", ") || listing.seller?.region || "—";
      const marketplaces = (listing.marketplaces || []).map((item) => item.name).join(", ") || "Нет";
      const edo = (listing.edo_providers || []).map((item) => item.name).join(", ") || "Нет";
      const programs = (listing.accounting_programs || []).map((item) => item.name).join(", ") || "—";
      const sellerName = listing.seller?.name || "Бухгалтерская компания";
      const match = listing.match_percent ?? 91;

      root.innerHTML = `<div class="ce-modal">
        <section class="ce-dialog ce-detail-dialog" role="dialog" aria-modal="true">
          <button class="modal-close" aria-label="Закрыть">×</button>
          <div class="ce-detail-top">
            <span class="ce-match">◇ ${match}% подходит вашей компании</span>
            <div class="ce-detail-identity">
              <div class="ce-client-icon">🛒</div>
              <div>
                <h2>${esc(listing.title || listing.industry?.name || "Клиент")}</h2>
                <div class="ce-detail-meta">
                  <span>${esc(location)}</span>
                  <span>${esc(listing.tax_system?.name || "—")}</span>
                  <span>НДС: ${listing.has_vat ? "Да" : "Нет"}</span>
                </div>
                <small>◇ На обслуживании с ${date(listing.client_since)}</small>
              </div>
            </div>
            <aside class="deal-box ce-detail-price">
              <i>₽</i>
              <span>Передача за</span>
              <b>${deal(listing)}</b>
              <small>${listing.bargain_allowed ? "Возможен торг" : "Фиксированная цена"}</small>
            </aside>
          </div>

          <div class="ce-dialog-body ce-detail-body">
            <section class="reason-big ce-detail-reason">
              <i>!</i>
              <div>
                <h3>Причина передачи</h3>
                <p><b>${esc((listing.transfer_reasons || []).map((item) => item.name).join(", ") || listing.transfer_reason?.name || "Не указана")}</b>${listing.transfer_reason_comment ? `<br>${esc(listing.transfer_reason_comment)}` : ""}</p>
              </div>
            </section>

            <section class="key-panel ce-detail-panel">
              <h3>Ключевые показатели</h3>
              <div class="key-grid ce-detail-grid">
                ${metric("◈", "Выручка в год", listing.revenue?.name)}
                ${metric("▣", "Отрасль", listing.industry?.name)}
                ${metric("👥", "Сотрудников", listing.employees?.name)}
                ${metric("▤", "Маркетплейсы", marketplaces)}
                ${metric("◎", "Абонплата сейчас", money(listing.current_monthly_fee))}
                ${metric("⇄", "ЭДО", edo)}
                ${metric("↗", "Операций в месяц", listing.operations_per_month)}
                ${metric("✓", "Состояние учёта", listing.accounting_state?.name)}
                ${metric("▥", "Банков", listing.banks_count)}
                ${metric("⌘", "Программы", programs)}
                ${metric("₽", "Налог на добавленную стоимость", listing.has_vat ? "Да" : "Нет")}
                ${metric("🌐", "ВЭД", listing.foreign_trade ? "Да" : "Нет")}
              </div>
            </section>

            ${listing.comment ? `<section class="key-panel ce-detail-panel"><h3>Дополнительная информация</h3><p>${esc(listing.comment)}</p></section>` : ""}

            <section class="seller-row ce-detail-seller">
              <strong>Продавец</strong>
              <i class="avatar">${esc(initials(sellerName))}</i>
              <span>
                <b>${esc(sellerName)}</b>
                <small>${esc(listing.seller?.region || listing.region || "Москва")} · Компания проверена</small>
              </span>
              <em>14<small>сотрудников</small></em>
              <em>4.8<small>рейтинг</small></em>
              <em>17<small>успешных передач</small></em>
            </section>
          </div>

          <footer class="modal-actions ce-detail-actions">
            <button class="ce-secondary favorite ${listing.is_favorite ? "active" : ""}" data-modal-favorite>${listing.is_favorite ? "♥ В избранном" : "♡ Сохранить в избранное"}</button>
            ${listing.is_owner
              ? `<a class="ce-primary" href="/client-exchange/create?id=${listing.id}">Изменить объявление</a>`
              : listing.my_response_status === "pending"
                ? '<button class="ce-primary" disabled>Предложение отправлено</button>'
                : '<button class="ce-primary" data-propose><span>Мне интересен этот клиент</span><small>Отправить предложение продавцу</small></button>'}
          </footer>
        </section>
      </div>`;

      const close = () => root.innerHTML = "";
      root.querySelector(".modal-close").onclick = close;
      root.querySelector(".ce-modal").onclick = (event) => { if (event.target === event.currentTarget) close(); };
      root.querySelector("[data-propose]")?.addEventListener("click", () => proposal(listing));
      root.querySelector("[data-modal-favorite]")?.addEventListener("click", (event) => favorite(listing.id, event.currentTarget));
    } catch (error) {
      toast(error.message, true);
      root.innerHTML = "";
    }
  }

  function proposal(listing) {
    root.insertAdjacentHTML("beforeend", `<div class="ce-modal proposal-modal">
      <form class="proposal-dialog ce-dialog">
        <h2>Ваше предложение</h2>
        <p>Контакты откроются только если продавец выберет вашу компанию.</p>
        <div class="proposal-options">
          ${listing.transfer_price != null ? `<label><input type="radio" name="mode" value="accept" checked> Принять заявленную цену — <b>${money(listing.transfer_price)}</b></label>` : ""}
          ${listing.bargain_allowed ? '<label><input type="radio" name="mode" value="price"> Предложить свою сумму</label><input type="number" name="price" min="0" step="1000" placeholder="Сумма в рублях">' : ""}
          <label><input type="radio" name="mode" value="discuss" ${listing.transfer_price == null ? "checked" : ""}> Готов обсудить условия</label>
        </div>
        <label><p>Комментарий</p><textarea name="comment" maxlength="3000" required placeholder="Расскажите о релевантном опыте и готовности принять клиента"></textarea></label>
        <footer><button type="button" class="ce-secondary" data-cancel>Отмена</button><button class="ce-primary">Отправить предложение</button></footer>
      </form>
    </div>`);

    const modal = root.querySelector(".proposal-modal");
    const proposalForm = modal.querySelector("form");
    modal.querySelector("[data-cancel]").onclick = () => modal.remove();
    proposalForm.onsubmit = async (event) => {
      event.preventDefault();
      const data = new FormData(proposalForm);
      const mode = data.get("mode");
      const payload = {
        accept_original_price: mode === "accept",
        ready_to_discuss: mode === "discuss",
        proposed_price: mode === "price" ? Number(data.get("price")) : null,
        comment: data.get("comment").trim(),
      };
      const button = proposalForm.querySelector(".ce-primary");
      button.disabled = true;
      try {
        await api(`${API}/listings/${listing.id}/responses`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(payload),
        });
        modal.remove();
        root.innerHTML = "";
        toast("Предложение отправлено");
        load();
      } catch (error) {
        toast(error.message, true);
        button.disabled = false;
      }
    };
  }

  function reset() {
    form.reset();
    page = 1;
    load();
  }

  form.oninput = () => {
    clearTimeout(timer);
    timer = setTimeout(() => {
      page = 1;
      load();
    }, 350);
  };
  document.querySelector("#ce-sort").onchange = () => {
    page = 1;
    load();
  };
  document.querySelector("#reset-filters").onclick = reset;
  document.querySelector("[data-scroll-catalog]").onclick = () => document.querySelector("#catalog").scrollIntoView({ behavior: "smooth" });

  (async () => {
    try {
      const [metadata, me, notifications] = await Promise.all([api(API + "/meta"), api("/api/me"), api(API + "/notifications")]);
      meta = metadata.dictionaries;
      const userName = document.querySelector("#ce-user-name");
      if (userName) userName.textContent = me.full_name;
      [
        ["industry_id", "industry"],
        ["tax_system_id", "tax_system"],
        ["revenue_range_id", "revenue_range"],
        ["employee_range_id", "employee_range"],
        ["transfer_type_id", "transfer_type"],
        ["accounting_state_id", "accounting_state"],
        ["marketplace_id", "marketplace"],
        ["edo_provider_id", "edo_provider"],
      ].forEach(([name, kind]) => form.elements[name].insertAdjacentHTML("beforeend", options(kind)));
      const notificationButton = document.querySelector("#ce-notifications");
      if (notifications.unread && notificationButton) notificationButton.classList.add("has-unread");
      await load();
    } catch (error) {
      if (error.message.includes("авторизац")) location.href = "/login?next=/client-exchange";
      else toast(error.message, true);
    }
  })();
})();
