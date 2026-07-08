#!/usr/bin/env bash
set -euo pipefail

APP_DIR="${APP_DIR:-$(pwd)}"
DATA_DIR="${DATA_DIR:-${APP_DIR}/data}"
CONFIG_FILE="${CONFIG_FILE:-${APP_DIR}/config.yaml}"
BACKUP_DIR="${BACKUP_DIR:-${DATA_DIR}/backups}"
TIMESTAMP="$(date +%Y%m%d%H%M%S)"
BACKUP_FILE="${BACKUP_DIR}/ibp-backup-${TIMESTAMP}.tar.gz"
STAGING_DIR="$(mktemp -d "${TMPDIR:-/tmp}/ibp-backup.XXXXXX")"
PAYLOAD_DIR="${STAGING_DIR}/payload"

mkdir -p "${BACKUP_DIR}"
trap 'rm -rf "${STAGING_DIR}"' EXIT

if [[ ! -d "${DATA_DIR}" ]]; then
  echo "data directory not found: ${DATA_DIR}" >&2
  exit 1
fi

echo "==> Creating backup ${BACKUP_FILE}"
mkdir -p "${PAYLOAD_DIR}"

if [[ -f "${CONFIG_FILE}" ]]; then
  cp "${CONFIG_FILE}" "${PAYLOAD_DIR}/config.yaml"
fi

cp -R "${DATA_DIR}" "${PAYLOAD_DIR}/data"
rm -rf "${PAYLOAD_DIR}/data/backups"
tar -C "${PAYLOAD_DIR}" -czf "${BACKUP_FILE}" .

echo "Backup created: ${BACKUP_FILE}"
