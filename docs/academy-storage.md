# Закрытое хранилище Академии

Обновление проверки 04.11.2026: базовый live-тест Yandex Cloud выполнен успешно с локальным окружением: HeadBucket, запись/метаданные/чтение, подписанный GET с Range (206), удаление и проверка отсутствия. Использован только случайный `integration-tests/{run_id}/sample.txt`; тестовый объект удалён. Это не заменяет проверку multipart/CORS/видео на реальном bucket.

`TestLiveS3IsolatedPrefix` запускается только с явным `RUN_S3_LIVE_TESTS=1`, остальное окружение — из защищённого окружения процесса. Go test самостоятельно `.env` не читает. Не передавайте ключи в командной строке.

Общий `objectstorage.UserKey(userID, category)` поддерживает academy/tests/files. Академия сохраняет новые S3-материалы по полным ключам `users/{storage_owner_id}/academy/{random48hex}`. Оригинальное имя хранится отдельно. Миграция 089 фиксирует storage_owner_id из создателя файла при INSERT и защищает от последующей замены. Передача курса не меняет владельца хранения или ключ. API принимает ID материала, а не пользовательский префикс. Остальные модули не переключены на S3.

## Архитектура и границы

`internal/objectstorage` содержит интерфейсы Store/Multipart и реализации Local/S3. Только S3-адаптер импортирует AWS SDK for Go v2. Yandex — первый провайдер, endpoint и bucket не входят в модель курса. Применяется подпись AWS Signature V4, проверки CRC SDK включены только для операций, где они обязательны: дополнительные AWS checksum-заголовки не предполагаются совместимыми со всеми провайдерами.

Приложение **не создаёт bucket, IAM, CORS или lifecycle**. В production обязательны `APP_ENV=production` и `ACADEMY_STORAGE_DRIVER=s3`. Неполная конфигурация или неуспешный HeadBucket прерывают запуск. Ошибка S3 никогда не включает LocalStorage автоматически. Ключи/секреты/подписанные URL не логируются приложением.

Файлы прежних версий имеют `storage_driver=local`. Для них сохраняется ACADEMY_STORAGE_DIR. Новые материалы и файлы ответов получают выбранный driver; новые CSV/XLSX-отчёты также записываются в S3. Временные файлы проверки/экспорта допускаются на локальном диске и удаляются после операции. Видео не записывается на сервер приложения.

Полный ключ нового S3-материала хранится в storage_key. Новые пользовательские отчёты также используют `users/{id}/academy/` с расширением CSV/XLSX. Прежние 48-символьные ключи разрешаются в `academy/media/{key}`, прежние отчёты — в `academy/reports/{key}`. Локальные записи используют прежний каталог. Существующие объекты не перемещаются. Связи и неизменяемые программы продолжают ссылаться на ID материалов.

Миграция 088 добавляет метаданные хранилища, checksum и очередь очистки, upload sessions; существующие файлы/версии не изменяются. Применение — обычная embed → prepareAcademyDatabase → prepareDatabase цепочка. Миграции файлов при запуске нет.

## Настройка Yandex Cloud администратором

Основные официальные источники (проверены 04.11.2026):

- [Создание bucket](https://yandex.cloud/ru/docs/storage/operations/buckets/create).
- [S3 API и аутентификация](https://yandex.cloud/ru/docs/storage/s3/), [подписание](https://yandex.cloud/ru/docs/storage/s3/signing-requests).
- [Сервисный аккаунт](https://yandex.cloud/ru/docs/iam/operations/sa/create), [статические ключи](https://yandex.cloud/ru/docs/iam/operations/authentication/manage-access-keys).
- [IAM для bucket](https://yandex.cloud/ru/docs/storage/operations/buckets/iam-access), [bucket policy](https://yandex.cloud/ru/docs/storage/operations/buckets/policy), [действия политики](https://yandex.cloud/ru/docs/storage/s3/api-ref/policy/actions).
- [Multipart](https://yandex.cloud/ru/docs/storage/s3/api-ref/multipart), [UploadPart](https://yandex.cloud/ru/docs/storage/s3/api-ref/multipart/uploadpart), [Complete](https://yandex.cloud/ru/docs/storage/s3/api-ref/multipart/completeupload).
- [CORS](https://yandex.cloud/ru/docs/storage/operations/buckets/cors), [lifecycle](https://yandex.cloud/ru/docs/storage/operations/buckets/lifecycles).
- [Range](https://yandex.cloud/ru/docs/storage/s3/api-ref/object/get), [временные ссылки](https://yandex.cloud/ru/docs/storage/concepts/pre-signed-urls).

1. В своём облаке/каталоге вручную создайте bucket STANDARD в регионе России. Установите лимит объёма/бюджет и уведомления о расходах. Отдельный bucket для production и тестов.
2. Доступ к чтению объектов, списку объектов и настройкам bucket — **ограниченный**, не публичный. Не включайте website hosting и ACL `public-read`. Не назначайте права `allUsers`/`allAuthenticatedUsers`. Проверьте, что анонимный GET известного объекта возвращает 403.
3. Создайте отдельный сервисный аккаунт backend. Управлять политикой, CORS, lifecycle и резервными копиями должен отдельный административный аккаунт. Не выдавайте backend роль администратора каталога/облака.
4. Минимальный рабочий набор разрешений задайте политикой именно этого bucket. Подставьте ID сервисного аккаунта (не access key ID):

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "AcademyObjects",
      "Effect": "Allow",
      "Principal": {"CanonicalUser": "SERVICE_ACCOUNT_ID"},
      "Action": ["s3:GetObject", "s3:PutObject", "s3:DeleteObject", "s3:AbortMultipartUpload", "s3:ListMultipartUploadParts"],
      "Resource": ["arn:aws:s3:::BUCKET/academy/*", "arn:aws:s3:::BUCKET/users/*/academy/*"]
    },
    {
      "Sid": "AcademyBucketCheck",
      "Effect": "Allow",
      "Principal": {"CanonicalUser": "SERVICE_ACCOUNT_ID"},
      "Action": ["s3:ListBucket"],
      "Resource": ["arn:aws:s3:::BUCKET"]
    }
  ]
}
```

HeadBucket требует разрешения на bucket; ListParts/Abort — на объект. Настройка bucket policy влияет и на административный доступ через консоль: сохраните отдельные права обслуживающего администратора, следуя официальной инструкции. Для этой реализации не нужны права backend менять ACL, CORS, lifecycle или создавать bucket. Не добавляйте публичный Allow ради исправления 403. Возможный альтернативный способ настройки — scoped IAM на bucket; проверяйте итоговые эффективные права, не выдавайте широкую роль на каталог.

5. В сервисном аккаунте создайте **статический ключ доступа Object Storage**. Сохраните ID и секрет в защищённом окружении backend, вне Git; секрет отображается при создании. IAM API key и обычный OAuth-токен не заменяют эти два значения в данной реализации. Ротация: создайте новый ключ, замените env, перезапустите backend по согласованной процедуре, проверьте доступ и отзовите старый.
6. Не включайте Object Lock по умолчанию для этого bucket: Yandex требует Content-MD5 для UploadPart при таких блокировках, а текущий browser uploader его не вычисляет. Не смешивайте это с версионированием bucket — версионирование можно включить для защиты от случайных удалений.

## Окружение

```dotenv
APP_ENV=production
ACADEMY_STORAGE_DRIVER=s3
ACADEMY_STORAGE_DIR=/var/lib/fintalent/academy
S3_ENDPOINT=https://storage.yandexcloud.net
S3_REGION=ru-central1
S3_BUCKET=YOUR_PRIVATE_BUCKET
S3_ACCESS_KEY=SET_OUTSIDE_GIT
S3_SECRET_KEY=SET_OUTSIDE_GIT
S3_FORCE_PATH_STYLE=true
S3_PRESIGN_TTL_SECONDS=300
ACADEMY_VIDEO_MAX_MB=2048
```

TTL допускается 60–900 секунд, по умолчанию 300. Максимум MP4 по умолчанию 2048 МиБ, допустимая конфигурация 1–10240 МиБ. Лимит документов/изображений и ответов учеников остаётся 20 МиБ (может быть уменьшен настройками Академии). Эти настройки не увеличивают HTTP upload limit для старых endpoint: большое видео загружается напрямую в S3.

Локальная разработка: `APP_ENV=development`, `ACADEMY_STORAGE_DRIVER=local`. Большое MP4 требует S3 или тестового S3-совместимого окружения; local продолжает обслуживать прежние документы/изображения и автоматизированные тесты. Нельзя включать local в production как способ замаскировать недоступность S3.

Для доступа к старым файлам ACADEMY_STORAGE_DIR должен указывать на прежний каталог. Новая конфигурация S3 не переносит их и не очищает автоматически. Каталог и временная директория ОС должны быть доступны процессу backend, не публикуются Nginx.

## CORS

Повторная реальная проверка после настройки bucket: CORS preflight OPTIONS и подписанные PUT разрешены для https://fintalent.ru и http://localhost:8080. Полный live-тест прошёл: multipart, проверка содержимого, изображения/PDF, signed GET/Range, истечение ссылки и удаление тестовых объектов. Это HTTP-проверка CORS, не отдельный запуск браузера против реального bucket.

Для браузера разрешите только фактические origin frontend. Пример AWS CLI-совместимого `cors.json`:

```json
{
  "CORSRules": [{
    "AllowedOrigins": ["https://fintalent.ru"],
    "AllowedMethods": ["GET", "PUT", "HEAD"],
    "AllowedHeaders": ["content-type", "content-md5", "range", "x-amz-*"],
    "ExposeHeaders": ["ETag", "Content-Length", "Content-Range", "Accept-Ranges"],
    "MaxAgeSeconds": 300
  }]
}
```

```sh
aws --endpoint-url=https://storage.yandexcloud.net s3api put-bucket-cors --bucket YOUR_PRIVATE_BUCKET --cors-configuration file://cors.json
```

Команду выполняет администратор, не приложение. Не используйте `*` вместо production origin. Origin dev/localhost добавляйте в отдельном тестовом bucket. POST Complete и DELETE Abort делает backend, браузеру такие методы S3 не нужны. CORS не заменяет приватность и IAM. При внешнем CSP добавьте **точный** S3 origin в `connect-src`, `media-src` и при необходимости `img-src`; не разрешайте произвольные домены. В приложении CSP сейчас report-only, настройки внешнего reverse proxy нужно проверять отдельно.

## Lifecycle

Дополнительная страховка от неизвестного результата CreateMultipartUpload и выключенного приложения:

```json
{"Rules":[{"ID":"AcademyIncompleteUploads","Status":"Enabled","Filter":{"Prefix":"academy/"},"AbortIncompleteMultipartUpload":{"DaysAfterInitiation":2}}]}
```

Загрузить вручную через `aws --endpoint-url=https://storage.yandexcloud.net s3api put-bucket-lifecycle-configuration --bucket YOUR_PRIVATE_BUCKET --lifecycle-configuration file://lifecycle.json`.

Не добавляйте Expiration на готовые учебные материалы: они могут использоваться неизменяемыми программами. Очистка отчётов выполняется по сроку в БД. Версионирование сохраняет нетекущие версии при удалении: их хранение и стоимость требуют отдельной осознанной политики администратора, не автоматического удаления вместе с незавершёнными multipart.

## API и загрузка

Существующие API upload/download сохраняются. Автор курса с общим допуском к Курсам использует:

- GET `/api/academy/courses/{id}/assets/uploads` — возможности и лимиты.
- POST того же URL — `{request_key, entity_key, name, size}`: резервирование сессии и object key.
- GET `.../uploads/{session}` — статус и реально загруженные части; готовый asset только после проверки.
- POST `.../uploads/{session}/parts/{number}` — короткая подписанная ссылка PUT с подписанным Content-Length.
- POST `.../uploads/{session}/complete` — постановка в проверку, HTTP 202.
- DELETE `.../uploads/{session}` — отмена ещё не финализируемой сессии.

Сессия действует 24 часа, части по 32 МиБ; до трёх параллельных частей и четырёх попыток на часть с задержкой. При сетевом сбое UI сохраняет только session ID, не подписанные ссылки; повторный выбор файла с теми же name/size/lastModified продолжает сесссию. Это идентификация выбранного файла для UX, а не криптографическое доказательство тождественности: пользователь должен выбирать исходный файл.

Worker получает ListParts у S3, проверяет последовательность/размеры всех частей, выполняет Complete, HeadObject, MIME/сигнатуру содержимого, затем повторно проверяет владельца, допуск и наличие урока. До этого asset отсутствует в программе. Потерянный успешный ответ Complete восстанавливается через HeadObject без второго материала. Незавершённые/отменённые сессии прерываются worker; ошибки очистки повторяются с ограниченной задержкой. Object key никогда не поступает от браузера.

Для MP4 проверяются размер, Content-Type и контейнерная сигнатура `ftyp`; транскодирования/антивируса/проверки всех кодеков нет. Для совместимого воспроизведения подготовьте MP4 H.264/AAC с faststart. Сама расширенная проверка/транскодирование — будущая отдельная очередь. Документы проверяются по содержимому, Office — по структуре ZIP без макросов.

ETag multipart **не является SHA-256 всего файла** и не используется вместо контрольной суммы. После сборки worker потоково читает объект из S3 по Range порциями до 32 МиБ, сохраняет смещение и состояние SHA-256 в PostgreSQL и только после завершения проверки создаёт ready asset с checksum_kind=sha256. RAM ограничена буфером io.Copy, перезапуск продолжает проверку со смещения. Это дополнительное полное чтение S3 для проверки целостности (его трафик/стоимость нужно учитывать), но не проксирование браузерной загрузки или воспроизведения через backend. Вся загрузка браузера идёт непосредственно в S3. Для независимого переноса всё равно нужен сверенный manifest, а не сравнение ETag разных провайдеров.

## Проверка после настройки (выполняет администратор)

1. Запустить приложение с настроенным private bucket. Убедиться, что неправильные ключи/endpoint не приводят к local fallback.
2. Автору дать допуск к Курсам. Создать/сохранить урок, загрузить документ и MP4 больше 32 МиБ. Проверить запросы PUT непосредственно в S3, прогресс, повтор части после временного отключения сети, отмену, повторный выбор файла.
3. Зафиксировать/опубликовать версию; зачислить тестового ученика с общим допуском. Проверить воспроизведение, перемотку (206 от S3), повторное получение ссылки после истечения TTL, скачать документ.
4. Отозвать зачисление/допуск, проверить отказ через API и прямой URL приложения. Урок по расписанию до открытия не выдаёт ссылку. Другой пользователь не получает материалы/сессии/файлы работы.
5. Сформировать CSV/XLSX и проверить закрытое скачивание владельцем отчёта. Результат теста не меняется.
6. Скопированная presigned URL работает до её истечения — это bearer-ссылка. Для будущего строгого доступа интерфейс Store.Open поддерживает Range, поэтому можно добавить backend-proxy с проверкой на каждый запрос. Сейчас видеотрафик идёт напрямую, DRM/защита от скачивания не заявляются.

Ключи читаются только из защищённого локального окружения. Облачные ресурсы не создаются. Реальный multipart (две части, первая 5 МиБ), signed PUT, Complete, чтение и удаление тестовых объектов проверены. CORS текущего bucket повторно проверен для production и localhost:8080; live-тест прошёл. Mock/изолированные тесты не заменяют браузерный smoke-тест реального bucket.

## Перенос старых материалов

Сделайте резервную копию БД и прежнего каталога. Запускайте команду с окружением backend (DATABASE_URL и S3-переменными), из корня репозитория:

```sh
go run ./cmd/academy-storage-migrate --local-root=/var/lib/fintalent/academy
go run ./cmd/academy-storage-migrate --local-root=/var/lib/fintalent/academy --apply
```

Первый запуск — dry run. `--apply` копирует только активные local assets, проверяет размер и SHA-256 повторным чтением из S3 и только затем меняет driver/checksum в БД. При ошибке исходная запись остаётся local. Повторный запуск пропускает уже переведённые записи. Исходные файлы не удаляются, включая работу очереди GC. Старые отчёты остаются local до их штатного истечения, это производные файлы и не часть миграции материалов.

Не менять legacy-каталог на пустой и не удалять source после одного успешного запроса. Сохраните копию каталога до проверенного резервного цикла и отдельного решения администратора об удалении.

## Резервное копирование и восстановление

Бэкап БД должен включать все academy_* таблицы, связи с courses/users и метаданные. Отдельно резервируйте объекты, прежний local-каталог, конфигурацию без утечки credentials. Bucket versioning помогает восстановить случайные удаления, но не является независимым бэкапом. Используйте отдельный backup account/bucket и регулярно проверяйте восстановление на изолированном окружении. Ключи восстанавливаются отдельно из менеджера секретов.

## Другой S3-провайдер и откат

1. Создать закрытый целевой bucket, IAM и CORS; проверить API/Range/multipart на тестовых объектах. Объекты копировать с сохранением **полных object keys** и Content-Type. URL из БД переписывать не нужно.
2. Для большого объёма выполнить предварительное копирование администраторским инструментом, построить независимый SHA-256 manifest (не сравнивать multipart ETag разных провайдеров).
3. На финальное окно остановить новые upload sessions, дождаться/отменить незавершённые и финализируемые загрузки. Приостановить workers/приложение согласованным деплоем. Проверить/дренировать очередь удалений. Это нужно, чтобы финальный delta-copy не гонялся с загрузками/GC.
4. Докопировать изменения, проверить количество, размеры и manifest. Переключить endpoint/region/bucket/keys, перезапустить backend, проверить авторский preview, обучение, файлы ответов, отчёты. Незавершённые старые upload sessions имеют fingerprint backend и не продолжаются в другом bucket.
5. Сохранить исходное хранилище и конфигурацию на период отката. Перед возвратом скопировать и проверить новые объекты, созданные на новом провайдере. Простое переключение endpoint назад без delta-sync потеряет доступ к новым материалам.

Откат к local возможен только для записей, чьи файлы реально восстановлены по соответствующим ключам и проверены. Нельзя ставить всем storage_driver=local: новые MP4 там отсутствуют. Не откатывайте схему удалением таблиц/исторических результатов; миграция 088 forward-only и совместима с прежними local записями.

## Диагностика

- Ошибка запуска: проверьте APP_ENV/driver, HTTPS endpoint, регион, bucket, статический ключ, HeadBucket. Не публикуйте env или подписанную ссылку в тикете.
- Браузерный CORS: точный origin, PUT/GET/HEAD, заголовки и разрешённый endpoint в CSP. Смотрите status без копирования секретных query в логи.
- SignatureDoesNotMatch: регион ru-central1 для Yandex, корректные часы сервера, endpoint, отсутствие изменения подписанных заголовков/Content-Length.
- Проверка зависла: статус session, expires_at/lease_until, worker и IAM на ListParts/Head/Get/Complete. `failed` не означает готовый материал.
- Старый файл 404: прежний ACADEMY_STORAGE_DIR, права процесса, существование исходного файла. Не включайте автоматический fallback S3→local.
- Рост объёма: просмотрите незавершённые multipart и lifecycle, повторные GC jobs, отчёты, нетекущие версии bucket. Не удаляйте объекты по одному только возрасту.

Логи показывают только класс операции/сбой, без SDK request URLs. Поля attempts/next_at в GC и status/lease в sessions пригодны для подключения мониторинга; точные содержимое файлов и credentials в метрики не включаются.

## Изменённые файлы

| Область | Файлы |
| --- | --- |
| Универсальный адаптер | `internal/objectstorage/storage.go`, `config.go`, `local.go`, `s3.go`, `storage_test.go` |
| Интеграция Академии | `internal/academy/storage.go`, `upload_sessions.go`, `upload_sessions_test.go`, `media.go`, `assignment_handler.go`, `analytics_reports.go`, `handler.go`, `domain.go`, `learning_domain.go`, `repository.go` |
| Запуск и worker | `academy_object_storage.go`, `academy_module.go`, `academy_analytics_worker.go`, `runtime_lifecycle.go` |
| Миграция схемы | `migrations/088_academy_object_storage.sql` |
| Явный перенос файлов | `cmd/academy-storage-migrate/main.go` |
| Frontend | `frontend/src/api/academyUploads.js`, `frontend/src/pages/academy/UploadedVideo.jsx`, `AcademyBlockEditor.jsx`, `LessonRenderer.jsx` |
| Интеграционные проверки | `academy_storage_integration_test.go`, `scripts/academy-browser-check.cjs` |
| Конфигурация и документация | `.env.example`, `go.mod`, `go.sum`, `docs/academy-storage.md`, `docs/CODEX_CONTEXT.md` |

## Выполненные проверки

- `go test ./...` и `go vet ./...` — успешно.
- `npm run build` — успешно; сохраняется предупреждение Vite о размере общего JS bundle.
- LocalStorage и AWS SDK v2 с TLS mock: потоковая запись/чтение, Range, метаданные, удаление, multipart, подпись размера части, повтор после 503, безопасные ошибки и смена fingerprint провайдера.
- `TestAcademyObjectStorageIsolatedPostgres` — успешно, в отдельной временной схеме удалённой тестовой PostgreSQL: IDOR сессий/материалов, неправильные части, отмена, восстановление после потерянного Complete, продолжение SHA-256 после перезапуска, миграция при сбое/повреждении/успехе, сохранение локального оригинала, защита используемого файла, GC неиспользуемого файла, закрытые CSV/XLSX и очистка истёкших отчётов.
- Существующие `TestCoursesIsolatedPostgres` и `TestAcademyAnalyticsIsolatedPostgres` — успешно в изолированных схемах, включая прежние сценарии обучения, ответов с файлами и отчётов.
- `node scripts/academy-browser-check.cjs` — успешно на 360/900/1440 px: прежний редактор, закрытые изображения/документы, все восемь типов блоков, multipart с повтором неудачных частей и продолжением, отмена, воспроизведение настоящего MP4 через общий renderer. Видеофикстура создаётся локально браузером; тест не обращается к облаку.

Проверка настоящего Yandex bucket, IAM/CORS, сетевых ограничений production и больших видео в реальной сети остаётся обязательным smoke-тестом администратора после настройки. Подписанная ссылка допускает доступ до своего истечения; немедленный отзыв уже выданной ссылки не гарантируется. HLS/CDN/DRM, транскодирование, антивирус и автоматическая межпровайдерная репликация в этот этап не входят.

## Постоянный владелец и учёт объёма

Миграция 089 расширяет storage_key до TEXT и добавляет storage_owner_id в материалы и отчёты. Прежние записи используют исторический created_by. Это атрибуция создателю загрузки, не финансовые квоты. Ответы учеников относятся к ученикам. Для общих файлов организаций нужен отдельно согласованный scope, а не случайный сотрудник.

SELECT storage_owner_id, storage_driver, SUM(size) FROM academy_media_assets WHERE deleted_at IS NULL GROUP BY storage_owner_id,storage_driver;

Это объём активных материалов; multipart, отчёты, версии bucket и ожидающие GC объекты считаются отдельно.

## Замена ключей

Создайте новый статический ключ существующего сервисного аккаунта. Замените S3_ACCESS_KEY и S3_SECRET_KEY только в защищённом .env. По согласованному деплою перезапустите backend, проверьте загрузку и чтение, затем отзовите старый ключ с учётом активных загрузок и TTL подписанных URL. При компрометации отзывайте сразу. В .env.example оставляйте пустые значения. Текущие ключи не найдены в достижимой истории локальных Git-веток; сторонние клоны, логи и бэкапы этим не проверяются.
