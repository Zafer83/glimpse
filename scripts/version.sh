#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BASE_FILE="${ROOT_DIR}/VERSION_BASE"
CACHE_DIR="${ROOT_DIR}/.version"
COUNTER_FILE="${CACHE_DIR}/build_counter"

usage() {
  echo "Usage: scripts/version.sh <current|next>"
  echo "  current  Print current version (Major.Minor.Build)"
  echo "  next     Increment build counter and print next version"
}

ensure_files() {
  mkdir -p "${CACHE_DIR}"
  if [[ ! -f "${BASE_FILE}" ]]; then
    echo "0.1" > "${BASE_FILE}"
  fi
  if [[ ! -f "${COUNTER_FILE}" ]]; then
    echo "0" > "${COUNTER_FILE}"
  fi
}

validate_base() {
  local base
  base="$(tr -d '[:space:]' < "${BASE_FILE}")"
  if [[ ! "${base}" =~ ^[0-9]+\.[0-9]+$ ]]; then
    echo "Error: ${BASE_FILE} must be Major.Minor (e.g. 1.2)" >&2
    exit 1
  fi
  echo "${base}"
}

current_version() {
  local base build
  base="$(validate_base)"
  build="$(tr -d '[:space:]' < "${COUNTER_FILE}")"
  if [[ ! "${build}" =~ ^[0-9]+$ ]]; then
    echo "Error: ${COUNTER_FILE} must contain an integer." >&2
    exit 1
  fi
  echo "${base}.${build}"
}

next_version() {
  local build
  build="$(tr -d '[:space:]' < "${COUNTER_FILE}")"
  if [[ ! "${build}" =~ ^[0-9]+$ ]]; then
    build=0
  fi
  build=$((build + 1))
  echo "${build}" > "${COUNTER_FILE}"
  current_version
}

if [[ $# -ne 1 ]]; then
  usage
  exit 1
fi

ensure_files
case "$1" in
  current) current_version ;;
  next) next_version ;;
  *)
    usage
    exit 1
    ;;
esac
