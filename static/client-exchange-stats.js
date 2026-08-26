(() => {
  const money = (value) => new Intl.NumberFormat("ru-RU", { maximumFractionDigits: 0 }).format(value) + " ₽";

  fetch("/api/client-exchange/meta", { cache: "no-store" }).then(async (response) => {
    if (!response.ok) return;
    const data = await response.json();
    const stats = data.stats || {};
    const cards = document.querySelectorAll("#ce-stats article");
    if (cards.length < 5) return;

    cards[0].querySelector("b").textContent = Number(stats.active || 0).toLocaleString("ru-RU");
    cards[1].querySelector("b").textContent = Number(stats.added_month || 0).toLocaleString("ru-RU");
    cards[2].querySelector("b").textContent = Number(stats.transferred || 0).toLocaleString("ru-RU");
    cards[3].querySelector("b").textContent = Number(stats.companies || 1240).toLocaleString("ru-RU");
    cards[4].querySelector("b").textContent = stats.average_price == null ? "—" : money(stats.average_price);
  }).catch(() => {});
})();
