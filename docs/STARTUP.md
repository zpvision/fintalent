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

## Уточнение: время SQL-скриптов Академии

Production-замер после первого этапа: prepareAcademyDatabase 62 197,645 мс,
schema_total 62 747,312 мс, database.total 62 766,018 мс, S3 check 154,084 мс,
pre_http 62 925,566 мс, listener 1,286 мс. Академия занимает около 99,1% времени
подготовки схемы. Разбивка ниже добавляет только 13 пар журналирования вокруг
существующих ExecContext: SQL, контекст, порядок, ошибки, транзакции и lock
не изменены; новых SQL-запросов нет. End пишется и при ошибке перед её возвратом.

| Этап после database.academy. | SQL-файл в migrations/ |
| --- | --- |
| schema | 078_academy.sql |
| learning | 079_academy_learning.sql |
| assessment | 080_academy_assessments.sql |
| cohorts | 081_academy_cohorts.sql |
| communications | 082_academy_communications.sql |
| analytics | 083_academy_analytics.sql |
| admin | 084_academy_admin.sql |
| publication_defaults | 085_academy_publication_defaults.sql |
| object_storage | 088_academy_object_storage.sql |
| storage_owners | 089_academy_storage_owners.sql |
| manual_payments | 092_course_manual_payments.sql |
| program_revisions | 093_academy_program_revisions.sql |
| default_automations | 094_academy_default_automations.sql |

Исходный анализ повторяемой работы до оптимизации 093 (на этапе диагностики
SQL не менялся и не выполнялся; номера строк ниже относятся к исходному файлу):

- **093, строки 43–56:** INSERT SELECT для всех версий, drafts и начальных
  revisions. Для revisions вычисляется коррелированный jsonb_agg(to_jsonb(a))
  по определениям проверок. ON CONFLICT DO NOTHING не гарантирует отсутствия
  работы по построению SELECT/JSON для уже существующих строк.
- **093, строки 113–114:** UPDATE drafts с test_sources='{}', использующий
  academy_current_assessment_versions и для EXISTS, и для JSON aggregation.
  Представление разворачивает JSON modules/lessons/blocks, проверяет publication
  revisions и отсутствие более свежих определений. Это приоритетный кандидат
  для дальнейшего разбора плана после подтверждения медленного скрипта.
- **089:** два безусловных ALTER COLUMN storage_key TYPE TEXT, UPDATE владельцев
  там, где они ещё NULL, DROP/CREATE двух триггеров. UPDATE может сканировать
  таблицы; DDL может ждать relation locks. Сам ALTER TYPE не доказывает, что
  таблица переписывается при каждом запуске, особенно если тип уже TEXT.
- **094, строка 32:** SELECT academy_seed_course_automations(...) для всех
  courses. Каждый вызов проверяет существующие сценарии по course_id/trigger
  и выполняет INSERT SELECT при их отсутствии. Проверки повторяются даже
  при наличии настроенных сценариев; стоимость зависит от числа курсов,
  сценариев, плана и фактических индексов.
- **092, строки 13–15:** повторный INSERT SELECT всех courses в commission terms
  с ON CONFLICT DO NOTHING. Новых строк может не быть, чтение и проверки
  конфликтов всё равно остаются.
- **078–085, 088, остальные части 089/092/093/094:** создание/проверка объектов,
  ALTER, CREATE OR REPLACE FUNCTION/VIEW, проверки pg_constraint/pg_trigger.
  CREATE INDEX IF NOT EXISTS не перестраивает существующий индекс; дорогая
  постройка возможна при его отсутствии. Обычный CREATE OR REPLACE VIEW не
  рассчитывает все строки представления; его JSON/LATERAL вычисления важны
  там, где представление реально используется запросом, например UPDATE в 093.

Длительность ExecContext включает возможное ожидание блокировок внутри SQL.
Отдельный быстрый advisory_lock_wait не исключает ожидание DDL/DML locks.
Имеющиеся 62 секунды ещё нельзя приписать конкретному скрипту или оператору.
Следующий шаг — получить эти 13 замеров; SQL-оптимизация пока не выполнялась.

После следующего отдельно разрешённого обновления, только чтение журнала:

```bash
invocation_id="$(systemctl show fintalent -p InvocationID --value)"
sudo journalctl "_SYSTEMD_INVOCATION_ID=$invocation_id" --no-pager -o cat |
  grep -E 'startup stage=database\.academy\.[a-z_]+ event=end duration_ms='

# Если запуск ещё выполняется или оборвался, увидеть также begin:
sudo journalctl "_SYSTEMD_INVOCATION_ID=$invocation_id" --no-pager -o cat |
  grep -F 'startup stage=database.academy.'
```

## Оптимизация повторного backfill 093

В 093 изменены только операции первичного заполнения. DDL, функции,
представления, ограничения, порядок запуска скриптов и общий advisory lock
сохранены. Другие SQL-миграции не менялись. Правка именно 093 необходима:
новая миграция после неё не устранит тяжёлую работу, уже выполненную 093.
Нового migration runner, журнала версий и глобального skip-флага нет.

- Первые публикации: дешёвый IF EXISTS проверяет отсутствие revision 1.
  При отсутствии кандидатов тяжёлый INSERT с JSON-агрегацией не исполняется.
  При наличии — MATERIALIZED missing_publications ограничивает агрегацию
  только отсутствующими публикациями; ON CONFLICT остаётся защитой от гонки.
- Черновики: надёжное происхождение определяется самим созданием строки.
  Только отсутствующие drafts получают content и test_sources одним INSERT.
  Существующие строки не UPDATE-ятся вообще, включая '{}' и частичные старые
  состояния. Ни пустой JSON, ни revision, ни совпадение content не используются
  как признак происхождения. Неизвестное содержимое не восстанавливается
  догадкой из опубликованной версии.
- Новые legacy snapshots по-прежнему подхватываются по отсутствующим строкам;
  современные createProgramVersion/publishProgramDraft работают штатно.
  Существующий ненулевой published_revision_id не сбрасывается на revision 1.

### Проверка на настоящей PostgreSQL

По отдельному разрешению использован временный Docker-контейнер из уже
имевшегося official image postgres:17-alpine: сервер PostgreSQL 17.10,
jit=on, расширение только plpgsql. SQL проекта не требует дополнительных
CREATE EXTENSION; полный prepareDatabase успешно выполнен на этом сервере.
PostgreSQL 17 поддерживает используемый NULLS NOT DISTINCT:
[документация](https://www.postgresql.org/docs/17/indexes-unique.html).
Версия и конфигурация production-сервера не запрашивались, их точное совпадение
с контейнером не подтверждено.
Финальная проверка исходников и доступных deployment-метаданных не установила
точную production-версию. Минимальное требование SQL не повышено: исходная 093
уже использовала NULLS NOT DISTINCT (PostgreSQL 15+), а добавленный MATERIALIZED
поддерживается PostgreSQL 12+. См. [PostgreSQL 15](https://www.postgresql.org/docs/15/release-15.html)
и [WITH в PostgreSQL 12](https://www.postgresql.org/docs/12/queries-with.html).

Контейнер: уникальное имя и ownership label, привязка порта только к 127.0.0.1,
случайный свободный порт, отдельная fintalent_revisions_test и отдельный
тестовый пользователь, tmpfs для данных, 2 CPU / 1 GiB RAM. Подключение и пароль
передавались только процессу тестирования; production DATABASE_URL не применялся.
Docker-режим тестового helper требует ACADEMY_REVISIONS_DOCKER=1, loopback,
точные тестовые database/user и непривилегированный порт. По умолчанию helper
никогда не подставляет DATABASE_URL вместо отдельного TEST_DSN.
После проверок подтверждено отсутствие временных тестовых схем. Созданный
контейнер проверен по ID и ownership label, остановлен и удалён; tmpfs-данные
и временный файл с тестовым подключением удалены. Другие контейнеры, образы
и тома не менялись.

SQL-тесты прошли без SKIP:

- EmptyAndRepeat: пустая схема и повторное исполнение;
- MigrationIsolated: переход с legacy схемы, immutable history и прогресс;
- PartialBackfill: существующие публикации 1/2 и head=2, авторский draft
  revision=7 с '{}', отсутствующие drafts/первые публикации;
- повторные полные снимки 12 таблиц, неизменность sequence публикаций;
- тестовый BEFORE UPDATE FOR EACH STATEMENT trigger запрещает даже нулевой
  UPDATE drafts, подтверждая отсутствие массового обновления;
- legacy snapshot после миграции, его публикация, клонирование новой версии,
  повтор до/после публикации;
- существующий ScenariosPostgres: потоки, независимые версии, результаты
  тестов, оценки работ, архивирование, конкурентная публикация, IDOR и файлы.

Последний отдельный замер на небольшой синтетической safety fixture:

| Операция | Длительность |
| --- | ---: |
| Первое исполнение 093 на чистой схеме, внутри полного prepare | 76,221 мс |
| Повтор на пустой базе | 18,628 мс |
| Первичный backfill частичного состояния: 3 версии, 3 проверки | 22,067 мс |
| Три повторных запуска того же набора | 19,707 / 21,010 / 15,659 мс |

Это проверки корректности, не production-size benchmark. Они не доказывают
ускорение в конкретное число раз против production 50 597 мс: размеры данных,
сетевые задержки, файловая система, версия сервера и нагрузка различаются.
Ожидается устранение повторного тяжёлого backfill на инициализированной базе;
DDL и ожидание relation locks остаются. Если действительно отсутствуют legacy
данные, их первичная инициализация всё ещё может быть дорогой.

Остаточное ограничение: существующие пустые test_sources неизвестного происхождения
сохраняются как есть. Автоматическое исправление таких строк намеренно отсутствует.
Изменения SQL перед внедрением требуют обычного review/backup-порядка; сами
production, S3, systemd, Nginx и .env в этой работе не менялись.
