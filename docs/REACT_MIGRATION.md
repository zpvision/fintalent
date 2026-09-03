# Миграция frontend FinTalent на React

## Цель и ограничения

Frontend переносится с HTML/CSS/vanilla JS на React поэтапно, без редизайна, смены публичных URL, бизнес-логики, cookie-сессий и структуры БД. Legacy-страница остаётся рабочей, пока соответствующий React-маршрут не перенесён и не проверен. Исходные файлы в `static/` на переходном этапе не удаляются.

## Аудит исходного состояния

На старте миграции в `static/` находятся 32 HTML-файла, 136 CSS-файлов и 66 JS-файлов. HTML занимает около 117 КБ, CSS — 701 КБ, JavaScript — 694 КБ. Основная интерактивность построена на прямых DOM-операциях: около 1510 вызовов `querySelector`, 497 операций `classList`, 464 присваивания `innerHTML`, 215 регистраций `addEventListener` и 66 прямых `fetch`.

Общие legacy-механизмы:

- `site-header.js` — общая публичная шапка и загрузка `/api/me`;
- `site-errors.js` — глобальный перехват ошибок `fetch`;
- `profile.js` — каркас личного кабинета;
- `catalog.js` — общий список вакансий и резюме;
- `duty-picker.js`, `searchable-select.js`, `geography.js` — повторно используемые элементы форм;
- `profimarket-components.js` — компоненты карточек ПрофиМаркета;
- Go-функция `servePage` — чтение HTML и внедрение общих CSS/JS.

Единственный inline script находится в `publication-view.html`: JSON-LD с серверными SEO-данными. Публичную страницу публикации нельзя переключать на чистый SPA до сохранения серверных title, description, canonical, Open Graph и structured data.

## Карта страниц

| Область | Публичные URL | Legacy HTML/JS | Целевой React-модуль | Статус |
|---|---|---|---|---|
| Главная | `/` | `index.html`, `auth.js`, `hero-*`, `home-showcase.js` | `pages/home`, `HomePage` | React |
| Авторизация | `/login`, `/register` | `login.*`, `register.*` | `pages/auth`, `AuthLayout` | React |
| Каталоги | `/vacancies`, `/resumes` | `vacancies.html`, `resumes.html`, `catalog.js` | `pages/catalog`, `CatalogPage` | React |
| Вакансии | `/vacancies/create`, `/vacancies/view` | `vacancy-create.*`, `vacancy-view.*` | `pages/vacancies`, `features/vacancies` | публичная карточка React; создание legacy |
| Резюме | `/resume/create`, `/resume/view/:id` | `resume-create.*`, `resume-view.*` | `pages/resumes`, `features/resumes` | публичная карточка React; создание legacy |
| Профиль | `/profile` | `profile.*` и profile feature scripts | `pages/profile`, `UserLayout` | legacy |
| Тесты | `/tests`, `/tests/create`, `/tests/take` | `tests.*`, `test-create.*`, `test-take.*` | `features/tests` | legacy |
| Тестирование сотрудников | `/employee-test` | `employee-test.*` | `features/employeeTesting` | legacy |
| Маркетплейс тестов | `/marketplace`, `/marketplace/create-test` | `marketplace.*`, `marketplace-create-test.*` | `pages/marketplace`, `features/marketplace` | каталог React; создание legacy |
| Клиентская биржа | `/client-exchange`, `/client-exchange/create` | `client-exchange.*` | `features/clientExchange` | legacy |
| Компании | `/accounting-companies`, `/accounting-companies/create`, `/accounting-companies/view`, `/accounting-companies/passport` | `accounting-company-*` | `pages/companies`, `features/companies` | каталог и публичная карточка React; создание/паспорт legacy |
| Публикации | `/publications*` | `publications.*`, editor/view/analytics | `pages/publications`, `features/publications` | список/сохранённые React; статья/editor/analytics legacy и SEO-sensitive |
| ПрофиМаркет | `/profimarket*` | `profimarket-*` | `pages/profimarket`, `features/profimarket` | каталог React; карточка/кабинет/редакторы legacy |
| Админка | `/admin*` | `admin.*` и admin feature scripts | `pages/admin`, `AdminLayout` | legacy |

## Целевая структура

```text
frontend/
  index.html
  vite.config.js
  src/
    api/
    components/
    context/
    features/
    hooks/
    layouts/
    pages/
    styles/
    utils/
```

Vite собирает приложение в `static/react/`. Go раздаёт hashed assets через существующий `/static/`, а для уже перенесённых URL возвращает React `index.html`. Если сборка отсутствует или `REACT_FRONTEND=false`, обработчик безопасно возвращает соответствующую legacy-страницу.

## Правила переходного периода

- React и Go работают на одном origin; cookie `fintalent_session` сохраняется, все API-запросы используют `credentials: "include"`.
- Общий `apiClient` отвечает за JSON, FormData, ошибки и статусы авторизации.
- Существующие CSS подключаются без переписывания до визуального подтверждения страницы.
- Ссылки на ещё не перенесённые страницы выполняют обычную навигацию, чтобы запрос обрабатывал Go.
- Старые HTML/CSS/JS удаляются только после переноса и визуальной проверки соответствующей страницы на 375, 768, 1024 и 1440 px.
- Публичные SEO-sensitive страницы переключаются только вместе с серверной выдачей метаданных или отдельным согласованным SSR/prerender-решением.

## Проверки этапа

```bash
npm run build
go test ./...
go vet ./...
go build ./...
```

Для разработки React используется `npm run dev:react`; Vite проксирует `/api`, `/static` и `/uploads` на Go по адресу `http://127.0.0.1:8080`. Переход на ещё не перенесённый маршрут в dev перенаправляет браузер на Go `:8080`, поэтому гибридные ссылки не попадают в SPA fallback Vite.
