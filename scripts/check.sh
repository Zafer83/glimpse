#!/usr/bin/env bash
#
# check.sh — Local/CI quality gate for Glimpse.
# Runs:
#   1) gofmt check (no writes)
#   2) go test ./...
#   3) go build ./cmd/glimpse
#
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

die() { echo "Error: $*" >&2; exit 1; }

command -v go >/dev/null 2>&1 || die "go is not installed"
command -v gofmt >/dev/null 2>&1 || die "gofmt is not installed"
command -v git >/dev/null 2>&1 || die "git is not installed"

cd "${ROOT_DIR}"

echo "==> Formatting check (gofmt -l)"
GO_FILES=()
while IFS= read -r line; do
  GO_FILES+=("${line}")
done < <(git ls-files '*.go')

if [[ "${#GO_FILES[@]}" -gt 0 ]]; then
  UNFORMATTED=()
  while IFS= read -r line; do
    [[ -n "${line}" ]] && UNFORMATTED+=("${line}")
  done < <(gofmt -l "${GO_FILES[@]}")

  if [[ "${#UNFORMATTED[@]}" -gt 0 ]]; then
    echo "Unformatted Go files:"
    printf '  - %s\n' "${UNFORMATTED[@]}"
    echo "Run: gofmt -w <files>"
    exit 1
  fi
fi

echo "==> Running tests"
go test ./... -count=1

echo "==> Building CLI"
go build ./cmd/glimpse

echo "✅ check.sh passed"
