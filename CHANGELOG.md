# Changelog

All notable changes to this project are documented in this file.

## v0.9.2 - 2026-04-06

### Added
- Added `scripts/check.sh` quality gate (`gofmt -l`, `go test ./...`, `go build ./cmd/glimpse`).
- Added GitHub Actions CI workflow (`.github/workflows/ci.yml`) to run the same quality gate on pushes to `main` and pull requests.

### Changed
- Updated `scripts/release.sh` to run the quality gate before building/tagging/releasing.
- Unified local LLM default URL to `http://localhost:8080` in CLI defaults and help output.
- Reworked `README.md` to be fully English and aligned with current CLI/release behavior.
- Removed git-based runtime version fallback from `cmd/glimpse/main.go` so build-time version injection is the single source of truth.
- Tracked `go.sum` again by removing it from `.gitignore`.
- Applied formatting cleanup in touched Go files.

