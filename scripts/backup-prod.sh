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
trap 'echo "Backup не завершён; файлы остались в $partial_dir" >&2' ERR

database_url="$(python3 - "$app_dir/.env" <<'PY'
import pathlib
import sys

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
        sys.stdout.write(value)
        break
else:
    raise SystemExit("DATABASE_URL не найден в .env")
PY
)"

export PGDATABASE="$database_url"
unset database_url
pg_dump --format=custom --no-owner --no-acl \
  --file="$partial_dir/database.dump"
unset PGDATABASE

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
