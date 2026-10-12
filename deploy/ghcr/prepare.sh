#!/usr/bin/env bash
set -euo pipefail
cd -- "$(dirname -- "${BASH_SOURCE[0]}")"
if [[ -e .env || -e config/config.yaml ]]; then
  echo 'Existing .env or config/config.yaml found; preserving them. For an existing deployment, use its original upgrade procedure.' >&2
  exit 1
fi
command -v openssl >/dev/null || { echo 'openssl is required to generate database passwords.' >&2; exit 1; }
if [[ ! -f release.env ]]; then
  echo 'release.env is missing.' >&2
  exit 1
fi
umask 077
mkdir -p config
root_password="$(openssl rand -hex 24)"
app_password="$(openssl rand -hex 24)"
{
  cat release.env
  printf '\nMYSQL_ROOT_PASSWORD=%s\nWATCHALERT_DB_PASSWORD=%s\n' "$root_password" "$app_password"
  printf 'WATCHALERT_HTTP_PORT=8088\nWATCHALERT_BIND_ADDRESS=0.0.0.0\nWATCHALERT_FRONTEND_SUFFIX=\n'
} > .env
cat > config/config.yaml <<EOF
Server:
  port: "9001"
  mode: release
Database:
  type: mysql
  host: w8t-mysql
  port: 3306
  user: watchalert
  pass: "$app_password"
  dbName: watchalert
  timeout: 10s
Redis:
  host: w8t-redis
  port: 6379
  pass: ""
  database: 0
Jwt:
  expire: 18000
EOF
echo 'Created private .env and config/config.yaml. Next: docker compose pull && docker compose up -d --wait'
