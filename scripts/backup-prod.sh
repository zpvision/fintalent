#!/usr/bin/env bash
set -euo pipefail
umask 077

app_dir=/var/www/fintalent.prod
backup_root=/var/backups/fintalent

if [[ ${EUID} -ne 0 ]]; then
  echo "Запустите скрипт от root" >&2
  exit 1
fi

for tool in python3 pg_dump pg_restore tar sha256sum; do
  command -v "$tool" >/dev/null || { echo "Не найден: $tool" >&2; exit 1; }
done

for path in "$app_dir/.env" "$app_dir/fintalent" \
            "$app_dir/static/react" "$app_dir/static/uploads" "$app_dir/uploads"; do
  [[ -e "$path" ]] || { echo "Не найдено: $path" >&2; exit 1; }
done

install -d -m 0700 "$backup_root"
backup_label="$(date -u +%Y%m%dT%H%M%SZ)-$$"
partial_dir="$backup_root/.${backup_label}-incomplete"
final_dir="$backup_root/$backup_label"
mkdir -m 0700 "$partial_dir"
service_file="$partial_dir/.pg_service.conf"
trap 'rm -f -- "$service_file"; echo "Backup не завершён; файлы остались в $partial_dir" >&2' ERR

python3 - "$app_dir/.env" "$service_file" <<'PY'
import os
import pathlib
import sys
from urllib.parse import parse_qsl, unquote, urlsplit

for line in pathlib.Path(sys.argv[1]).read_text(encoding="utf-8").splitlines():
    if not line.strip() or line.lstrip().startswith("#") or "=" not in line:
        continue
    name, value = line.split("=", 1)
    if name.strip() == "DATABASE_URL":
        value = value.strip()
        if len(value) >= 2 and value[0] == value[-1] and value[0] in "\"'":
            value = value[1:-1]
        if not value:
            raise SystemExit("DATABASE_URL пуст")
        break
else:
    raise SystemExit("DATABASE_URL не найден в .env")

url = urlsplit(value)
if url.scheme not in ("postgres", "postgresql"):
    raise SystemExit("DATABASE_URL должен быть PostgreSQL URI")
params = {
    "host": url.hostname or "",
    "port": str(url.port or 5432),
    "dbname": unquote(url.path.lstrip("/")),
    "user": unquote(url.username or ""),
    "password": unquote(url.password or ""),
}
for key, item in parse_qsl(url.query, keep_blank_values=True):
    if not key.replace("_", "").isalnum() or key in ("service", "servicefile"):
        raise SystemExit("Недопустимый параметр в DATABASE_URL")
    params[key] = item
if not params["host"] or not params["dbname"] or not params["user"]:
    raise SystemExit("В DATABASE_URL отсутствуют host, база или пользователь")
if any("\n" in item or "\r" in item or "\0" in item for item in params.values()):
    raise SystemExit("Недопустимый символ в DATABASE_URL")

service_path = pathlib.Path(sys.argv[2])
service_path.write_text(
    "[fintalent_backup]\n" + "".join(f"{key}={item}\n" for key, item in params.items()),
    encoding="utf-8",
)
os.chmod(service_path, 0o600)
PY

PGSERVICEFILE="$service_file" PGSERVICE=fintalent_backup \
pg_dump --format=custom --no-owner --no-acl \
  --file="$partial_dir/database.dump"
rm -f -- "$service_file"

tar -C "$app_dir" -czf "$partial_dir/uploads.tar.gz" \
  static/uploads uploads
tar -C "$app_dir" -czf "$partial_dir/release.tar.gz" \
  .env fintalent static/react

pg_restore --list "$partial_dir/database.dump" >/dev/null
tar -tzf "$partial_dir/uploads.tar.gz" >/dev/null
tar -tzf "$partial_dir/release.tar.gz" >/dev/null

(
  cd "$partial_dir"
  sha256sum database.dump uploads.tar.gz release.tar.gz > SHA256SUMS
  sha256sum -c SHA256SUMS >/dev/null
)

mv "$partial_dir" "$final_dir"
trap - ERR
echo "Backup готов: $final_dir"
du -h "$final_dir"/*
