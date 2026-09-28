# Развёртывание FinTalent на Ubuntu

## 1. Подготовка

Установите Go 1.26.8 или более новый проверенный security patch поддерживаемой ветки либо соберите Linux-бинарник заранее. Перед выпуском выполните gates и порядок из [PRELAUNCH_RELEASE.md](PRELAUNCH_RELEASE.md); локальные результаты и незакрытые блокеры находятся в [PRELAUNCH_STATUS.md](PRELAUNCH_STATUS.md). Приложение должно запускаться из корня проекта: статические файлы и загружаемые документы используют относительные каталоги.

```bash
git clone https://github.com/zpvision/fintalent.git
cd fintalent
cp .env.example .env
npm ci
npm run build
go build -o fintalent .
```

`npm run build` собирает legacy-бандл Editor.js и React/Vite frontend в `static/react/`. В production отсутствие React-сборки останавливает запуск, если React явно не выключен. `REACT_FRONTEND=false` сохраняет явный legacy fallback; этот режим тоже необходимо проверить перед применением.

Обязательные настройки `.env` для сервера:

```dotenv
APP_ENV=production
APP_BASE_URL=https://fintalent.ru
DATABASE_URL=postgres://USER:PASSWORD@HOST:5432/fintalent?sslmode=require
PORT=8080
ADMIN_LOGIN=your-admin-login
ADMIN_PASSWORD=use-a-long-random-password
COOKIE_SECURE=true
SEED_DEMO_DATA=false
SYNC_GEOGRAPHY=false
TRUSTED_PROXY_CIDRS=127.0.0.1/32,::1/128
DB_MAX_OPEN_CONNS=16
```

Задайте случайный PASSWORD_RESET_SECRET (не менее 32 символов), ADMIN_PASSWORD (не менее 16), SMTP через приватную конфигурацию. Значения-подсказки выше не являются безопасными паролями. Используйте только удалённую облачную PostgreSQL. Режим TLS укажите согласно требованиям облачного провайдера. Production всегда запускайте с SEED_DEMO_DATA=false и SYNC_GEOGRAPHY=false.

## 2. Systemd

Создайте `/etc/systemd/system/fintalent.service`:

```ini
[Unit]
Description=FinTalent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=www-data
Group=www-data
WorkingDirectory=/opt/fintalent
ExecStart=/opt/fintalent/fintalent
Restart=always
RestartSec=5
TimeoutStopSec=150
EnvironmentFile=/opt/fintalent/.env
NoNewPrivileges=true
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

Каталоги с загрузками должны быть доступны на запись:

```bash
sudo mkdir -p /opt/fintalent/uploads/resume-certificates
sudo mkdir -p /opt/fintalent/static/uploads/position-icons
sudo chown -R www-data:www-data /opt/fintalent/uploads /opt/fintalent/static/uploads
sudo systemctl daemon-reload
sudo systemctl enable --now fintalent
sudo journalctl -u fintalent -f
```

## 3. Nginx

Проксируйте запросы на `127.0.0.1:8080`, передавая `Host`, `X-Real-IP`, `X-Forwarded-For` и `X-Forwarded-Proto`. HTTPS обязателен при `COOKIE_SECURE=true`.

Приложение самостоятельно применяет идемпотентные изменения схемы при запуске. Перед первым запуском на сервере всё равно сделайте резервную копию существующей базы.
