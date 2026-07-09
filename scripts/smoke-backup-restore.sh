#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/ibp-backup-restore-smoke.XXXXXX")"
APP_DIR="${WORK_DIR}/app"
BACKUP_DIR="${WORK_DIR}/backups"
SAFETY_DIR="${WORK_DIR}/safety"

trap 'rm -rf "${WORK_DIR}"' EXIT

mkdir -p "${APP_DIR}/data/logs" "${APP_DIR}/data/contexts" "${APP_DIR}/data/backups"
printf 'original-db' > "${APP_DIR}/data/app.db"
printf 'original-log' > "${APP_DIR}/data/logs/build.log"
printf 'original-context' > "${APP_DIR}/data/contexts/context.tar"
printf 'old-backup' > "${APP_DIR}/data/backups/old.tar.gz"
printf 'original-config' > "${APP_DIR}/config.yaml"

echo "==> Creating smoke backup"
APP_DIR="${APP_DIR}" BACKUP_DIR="${BACKUP_DIR}" "${ROOT_DIR}/scripts/backup.sh"

BACKUP_FILE="$(find "${BACKUP_DIR}" -maxdepth 1 -name 'ibp-backup-*.tar.gz' -print -quit)"
[[ -n "${BACKUP_FILE}" ]] || {
  echo "backup file was not created" >&2
  exit 1
}

if tar -tzf "${BACKUP_FILE}" | grep -q 'data/backups'; then
  echo "backup archive must not include nested backup files" >&2
  exit 1
fi

printf 'changed-db' > "${APP_DIR}/data/app.db"
printf 'changed-log' > "${APP_DIR}/data/logs/build.log"
printf 'changed-config' > "${APP_DIR}/config.yaml"

echo "==> Restoring smoke backup"
make -C "${ROOT_DIR}" restore BACKUP_FILE="${BACKUP_FILE}" APP_DIR="${APP_DIR}" SAFETY_DIR="${SAFETY_DIR}"

[[ "$(cat "${APP_DIR}/data/app.db")" == "original-db" ]] || {
  echo "data/app.db was not restored" >&2
  exit 1
}
[[ "$(cat "${APP_DIR}/data/logs/build.log")" == "original-log" ]] || {
  echo "data/logs/build.log was not restored" >&2
  exit 1
}
[[ "$(cat "${APP_DIR}/data/contexts/context.tar")" == "original-context" ]] || {
  echo "data/contexts/context.tar was not restored" >&2
  exit 1
}
[[ "$(cat "${APP_DIR}/config.yaml")" == "original-config" ]] || {
  echo "config.yaml was not restored" >&2
  exit 1
}

SAFETY_FILE="$(find "${SAFETY_DIR}" -maxdepth 1 -name 'pre-restore-*.tar.gz' -print -quit)"
[[ -n "${SAFETY_FILE}" ]] || {
  echo "pre-restore safety backup was not created" >&2
  exit 1
}

SAFETY_EXTRACT="${WORK_DIR}/safety-extract"
mkdir -p "${SAFETY_EXTRACT}"
tar -xzf "${SAFETY_FILE}" -C "${SAFETY_EXTRACT}"

[[ "$(cat "${SAFETY_EXTRACT}/data/app.db")" == "changed-db" ]] || {
  echo "pre-restore safety backup did not capture changed data" >&2
  exit 1
}
[[ "$(cat "${SAFETY_EXTRACT}/config.yaml")" == "changed-config" ]] || {
  echo "pre-restore safety backup did not capture changed config" >&2
  exit 1
}

echo "Backup and restore smoke ok"
