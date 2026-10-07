# Порядок проверяемого выпуска FinTalent

Это инструкция, а не свидетельство выполнения production-проверок. На 27.09.2026 выпуск заблокирован пунктами `PRELAUNCH_STATUS.md`. Никаких push/deploy этот этап работы не выполняет.

## 1. Зафиксировать поставку и конфигурацию

После закрытия блокеров провести review локального diff относительно d63e208. Не включать `.env`, `.codex-run/`, node_modules, реальные uploads/сертификаты. Поставлять одним релизом Linux binary, legacy `static/` и собранный `static/react/`. Зафиксировать commit и SHA256 артефактов; проверять `go version -m` бинарника (Go 1.26.8 и исправленные модули).

В приватном EnvironmentFile с правами 0600 задать:

```dotenv
APP_ENV=production
APP_BASE_URL=https://fintalent.ru
COOKIE_SECURE=true
SEED_DEMO_DATA=false
SYNC_GEOGRAPHY=false
REACT_FRONTEND=true
DB_MAX_OPEN_CONNS=16
TRUSTED_PROXY_CIDRS=127.0.0.1/32,::1/128
```

`ADMIN_LOGIN`, случайный `ADMIN_PASSWORD` (не менее 16 символов), случайный `PASSWORD_RESET_SECRET` (не менее 32), DATABASE_URL, SMTP credentials задать через защищённую конфигурацию, не выводить в терминал/отчёт. Пароли из примеров запрещены. TLS PostgreSQL настроить по документации провайдера, предпочтительно с проверкой hostname/CA; не ослаблять проверку сертификата ради запуска.

`PASSWORD_RESET_SECRET` также защищает AES-GCM envelope кодов восстановления в outbox. Сохранять его в защищённой конфигурации при замене релиза. Ротация делает прежние reset-коды недействительными и не позволяет расшифровать уже ожидающие отправки сообщения; планировать её отдельно, учитывать состояние очереди и возможность повторного запроса восстановления. Секрет не хранить рядом с публично доступными backup.

16 соединений — начальная настройка одного процесса, не результат load test. Общий бюджет всех процессов, worker и миграций должен помещаться в лимит облака с резервом для эксплуатации. Не снижать ниже 8 без устранения вложенных запросов и load проверки. Limiter приложения локален процессу; общий anti-abuse при нескольких экземплярах должен быть на общем proxy.

## 2. Backup и проверка восстановления — до изменения схемы

Согласовать короткое окно запрета записей/maintenance, чтобы snapshot БД и файлов относился к одному состоянию. Сохранить PostgreSQL, `static/uploads/` и `uploads/resume-certificates/`; backup зашифровать и вынести за пределы webroot. Примеры команд выполняет оператор в защищённой shell с настроенным PGSERVICE/pgpass (секрет не в argv):

```bash
umask 077
pg_dump --dbname=service=fintalent_backup --format=custom --file="$BACKUP_DIR/database.dump"
tar -C "$SHARED_DIR" -czf "$BACKUP_DIR/uploads.tar.gz" static/uploads uploads/resume-certificates
sha256sum "$BACKUP_DIR/database.dump" "$BACKUP_DIR/uploads.tar.gz" > "$BACKUP_DIR/SHA256SUMS"
pg_restore --list "$BACKUP_DIR/database.dump" > "$BACKUP_DIR/restore-list.txt"
```

`BACKUP_DIR`, `SHARED_DIR` — заранее проверенные абсолютные пути оператора. `pg_restore --list` **не** заменяет restore. На отдельной облачной тестовой БД восстановить в пустую цель:

```bash
pg_restore --dbname=service=fintalent_restore_test --no-owner --exit-on-error "$BACKUP_DIR/database.dump"
tar -C "$RESTORE_FILES_DIR" -xzf "$BACKUP_DIR/uploads.tar.gz"
```

До команды проверить, что сервис `fintalent_restore_test` указывает на отдельную тестовую БД, не production; каталог файлов изолирован. После восстановления проверить количество users/purchases/attempts, FK, чтение нескольких случайных изображений и закрытых сертификатов владельцем, запрет сертификата гостю/чужому пользователю. На восстановленном стенде отключить SMTP/FinKoper или направить только в mock. Зафиксировать время, контрольные суммы и результат. Не использовать реальные почтовые адреса для отправок.

## 3. Миграции на восстановленной копии

Новые повторяемые SQL: 065 admin_sessions, 066 attempt revision, 067 invitation expiry, 068 notification outbox + purchase idempotency. Они подключены embed→prepare. 044 перенесена в правильный этап после таблиц тестирования; исторические SQL не запускать подряд.

До запуска сделать read-only preview:

```sql
SELECT status, count(*) FROM company_test_invitations GROUP BY status;
SELECT count(*) FROM cities WHERE external_id IS NULL;
SELECT t.id,t.slug,t.status,t.current_version
FROM tests t JOIN users u ON u.id=t.author_id WHERE u.is_system;
SELECT count(*) FROM test_attempts;
SELECT count(*) FROM profimarket_purchases;
```

067 назначает существующим ссылкам срок 30 дней от применения; уведомление владельцев и этот переход необходимо учитывать в выпуске. Старые личные/employee попытки не удаляются и не пересчитываются. Отдельно проверить default/nullable/version fields на восстановленных исторических данных. Не архивировать системные тесты массово: подготовить точный список некорректных тестов/версий, утвердить действие, сохранить историю.

Первый запуск выполняет prepareDatabase. Проверить пустой/повторный/конкурентный старт на стенде, длительность до readiness, сохранение статусов системных тестов и city IDs. На холодной облачной схеме инициализация в проверке занимала около 150с; readiness до завершения не появится. Не объявлять процесс неисправным через 30с на первом старте.

## 4. Reverse proxy и файлы

Backend порт закрыть firewall/security group для внешнего мира; проверить с внешнего узла, не только localhost. Nginx передаёт `Host $host`, `X-Forwarded-Proto $scheme`, `X-Forwarded-For $proxy_add_x_forwarded_for`; в TRUSTED_PROXY_CIDRS указать только фактические доверенные hops. Не доверять всему private subnet без необходимости.

Ссылка `/employee-test?token=...` и путь `/api/employee-test/<token>` содержат bearer token. Приложение устанавливает `no-referrer`/`no-store`; React также меняет referrer policy при SPA-переходе. Это **не маскирует URL самого запроса в proxy-логах**. Для access log использовать формат без query, Referer и тела запроса, с маскированием token в пути. Пример для контекста `http` nginx:

```nginx
map $uri $fintalent_log_uri {
    ~^/api/employee-test/ "/api/employee-test/[redacted]";
    default $uri;
}
log_format fintalent_safe '$remote_addr "$request_method $fintalent_log_uri $server_protocol" '
                         '$status $body_bytes_sent $request_time';
```

В соответствующем `server`: `access_log /var/log/nginx/fintalent.access.log fintalent_safe;`. Проверить, что нет второго inherited/дублирующего access log с `$request`, `$request_uri`, `$args` или `$http_referer`. Отдельно исключить token из error logs, tracing/APM и CDN-логов: формат access log не меняет их. Не включать debug request logging. На стенде отправить синтетическую ссылку с заведомо нерабочим `FAKE_AUDIT_TOKEN`, включая ошибку upstream, и убедиться, что эта строка отсутствует во **всех** собранных логах. До такой проверки logging gate остаётся открытым.

Для `/api/`: `client_max_body_size 8m`, `proxy_read_timeout 65s`, `proxy_send_timeout 65s`; модульные меньшие лимиты сохраняются. Проверить 413 для oversized multipart и работу штатных uploads. Запретить cache API/персонализированных HTML. Только хешированные `/static/react/assets/` — `Cache-Control: public,max-age=31536000,immutable`; HTML и legacy JS не получать этот cache. `.map` не отдавать публично (сохранять отдельно в artifact/debug storage).

Запретить directory listing и выдачу `.env`, `.git`, backup, внутренних `uploads/resume-certificates/`. Последний каталог выдаётся только через авторизованный handler. Старые SVG в uploads могли быть приняты прежним blacklist: новый upload validator не очищает их автоматически. До выпуска инвентаризировать SVG, проверить на стенде; обслуживать недоверенные uploads с CSP sandbox/nosniff или отдельного origin без auth cookies. Не удалять файлы вслепую.

`static/uploads/` и `uploads/resume-certificates/` разместить в постоянном shared storage и подключить к каждому релизу. Новый release directory не должен заменять эти данные. Проверить реальные итоговые пути и права владельца сервиса.

## 5. Проверки перед переключением

На отдельном стенде выполнить матрицу `PRELAUNCH_STATUS.md` с A/B/admin/system, mock SMTP/FinKoper, включая parent children, finish/answer и archive/accept races. Проверить фактическую доставку в mock, retry queue, final failure и отсутствие HTTP ожидания SMTP. Для очереди мониторить:

```sql
SELECT status,count(*),min(created_at) FROM notification_outbox GROUP BY status;
SELECT count(*) FROM notification_outbox
WHERE status IN ('pending','sending') AND next_attempt_at < now()-interval '5 minutes';
```

Не логировать payload, reset code, invitation URL, email или DSN. Failed сообщения не удалять/не ретраить массово без разбора причины. Доставка at-least-once допускает редкий повтор после crash; idempotency заявки защищается отдельно.

Очередь принимает максимум 10000 pending/sending сообщений. Настроить предупреждение задолго до насыщения, например по устойчивому росту возраста самого старого pending. При насыщении создание критической заявки откатывается с ошибкой; необходимо восстановить доставку, а не удалять очередь для освобождения места.

`/health/live` и `/health/ready` должны вернуть 204, ready — 503 при недоступной БД. Проверить точное имя live route в main.go перед настройкой балансировщика. При SIGTERM сервер завершает HTTP до 30с, затем ожидает bounded SMTP workers; `TimeoutStopSec=150` для systemd, проверить фактическое завершение на стенде. Не принимать readiness как подтверждение бизнеса/SMTP.

## 6. Применение и откат

Только после закрытия блокеров и отдельного разрешения на deployment: backup/restore gate, короткое окно записи, подготовленный release, один мигрирующий запуск, проверка readiness/logs, переключение proxy, smoke регистрации/входа/карточек/заявки/test attempt, наблюдение errors/latency/queue/pool.

Сохранить прежний бинарник, assets и env. Откат application release возможен только после проверки его совместимости с дополненной схемой; новые таблицы/колонки оставлять, down migration/restore поверх новых пользовательских записей не выполнять. Старый release может вернуть уязвимости и игнорировать expiry/idempotency: использовать maintenance и исправляющий релиз, если безопасный совместимый rollback не подтверждён. Восстановление backup — отдельная аварийная процедура с оценкой потери новых данных, не обычный rollback.
