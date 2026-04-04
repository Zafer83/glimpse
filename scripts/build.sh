#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT="${ROOT_DIR}/glimpse"

VERSION="$("${ROOT_DIR}/scripts/version.sh" next)"
echo "Building Glimpse ${VERSION} ..."

(
  cd "${ROOT_DIR}"
  go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o "${OUT}" ./cmd/glimpse
)

echo "Done: ${OUT}"
echo "Version: ${VERSION}"
