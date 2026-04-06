# Changelog

All notable changes to this project are documented in this file.

## v0.9.2 - 2026-04-06

### What's New in v0.9.2

#### Quality Gate and CI
- Added `scripts/check.sh` to enforce `gofmt -l`, `go test ./...`, and `go build ./cmd/glimpse`.
- Added GitHub Actions workflow (`.github/workflows/ci.yml`) to run the same gate on pushes to `main` and pull requests.
- Updated `scripts/release.sh` to run the quality gate before building and publishing releases.

#### CLI and Versioning Consistency
- Unified local LLM default URL to `http://localhost:8080` in prompts, flags, and help output.
- Removed git-based runtime version fallback in `cmd/glimpse/main.go` so build-time version injection is the single source of truth.
- Restored `go.sum` tracking by removing it from `.gitignore`.

#### Documentation Alignment
- Reworked `README.md` to be fully English and aligned with current CLI and release behavior.
- Applied formatting cleanup to touched Go source files.

**Full Changelog**: https://github.com/Zafer83/glimpse/compare/v0.9.1...v0.9.2
