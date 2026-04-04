#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST_DIR="${ROOT_DIR}/dist"

if [[ $# -gt 1 ]]; then
  echo "Usage: scripts/release.sh [major.minor.build]"
  echo "Example (auto): scripts/release.sh"
  echo "Example (manual): scripts/release.sh 1.2.3"
  exit 1
fi

auto_version() {
  "${ROOT_DIR}/scripts/version.sh" next
}

VERSION="${1:-$(auto_version)}"
if [[ ! "${VERSION}" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "Error: version must match Major.Minor.Build (e.g. 1.2.3)"
  exit 1
fi

mkdir -p "${DIST_DIR}"
RELEASE_DIR="${DIST_DIR}/v${VERSION}"
rm -rf "${RELEASE_DIR}"
mkdir -p "${RELEASE_DIR}"

targets=(
  "darwin amd64"
  "darwin arm64"
  "linux amd64"
  "linux arm64"
  "windows amd64"
  "windows arm64"
)

echo "Building Glimpse ${VERSION} ..."
for target in "${targets[@]}"; do
  read -r GOOS GOARCH <<< "${target}"
  ext=""
  extra_ldflags=""
  if [[ "${GOOS}" == "windows" ]]; then
    ext=".exe"
    extra_ldflags=" -X main.forcePlainMode=1"
  fi

  target_dir="${RELEASE_DIR}/${GOOS}-${GOARCH}"
  mkdir -p "${target_dir}"
  out="${target_dir}/glimpse${ext}"
  echo "  -> ${GOOS}/${GOARCH}"

  (
    cd "${ROOT_DIR}"
    CGO_ENABLED=0 GOOS="${GOOS}" GOARCH="${GOARCH}" \
      go build \
      -trimpath \
      -ldflags "-s -w -X main.version=${VERSION}${extra_ldflags}" \
      -o "${out}" \
      ./cmd/glimpse
  )
done

(
  cd "${RELEASE_DIR}"
  shasum -a 256 ./*/glimpse* > checksums.txt
)

echo
echo "Done."
echo "Artifacts: ${RELEASE_DIR}"
echo "Checksums: ${RELEASE_DIR}/checksums.txt"
