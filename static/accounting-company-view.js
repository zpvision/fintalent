(() => {
  const root = document.querySelector("#ac-company-view");
  const modal = document.querySelector("#ac-contact-modal");
  const params = new URLSearchParams(location.search);
  const key = params.get("id") ? params.get("id") : "slug/" + encodeURIComponent(params.get("slug") || "");
  let company;
  let passport;

  const esc = (value) => {
    const element = document.createElement("span");
    element.textContent = value ?? "";
    return element.innerHTML;
  };
  const money = (value) => value == null ? "По запросу" : new Intl.NumberFormat("ru-RU").format(value) + " ₽";
  const priceType = {
    from_month: "от {p} / мес.",
    month: "{p} / мес.",
    from_hour: "от {p} / час",
    hour: "{p} / час",
    from_once: "от {p}",
    request: "По запросу",
  };
  const iconMap = {
    laptop: "▣",
    briefcase: "◆",
    globe: "◎",
    store: "▤",
    factory: "▥",
    video: "▶",
    basket: "WB",
    package: "OZ",
    building: "▦",
    users: "♟",
    home: "⌂",
    market: "▦",
    monitor: "▣",
    gears: "⚙",
  };

  async function api(url) {
    const response = await fetch(url, { cache: "no-store" });
    const data = await response.json().catch(() => ({}));
    if (!response.ok) throw Error(data.error || "Ошибка загрузки");
    return data;
  }

  function logo() {
    if (company.logo) return `<img src="${esc(company.logo)}" alt="">`;
    return esc((company.name || "Б").trim().charAt(0).toUpperCase());
  }

  function initials(name) {
    return esc(String(name || "Б").trim().charAt(0).toUpperCase());
  }

  function servicePrice(service) {
    return (priceType[service.price_type] || "{p}").replace("{p}", money(service.price_from));
  }

  function websiteHref(value) {
    if (!value) return "";
    return /^https?:\/\//i.test(value) ? value : "https://" + value;
  }

  function contactRow(kind, label, value, href = "") {
    if (!value) return "";
    const attrs = href ? `href="${esc(href)}" target="_blank" rel="noopener"` : "";
    return `<${href ? "a" : "div"} class="ac-profile-contact-row" ${attrs}>
      <i>${kind}</i>
      <span><b>${esc(value)}</b>${label ? `<small>${esc(label)}</small>` : ""}</span>
    </${href ? "a" : "div"}>`;
  }

  function socialLink(label, value, icon) {
    if (!value) return "";
    const href = label === "email" ? "mailto:" + value : websiteHref(value);
    return `<a class="ac-profile-social" href="${esc(href)}" target="_blank" rel="noopener">${icon}</a>`;
  }

  function heroImage() {
    const image = company.header_image || "/static/accounting-company-headers/header-01.jpg";
    return `<div class="ac-profile-visual" style="background-image:url('${esc(image)}')">
      <div>
        <h3>Порядок в учёте —<br>уверенность в бизнесе</h3>
        <p>Берём на себя рутину, чтобы вы могли развивать бизнес.</p>
      </div>
    </div>`;
  }

  function breadcrumbs() {
    return `<nav class="ac-profile-breadcrumbs">
      <a href="/">Главная</a><span>›</span>
      <a href="/accounting-companies">Компании</a><span>›</span>
      <span>Бухгалтерские компании</span><span>›</span>
      <b>${esc(company.name)}</b>
    </nav>`;
  }

  function leftCard() {
    const managerName = company.manager_name || "Руководитель";
    return `<aside class="ac-profile-brand-card">
      <div class="ac-profile-logo">${logo()}</div>
      <h2>${esc(company.name)}</h2>
      <p>${esc(company.short_description || "Бухгалтерские услуги")}</p>
      <div class="ac-profile-manager">
        ${company.manager_photo ? `<img src="${esc(company.manager_photo)}" alt="">` : `<i>${initials(managerName)}</i>`}
        <span>
          <b>${esc(managerName)}</b>
          <small>${esc(company.manager_position || "Основатель и главный бухгалтер")}</small>
          ${company.manager_description ? `<a href="#about">Подробнее о руководителе →</a>` : ""}
        </span>
      </div>
    </aside>`;
  }

  function mainHero() {
    const taxes = (company.tax_systems || []).map((item) => item.name).join(", ") || "Все системы налогообложения";
    return `<section class="ac-profile-intro">
      <div class="ac-profile-intro-copy">
        ${company.verified ? '<span class="ac-profile-verified">✓ Проверенная компания FinTalent</span>' : ""}
        <h1>${esc(company.name)} <button type="button" aria-label="Добавить в избранное">♡</button></h1>
        <p>${esc(company.short_description || company.full_description || "Комплексное бухгалтерское сопровождение бизнеса.")}</p>
        <div class="ac-profile-facts">
          <span>⌖ ${esc(company.city || "Онлайн")}</span>
          ${company.founded_year ? `<span>▣ Работаем с ${esc(company.founded_year)} года</span>` : ""}
          ${company.employee_count ? `<span>♟ ${esc(company.employee_count)} специалистов</span>` : ""}
          ${company.remote_all_russia ? "<span>◎ Онлайн по всей России</span>" : ""}
        </div>
        <div class="ac-profile-tags">
          ${(company.tax_systems || []).slice(0, 3).map((item) => `<span>${esc(item.name)}</span>`).join("") || `<span>${esc(taxes)}</span>`}
          ${company.remote_all_russia ? "<span>Онлайн</span>" : ""}
        </div>
        <div class="ac-profile-actions">
          <button class="ac-profile-primary" id="ac-contact" type="button">✈ Связаться с компанией</button>
          ${company.services?.length ? '<a class="ac-profile-secondary" href="#services">Смотреть услуги</a>' : ""}
          <button class="ac-profile-more" type="button" aria-label="Больше действий">…</button>
        </div>
      </div>
      ${heroImage()}
    </section>`;
  }

  function contactsCard() {
    const rows = [
      contactRow("☎", company.work_hours || "Пн-Пт 9:00-18:00", company.phone, company.phone ? "tel:" + company.phone : ""),
      contactRow("✉", "", company.email, company.email ? "mailto:" + company.email : ""),
      contactRow("◎", "", company.website, websiteHref(company.website)),
      contactRow("⌖", "", company.address || company.city),
    ].join("");
    return `<aside class="ac-profile-contacts">
      <h2>Контакты</h2>
      <div class="ac-profile-contact-list">${rows || '<p>Компания пока не добавила контакты.</p>'}</div>
      <div class="ac-profile-socials">
        ${socialLink("whatsapp", company.whatsapp, "WA")}
        ${socialLink("telegram", company.telegram, "TG")}
        ${socialLink("vk", company.vk, "VK")}
        ${socialLink("email", company.email, "✉")}
      </div>
    </aside>`;
  }

  function directions() {
    const items = (company.directions || []).slice(0, 8);
    if (!items.length) return "";
    return `<section class="ac-profile-section ac-profile-directions">
      <div class="ac-profile-section-head"><h2>Наши направления</h2>${company.directions.length > 8 ? '<button id="ac-all-directions">Смотреть все направления →</button>' : ""}</div>
      <div class="ac-profile-direction-list">
        ${(company.directions || []).map((item, index) => `<article ${index >= 8 ? "data-extra hidden" : ""}>
          <i>${esc(iconMap[item.icon] || "◇")}</i><b>${esc(item.name)}</b>
        </article>`).join("")}
      </div>
    </section>`;
  }

  function services() {
    const items = (company.services || []).slice(0, 7);
    return `<section class="ac-profile-panel" id="services">
      <div class="ac-profile-section-head"><h2>Услуги и цены</h2></div>
      <div class="ac-profile-service-list">
        ${items.length ? items.map((item) => `<div><span><i>${esc(iconMap[item.icon] || "▧")}</i>${esc(item.name)}</span><b>${esc(servicePrice(item))}</b></div>`).join("") : '<p class="ac-profile-muted">Услуги и цены уточняются.</p>'}
      </div>
      ${company.services?.length > 7 ? '<a class="ac-profile-link" href="#services">Смотреть все услуги →</a>' : ""}
    </section>`;
  }

  function tariffCard(tariffItem) {
    return `<article class="ac-profile-tariff ${tariffItem.popular ? "popular" : ""}">
      ${tariffItem.popular ? '<em>Популярный</em>' : ""}
      <h3>${esc(tariffItem.name || "Тариф")}</h3>
      <p>${esc(tariffItem.subtitle || "Для бизнеса")}</p>
      <strong>${tariffItem.price == null ? "По запросу" : money(tariffItem.price)} ${tariffItem.price == null ? "" : `<small>${esc(tariffItem.period || "")}</small>`}</strong>
      <ul>${(tariffItem.benefits || []).slice(0, 5).map((item) => `<li>${esc(item)}</li>`).join("")}</ul>
      <button type="button" data-calculate>Выбрать тариф</button>
    </article>`;
  }

  function tariffs() {
    const items = (company.tariffs || []).slice(0, 3);
    return `<section class="ac-profile-panel ac-profile-tariffs">
      <div class="ac-profile-section-head"><h2>Тарифы на бухгалтерское сопровождение</h2></div>
      <div class="ac-profile-tariff-grid">
        ${items.length ? items.map(tariffCard).join("") : '<p class="ac-profile-muted">Тарифы появятся после публикации компанией.</p>'}
      </div>
      <p>Точная стоимость зависит от системы налогообложения и количества операций.</p>
      <a class="ac-profile-link" href="#contacts">Получить индивидуальный расчёт →</a>
    </section>`;
  }

  function renderPassport() {
    if (!passport?.scores?.length) {
      return `<div class="ac-profile-passport-empty">
        <div class="ac-passport-empty-copy">
          <span>Паспорт компетенций</span>
          <b>Пока формируется</b>
          <p>После тестирования сотрудников здесь появится общий индекс и карта компетенций компании.</p>
        </div>
        <svg viewBox="0 0 340 260" role="img" aria-label="Пример графика компетенций">
          <g class="grid">
            <circle cx="170" cy="130" r="32"></circle>
            <circle cx="170" cy="130" r="64"></circle>
            <circle cx="170" cy="130" r="96"></circle>
            <path d="M170 34v192M74 130h192M102 62l136 136M238 62L102 198"></path>
          </g>
          <path class="area" d="M170 58 223 76 250 130 214 176 170 207 115 185 88 130 124 80Z"></path>
          <path class="line" d="M170 58 223 76 250 130 214 176 170 207 115 185 88 130 124 80Z"></path>
          <g class="points">
            <circle cx="170" cy="58" r="5"></circle><circle cx="223" cy="76" r="5"></circle><circle cx="250" cy="130" r="5"></circle><circle cx="214" cy="176" r="5"></circle>
            <circle cx="170" cy="207" r="5"></circle><circle cx="115" cy="185" r="5"></circle><circle cx="88" cy="130" r="5"></circle><circle cx="124" cy="80" r="5"></circle>
          </g>
        </svg>
      </div>`;
    }
    return `<div class="ac-profile-passport-ready">
      <div class="ac-profile-passport-index">
        <span>Общий индекс</span>
        <b>${Math.round(passport.overall_index)}%</b>
        <p>На основе независимого тестирования сотрудников на FinTalent</p>
      </div>
      <canvas class="ac-radar" id="ac-public-radar" width="620" height="620"></canvas>
      <div class="ac-profile-passport-legend"><span class="green">90% и выше</span><span class="orange">70-89%</span><span class="red">ниже 70%</span></div>
      <a href="/accounting-companies/passport?id=${company.id}">Подробнее →</a>
    </div>`;
  }

  function passportCard() {
    return `<aside class="ac-profile-panel ac-profile-passport">
      <div class="ac-profile-section-head"><h2>Паспорт компетенций</h2>${passport?.scores?.length ? '<a href="/accounting-companies/passport?id=' + company.id + '">Подробнее →</a>' : ""}</div>
      ${renderPassport()}
    </aside>`;
  }

  function reviews() {
    const items = (company.reviews || []).slice(0, 2);
    return `<section class="ac-profile-panel ac-profile-reviews">
      <div class="ac-profile-section-head"><h2>Отзывы клиентов</h2>${company.reviews?.length ? `<a href="#reviews">Смотреть все (${company.reviews.length}) →</a>` : ""}</div>
      ${items.length ? items.map((review) => `<article>
        <i>“</i>
        <p>${esc(review.text)}</p>
        <footer><b>${esc(review.author_name)}</b><span>${esc(review.author_company || "")}</span><small>${new Date(review.created_at).toLocaleDateString("ru-RU")}</small></footer>
      </article>`).join("") : '<p class="ac-profile-muted">Отзывы появятся после модерации.</p>'}
    </section>`;
  }

  function about() {
    const advantages = company.advantages || [];
    return `<section class="ac-profile-panel ac-profile-about" id="about">
      <h2>О компании</h2>
      <p>${esc(company.full_description || company.short_description || "Компания оказывает бухгалтерские услуги для бизнеса.")}</p>
      <div>
        ${advantages.length ? advantages.slice(0, 5).map((item) => `<span>✓ ${esc(item)}</span>`).join("") : ["Индивидуальный подход", "Всегда на связи", "Профессиональная ответственность", "Работаем с ЭДО и банками"].map((item) => `<span>✓ ${item}</span>`).join("")}
      </div>
    </section>`;
  }

  function render() {
    document.title = `${company.name} — FinTalent`;
    const color = company.accent?.color_value || "#6d35ff";
    root.innerHTML = `<div class="ac-profile-shell" style="--company-accent:${esc(color)}">
      ${breadcrumbs()}
      <section class="ac-profile-hero">
        ${leftCard()}
        ${mainHero()}
        ${contactsCard()}
      </section>
      ${directions()}
      <section class="ac-profile-main-grid">
        ${services()}
        ${tariffs()}
        ${passportCard()}
        ${reviews()}
        ${about()}
      </section>
    </div>`;

    document.querySelectorAll("[data-extra]").forEach((item) => item.style.display = "none");
    document.querySelector("#ac-all-directions")?.addEventListener("click", (event) => {
      document.querySelectorAll("[data-extra]").forEach((item) => item.style.display = "block");
      event.currentTarget.remove();
    });
    document.querySelector("#ac-contact")?.addEventListener("click", openContacts);
    document.querySelectorAll("[data-calculate]").forEach((button) => button.addEventListener("click", openContacts));
    if (passport?.scores?.length) drawRadar(document.querySelector("#ac-public-radar"), passport.scores.slice(0, 12), color);
  }

  function openContacts() {
    modal.hidden = false;
    modal.innerHTML = `<section class="ac-modal-box">
      <div class="ac-modal-head"><h2>Связаться с «${esc(company.name)}»</h2><button class="ac-modal-close" type="button">×</button></div>
      <p style="font-size:12px;color:#7180a4">Выберите удобный способ связи.</p>
      <div class="ac-profile-contact-list" style="margin-top:18px">
        ${contactRow("☎", "", company.phone, company.phone ? "tel:" + company.phone : "")}
        ${contactRow("✉", "", company.email, company.email ? "mailto:" + company.email : "")}
        ${contactRow("TG", "", company.telegram, websiteHref(company.telegram))}
        ${contactRow("WA", "", company.whatsapp, websiteHref(company.whatsapp))}
      </div>
    </section>`;
    modal.querySelector(".ac-modal-close").onclick = () => modal.hidden = true;
    modal.onclick = (event) => { if (event.target === modal) modal.hidden = true; };
  }

  function drawRadar(canvas, scores, color) {
    if (!canvas || scores.length < 3) return;
    const ctx = canvas.getContext("2d");
    const width = canvas.width;
    const height = canvas.height;
    const cx = width / 2;
    const cy = height / 2;
    const radius = Math.min(width, height) * 0.31;
    const count = scores.length;
    ctx.clearRect(0, 0, width, height);
    ctx.font = "600 17px Inter";
    ctx.textAlign = "center";
    ctx.textBaseline = "middle";
    for (let level = 1; level <= 5; level++) {
      ctx.beginPath();
      for (let index = 0; index < count; index++) {
        const angle = -Math.PI / 2 + index * 2 * Math.PI / count;
        const x = cx + Math.cos(angle) * radius * level / 5;
        const y = cy + Math.sin(angle) * radius * level / 5;
        index ? ctx.lineTo(x, y) : ctx.moveTo(x, y);
      }
      ctx.closePath();
      ctx.strokeStyle = "#e4e9f4";
      ctx.stroke();
    }
    scores.forEach((score, index) => {
      const angle = -Math.PI / 2 + index * 2 * Math.PI / count;
      ctx.beginPath();
      ctx.moveTo(cx, cy);
      ctx.lineTo(cx + Math.cos(angle) * radius, cy + Math.sin(angle) * radius);
      ctx.strokeStyle = "#edf1f7";
      ctx.stroke();
      const label = score.name.length > 16 ? score.name.slice(0, 15) + "…" : score.name;
      ctx.fillStyle = "#25306f";
      ctx.fillText(label, cx + Math.cos(angle) * (radius + 54), cy + Math.sin(angle) * (radius + 45));
    });
    ctx.beginPath();
    scores.forEach((score, index) => {
      const angle = -Math.PI / 2 + index * 2 * Math.PI / count;
      const r = radius * score.percent / 100;
      const x = cx + Math.cos(angle) * r;
      const y = cy + Math.sin(angle) * r;
      index ? ctx.lineTo(x, y) : ctx.moveTo(x, y);
    });
    ctx.closePath();
    ctx.fillStyle = color + "22";
    ctx.fill();
    ctx.strokeStyle = color;
    ctx.lineWidth = 4;
    ctx.stroke();
  }

  async function init() {
    try {
      const data = await api("/api/accounting-companies/" + key);
      company = data.company;
      passport = await api(`/api/accounting-companies/${company.id}/passport`).catch(() => ({ scores: [] }));
      render();
    } catch (error) {
      root.innerHTML = `<div class="ac-empty"><i>!</i><h3>Страница компании не найдена</h3><p>${esc(error.message)}</p><a class="ac-button" href="/accounting-companies">Вернуться в каталог</a></div>`;
    }
  }

  init();
})();
