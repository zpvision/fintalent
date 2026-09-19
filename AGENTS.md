# AGENTS.md — FinTalent

## Главное правило

Работай локально по задаче: сначала найди связанные файлы и существующий аналог, затем меняй только необходимое.

Не сканируй весь репозиторий, не читай все `docs/` и не запускай полный набор проверок без необходимости.

`docs/CODEX_CONTEXT.md`, `docs/REACT_MIGRATION.md` и другие документы читай только если они относятся к текущей задаче или без них не хватает контекста.

## Проект

FinTalent — русскоязычная платформа для бухгалтеров и финансовых специалистов: вакансии/профили, тесты, публикации, маркетплейсы, клиентская биржа, каталог бухгалтерских компаний и взаимопомощь.

Стек:
- Backend: Go 1.26, `net/http`, `http.DefaultServeMux`, `database/sql`.
- PostgreSQL: `pgx/v5/stdlib`, ручной SQL, без ORM.
- Auth: DB-сессии, cookie `fintalent_session`, bcrypt; admin-сессия отдельно.
- Frontend: React 19 + Vite + React Router и legacy HTML/CSS/vanilla JS в процессе миграции.
- React build: `npm run build` → `static/react/`; dev: `npm run dev:react`.
- Тесты: Go tests; часть integration-тестов требует `DATABASE_URL`.

Команды запускай из корня репозитория с `go.mod`, `main.go`, `static/`, `frontend/`.

## Структура

- `main.go` — запуск, env, БД, auth/helpers, страницы и регистрация модулей.
- Корневые `*.go` — исторические модули `package main`.
- `internal/<module>/` — новые/выделенные feature-модули. Ориентиры: `testmodule`, `vacancymodule`, `clientexchange`, `accountingcompany`.
- `migrations/` — идемпотентные SQL-схемы и seed.
- `frontend/` — React/Vite и исходники frontend.
- `static/` — legacy frontend и собранные assets; `static/react/` генерируется и не коммитится.
- `docs/` — дополнительный контекст; открывай только релевантные файлы.

Архитектура — модульный монолит в переходном состоянии. Не переносить legacy-код в `internal/` попутно с feature-задачей.

## Как работать с задачей

1. Найди точку входа через точечный `rg` по route, таблице, компоненту, тексту или похожей функции.
2. Посмотри связанные файлы и при необходимости 1–2 существующих аналога.
3. Сохрани стиль текущего модуля и существующие контракты.
4. Сделай минимальный diff без unrelated refactoring.
5. Проверь только затронутую область; расширяй проверки только если изменение действительно сквозное.
6. В финале кратко укажи изменённые файлы и выполненные/невыполненные проверки.

Если задача полностью понятна по найденному коду, не продолжай исследование проекта «на всякий случай».

Не пересказывай найденный код и архитектуру перед реализацией, если пользователь этого не просил.

## Backend

- Малое изменение делай в стиле текущего модуля.
- Новый существенный домен обычно: `internal/<module>` + тонкий adapter в `package main`.
- Routes регистрируй через существующий `http.DefaultServeMux`; сохраняй API namespace модуля (`/api`, `/api/v1`, `/api/public`, `/api/admin`).
- Переиспользуй существующие auth/session, JSON, timeout, page и resolver helpers. `internal/` не импортирует `package main`.
- Проверяй method, input limits, validation, owner/admin, status transitions и `RowsAffected`.
- SQL параметризованный; списки со стабильным `ORDER BY`; связанные записи при необходимости меняй транзакционно.
- Не возвращай клиенту внутренние DB errors.
- Не добавляй новый router, ORM, DI, auth-механизм или response envelope без отдельной задачи.
- Upload: лимит размера, MIME по содержимому, безопасное имя; используй существующий механизм модуля.

## Frontend

- Сохраняй публичные URL, auth-модель и текущий дизайн, если задача явно не требует редизайна.
- При React-миграции не удаляй legacy-версию, пока новая не перенесена и не проверена; `REACT_FRONTEND=false` сохраняет legacy fallback.
- Для React переиспользуй существующие компоненты и стили; page CSS подключай существующим способом, API — через `src/api/client.js` с `credentials: "include"`.
- Для legacy сохраняй существующие shared components/helpers и cache-busting подход.
- Не создавай второй global `fetch` patch.
- Не вставляй пользовательский текст через небезопасный `innerHTML`; в React не используй `dangerouslySetInnerHTML` без санитизации.
- Не правь generated/vendor bundle вручную; меняй исходник и пересобирай.
- Перед созданием нового UI-компонента сначала найди существующий похожий.
- Для адаптивности не допускай horizontal overflow; существенный UI проверяй на mobile/tablet/desktop.

## PostgreSQL и миграции

Для разработки, запуска и проверок всегда используй только удалённую облачную PostgreSQL из локального `DATABASE_URL`; локальный PostgreSQL не запускать. Секреты из `.env` не выводить, не документировать и не коммитить.

- `prepareDatabase()` применяет схему при старте; version table нет.
- Новые SQL-изменения должны быть повторяемыми (`IF NOT EXISTS`, безопасный `DO $$`, `ON CONFLICT` и т.п.).
- Не изменяй применённую migration, если можно добавить forward-only migration; перед номером проверь существующие имена.
- Новую migration обязательно подключи полной цепочкой `//go:embed` → `Exec` → `prepare<Module>Database()` → `prepareDatabase()`.
- Учитывай FK, soft delete/status и зависимости порядка инициализации.
- Destructive schema/data changes запрещены без явного указания и backup plan.

## Справочники и общие механизмы

Перед созданием нового механизма проверь, нет ли уже подходящего: auth/sessions, JSON helpers, `dictionaries` / `dictionary_items`, admin CRUD, geography/ОКВЭД/duties, test results/matching, shared catalog/header/errors/selects, adapters/resolvers и UI components.

- auth, профиль/avatar/settings; admin/пользователи;
- справочники, ОКВЭД, география, обязанности, survey;
- профиль специалиста: анкета, опыт, образование, языки, финансы, знания, публикация;
- вакансии: мастер, требования, обязанности, тесты, matching, публикация;
- тесты: создание/версии/вопросы, модерация, прохождение, результаты;
- employee testing по приглашениям; marketplace тестов и каталоги;
- ПрофиМаркет решений/регламентов;
- публикации, серии, реакции, сохранения, аналитика, модерация;
- клиентская биржа: объявления, отклики, избранное, статусы, уведомления;
- бухгалтерские компании: профиль, услуги/тарифы, отзывы, паспорт компетенций;
- взаимопомощь через профиль специалиста: темы, заявки, сообщения, отзывы (активная разработка).

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

Не дублируй models, DTO, services, helpers, справочники и компоненты.

## Проверки

Проверки должны быть соразмерны изменению.

Всегда:
- `gofmt` для изменённых `.go`;
- релевантные Go tests для изменённого package/module;
- `npm run build`, если менялся React/Vite/Editor.js frontend.

Дополнительно при необходимости:
- `go test ./...`, `go vet ./...`, `go build ./...` — для сквозных, архитектурных или release-критичных backend-изменений;
- DB integration tests — когда затронут SQL/repository/migrations;
- визуальная проверка mobile/desktop — когда существенно менялся UI.

Не запускай полный test/vet/build цикл многократно без причины. Если проверку нельзя выполнить, сообщи почему.

## Запрещено без отдельного указания

- Менять бизнес-правила, scoring/matching, статусы, права или публичный API вне задачи.
- Делать массовый refactor, переносить модули между архитектурными слоями, менять framework/router/ORM.
- Удалять/обнулять БД, таблицы, migrations, uploads или пользовательские данные.
- Менять auth/session/cookie/security/admin credentials без необходимости задачи.
- Коммитить `.env`, секреты или персональные данные.
- Перезаписывать чужой dirty worktree, форматировать несвязанные файлы.
- Править generated/vendor файлы вручную.
- Добавлять dependency/CDN/service/telemetry без необходимости.
- Массово исправлять encoding/mojibake без проверки реальной кодировки.

## Документация

Не обновляй документацию после обычных мелких задач.

`docs/CODEX_CONTEXT.md` обновляй только после нового крупного модуля, существенного изменения архитектуры или нового общего механизма, который важно знать следующим агентам.

Записывай только устойчивый контекст, а не changelog и не пересказ кода.
