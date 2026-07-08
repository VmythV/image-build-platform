#!/usr/bin/env bash
set -euo pipefail

if [[ "${EUID}" -ne 0 ]]; then
  echo "install-systemd.sh must be run as root" >&2
  exit 1
fi

APP_DIR="${APP_DIR:-/opt/image-build-platform}"
SERVICE_FILE="/etc/systemd/system/image-build-platform.service"
SOURCE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if ! id ibp >/dev/null 2>&1; then
  useradd --system --home-dir "${APP_DIR}" --shell /usr/sbin/nologin ibp
fi

mkdir -p "${APP_DIR}/data"

install -m 0755 "${SOURCE_DIR}/ibp-server" "${APP_DIR}/ibp-server"
install -m 0644 "${SOURCE_DIR}/config.example.yaml" "${APP_DIR}/config.example.yaml"
if [[ ! -f "${APP_DIR}/config.yaml" ]]; then
  cp "${APP_DIR}/config.example.yaml" "${APP_DIR}/config.yaml"
fi
if [[ -d "${SOURCE_DIR}/web/dist" ]]; then
  mkdir -p "${APP_DIR}/web"
  rm -rf "${APP_DIR}/web/dist"
  cp -R "${SOURCE_DIR}/web/dist" "${APP_DIR}/web/dist"
fi

install -m 0644 "${SOURCE_DIR}/deploy/systemd/image-build-platform.service" "${SERVICE_FILE}"
chown -R ibp:ibp "${APP_DIR}"

systemctl daemon-reload
systemctl enable image-build-platform

echo "Installed image-build-platform systemd service."
echo "Review ${APP_DIR}/config.yaml, then run:"
echo "  systemctl start image-build-platform"
