#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST="${ROOT}/dist"
VERSION="3.0.1"
BINS=(doctortools craftdoctor stackdoctor depdoctor configdoctor deploydoctor apidoctor botdoctor logdoctor)
TARGETS="${DOCTORTOOLS_RELEASE_TARGETS:-linux/amd64 linux/arm64 windows/amd64 darwin/amd64 darwin/arm64}"
# Фиксированная дата нужна для воспроизводимых release-пакетов и отсутствия clock skew.
RELEASE_MTIME="${DOCTORTOOLS_RELEASE_MTIME:-2026-06-03 00:00:00 UTC}"
export SOURCE_DATE_EPOCH="${SOURCE_DATE_EPOCH:-1780444800}"

cd "${ROOT}"
go test ./...
go vet ./...

rm -rf "${DIST}"
mkdir -p "${DIST}/packages"

normalize_timestamps() {
  local target_dir="$1"
  find "${target_dir}" -exec touch -h -d "${RELEASE_MTIME}" {} +
}

build_one() {
  local goos="$1"
  local goarch="$2"
  local ext=""
  if [[ "${goos}" == "windows" ]]; then ext=".exe"; fi
  local dir="${DIST}/doctortools-${VERSION}-${goos}-${goarch}"
  mkdir -p "${dir}/rules" "${dir}/schemas"
  echo "Сборка DoctorTools ${VERSION} для ${goos}/${goarch}"
  for bin in "${BINS[@]}"; do
    GOOS="${goos}" GOARCH="${goarch}" CGO_ENABLED=0 go build -trimpath -ldflags='-s -w -buildid=' -o "${dir}/${bin}${ext}" "./cmd/${bin}"
    [[ "${goos}" != "windows" ]] && chmod 0755 "${dir}/${bin}${ext}"
  done
  cp README.md CHANGELOG.md LICENSE SECURITY.md "${dir}/"
  cp -R schemas/* "${dir}/schemas/"
  cp -R rules/* "${dir}/rules/"
  normalize_timestamps "${dir}"
  if [[ "${goos}" == "windows" ]]; then
    (cd "${DIST}" && zip -X -qr "packages/doctortools-${VERSION}-${goos}-${goarch}.zip" "doctortools-${VERSION}-${goos}-${goarch}")
  else
    (cd "${DIST}" && tar --sort=name --mtime="${RELEASE_MTIME}" --owner=0 --group=0 --numeric-owner -czf "packages/doctortools-${VERSION}-${goos}-${goarch}.tar.gz" "doctortools-${VERSION}-${goos}-${goarch}")
  fi
}

for target in ${TARGETS}; do
  build_one "${target%/*}" "${target#*/}"
done

(
  cd "${DIST}/packages"
  sha256sum doctortools-${VERSION}-*.tar.gz doctortools-${VERSION}-*.zip > "doctortools-${VERSION}-checksums.sha256" 2>/dev/null || sha256sum doctortools-${VERSION}-* > "doctortools-${VERSION}-checksums.sha256"
)

echo "Готово: ${DIST}/packages"
