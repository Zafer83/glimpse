#!/usr/bin/env bash
#
# release.sh — Build, tag, and publish a Glimpse release.
#
# Usage:
#   scripts/release.sh 0.9.2          # explicit version
#   scripts/release.sh patch          # auto-bump: 0.9.1 → 0.9.2
#   scripts/release.sh minor          # auto-bump: 0.9.2 → 0.10.0
#   scripts/release.sh major          # auto-bump: 0.9.2 → 1.0.0
#
# What it does:
#   1. Validates you're on main with a clean working tree
#   2. Determines the version (from argument or auto-bump)
#   3. Cross-compiles for macOS, Linux, Windows (amd64 + arm64)
#   4. Creates a git tag
#   5. Pushes tag to origin
#   6. Creates a GitHub release with all binaries + checksums
#
# Requirements: go, git, gh (GitHub CLI)
#
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST_DIR="${ROOT_DIR}/dist"

# ---------- helpers ----------

die() { echo "Error: $*" >&2; exit 1; }

check_deps() {
  command -v go  >/dev/null 2>&1 || die "go is not installed"
  command -v git >/dev/null 2>&1 || die "git is not installed"
  command -v gh  >/dev/null 2>&1 || die "gh (GitHub CLI) is not installed — brew install gh"
  [[ -x "${ROOT_DIR}/scripts/check.sh" ]] || die "scripts/check.sh not found or not executable"
}

check_clean() {
  local branch
  branch="$(git -C "${ROOT_DIR}" branch --show-current)"
  if [[ "${branch}" != "main" ]]; then
    die "Must be on 'main' branch (currently on '${branch}')"
  fi

  if ! git -C "${ROOT_DIR}" diff --quiet HEAD 2>/dev/null; then
    die "Working tree is not clean. Commit or stash changes first."
  fi
}

latest_tag() {
  # Get the latest semver tag, sorted properly.
  git -C "${ROOT_DIR}" tag -l 'v[0-9]*' \
    | sort -V \
    | tail -1 \
    | sed 's/^v//'
}

bump_version() {
  local current="$1" part="$2"
  local major minor patch

  IFS='.' read -r major minor patch <<< "${current}"
  patch="${patch:-0}"

  case "${part}" in
    major) echo "$((major + 1)).0.0" ;;
    minor) echo "${major}.$((minor + 1)).0" ;;
    patch) echo "${major}.${minor}.$((patch + 1))" ;;
    *) die "Unknown bump type: ${part}" ;;
  esac
}

# ---------- main ----------

check_deps

echo "Running quality gate..."
"${ROOT_DIR}/scripts/check.sh"

if [[ $# -ne 1 ]]; then
  echo "Usage: scripts/release.sh <version|patch|minor|major>"
  echo ""
  echo "Examples:"
  echo "  scripts/release.sh 1.0.0    # explicit version"
  echo "  scripts/release.sh patch    # auto-bump patch (0.9.1 → 0.9.2)"
  echo "  scripts/release.sh minor    # auto-bump minor (0.9.1 → 0.10.0)"
  echo "  scripts/release.sh major    # auto-bump major (0.9.1 → 1.0.0)"
  echo ""
  echo "Current latest tag: v$(latest_tag)"
  exit 1
fi

check_clean

ARG="$1"
CURRENT="$(latest_tag)"

if [[ "${ARG}" =~ ^(patch|minor|major)$ ]]; then
  if [[ -z "${CURRENT}" ]]; then
    die "No existing tags found. Use an explicit version: scripts/release.sh 0.1.0"
  fi
  VERSION="$(bump_version "${CURRENT}" "${ARG}")"
  echo "Auto-bump: v${CURRENT} → v${VERSION} (${ARG})"
elif [[ "${ARG}" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  VERSION="${ARG}"
else
  die "Invalid version '${ARG}'. Use semver (1.2.3) or patch/minor/major."
fi

TAG="v${VERSION}"

# Check tag doesn't already exist.
if git -C "${ROOT_DIR}" tag -l "${TAG}" | grep -q "${TAG}"; then
  die "Tag ${TAG} already exists. Choose a different version."
fi

echo ""
echo "╔══════════════════════════════════════╗"
echo "║  Glimpse Release ${TAG}"
echo "╚══════════════════════════════════════╝"
echo ""

# ---------- build ----------

RELEASE_DIR="${DIST_DIR}/${TAG}"
rm -rf "${RELEASE_DIR}"
mkdir -p "${RELEASE_DIR}"

targets=(
  "darwin  amd64"
  "darwin  arm64"
  "linux   amd64"
  "linux   arm64"
  "windows amd64"
)

echo "Building for all platforms..."
for target in "${targets[@]}"; do
  read -r os arch <<< "${target}"
  ext=""
  extra=""
  if [[ "${os}" == "windows" ]]; then
    ext=".exe"
    extra=" -X main.forcePlainMode=1"
  fi

  outname="glimpse-${TAG}-${os}-${arch}${ext}"
  out="${RELEASE_DIR}/${outname}"
  echo "  → ${os}/${arch}"

  (
    cd "${ROOT_DIR}"
    CGO_ENABLED=0 GOOS="${os}" GOARCH="${arch}" \
      go build \
        -trimpath \
        -ldflags "-s -w -X main.version=${VERSION}${extra}" \
        -o "${out}" \
        ./cmd/glimpse
  )
done

# Generate checksums.
echo ""
echo "Generating checksums..."
(
  cd "${RELEASE_DIR}"
  shasum -a 256 glimpse-* > checksums.txt
)

# ---------- tag & push ----------

echo ""
echo "Creating tag ${TAG}..."
git -C "${ROOT_DIR}" tag -a "${TAG}" -m "Release ${TAG}"
git -C "${ROOT_DIR}" push origin "${TAG}"

# ---------- GitHub release ----------

echo "Creating GitHub release..."

# Build changelog from commits since last tag.
CHANGELOG=""
if [[ -n "${CURRENT}" ]]; then
  CHANGELOG="$(git -C "${ROOT_DIR}" log "v${CURRENT}..HEAD" --pretty="- %s" --no-merges 2>/dev/null || true)"
fi

NOTES="$(cat <<NOTES_EOF
## Glimpse ${TAG}

### Changes
${CHANGELOG:-_No changelog available._}

### Downloads

| OS | Architecture | Binary |
|---|---|---|
| macOS | Apple Silicon (M1/M2/M3/M4) | \`glimpse-${TAG}-darwin-arm64\` |
| macOS | Intel | \`glimpse-${TAG}-darwin-amd64\` |
| Linux | x86_64 | \`glimpse-${TAG}-linux-amd64\` |
| Linux | ARM64 | \`glimpse-${TAG}-linux-arm64\` |
| Windows | x86_64 | \`glimpse-${TAG}-windows-amd64.exe\` |

### Verify checksums
\`\`\`bash
shasum -a 256 -c checksums.txt
\`\`\`

**Full Changelog**: https://github.com/Zafer83/glimpse/compare/v${CURRENT}...${TAG}
NOTES_EOF
)"

gh release create "${TAG}" \
  "${RELEASE_DIR}"/glimpse-* \
  "${RELEASE_DIR}/checksums.txt" \
  --title "Glimpse ${TAG}" \
  --notes "${NOTES}" \
  --repo "$(git -C "${ROOT_DIR}" remote get-url origin | sed 's|.*github.com[:/]||;s|\.git$||')"

echo ""
echo "✅ Release ${TAG} published!"
echo "   https://github.com/$(git -C "${ROOT_DIR}" remote get-url origin | sed 's|.*github.com[:/]||;s|\.git$||')/releases/tag/${TAG}"
