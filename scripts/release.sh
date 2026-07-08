#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="${VERSION:-dev}"
TARGET_OS="${TARGET_OS:-$(go env GOOS)}"
TARGET_ARCH="${TARGET_ARCH:-$(go env GOARCH)}"
APP_NAME="image-build-platform"
PACKAGE_NAME="${APP_NAME}_${VERSION}_${TARGET_OS}_${TARGET_ARCH}"
DIST_DIR="${ROOT_DIR}/dist/release"
PACKAGE_DIR="${DIST_DIR}/${PACKAGE_NAME}"

rm -rf "${PACKAGE_DIR}"
mkdir -p "${PACKAGE_DIR}/web" "${DIST_DIR}"

echo "==> Building frontend"
npm --prefix "${ROOT_DIR}/web" run build

echo "==> Building backend ${TARGET_OS}/${TARGET_ARCH}"
GO111MODULE=on CGO_ENABLED=0 GOOS="${TARGET_OS}" GOARCH="${TARGET_ARCH}" \
  go build -ldflags "-s -w -X main.version=${VERSION}" \
  -o "${PACKAGE_DIR}/ibp-server" "${ROOT_DIR}/cmd/ibp-server"

echo "==> Assembling package"
cp "${ROOT_DIR}/README.md" "${PACKAGE_DIR}/README.md"
cp "${ROOT_DIR}/config.example.yaml" "${PACKAGE_DIR}/config.example.yaml"
cp -R "${ROOT_DIR}/web/dist" "${PACKAGE_DIR}/web/dist"
cp -R "${ROOT_DIR}/docs" "${PACKAGE_DIR}/docs"
cp -R "${ROOT_DIR}/scripts" "${PACKAGE_DIR}/scripts"
cp -R "${ROOT_DIR}/deploy" "${PACKAGE_DIR}/deploy"

if [[ -f "${ROOT_DIR}/LICENSE" ]]; then
  cp "${ROOT_DIR}/LICENSE" "${PACKAGE_DIR}/LICENSE"
fi

echo "==> Creating archive"
tar -C "${DIST_DIR}" -czf "${DIST_DIR}/${PACKAGE_NAME}.tar.gz" "${PACKAGE_NAME}"

echo "==> Writing checksum"
(
  cd "${DIST_DIR}"
  shasum -a 256 "${PACKAGE_NAME}.tar.gz" > "${PACKAGE_NAME}.checksums.txt"
)

echo "Release package:"
echo "  ${DIST_DIR}/${PACKAGE_NAME}.tar.gz"
echo "  ${DIST_DIR}/${PACKAGE_NAME}.checksums.txt"
