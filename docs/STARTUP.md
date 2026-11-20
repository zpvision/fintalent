# Первый этап: диагностика запуска

Этот этап добавляет только замеры. Обычный запуск сохраняет полный синхронный
проход prepareDatabase, порядок SQL, seeds, advisory lock, проверку S3,
регистрацию маршрутов и прежний порядок запуска workers перед HTTP listener.
DB_STARTUP_MODE не задан или auto: прежний путь. Другие значения отклоняются,
чтобы случайно не принять verify за включённый режим. Быстрый режим и команда
миграции в этот этап не входят. Нет новых migrations, таблиц, отметок схемы,
хеширования бинарника или дополнительных SQL-запросов.

Production .env, БД, systemd, Nginx и updater не менялись. Readiness-скрипты из
предыдущего исследования не входят в диагностический коммит.

## Формат журнала

```text
startup stage=database.advisory_lock_wait event=begin
startup stage=database.advisory_lock_wait event=end duration_ms=52.123
startup stage=http.listening event=bound
```

Пары begin/end фиксируют длительность, включая неуспешную попытку. End не
означает успех. Отдельная строка event=failed содержит только безопасный
error_code: SQLSTATE из пяти символов, deadline_exceeded, canceled либо
operation_failed. Исходный текст ошибки соединения не печатается: он может
содержать DSN/пароль. Имена этапов заданы константами, значения env, SQL,
токены, ключи S3, адреса пользователей в диагностические записи не передаются.

Этапы:

- config: чтение .env и проверка конфигурации;
- database.open_pool: sql.Open и параметры пула;
- database.total: весь прежний prepareDatabase, включая освобождение lock;
- database.connect_ping, database.lock_connection, database.advisory_lock_wait;
- database.schema_total и каждый database.prepare*: inline schema, модули,
  вложенные справочники, условные demo seeds;
- routes: регистрация всех маршрутов; вложенные storage.initialize и
  storage.s3_check показывают отдельную длительность синхронной проверки S3;
- pre_http: накопленное время от входа в main до создания HTTP server;
- workers.total и workers.notification_outbox/cohorts/storage/communications/analytics;
- http.listen: фактический net.Listen; http.listening event=bound появляется
  только после успешного открытия порта. Это не результат GET /health/ready.

Вложенные длительности входят в родительские: складывать все строки нельзя.
Старое сообщение «FinTalent запущен» остаётся прежним и не доказывает bind;
для открытия порта используйте новый event=bound.

## Команды после будущего разрешённого развёртывания

Выполнять на сервере с доступом к systemd journal. Команды только читают журнал,
не перезапускают сервис, не запускают приложение или миграции.

```bash
# Только текущий запуск сервиса.
invocation_id="$(systemctl show fintalent -p InvocationID --value)"
test -n "$invocation_id" || { echo 'Нет InvocationID сервиса' >&2; exit 1; }
sudo journalctl "_SYSTEMD_INVOCATION_ID=$invocation_id" --no-pager -o cat |
  grep 'startup stage='

# Только завершённые замеры текущего запуска.
sudo journalctl "_SYSTEMD_INVOCATION_ID=$invocation_id" --no-pager -o cat |
  grep 'startup stage=' | grep 'event=end'

# Ожидание lock, схема, S3 и открытие порта отдельно.
sudo journalctl "_SYSTEMD_INVOCATION_ID=$invocation_id" --no-pager -o cat |
  grep -E 'startup stage=(database\.(connect_ping|advisory_lock_wait|schema_total)|storage\.|routes|http\.)'

# Следить за новыми записями, в том числе при следующем запуске.
sudo journalctl -u fintalent -f -o cat |
  grep --line-buffered 'startup stage='
```

## Проверки и стоимость

Unit-тесты проверяют отсутствие секретов в success/failure logs, безопасные
SQLSTATE, постоянные имена этапов, default auto, неизменность исходного SQL
и порядка prepare-вызовов, порядок schema/routes/workers/listener и bind logs.
Listener в тестах подменён; приложение, реальные workers и PostgreSQL не запускаются.

Локальный benchmark: один timer (две записи с форматированием) — около 414 нс,
80 байт, 3 allocations, с принимающим записи тестовым writer. Это не замер
systemd journal. При примерно 100 строках на обычный запуск CPU/allocations
незначительны; стоимость записи в stderr зависит от journal. Дополнительных
DB/network calls, фонового сборщика метрик и логирования каждого запроса нет.

До первого этапа read-only probe облачной БД с машины разработки дал 275 мс
connection+ping и около 53 мс median warm SELECT 1. Production запуск и его
S3/DDL/lock здесь не измерялись. Назначение этого коммита — получить точный
профиль на следующем отдельно разрешённом деплое, не ускорять его пропуском SQL.

## Файлы диагностического коммита

- main.go, runtime_lifecycle.go, startup.go, academy_object_storage.go;
- startup_test.go, runtime_lifecycle_test.go, docs/STARTUP.md;
- таймеры в academy_module.go, accounting_company_module.go, admin.go,
  admin_sessions.go, client_exchange_module.go, contact_messages.go,
  courses_module.go, employee_testing.go, geography.go, help_module.go,
  marketplace.go, notification_outbox.go, okved.go, password_reset.go,
  profile_purpose.go, profimarket.go, publication_module.go,
  shared_object_files.go, survey.go, test_categories.go, testing_module.go,
  vacancy_module.go, vacancy_survey.go.
