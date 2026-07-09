#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "Usage: $0 dist/release/image-build-platform_<version>_<os>_<arch>.tar.gz" >&2
}

fail() {
  echo "$1" >&2
  exit 1
}

ARCHIVE="${1:-}"
if [[ -z "${ARCHIVE}" ]]; then
  usage
  exit 1
fi

[[ -f "${ARCHIVE}" ]] || fail "release archive not found: ${ARCHIVE}"

CHECKSUM_FILE="${ARCHIVE%.tar.gz}.checksums.txt"
[[ -f "${CHECKSUM_FILE}" ]] || fail "checksum file not found: ${CHECKSUM_FILE}"

DIST_DIR="$(cd "$(dirname "${ARCHIVE}")" && pwd)"
ARCHIVE_NAME="$(basename "${ARCHIVE}")"
CHECKSUM_NAME="$(basename "${CHECKSUM_FILE}")"
STAGING_DIR="$(mktemp -d "${TMPDIR:-/tmp}/ibp-release-check.XXXXXX")"

trap 'rm -rf "${STAGING_DIR}"' EXIT

echo "==> Verifying checksum ${CHECKSUM_NAME}"
(
  cd "${DIST_DIR}"
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 -c "${CHECKSUM_NAME}"
  else
    sha256sum -c "${CHECKSUM_NAME}"
  fi
)

echo "==> Extracting release archive ${ARCHIVE_NAME}"
tar -xzf "${ARCHIVE}" -C "${STAGING_DIR}"

PACKAGE_DIR="$(find "${STAGING_DIR}" -mindepth 1 -maxdepth 1 -type d -print -quit)"
[[ -n "${PACKAGE_DIR}" ]] || fail "release archive does not contain a package directory"

required_files=(
  "ibp-server"
  "README.md"
  "config.example.yaml"
  "web/dist/index.html"
  "docs/01-requirements.md"
  "docs/09-deployment.md"
  "docs/10-roadmap.md"
  "docs/11-acceptance.md"
  "deploy/compose/sqlite.yml"
  "deploy/compose/postgres.yml"
  "deploy/systemd/image-build-platform.service"
  "scripts/backup.sh"
  "scripts/restore.sh"
  "scripts/release.sh"
)

for file in "${required_files[@]}"; do
  [[ -f "${PACKAGE_DIR}/${file}" ]] || fail "release package missing ${file}"
done

for executable in "ibp-server" "scripts/backup.sh" "scripts/restore.sh" "scripts/release.sh"; do
  [[ -x "${PACKAGE_DIR}/${executable}" ]] || fail "release package file is not executable: ${executable}"
done

echo "Release package check ok: ${ARCHIVE}"
