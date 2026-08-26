(() => {
  const nav = document.querySelector(".sidebar nav");
  if (!nav) return;

  document.head.insertAdjacentHTML("beforeend", '<link rel="stylesheet" href="/static/admin-client-exchange.css?v=1">');
  nav.insertAdjacentHTML("beforeend", '<small>КЛИЕНТСКАЯ БИРЖА</small><button id="client-exchange-nav">◇ <span>Справочники биржи</span></button>');

  const workspace = document.querySelector(".workspace");
  workspace.insertAdjacentHTML("beforeend", '<section id="client-exchange-admin" class="ce-admin-section"><div class="ce-admin-head"><div><h2>Клиентская биржа → Справочники</h2><p>Независимые значения используются только внутри модуля.</p></div><button class="primary" id="ce-admin-new">＋ Добавить значение</button></div><div class="ce-admin-kinds"></div><div class="ce-admin-table"></div></section>');

  const section = document.querySelector("#client-exchange-admin");
  const kinds = {
    employee_range: "Количество сотрудников",
    industry: "Направления",
    marketplace: "Маркетплейсы",
    accounting_state: "Состояние учёта",
    transfer_reason: "Причины передачи",
    edo_provider: "ЭДО",
    transfer_type: "Условия передачи",
    tax_system: "Системы налогообложения",
    revenue_range: "Размер бизнеса",
    accounting_program: "Учётные программы",
  };
  let current = "employee_range";
  let items = [];

  const esc = (value) => String(value ?? "").replace(/[&<>"']/g, (char) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[char]);

  async function request(url, options = {}) {
    const response = await fetch(url, { cache: "no-store", ...options });
    let data = {};
    try { data = await response.json(); } catch {}
    if (!response.ok) throw Error(data.error || "Не удалось выполнить запрос");
    return data;
  }

  function activate() {
    document.querySelectorAll(".workspace>section,.dictionary-list,.dictionary-editor").forEach((node) => node.classList.add("hidden"));
    section.classList.remove("hidden");
    section.classList.add("active");
    document.querySelectorAll(".sidebar nav button").forEach((button) => button.classList.remove("active"));
    document.querySelector("#client-exchange-nav").classList.add("active");
    document.querySelector(".workspace>header h1").textContent = "Клиентская биржа";
    document.querySelector(".workspace>header p").textContent = "Управление независимыми справочниками модуля";
    document.querySelectorAll(".workspace>header .primary").forEach((button) => button.classList.add("hidden"));
    load();
  }

  async function load() {
    const data = await request("/api/admin/client-exchange/dictionaries?kind=" + current);
    items = data.items;
    section.querySelector(".ce-admin-kinds").innerHTML = Object.entries(kinds).map(([key, label]) => `<button class="${key === current ? "active" : ""}" data-kind="${key}">${label}</button>`).join("");
    section.querySelector(".ce-admin-table").innerHTML = `<table><thead><tr><th>Порядок</th><th>Название</th><th>Иконка</th><th>Code</th><th>Статус</th><th>Использование</th><th></th></tr></thead><tbody>${items.map((item) => `<tr class="${item.active ? "" : "inactive"}"><td>${item.sort_order}</td><td><b>${esc(item.name)}</b><small>${esc(item.description)}</small></td><td>${esc(item.icon || "")}</td><td><code>${esc(item.code)}</code></td><td>${item.active ? "Активно" : "Отключено"}</td><td>${item.used ? "Используется" : "Свободно"}</td><td><button data-edit="${item.id}">Изменить</button> <button data-delete="${item.id}">Удалить</button></td></tr>`).join("")}</tbody></table>`;
    section.querySelectorAll("[data-kind]").forEach((button) => button.onclick = () => { current = button.dataset.kind; load(); });
    section.querySelectorAll("[data-edit]").forEach((button) => button.onclick = () => edit(items.find((item) => item.id === Number(button.dataset.edit))));
    section.querySelectorAll("[data-delete]").forEach((button) => button.onclick = () => remove(Number(button.dataset.delete)));
  }

  function edit(item = { kind: current, code: "", name: "", description: "", color: "blue", icon: "", sort_order: items.length + 1, active: true }) {
    const modal = document.createElement("div");
    modal.className = "ce-admin-modal";
    modal.innerHTML = `<form>
      <h2>${item.id ? "Изменить значение" : "Новое значение"}</h2>
      <div class="grid"><label>Справочник<select name="kind">${Object.entries(kinds).map(([key, label]) => `<option value="${key}" ${item.kind === key ? "selected" : ""}>${label}</option>`).join("")}</select></label><label>Code<input name="code" pattern="[a-z][a-z0-9_]*" value="${esc(item.code)}" required></label></div>
      <label>Название<input name="name" maxlength="300" value="${esc(item.name)}" required></label>
      <label>Описание<textarea name="description">${esc(item.description)}</textarea></label>
      <div class="grid"><label>Минимум<input type="number" step="any" name="min" value="${item.min ?? ""}"></label><label>Максимум<input type="number" step="any" name="max" value="${item.max ?? ""}"></label><label>Цвет<input name="color" value="${esc(item.color || "blue")}"></label><label>Иконка<input name="icon" maxlength="500" value="${esc(item.icon || "")}" placeholder="₽, 👥 или /static/icon.svg"></label><label>Порядок<input type="number" name="sort_order" value="${item.sort_order || 0}"></label></div>
      <div class="grid"><label>Юридическое название<input name="legal_name" value="${esc(item.legal_name || "")}"></label><label>Код оператора<input name="operator_code" value="${esc(item.operator_code || "")}"></label></div>
      <label><span><input type="checkbox" name="active" ${item.active !== false ? "checked" : ""}> Активно</span></label>
      <footer><button type="button" data-cancel>Отмена</button><button class="primary">Сохранить</button></footer>
    </form>`;
    document.body.append(modal);
    modal.querySelector("[data-cancel]").onclick = () => modal.remove();
    modal.querySelector("form").onsubmit = async (event) => {
      event.preventDefault();
      const form = new FormData(event.currentTarget);
      const value = {
        kind: form.get("kind"),
        code: form.get("code").trim(),
        name: form.get("name").trim(),
        description: form.get("description").trim(),
        min: form.get("min") === "" ? null : Number(form.get("min")),
        max: form.get("max") === "" ? null : Number(form.get("max")),
        color: form.get("color").trim(),
        icon: form.get("icon").trim(),
        legal_name: form.get("legal_name").trim(),
        operator_code: form.get("operator_code").trim(),
        sort_order: Number(form.get("sort_order")) || 0,
        active: form.get("active") === "on",
      };
      try {
        await request(item.id ? "/api/admin/client-exchange/dictionaries/" + item.id : "/api/admin/client-exchange/dictionaries", {
          method: item.id ? "PUT" : "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(value),
        });
        current = value.kind;
        modal.remove();
        load();
      } catch (error) {
        alert(error.message);
      }
    };
  }

  async function remove(id) {
    if (!confirm("Удалить значение? Используемое значение будет только отключено.")) return;
    try {
      await request("/api/admin/client-exchange/dictionaries/" + id, { method: "DELETE" });
      load();
    } catch (error) {
      alert(error.message);
      load();
    }
  }

  document.querySelector("#client-exchange-nav").onclick = activate;
  document.querySelector("#ce-admin-new").onclick = () => edit();
  nav.querySelectorAll("button:not(#client-exchange-nav)").forEach((button) => button.addEventListener("click", () => section.classList.remove("active")));
})();
