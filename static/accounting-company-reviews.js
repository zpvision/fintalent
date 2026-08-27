(() => {
  if (location.pathname !== "/accounting-companies/view") return;

  const params = new URLSearchParams(location.search);
  const key = params.get("id") ? params.get("id") : "slug/" + encodeURIComponent(params.get("slug") || "");
  const esc = (value) => {
    const element = document.createElement("span");
    element.textContent = value ?? "";
    return element.innerHTML;
  };

  async function api(url, options = {}) {
    const response = await fetch(url, { cache: "no-store", headers: { "Content-Type": "application/json" }, ...options });
    const data = await response.json().catch(() => ({}));
    if (!response.ok) throw Error(data.error || "Ошибка запроса");
    return data;
  }

  async function mount(attempt = 0) {
    const shell = document.querySelector(".ac-profile-shell") || document.querySelector(".ac-company-shell");
    if (!shell) {
      if (attempt < 40) setTimeout(() => mount(attempt + 1), 120);
      return;
    }
    let company;
    try {
      company = (await api("/api/accounting-companies/" + key)).company;
    } catch {
      return;
    }
    const section = document.createElement("section");
    section.className = "ac-profile-panel ac-profile-review-cta";
    section.innerHTML = `<div>
      <h2>Работали с этой компанией?</h2>
      <p>Поделитесь опытом — отзыв появится после модерации.</p>
    </div><button class="ac-profile-secondary" id="ac-review-open" type="button">Оставить отзыв</button>`;
    shell.append(section);
    section.querySelector("#ac-review-open").onclick = () => open(company);
  }

  function open(company) {
    const modal = document.querySelector("#ac-contact-modal");
    modal.hidden = false;
    modal.innerHTML = `<section class="ac-modal-box">
      <form id="ac-review-form">
        <div class="ac-modal-head"><h2>Отзыв о «${esc(company.name)}»</h2><button type="button" class="ac-modal-close">×</button></div>
        <label class="ac-field" style="margin-top:18px"><span>Оценка</span><select name="rating" required><option value="5">5 — Отлично</option><option value="4">4 — Хорошо</option><option value="3">3 — Нормально</option><option value="2">2 — Есть проблемы</option><option value="1">1 — Плохо</option></select></label>
        <label class="ac-field" style="margin-top:12px"><span>Ваша компания (необязательно)</span><input name="author_company"></label>
        <label class="ac-field" style="margin-top:12px"><span>Отзыв</span><textarea name="text" minlength="10" required placeholder="Расскажите о качестве работы"></textarea></label>
        <button class="ac-button" style="width:100%;margin-top:14px">Отправить отзыв</button>
        <p id="ac-review-error" style="font-size:10px;color:#b83246"></p>
      </form>
    </section>`;
    modal.querySelector(".ac-modal-close").onclick = () => modal.hidden = true;
    modal.querySelector("form").onsubmit = async (event) => {
      event.preventDefault();
      const form = new FormData(event.target);
      const button = event.target.querySelector(".ac-button");
      button.disabled = true;
      try {
        const data = await api(`/api/accounting-companies/${company.id}/reviews`, {
          method: "POST",
          body: JSON.stringify({
            rating: +form.get("rating"),
            author_company: form.get("author_company"),
            text: form.get("text"),
          }),
        });
        event.target.innerHTML = `<div class="ac-success" style="padding:25px"><i>✓</i><h2>Спасибо за отзыв</h2><p>${esc(data.message)}</p><button type="button" class="ac-button">Закрыть</button></div>`;
        event.target.querySelector("button").onclick = () => modal.hidden = true;
      } catch (error) {
        document.querySelector("#ac-review-error").textContent = error.message;
        button.disabled = false;
      }
    };
  }

  mount();
})();
