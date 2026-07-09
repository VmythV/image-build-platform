#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage:
  BACKUP_FILE=/path/to/ibp-backup-YYYYmmddHHMMSS.tar.gz ./scripts/restore.sh
  ./scripts/restore.sh /path/to/ibp-backup-YYYYmmddHHMMSS.tar.gz

Environment:
  APP_DIR       Application directory. Defaults to the current directory.
  DATA_DIR      Data directory to restore. Defaults to ${APP_DIR}/data.
  CONFIG_FILE   Config file to restore. Defaults to ${APP_DIR}/config.yaml.
  SAFETY_DIR    Directory for pre-restore safety archives. Defaults to ${APP_DIR}/restore-backups.
USAGE
}

fail() {
  echo "$1" >&2
  exit 1
}

BACKUP_FILE="${1:-${BACKUP_FILE:-}}"
APP_DIR="${APP_DIR:-$(pwd)}"
DATA_DIR="${DATA_DIR:-${APP_DIR}/data}"
CONFIG_FILE="${CONFIG_FILE:-${APP_DIR}/config.yaml}"
SAFETY_DIR="${SAFETY_DIR:-${APP_DIR}/restore-backups}"
TIMESTAMP="$(date +%Y%m%d%H%M%S)"
STAGING_DIR="$(mktemp -d "${TMPDIR:-/tmp}/ibp-restore.XXXXXX")"
PAYLOAD_DIR="${STAGING_DIR}/payload"
SAFETY_PAYLOAD_DIR="${STAGING_DIR}/safety"
SAFETY_FILE="${SAFETY_DIR}/pre-restore-${TIMESTAMP}.tar.gz"

trap 'rm -rf "${STAGING_DIR}"' EXIT

if [[ -z "${BACKUP_FILE}" ]]; then
  usage >&2
  exit 1
fi

[[ -f "${BACKUP_FILE}" ]] || fail "backup file not found: ${BACKUP_FILE}"
[[ -n "${APP_DIR}" && "${APP_DIR}" != "/" ]] || fail "APP_DIR must not be empty or /"
[[ -n "${DATA_DIR}" && "${DATA_DIR}" != "/" ]] || fail "DATA_DIR must not be empty or /"
[[ -n "${CONFIG_FILE}" && "${CONFIG_FILE}" != "/" ]] || fail "CONFIG_FILE must not be empty or /"

echo "==> Validating backup archive ${BACKUP_FILE}"
while IFS= read -r entry; do
  case "${entry}" in
    "" | "/" | /* | "../"* | */../* | "..")
      fail "unsafe archive entry: ${entry}"
      ;;
  esac
done < <(tar -tzf "${BACKUP_FILE}")

mkdir -p "${PAYLOAD_DIR}"
tar -xzf "${BACKUP_FILE}" -C "${PAYLOAD_DIR}"

[[ -d "${PAYLOAD_DIR}/data" ]] || fail "backup archive does not contain a data directory"

mkdir -p "${SAFETY_DIR}"
if [[ -d "${DATA_DIR}" || -f "${CONFIG_FILE}" ]]; then
	echo "==> Saving current state to ${SAFETY_FILE}"
	mkdir -p "${SAFETY_PAYLOAD_DIR}"
	if [[ -f "${CONFIG_FILE}" ]]; then
		cp "${CONFIG_FILE}" "${SAFETY_PAYLOAD_DIR}/config.yaml"
	fi
	if [[ -d "${DATA_DIR}" ]]; then
		cp -R "${DATA_DIR}" "${SAFETY_PAYLOAD_DIR}/data"
	fi
	tar -C "${SAFETY_PAYLOAD_DIR}" -czf "${SAFETY_FILE}" .
fi

echo "==> Restoring data directory ${DATA_DIR}"
mkdir -p "$(dirname "${DATA_DIR}")"
rm -rf "${DATA_DIR}"
cp -R "${PAYLOAD_DIR}/data" "${DATA_DIR}"

if [[ -f "${PAYLOAD_DIR}/config.yaml" ]]; then
  echo "==> Restoring config file ${CONFIG_FILE}"
  mkdir -p "$(dirname "${CONFIG_FILE}")"
  cp "${PAYLOAD_DIR}/config.yaml" "${CONFIG_FILE}"
else
  echo "==> Backup does not include config.yaml; existing config file was left unchanged"
fi

echo "Restore completed from: ${BACKUP_FILE}"
if [[ -f "${SAFETY_FILE}" ]]; then
  echo "Pre-restore safety backup: ${SAFETY_FILE}"
fi
