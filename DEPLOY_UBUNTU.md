# Развёртывание FinTalent на Ubuntu

## 1. Подготовка

Установите Go 1.26 или соберите Linux-бинарник заранее. Приложение должно запускаться из корня проекта: статические файлы и загружаемые документы используют относительные каталоги.

```bash
git clone https://github.com/zpvision/fintalent.git
cd fintalent
cp .env.example .env
npm ci
npm run build
go build -o fintalent .
```

`npm run build` собирает legacy-бандл Editor.js и React/Vite frontend в `static/react/`. Без этого шага Go безопасно вернёт старые HTML-страницы для уже мигрированных маршрутов, но React-версия в production включена не будет. Для аварийного отката React-маршрутов без новой сборки установите `REACT_FRONTEND=false` и перезапустите приложение.

Обязательные настройки `.env` для сервера:

```dotenv
APP_ENV=production
DATABASE_URL=postgres://USER:PASSWORD@HOST:5432/fintalent?sslmode=require
PORT=8080
ADMIN_LOGIN=your-admin-login
ADMIN_PASSWORD=use-a-long-random-password
COOKIE_SECURE=true
SEED_DEMO_DATA=true
SYNC_GEOGRAPHY=false
```

Используйте только удалённую облачную PostgreSQL. Локальная БД и `localhost` не поддерживаются рабочим процессом проекта. Режим SSL укажите согласно требованиям облачного провайдера. После презентации можно установить `SEED_DEMO_DATA=false`.

## 2. Systemd

Создайте `/etc/systemd/system/fintalent.service`:

```ini
[Unit]
Description=FinTalent
After=network-online.target postgresql.service
Wants=network-online.target

[Service]
Type=simple
User=www-data
Group=www-data
WorkingDirectory=/opt/fintalent
ExecStart=/opt/fintalent/fintalent
Restart=always
RestartSec=5
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
