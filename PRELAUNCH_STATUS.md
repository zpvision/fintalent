# FinTalent: статус предпусковых исправлений

Актуализировано: 28.09.2026. База изменений: `d63e208` (после `git fetch origin` и fast-forward с `fd689ba`). Новый fetch 28 сентября подтвердил HEAD/origin/main: 0/0, тот же d63e208. Все перечисленные исправления находятся в рабочей копии, без commit/push/deployment. Production не изменялся. Этот документ заменяет предположение, что старый аудит полностью описывает текущую версию.

**Выпуск пока не подтверждён.** Исправлена значительная часть дефектов, но полная матрица прав/гонок и восстановление резервной копии ещё не подтверждены. «Исправлено» ниже означает конкретный закрытый путь, а не независимый аудит всего модуля.

## Проверки и доказательства

- `go test ./...` при `RUN_DB_TESTS=0`, `RUN_STARTUP_DB_TESTS=0`, `RUN_RELEASE_SECURITY_DB_TESTS=0`: успешно. Это unit/обычные тесты, не SQL-интеграция.
- `go vet ./...`: успешно.
- `GOOS=linux GOARCH=amd64 go build -o .codex-run/fintalent-linux-amd64 .`: успешно, реальный toolchain Go 1.26.8.
- `npm run build`: успешно. Бандл ~758 КБ, gzip ~209 КБ; предупреждение размера сохраняется. React build игнорируется git и должен включаться в поставку отдельно.
- `npm audit --json`: 0 известных уязвимостей.
- `govulncheck@latest ./...`: 0 на вызываемых путях, 0 в импортированных пакетах. Сканер отдельно сообщает 18 находок в требуемых модулях, не достигаемых анализируемым кодом. Это не гарантия отсутствия всех уязвимостей.
- `tests/security/profimarket-rendering.cjs`: Chromium, локальные fixtures без сети. Кавычки, обработчики, HTML, javascript/data, iframe чужого домена, CSS injection не выполняются; обычные ресурсы сохранены.
- `release_security_integration_test.go`: настоящая облачная PostgreSQL, новая случайная схема с проверенным `search_path`, без запуска приложения/SMTP. Холодная инициализация и первоначальные пять сценариев прошли. Расширенный сценарий выявил отсутствие проверки нулевой вставки справочника компании; исправлено, повторный прогон отмечается ниже.
- В ходе SQL-проверки выявлена и исправлена ошибка порядка холодного старта: миграция 044 обращалась к `test_attempts` до создания таблицы. Теперь применяется после 003 в test-module prepare-цепочке. Таймаут всей подготовки увеличен до 5 минут; ошибки содержат имя этапа.
- Браузерный smoke новой сборки: `tests/security/solution-smoke.cjs`, API fixtures, семь типов Решений, ширины 360/900/1440, reload и повторные ссылки вопросов. Это не проверка авторизованного CRUD и не production.

Повторный SQL прогон: **PASS, 143.85с**, все пять групп, включая неверный dictionary ID → полный rollback компании, admin-only review moderation и повторную обработку →409. Browser smoke: **PASS**, 7 типов × 3 ширины, reload и повторные question links. Использованы fixtures; авторизованный сквозной browser CRUD этим не покрыт.

## P0 / P1

| ID | Статус, изменения и пределы проверки |
|---|---|
| P0-01 | Подтверждён, исправлен воспроизведённый XSS в `static/profimarket-components.js`: экранирование атрибутов, URL/цвета, allowlist iframe; `profimarket_validation.go` проверяет новые записи, renderer защищает старые. Исправлены конкретные attribute-sinks в legacy admin/profile/catalog/company/client-exchange/publication скриптах; версии подключений обновлены. React product URLs валидируются. Browser regression прошёл. Полного независимого анализа каждого HTML/CSS sink всех редакторов пока нет. |
| P1-01 | Исправлено: Go 1.26.8, pgx 5.9.2, x/text 0.39.0, x/net 0.55.0; `go.mod`, `go.sum`. Сборка и govulncheck выполнены. Production binary/toolchain отдельно неизвестны. PGX simple-protocol exploit через endpoint не утверждается. |
| P1-02 | Исправлены DB admin sessions (065, `admin_sessions.go`), TTL 12ч, logout отзывается на сервере, credential fingerprint, работа нескольких процессов. SQL TTL/logout прошли. Смена пароля атомарно отзывает все пользовательские сессии и reset credentials, текущая cookie удаляется; требуется повторный вход. Изменение email/блокировка админом инвалидируют reset/session. Production config отклоняет известные placeholders, небезопасные cookies/origin и отсутствующую React-сборку. Полная browser-матрица смены credentials не пройдена. |
| P1-03 | Исправлены доверие XFF, bounded per-IP limiter, concurrent bcrypt cap, atomic reset request/verify locks (`request_security.go`, `password_reset.go`). Unit parallel limiter/XFF прошли. Limiter локален процессу; для нескольких инстансов нужен общий лимит на proxy. SQL concurrent verify, строгий лимит неверных кодов, expiry и single-use reset прошли. Distributed brute-force, совместная работа proxy и concurrent request-reset ещё не покрыты полной матрицей. |
| P1-04 | Общий Go CrossOriginProtection + точное сравнение Origin с APP_BASE_URL, включая unsafe auth/multipart. Клиенты без Origin/Fetch Metadata допускаются по стандартной небраузерной модели. Unit own/cross/same-site origin прошли. CSP только Report-Only; совместимость всех Editor.js/iframe на стенде требует проверки, enforcement не объявлен. |
| P1-05 | Accounting upload получает MaxBytesReader до parsing и удаление multipart temp; общий unsafe API cap 8 МиБ. SVG blacklist заменён static XML allowlist (`svg_validation.go`), unit безопасных/опасных SVG прошли. Полный stress upload/temp-disk и nginx cap не проверены. |
| P1-06 | Общие проверки родителя `public_access.go`; закрыты дочерние Решений/публикаций, профиль знаний, company passport/reviews, response биржи, test reviews, прикреплённые тесты вакансий/публикаций, invitation owner и подписка на заблокированного автора. Новые личные сообщения также проверяют блокировку и сериализуют действия. SQL-матрица вложенных публикаций/знаний/паспорта public/draft/deleted/blocked/system и блокировка смены публикации до commit взаимодействия — PASS. Полная матрица каждого endpoint, включая все личные исторические права, **не завершена**. |
| P1-07 | Убран login email из public publication DTO, SEO no-store, приватный профиль помощи использует профессиональную проекцию без скрытых рабочих ожиданий (`resume_public.go`, `demo_content.go`). DTO и draft child SQL проверки прошли. Каталог/все фильтры приватного профиля требуют расширенной SQL/browser проверки. Публичные индивидуальные данные паспорта сотрудников оставлены до решения владельца продукта, не объявлены согласованными. |
| P1-08 | Participant DTO скрывает explanation и text эталон до finish; серверный deadline и snapshot таймера; employee API тоже редактирован. Reload восстанавливает attempt, ответы, индекс, время и видимую переписку. SQL single/multiple/boolean/text, неверные/чужие/повторные ids, grade redaction, завершение и восстановление попытки — PASS. Chromium 360/900/1440 с API fixtures: все типы, reload, finish retry и история — PASS. Полный employee browser end-to-end не пройден. |
| P1-09 | Historical question/answer mutations ограничены current editable version без попыток; answer transaction locks attempt, валидирует ids/type, revision 066; finish отвергает устаревшее grading snapshot, employee finish блокирует попытку перед расчётом. Vacancy start берёт прикреплённую версию. SQL чужой question/late answer/stale revision прошли; три настоящие parallel finish/answer/finish проверки прошли. После ForkDraft SQL подтвердил запрет Update/DeleteQuestion, Add/Update/DeleteAnswer для исторической версии и сохранение эталона. SQL четырёх типов вопросов и назначенной/текущей версии вакансии прошёл. Полная матрица всех прав вакансии остаётся открытой. Формулы оценивания сохранены. |
| P1-10 | 067 вводит expiry приглашений (30 дней, для существующих начинается при миграции), revoke endpoint/UI, blocked/revoked/expired guards. Личные результаты исключают employee context; паспорт исключает повтор попытки и использует разные пространства identity. Создание/пересдача + очередь письма атомарны. История не удаляется. Токены приглашений пока хранятся в исходном виде для совместимости; переход к hash не реализован. SQL employee start/answer/finish, expired/revoked/blocked во всех четырёх endpoints, личная история и запрет resume visibility прошли. SQL passport aggregate, дедупликация attempt_id, составная identity и общий счётчик по компетенциям прошли; полная browser матрица не завершена. Referrer и cache URL приглашений защищены; proxy logging остаётся отдельным gate. |
| P1-11 | Часть create/update биржи уже транзакционна в d63e208; сохранена. Добавлены conditional state transitions, согласованный порядок parent/response locks, проверка публикации под lock, запрет удаления завершённой передачи в UPDATE, атомарное уведомление завершения. Компания create/update и связи в одной tx; нулевая вставка неверных dictionary IDs теперь ошибка. SQL: rollback компании и биржи, модерация, три гонки archive/accept, приватные контакты owner/buyer/other, deleted parent и transferred delete — PASS. Все остальные сочетания параллельных переходов пока не подтверждены. |
| P1-12 | 068, `notification_outbox.go`: durable очередь, 2 worker, leases, bounded retries, восстановление expired final lease. Purchase request key/fingerprint, advisory serialization, одна tx purchase+notifications+outbox. React/legacy переиспользуют ключ при retry, новые действия получают новый. SQL повтор key даёт одну заявку и два сообщения. SMTP не вызывался, реальная доставка/restore очереди не проверены. Delivery at-least-once: crash после SMTP success может повторить письмо. Старый API-клиент без key сохраняет прежний контракт. Коммерческая модель не изменена; решение заявки/оплата не подтверждено владельцем. |
| P1-13 | Чужая серия отклоняется, parent comment привязан к publication; nil RowsAffected panic устранена; test handler скрывает SQL/internal errors. SQL своя/чужая серия и сохранение прежней связи после rollback — PASS. Comments GET проверяет Scan/rows.Err, ошибка БД при INSERT возвращает 500. HTTP/SQL чужой parent comment отклоняется, корректный принимается. Полный sweep всех handler errors не выполнен; часть legacy handlers ещё требует безопасного mapping. |
| P1-14 | Geography больше не удаляет legacy cities по external_idNULL; ссылки сохраняются. Seed system test versions не перезаписывает тексты опубликованной версии; upstream сохранение статуса сохранено. Холодный старт и повторный seed geography/marketplace на PostgreSQL проверены: legacy city ID, archived status, difficulty, version title/description сохранены. Дополнительно в кандидате для dev: position-skill seed с unsupported case и без эталона text отключён при SEED_DEMO_DATA=false. Существующие тесты и результаты не меняются; новые тесты должностей требуют утверждённого набора. Пустой, повторный и конкурентный старт в изолированной схеме — PASS, 140.42с. |
| P1-15 | `runtime_lifecycle.go`: pool default16 (конфиг), API context deadline45с, health live/ready, graceful shutdown, bounded notification workers. Linux build прошёл. Production pool budget, shutdown под нагрузкой, nginx/firewall/TLS, backup restore **не проверены**. Порядок выпуска — `PRELAUNCH_RELEASE.md`. |

## P2 (локальные идентификаторы для пунктов исходного списка)

| ID | Статус |
|---|---|
| P2-01 company review moderation | Добавлены существующая admin community вкладка и pending→published/rejected API, без массовой публикации. SQL прав/повторной обработки включён. Browser admin CRUD ещё не проверен. |
| P2-02 vacancy aggregate | Исправлен выбор latest attempt и устойчивый tie-break id в статистике и строке кандидата. SQL сценарий success→failure отдельно ещё не выполнен. |
| P2-03 paid tests | Требует бизнес-решения: enforcement entitlement не добавлен, текущий бесплатный сценарий не изменён. |
| P2-04 checklist preview | Требует определения платного/маркетингового содержимого. Полный product_data пока выдаётся по прежнему контракту; UI-lock не защита платных данных. |
| P2-05 unknown URL | `react_frontend.go` возвращает 404 для незарегистрированного пути вместо legacy home200. Полный redirect map не проверен браузером. |
| P2-06 public metrics | Основные blocked filters сохранены; удалён фиктивный fallback 243 покупки при нулевом счётчике Решения. Весь набор aggregate не перепроверен SQL. Views не объявлены уникальными людьми. |
| P2-07 bundles/maps | Измерена сборка, оптимизация не выполнена; sourcemaps не поставлять публично, см. release instruction. |
| P2-08 performance/pagination | Репрезентативная нагрузка/N+1/полная пагинация не выполнены; готовность к нагрузке не установлена. |
| P2-09 CSS/JS recovery | Отклонённый loadPresentation Promise сбрасывается. usePageStyles ожидает реальную загрузку, повторяет ошибки автоматически и вручную, очищает handlers/timers. Browser slow CSS/retry/navigation — PASS на трёх ширинах. |
| P2-10 solutions navigation | Локальный offline browser smoke всех семи типов, повторные question ссылки/reload. Полные CRUD/modal/back/forward/slow network ещё не закрыты. |
| P2-11 CityPicker | Abort предыдущего fetch на ввод/выбор/закрытие, stale результат не открывает список. Combobox/listbox, стрелки/Enter/Escape/Tab, одинаковые названия разных регионов, выбор до debounce, очистка и отмена медленного ответа — browser PASS на трёх ширинах. Text filter контракт сохранён. |
| P2-12 verification claims | Константа «Проверенный профессионал» заменена нейтральным автором; seller.verified берётся из подтверждённой компании. Новая проверка личности не придумана. |
| P2-13 invite messages | Создание/пересдача возвращают email_queued, не ложный email_sent; реальная отправка через очередь. SMTP на стенде не проверен. |
| P2-14 accessibility | Полная клавиатура/focus возврат/combobox a11y не проверены. |
| P2-15 consent/legal | Актуальность политик, согласий, публичности и процесса удаления требует проверки владельцем; юридическое соответствие не подтверждается этим кодовым review. |
| P2-16 release checks | Добавлены самостоятельные security unit/SQL/browser checks; команды ниже. |

## Матрица покрытия разделов

«Частично» не означает весь CRUD. A/B — синтетические пользователи, admin с тестовой конфигурацией; системные авторы возникают только в изолированном startup.

| Раздел | Код | Unit | PostgreSQL | Браузер новой сборки | Осталось |
|---|---|---|---|---|---|
| Auth/settings | Да | Origin/limiter | Сессии частично | Нет | Credential смена/email/reset end-to-end |
| Password reset | Да | IP/limiter/AES | Concurrent request/verify, uniform response, TTL/single-use/error budget/queue full | Нет | SMTP mock и browser end-to-end |
| Admin | Да | SVG/config частично | TTL/logout, reviews | Нет | Справочники и модерация UI |
| Specialist profile | Да | Существующие | Knowledge/privacy не полностью | Нет | Оба режима, private/help, сертификаты |
| Vacancies | Да | Существующие matching | Pinned/current version, blocked/deleted/private test, rollback attachment | Нет | Полный CRUD/candidates/latest и гонки |
| Tests/marketplace | Да | Да | Все 4 типа, secrets/deadline/revision/history, parallel finish | Reload/finish retry/4 типа с fixtures | Browser через настоящий backend, оставшиеся rights/races |
| Employees | Да | Существующие | Expiry/revoke/blocked, 3 answer/finish/finish races, passport dedup/identity | SPA privacy; не полный CRUD | Publicity decision/FinKoper mock/полный employee browser |
| Solutions | Да | URL validation | Parent access/idempotency | 7 типов fixtures | Полный CRUD/blocked transitions/SMTP mock |
| Publications | Да | Существующие | Draft children/DTO, own/foreign series; расширенная матрица ниже | Нет | Editor/SEO/browser, оставшиеся гонки |
| Client exchange | Да | Существующие | Rollback, 3 archive/accept races, contacts owner/buyer/other, favorites | Нет | Остальные параллельные update/response/delete; browser CRUD |
| Companies | Да | Существующие | Rollback/moderation, passport/review access, passport counts/dedup | Нет | CRUD/publicity/upload stress |
| Help and messaging | Частично | Существующие | Нет новой полной матрицы | Нет | Participants/block/status/races |
| Dictionaries/geography | Да | Существующие | Cold startup + targeted repeat seed | Нет | Concurrent full startup/все ссылки городов |
| Operations | Да | Config/IP/limiter | Isolated cold startup | Не применимо | Backup restore/firewall/nginx/shutdown/load |

## Команды продолжения

Запускать из корня, без production startup. Обычные DB flags оставить выключенными: существующие интеграционные тесты нельзя направлять на рабочую схему.

```powershell
$env:RUN_DB_TESTS='0'
$env:RUN_STARTUP_DB_TESTS='0'
$env:RUN_RELEASE_SECURITY_DB_TESTS='0'
go test ./...
go vet ./...
npm run build
npm audit
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
# Только этот opt-in тест сам создаёт и проверяет новую изолированную схему:
$env:RUN_RELEASE_SECURITY_DB_TESTS='1'
go test . -run '^TestReleaseSecurityIsolatedPostgres$' -count=1 -v -timeout 12m
$env:RUN_RELEASE_SECURITY_DB_TESTS='0'
# Установленный отдельно playwright-core, не временные node_modules в поставке:
$env:NODE_PATH='<путь к node_modules с playwright-core>'
node tests/security/profimarket-rendering.cjs
node tests/security/solution-smoke.cjs
```

## Блокеры вывода «можно выпускать»

1. Не проверено восстановление БД **и обоих каталогов uploads** на отдельном стенде.
2. Не завершены критические SQL/HTTP concurrency и ownership матрицы, обозначенные выше; не все P1 можно считать закрытыми.
3. Новые некорректные тесты должностей больше не публикуются при SEED_DEMO_DATA=false; уже существующие данные проверить на восстановленной копии production отдельно. До утверждения экспертного набора автоматическое добавление теста для новой должности не предусмотрено.
4. Владелец подтвердил: покупка Решения — заявка автору без оплаты на сайте, индивидуальные employee результаты и имена публичны в паспорте компании. Эти решения больше не ожидают согласования. Эквайринг — отдельная будущая задача; платные тесты требуют отдельного уточнения. Юридическая проверка информирования о публичности не выполнена.
5. Не проверены production proxy/HTTPS/Secure/env/cloud pool/migration duration/SMTP queue delivery.
6. Не выполнен полный авторизованный browser smoke A/B/admin по всем разделам.

Нельзя заменить эти пункты зелёными unit/build. Продолжать с перечисленных пробелов; не повторять исправления уже закрытых путей и не считать отсутствие ошибки в mock доказательством SQL-прав.

## Текущий дополнительный проход

- Очередь ограничена 10000 pending/sending сообщениями. Admission сериализован transaction advisory lock; при переполнении критическая бизнес-транзакция откатывается, а не теряет уведомление молча. Dedup уже существующего ключа остаётся успешным.
- Загрузка вопросов/ответов теста и попытки теперь пакетная, без вложенного удержания соединения; employee participant DTO использует тот же repository. Вопросы и элементы регламента также загружаются раздельными пакетами; company catalog освобождает cursor перед чтением карточек.
- Для прямой выдачи исторических SVG из `/static/uploads/` Go устанавливает enforcement CSP sandbox. Если uploads выдаёт nginx, заголовок нужно зеркалировать там. Само содержимое старых файлов не перезаписывалось.
- SQL-проверки parallel finish/answer, employee expiry/revoke/blocked, отсутствие employee результата в личном профиле, concurrent reset verify/error budget/single-use и переполнение outbox: **PASS, расширенный прогон 251.27с**. Это три гонки обычной попытки, а не исчерпывающий stress test. Следующий полный SQL-прогон **PASS, 289.93с**: дополнительно подтверждены чтение теста/попытки/регламента при одном соединении и сохранение системного контента/legacy cities при повторном seed.

Новый фильтр professional/job_search для private-help профиля применяет ту же публичную проекцию, что DTO. Отдельный PostgreSQL-тест каталога/прямой ссылки/скрытых рабочих полей: **PASS**, 203.66с с холодной инициализацией (сам сценарий 2.38с).

Проверен локальный режим pgx без соединения и вывода DSN: `cache statement`, не simple protocol. Это не подтверждает конфигурацию развёрнутого production процесса.

При дополнительном review request-reset исправлена ошибка обработки `sql.ErrNoRows` неизвестного email. Неизвестный адрес теперь получает тот же нейтральный ответ и расходует лимит запросов; отказ admission в outbox также не раскрывает существование аккаунта. SQL-проверка uniform response/concurrent request и full-queue rollback: **PASS, 219.27с**, сценарий 5.52с.

Код восстановления в durable outbox зашифрован AES-GCM с отдельным назначением ключа, производного от PASSWORD_RESET_SECRET; случайный nonce, проверка подмены/неверного ключа/отказ plaintext покрыты unit tests. SMTP delivery на стенде остаётся отдельным gate. Ротация секрета инвалидирует старые reset-коды и делает неотправленные envelope нечитаемыми: согласовать ротацию с завершением очереди и повторным запросом восстановления.

Дополнительный проход Клиентской биржи: исправлены обход статуса через ownerView в избранном, favorite/view для недоступного родителя, race удаления завершённой передачи, проверка публикации вне блокировки, принятие отклика удалённой карточки. Переход статуса и уведомление завершения теперь в одной транзакции; принятие отклика блокирует сначала родителя. Чтение списка освобождает cursor до загрузки карточек. Изолированный SQL-сценарий **PASS, 161.70с**, сам сценарий 9.38с (`release_client_exchange_integration_test.go`, вызывается только из изолированной fixture).

Расширенный SQL-прогон исторических ответов после ForkDraft, владельца серии публикации и request-reset с AES outbox: **PASS, 166.93с**. Повторены параллельные answer/finish, employee expiry/revoke/blocked, чтение вопросов при одном соединении. Ошибка очереди в логе ожидаема: это проверка переполнения. SMTP не запускается. Публичные варианты тестов в metadata публикаций теперь также исключают private/deleted/blocked. Comments GET проверяет Scan/rows.Err, ошибка БД при записи комментария возвращает 500 вместо ложной ошибки валидации. Итоговые `go test ./...` (DB flags=0), `go vet ./...` и Linux build после изменений — PASS. Frontend в этом дополнительном проходе не менялся; browser/build результаты выше относятся к предшествующей frontend-сборке.

Следующий проход: оставшаяся HTTP/SQL parent-access матрица (профили знаний, паспорт/отзывы компании, публикационные comments/reactions, вложенные тесты вакансий), все типы вопросов и browser reload, конкурентные переходы employee и остальных действий биржи. Затем функциональные P2, включая восстановление CSS после сбоя. Не повторять уже успешно завершённые сценарии без новых изменений или причины.

## Продолжение: вложенные API и прохождение тестов

- `resume_knowledge.go`: подтверждение требует существующего личного finished/show_in_resume результата, исключает employee context, нулевая вставка больше не отвечает confirmed=true. Повторное подтверждение идемпотентно. Проверка профиля и запись в одной транзакции с блокировкой родителя/автора; body ограничен 8 КБ. Чтение знаний освобождает основной cursor перед подтверждениями и проверяет ошибки выборок.
- `internal/accountingcompany/handler.go`: проверка опубликованной компании и запись pending review теперь атомарны относительно скрытия/блокировки. Passport проверяет Scan/rows.Err, освобождает основной cursor до history и не выдаёт молча пустую историю при ошибке БД. Публичность индивидуальных данных не изменена; бизнес-решение остаётся открытым.
- `public_access.go`, `publication_actions.go`, `publication_detail.go`: общий transaction guard для comments/reaction/bookmark/report/progress удерживает доступность публикации и автора до commit. Комментарий и уведомление в одной транзакции; progress больше не отвечает успехом при ошибке БД.
- `release_nested_access_integration_test.go`: первая матрица PostgreSQL **PASS, 161.97с** (сценарий 18.99с). Guest/owner/other × public/draft/blocked/system/deleted для знаний, company passport, publication comments/versions; admin company passport; закрытые publication writes, company reviews; отсутствие/employee/self/повтор подтверждения знаний; чужой parent comment; чтение знаний и паспорта при одном DB connection. Это не полная матрица всех модулей. Последующий прогон transaction guard и четырёх типов вопросов прошёл 28 сентября; результаты приведены ниже.
- `tests/security/test-attempt-reload.cjs`: **PASS**, Chromium с локальными built assets и API fixtures, 360/900/1440. Single/multiple/boolean/text, ответ клавиатурой, reload сохраняет attempt_id/индекс/серверное оставшееся время, ошибка finish → reload → повтор finish, история, отсутствие pageerror/overflow. Это не реальный браузерный проход через PostgreSQL/сервер.
- `go test ./...` (три DB flags=0), `go vet ./...`, Linux artifact после backend-изменений — PASS. Позднее дополнительно изменены QuestionChat/TestTakePage для восстановления видимой переписки; `npm run build` и повтор `test-attempt-reload.cjs` после этого изменения — PASS.

## Проверки 28 сентября

Прерванный ночью SQL-прогон не дал итогового результата и не считается PASS. Повтор с исправленными тестовыми ожиданиями завершён: **PASS, 170.42с**, `.codex-run/release-parent-types-vacancy-sql.log`.

- Вложенные API: матрица guest/owner/other, draft/public/deleted/blocked/system; положительные reaction/bookmark/report/progress; подтверждена блокировка смены статуса публикации до commit взаимодействия; чужой parent comment отвергается.
- Все четыре типа вопросов: безопасный DTO до завершения, собственный введённый ответ сохраняется при reload, чужие/дублированные answer IDs не уничтожают правильный предыдущий ответ, админ сохраняет доступ к эталонам, итог 100% и повторный finish отклонён.
- `internal/vacancymodule/repository/repository.go`: добавлена проверка deleted/blocked/system при новом прикреплении теста. SQL подтвердил полный rollback полей вакансии при недоступном тесте, сохранение назначенной версии при редактировании вакансии, совпадение публичного описания и запуска по старой версии, обычный запуск по новой версии. Private/deleted/blocked test и archived/blocked-owner vacancy не позволяют новый старт; системный автор остаётся исключением.
- Повторный Chromium smoke восстановленного чата — PASS на трёх ширинах, сохранённый ответ виден после reload. Итоговые Go tests/vet/Linux build — PASS.
- SQL employee answer/finish/finish и passport counts: первоначально **PASS, 157.65с**. Расширенный повтор после исправления общего счётчика специалистов и дедупликации: **PASS, 160.00с**, `.codex-run/release-passport-counts-sql.log`. Три параллельных гонки: один успешный finish, согласованность сохранённого выбора/HTTP результата/баллов. Четыре типа вопросов проходят также через employee answer API. Passport различает одинаковые числовые employee/user IDs, считает людей по всем компетенциям, не удваивает одну попытку и history при повторной ссылке invitation. Индивидуальная публичность employee history по-прежнему требует решения владельца продукта.
- В `publication_actions.go` подписка на автора проверяет blocked/system в транзакции; подписка и уведомление сохраняются вместе. Повтор вложенной матрицы с проверкой subscribe/block/unsubscribe — **PASS, 152.55с**, `.codex-run/release-subscription-sql.log`.
- В `main.go`, `useDocumentPage.js`, `EmployeeTestPage.jsx`, legacy `employee-test.html` защищены URL приглашений: `no-referrer`, HTTP `no-store`, установка SPA-политики до подключения CSS, cleanup при уходе со страницы. Unit header checks и Chromium SPA test (включая same-origin CSS/fetch Referer) — **PASS**. Первый browser test выявил неверный порядок effects; он исправлен, повторный тест прошёл. Новая React build — PASS; остаётся предупреждение размера ~758 КБ.
- Финальные Go tests/vet/Linux build после защиты invitation headers — PASS. Это не подтверждает production proxy. В `PRELAUNCH_RELEASE.md` добавлен безопасный access-log формат; error logs/APM/CDN требуют отдельной проверки на стенде с синтетическим токеном.
- Оставшаяся после прерывания временная схема `release_security_1790541641159166200` удалена по точному имени из журнала собственного теста. Реальные схемы и пользовательские данные не затрагивались.

Следующие незакрытые проверки: полный auth/settings browser и SQL lifecycle смены credentials; помощь/переписка и оставшиеся вложенные права; SMTP/FinKoper mock; полный concurrent startup и upload stress; публичность паспорта и коммерческая модель; production backup restore/proxy/logging/shutdown/load. После блокирующих P1 — оставшиеся P2 (CSS retry, CityPicker keyboard/abort, пагинация и прочие пункты таблицы). Завершённые выше сценарии не считать ожидающими первого прогона.

## Продолжение 28 сентября: выдача сессий после смены credentials

- P1-02: подтверждена гонка между проверкой bcrypt и INSERT sessions. `main.go` теперь повторно проверяет проверенные hash/email и допустимый статус пользователя в транзакции с `FOR SHARE`, удерживая блокировку до фиксации сессии. Login возвращает 401 при изменившихся credentials; cookie выдаётся только после commit. Регистрация также передаёт исходные credentials.
- `release_auth_lifecycle_integration_test.go`: SQL-проверка смены пароля через HTTP handler, отзыва двух сессий, запрета выдачи сессии со старым hash/email, blocked/system аккаунтом — PASS, 148.05с вместе с подготовкой и удалением изолированной схемы. Сервер приложения и SMTP не запускались.
- `go test ./...` с тремя DB flags=0, `go vet ./...`, Linux amd64 build — PASS. Frontend не менялся.
- Расширенный SQL-сценарий конкурирующей выдачи сессии при заблокированной транзакцией строке пользователя — PASS, 145.27с вместе с инициализацией и cleanup; `.codex-run/release-credential-race-sql.log`. Устаревшие credentials отклоняются после commit изменения пароля; тестовая схема удалена.
- Ошибки `createContactThread` (игнорирование COUNT и проверка профиля до транзакции) исправлены следующим проходом, результаты ниже.
- Полная auth/browser матрица и остальные release gates выше остаются незакрытыми. Push и deployment не выполнялись.

## Продолжение 28 сентября: запросы на связь и помощь коллегам

- `contact_messages.go`: профиль/его владелец и отправитель проверяются с удержанием блокировок до commit; недельный лимит читается через ту же транзакцию, ошибки не игнорируются. Создание и блокировка диалога используют один advisory lock пары участников. Write-action удерживает строку диалога и обоих пользователей. Диалог, сообщение и outbox фиксируются вместе; лимит темы согласован с VARCHAR(200). Scan/rows.Err списков больше не дают молча неполный успешный ответ.
- `release_contact_integration_test.go`: guest/self/other/participants, draft/private/blocked/deleted profile, четыре одновременных запроса (два приняты, два отклонены по прежнему лимиту), pending/accepted/blocked, сохранение личной истории после блокировки другого участника, запрет нового обращения после блокировки диалога, rollback при ошибке outbox — PASS, 163.65с с isolated startup/cleanup. Реальные письма не отправлялись.
- `help_module.go`: создание запроса удерживает профиль/выбранную тему/участников до commit; outbox включён в транзакцию создания, принятия и отказа. Общая блокировка обращения и участников закрывает гонку message vs complete/cancel и новые сообщения/отзывы при заблокированном участнике. Сохранены публичная помощь при скрытых рабочих данных, системный автор и чтение разрешённой личной истории.
- `release_help_integration_test.go`: права участников, private-help/system exception, отзыв только requester после completed, запрет повтора, три конкурентных message/complete, отказ, откат create/accept при ошибке outbox — PASS. Совместный SQL-прогон contact/help — PASS, 179.00с, включая startup/cleanup; `.codex-run/release-help-contact-sql.log`. Созданная этим прогоном временная схема удалена.
- После contact и после help исправлений `go test ./...` (DB flags=0), `go vet ./...`, Linux build — PASS (`release-help-unit.log`, `release-help-vet.log`). Frontend не изменялся; это не браузерная проверка полного CRUD.
- Не закрыты практическая доставка SMTP/FinKoper mock, полная auth/admin/browser матрица, concurrent startup/upload stress, backup restore и production proxy/logging/load. Для переписки отдельно остаются browser сценарии, пагинация длинных историй и расширенные гонки отмены/блокировки; выполненный message/complete не считать проверкой всех переходов. Публичность паспорта и коммерческая модель требуют решения владельца продукта. Проект целиком пока не объявляется готовым к выпуску; push/deployment не выполнялись.

## Принятые бизнес-решения и следующий проход

- Владелец подтвердил модель заявок для Решений и публичность имён/индивидуальных результатов сотрудников в паспорте компании. Предыдущие строки отчёта об ожидании этих двух решений устарели. Платёжный процесс не добавляется; будущий эквайринг — отдельная задача.
- UI Решений, личные статусы, отзывы, уведомления и email уточнены: заявка без оплаты на сайте. COMPLETED сохранён в API/БД как исторический статус отправленной заявки, не доказательство платежа; amount — указанная автором цена. Счётчик подписан как заявки; админская сумма не называется выручкой. Превью инструкции/чек-листа не обещает автоматическое снятие платного замка.
- Перед employee test и в форме компании (React/legacy) добавлено объяснение публичности имени и результатов при публикации компании. Это информирование, не юридическая гарантия согласия сотрудников.
- `npm run build`, Go tests корневого package (DB flags=0), browser 7 типов Решений × 3 ширины с проверкой текста заявок — PASS. Первый прогон нашёл отсутствие видимого пояснения на AI mobile; исправлено, повтор прошёл. Employee browser с проверкой предупреждения и сохранением no-referrer — PASS.
- `usePageStyles` больше не считает error/3сек успешной загрузкой; добавлены ограниченные автоматические повторы, ручной retry и cleanup. Проверка медленного CSS/отказов/навигации — PASS на 360/900/1440 (`release-css-browser.log`). Повтор 7 типов Решений и сценариев прохождения/reload/referrer после общего изменения — PASS (`release-css-solutions.log`, `release-css-attempt.log`).
- CityPicker: добавлены combobox/listbox, активный вариант, ArrowUp/Down/Enter, Escape/Tab с отменой debounce/fetch и видимое выделение. Сохранён текстовый контракт фильтра. Browser одинаковых названий разных регионов, клавиатуры, отмены медленного fetch, выбора до debounce, очистки и Tab — PASS на трёх ширинах (`release-city-browser.log`). React build — PASS. Первое падение теста было неоднозначным locator для native select options; locator ограничен списком городов, повтор прошёл.
- Следующий проход: некорректные seed-тесты (preview затрагиваемых IDs и безопасное точечное исключение без изменения истории), оставшиеся SQL/HTTP права и гонки, SMTP/FinKoper mock, uploads/concurrent startup, полная auth/admin/browser матрица. Затем прочие P2 из таблицы. Отдельные production gates: backup restore, proxy/TLS/logging/pool/graceful shutdown/load. Вопрос о платных тестах задан отдельно: модель заявок подтверждена именно для Решений. Эквайринг не реализовывать попутно.

## Пункт 1: подготовка кандидата для dev

- `marketplace.go`: при SEED_DEMO_DATA=false больше не создаются автоматически position-skill тесты с неподдерживаемым `case` и текстовыми заданиями без эталона. Существующие тесты, их статусы и история попыток не изменяются. Accounting-topic seed остаётся с поддерживаемыми single/multiple вариантами. Новые должности получат тест после подготовки проверенного набора заданий.
- `startup_integration_test.go`: явная проверка отдельной схемы, отсутствие новых position-skill тестов при production seed=false, сохранность данных после повторного/конкурентного запуска. Исправлена старая ошибка тестовой фикстуры: `pgx` не принимает две SQL-команды одним prepared statement. После исправления SQL-прогон — PASS, 140.42с (`.codex-run/release-candidate-startup-sql.log`).
- `go test ./...` при DB flags=0, `go vet ./...`, Linux amd64 build — PASS. `go version -m` артефакта подтверждает Go 1.26.8 и pgx 5.9.2. React и Editor assets присутствуют; актуальный `npm run build` после последних изменений frontend — PASS, browser solution/CSS/city/attempt regression — PASS ранее, после них менялся только Go/test.
- Локальная рабочая копия пока dirty: 109 изменённых отслеживаемых файлов и 35 новых, включая четыре обязательных SQL. `git diff --check` — PASS. `static/react/` игнорируется Git и должен собираться в поставке. `.env` и `.codex-run/` игнорируются; чужой `PRELAUNCH_FIX_PROMPT.md` не включать в commit без отдельного решения. Branch/commit/push/deploy не выполнялись.
- Кандидат может проверяться на dev после создания защищённой копии production-БД и обоих uploads, конфигурации без отправки реальных писем/внешних действий и сборки согласованных artifacts. Это **не** подтверждение готовности к production: восстановленная копия production, полный browser A/B/admin, SMTP/FinKoper mock, nginx/TLS/logging и нагрузка ещё не проверены.
- Повторный `git fetch origin` после подготовки: HEAD и origin/main совпадают на `d63e208`, расхождение 0/0. Ветку release создавать от этого commit после сообщения владельца о готовности dev-копии; текущую рабочую копию не очищать и в main не пушить.
- Все остальные пункты исходного аудита сохраняются в работе. Ничего не опубликовано, push/deployment запрещены текущей задачей.
