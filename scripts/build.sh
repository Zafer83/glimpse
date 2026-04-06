#!/usr/bin/env bash
#
# build.sh — Build Glimpse for the current platform.
#
# Usage:
#   scripts/build.sh           # version from latest git tag
#   scripts/build.sh 1.2.3     # explicit version override
#
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT="${ROOT_DIR}/glimpse"

# Determine version: argument > latest git tag > fallback.
if [[ $# -ge 1 ]]; then
  VERSION="$1"
elif git -C "${ROOT_DIR}" describe --tags --abbrev=0 2>/dev/null | grep -qE '^v[0-9]'; then
  VERSION="$(git -C "${ROOT_DIR}" describe --tags --match 'v[0-9]*' 2>/dev/null | sed 's/^v//')"
else
  VERSION="dev"
fi

echo "Building Glimpse ${VERSION} ..."

(
  cd "${ROOT_DIR}"
  go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o "${OUT}" ./cmd/glimpse
)

echo "Done: ${OUT}"
echo "Version: ${VERSION}"
