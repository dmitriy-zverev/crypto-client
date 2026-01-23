#!/usr/bin/env bash
set -euo pipefail

log() { echo "[$(date -u +'%Y-%m-%dT%H:%M:%SZ')] $*"; }

# Требуемые переменные из .env
: "${POSTGRES_USER:?POSTGRES_USER is required}"
: "${POSTGRES_PASSWORD:?POSTGRES_PASSWORD is required}"
: "${POSTGRES_DB:?POSTGRES_DB is required}"

# Настройки бэкапов (можно переопределять в .env)
BACKUP_DIR="${BACKUP_DIR:-/backups}"
BACKUP_EVERY_SECONDS="${BACKUP_EVERY_SECONDS:-900}"     # 15 минут
RETENTION_DAYS="${RETENTION_DAYS:-7}"                   # хранить 7 дней
PGHOST="${PGHOST:-db}"
PGPORT="${PGPORT:-5432}"

mkdir -p "$BACKUP_DIR"

log "backup service started. dir=$BACKUP_DIR every=${BACKUP_EVERY_SECONDS}s retention=${RETENTION_DAYS}d db=${POSTGRES_DB} host=${PGHOST}:${PGPORT}"

while true; do
  ts="$(date -u +'%Y%m%d_%H%M%S')"
  file="${BACKUP_DIR}/${POSTGRES_DB}_${ts}.dump"

  export PGPASSWORD="${POSTGRES_PASSWORD}"

  log "starting pg_dump -> ${file}"
  pg_dump \
    -h "${PGHOST}" -p "${PGPORT}" \
    -U "${POSTGRES_USER}" \
    -d "${POSTGRES_DB}" \
    -F c \
    -f "${file}.tmp"

  mv "${file}.tmp" "${file}"
  log "backup done: ${file}"

  log "cleanup old backups > ${RETENTION_DAYS} days"
  find "${BACKUP_DIR}" -type f -name "${POSTGRES_DB}_*.dump" -mtime "+${RETENTION_DAYS}" -print -delete || true

  sleep "${BACKUP_EVERY_SECONDS}"
done
