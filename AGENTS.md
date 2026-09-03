# AGENTS.md — постоянная инструкция FinTalent

## Проект и стек

FinTalent — русскоязычная платформа для бухгалтеров и финансовых специалистов: вакансии/резюме, профессиональные тесты, тестирование сотрудников, публикации, маркетплейсы, клиентская биржа, каталог бухгалтерских компаний и взаимопомощь.

Репозиторий — каталог с `go.mod`, `main.go`, `static/` и этим файлом. Запускай команды именно отсюда: пути приложения относительные. Перед задачей также прочитай [docs/CODEX_CONTEXT.md](docs/CODEX_CONTEXT.md).

- Backend: Go 1.26, `net/http`, `http.DefaultServeMux`, `database/sql`.
- БД: PostgreSQL через `pgx/v5/stdlib`, ручной SQL, без ORM/migration CLI.
- Auth: DB-сессии, cookie `fintalent_session`, bcrypt; отдельная admin-сессия.
- Frontend: поэтапная гибридная миграция с legacy HTML/CSS/vanilla JS на React 19 + Vite + React Router; внешний вид и публичные URL сохраняются.
- Frontend-сборка: `npm run build` собирает Editor.js через esbuild и React в `static/react/`; `npm run dev:react` запускает Vite с proxy на Go.
- Тесты: `go test`; часть integration-тестов требует `DATABASE_URL`.

## Структура

- `main.go` — запуск, env, БД, users/sessions, auth/HTTP helpers, страницы и регистрация модулей.
- Корневые `*.go` — исторические модули `package main`: admin, резюме, survey, география, публикации, ПрофиМаркет и др.
- `internal/testmodule/` — domain/dto/validation/repository/service/handler тестов.
- `internal/vacancymodule/` — domain/dto/matching/repository/service/handler вакансий.
- `internal/clientexchange/`, `internal/accountingcompany/` — feature-пакеты; корневые `*_module.go` связывают их с user/admin resolvers.
- `migrations/` — идемпотентные SQL-схемы/demo seed; файл сам не исполняется.
- `static/` — legacy-страницы, CSS/JS, общие browser-компоненты, изображения и собранные frontend assets; `static/react/` генерируется Vite и не коммитится.
- `frontend/` — React/Vite-приложение и исходник Editor.js; `data/` — ОКВЭД; `docs/` — OpenAPI тестов, Codex-контекст и карта React-миграции.

## Архитектура

Это модульный монолит в переходном состоянии. Старый код использует функции и глобальный `db` в `package main`; новые части выделены в `internal/`. Не переносить старые модули попутно с feature-задачей.

Routes регистрируются на `http.DefaultServeMux`; internal-пакеты получают `*sql.DB` и resolvers через корневой adapter. HTML раздаёт `servePage`, данные приходят через JSON API. Одновременно существуют `/api`, `/api/v1`, `/api/public`, `/api/admin`; сохраняй соглашение модуля. Ошибка API обычно `{"error":"..."}`. Ownership, статусы, soft delete и переходы проверяются на backend. Связанные записи меняются транзакционно; SQL параметризован `$1...`, списки имеют стабильный `ORDER BY`.

## Backend

- Сначала найди аналог. Крупный новый домен обычно оформляй как `internal/<module>` (образцы: `testmodule`, `vacancymodule`) плюс тонкий корневой adapter; малое расширение сохраняет стиль текущего файла.
- Routes держи в `register<Module>Routes()` и подключай в `main()`. DB init подключай из `prepareDatabase()`.
- Переиспользуй `userFromRequest`, `isAdmin`/`requireAdmin`, `contextWithTimeout`, `servePage`, JSON helpers слоя и resolver adapters. Internal-пакет не импортирует `package main`.
- Ограничивай methods/размер body, нормализуй ввод, проверяй owner/admin и `RowsAffected`. Не выдавай внутреннюю DB error клиенту.
- Upload: проверяй размер, MIME по содержимому, размеры изображения, безопасное случайное имя; используй uploads-структуру модуля.
- Не вводи новый router, ORM, DI, auth или response envelope без отдельной задачи.

## Frontend

- Миграция идёт по маршрутам согласно `docs/REACT_MIGRATION.md`: не удаляй legacy HTML/CSS/JS, пока React-версия не перенесена и не проверена. Go `serveFrontendPage` автоматически возвращает legacy при отсутствии React build; `REACT_FRONTEND=false` принудительно включает legacy.
- Для React сохраняй существующие DOM-структуру и CSS-классы, подключай page CSS через `usePageStyles`, запросы делай через `src/api/client.js` с `credentials: "include"`, общие layout — через React-компоненты. Не меняй URL, auth-модель и дизайн.
- Для legacy страница остаётся связкой `static/<feature>.html/.css/.js`; сохраняй порядок зависимостей/cache-busting `?v=N` и переиспользуй `site-header`, `site-errors`, `fintalent-theme`, `layout-safety`, `searchable-select`, `geography`, `duty-picker`, `catalog`, profile sidebar/buttons/avatar и `*-components.js`.
- `site-errors.js` глобально оборачивает `fetch`; не создавай второй global patch. Ожидаемые validation errors можно показывать локально.
- Пользовательский текст в legacy вставляй через `textContent`/escape helper, не напрямую в `innerHTML`; в React используй обычный JSX, не `dangerouslySetInnerHTML` без обязательной санитизации.
- Не правь `static/vendor/publication-editor-editorjs.js`: меняй `frontend/...`, затем `npm run build`.
- После frontend-изменений запускай `npm run build`; крупные страницы визуально сравнивай со старой версией на 375, 768, 1024 и 1440 px.

## PostgreSQL и миграции

- Для локальной разработки, запуска приложения, тестов и любых проверок используй только удалённую облачную PostgreSQL, заданную через `DATABASE_URL` в локальном `.env`. Не запускай и не используй локальный PostgreSQL; приложение намеренно не имеет локального DB fallback.
- Строка подключения содержит секреты: не добавляй её в документацию, код, логи или Git. В документации указывай только имя переменной `DATABASE_URL`; `.env` должен оставаться локальным и игнорироваться Git.
- `prepareDatabase()` применяет схему при каждом старте; таблицы версий нет. SQL обязан быть повторяемым: `IF NOT EXISTS`, `ADD COLUMN IF NOT EXISTS`, безопасные `DO $$`, `ON CONFLICT`.
- Новый файл: очередной номер + имя. Номера `011`, `012`, `040`, `041` уже повторяются — проверяй полное имя.
- Миграция не запускается автоматически. Нужна цепочка: `//go:embed` → Exec в `prepare<Module>Database()` → вызов из `prepareDatabase()` по зависимостям.
- Ранние схемы частично продублированы inline DDL; `001...`/`002...` не проигрываются автоматически. Это особенность, не повод для рефакторинга.
- Не меняй применённую миграцию, если можно добавить forward-only. Destructive schema/data changes запрещены без явного указания и backup plan.
- Учитывай FK/`ON DELETE`; для бизнес-сущностей типичен soft delete/status. Demo seed идемпотентен и учитывает `SEED_DEMO_DATA=false`; география — `SYNC_GEOGRAPHY`.

## Общие механизмы — переиспользовать

- user/admin auth и sessions из `main.go`/`admin.go`;
- JSON/decode helpers текущего слоя, без дублей;
- `dictionaries`/`dictionary_items`, survey blocks/admin CRUD;
- ОКВЭД, geography, duties/duty picker;
- `internal/testmodule` и результаты тестов для вакансий/resume/компаний;
- catalog, header, error toast, searchable select;
- Editor.js и legacy↔Editor.js преобразование публикаций;
- adapters/resolvers между internal-пакетами и общими сессиями.

## Основные модули

- auth, профиль/avatar/settings; admin/пользователи;
- справочники, ОКВЭД, география, обязанности, survey;
- резюме: анкета, опыт, образование, языки, финансы, знания, публикация;
- вакансии: мастер, требования, обязанности, тесты, matching, публикация;
- тесты: создание/версии/вопросы, модерация, прохождение, результаты;
- employee testing по приглашениям; marketplace тестов и каталоги;
- ПрофиМаркет решений/регламентов;
- публикации, серии, реакции, сохранения, аналитика, модерация;
- клиентская биржа: объявления, отклики, избранное, статусы, уведомления;
- бухгалтерские компании: профиль, услуги/тарифы, отзывы, паспорт компетенций;
- взаимопомощь по резюме: темы, заявки, сообщения, отзывы (активная разработка).

## Новые модули

1. Найди аналог; определи entities, ownership/status lifecycle, API и связи с users/dictionaries/tests.
2. Выбери стиль: substantial domain — `internal/<module>` + adapter; маленькое расширение — текущий модуль.
3. Добавь идемпотентную миграцию и полную embed/prepare chain.
4. Реализуй server-side validation/auth, routes/pages и UI на общих компонентах.
5. Добавь unit-тесты доменной логики и уместные SQL integration-тесты.
6. Для крупного модуля/общего механизма обнови `docs/CODEX_CONTEXT.md`.

## CRUD-справочники

- Сначала проверь `dictionaries` + `dictionary_items` и существующий admin CRUD. Отдельная таблица нужна лишь для особых полей, связей/lifecycle/масштаба (ОКВЭД, duties, test categories, help topics).
- Контракт: admin GET/POST collection, PUT/DELETE item; public GET активных. Обязательны `requireAdmin`, trimming/limits, uniqueness, `sort_order,id`, понятные конфликты.
- Предпочитай deactivate/soft delete. Не меняй id/alias/code используемых значений без анализа ссылок.
- Переиспользуй admin tabs/forms/table/modal/compact-option, не делай отдельную мини-админку.

## Адаптивность

UI проверяй на ~360 px, 760–1100 px и desktop. Используй breakpoints модуля, `minmax(0,1fr)`, stacking/wrapping; не допускай horizontal overflow. На mobile формы/actions обычно одноколоночные/полноширинные; sticky, modal, table и длинные строки остаются в viewport. Не скрывай критическую функцию. Сохраняй viewport meta, labels, focus и удобные touch targets.

## Проверки

Из корня выполняй соразмерно изменению:

```bash
gofmt -w <изменённые .go>
go test ./...
go vet ./...
go build ./...
npm run build  # если менялся исходник/dependencies Editor.js
```

DB-тесты запускай на отдельной БД. Миграцию проверь на пустой и повторно на мигрированной БД. UI проверь вручную mobile/desktop. Невыполненную проверку и причину сообщай явно.

## Нельзя без отдельного указания

- Менять бизнес-правила, scoring/matching, статусы, права или публичный API вне задачи.
- Делать массовый refactor, переносить все модули в `internal`, менять router/ORM/framework/архитектуру.
- Удалять/обнулять БД, таблицы, migrations, uploads/данные; выполнять destructive migration.
- Менять auth/session/cookie/security headers/admin credentials; коммитить `.env`, секреты/персональные данные.
- Перезаписывать dirty worktree, вручную править generated/vendor bundle, форматировать несвязанные файлы.
- Добавлять CDN/сервис/dependency/telemetry без необходимости и согласования.
- Массово исправлять mojibake: терминал может неверно показывать корректный UTF-8. Сначала проверь кодировку.

## Правила для Codex

- Перед каждой задачей читай `AGENTS.md`, затем релевантный `docs/CODEX_CONTEXT.md`.
- Не исследуй весь проект заново, если достаточно связанных файлов.
- Сначала ищи аналог (`rg` по route/table/component/UI pattern).
- Не дублируй components, models, DTO, services, helpers, CRUD/справочники.
- Не переписывай рабочую архитектуру; `package main` + `internal/` — текущее состояние.
- Делай минимальные изменения, сохраняй чужой dirty worktree.
- После задачи проверяй build/tests; сообщай пропуски/причины.
- В финале перечисляй изменённые файлы и результаты проверок.
- После крупного модуля, существенной архитектурной правки или важного общего механизма кратко обновляй `docs/CODEX_CONTEXT.md`. Не заноси мелочи, не делай changelog и не пересказывай очевидный код — храни только полезное следующему агенту.
