# Glimpse ✨

[![Go Report Card](https://goreportcard.com/badge/github.com/Zafer83/glimpse)](https://goreportcard.com/report/github.com/Zafer83/glimpse)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Version](https://img.shields.io/github/go-mod/go-version/Zafer83/glimpse)](https://golang.org/doc/devel/release.html)

Glimpse is an AI-driven Go CLI that turns source code into Slidev presentations.

## Features

- Interactive CLI flow (path, theme, model, language, output)
- Theme validation and auto-install for Slidev themes
- Multi-provider support:
  - OpenAI (`gpt-*`)
  - Gemini (`gemini-*`)
  - Anthropic (`claude-*`)
  - Local LLM (`local` / `local/<model>` / `ollama/<model>`)
- Automatic Slidev launch option after generation
- Progress bar with ETA while AI analysis is running
- Recursive source crawler with common folder exclusions

Terminal rendering behavior:

- Linux/macOS: colored ANSI output enabled by default
- Windows release binaries: plain fallback mode enabled automatically

## Installation

```bash
git clone https://github.com/Zafer83/glimpse
cd glimpse
go mod tidy
go build -o glimpse ./cmd/glimpse
```

## Usage

```bash
./glimpse
```

Version:

```bash
./glimpse --version
```

## Versioning

Glimpse uses `Major.Minor.Build`.

- Runtime output:
  - `./glimpse --version`
- Base version is stored in `VERSION_BASE` (`Major.Minor`).
- Build number is persisted in `.version/build_counter` and auto-incremented by scripts.

Automatic local build with version increment:

```bash
scripts/build.sh
```

Check current/next version manually:

```bash
scripts/version.sh current
scripts/version.sh next
```

## Release Script

Build all release binaries from macOS/Linux with one command (auto version):

```bash
scripts/release.sh
```

Optional manual release version:

```bash
scripts/release.sh 1.2.3
```

This creates artifacts in a versioned folder:

- `dist/v1.2.3/darwin-amd64/glimpse`
- `dist/v1.2.3/darwin-arm64/glimpse`
- `dist/v1.2.3/linux-amd64/glimpse`
- `dist/v1.2.3/linux-arm64/glimpse`
- `dist/v1.2.3/windows-amd64/glimpse.exe`
- `dist/v1.2.3/windows-arm64/glimpse.exe`
- `dist/v1.2.3/checksums.txt`

### Model examples

- OpenAI: `gpt-4o`
- Gemini: `gemini-2.0-flash`
- Anthropic: `claude-3-5-sonnet-latest`
- Local:
  - `local` (defaults to `llama3.2`)
  - `local/Qwen3-Coder-30B-A3B-Instruct-BF16-00001-of-00002.gguf`

### Local server URL input

If you choose a local model, Glimpse asks for:

- `Local LLM URL (e.g. http://localhost:8080)`

For `llama.cpp` OpenAI-compatible server, use:

- `http://localhost:8080` (Glimpse uses `/v1/chat/completions`)

If needed, it can also fall back to Ollama-style `/api/chat`.

## Slidev Theme Validation

- `default` and `seriph` work directly.
- For custom themes, Glimpse validates package existence on npm.
- If valid but missing locally, Glimpse auto-installs it via:
  - `npm install -D <theme-package>`
- If the theme name is invalid (spelling/package), Glimpse exits with a clear error.

### Official vs Community Themes

- Official themes can be entered as short names:
  - `seriph`, `default`, `bricks`, `shibainu`, `apple-basic`
- Community themes should be entered as full npm package names:
  - Example: `@your-scope/slidev-theme-awesome`

How Glimpse resolves theme input:

- If you enter a short name, Glimpse maps it to `@slidev/theme-<name>`.
- If you enter a full package name (`@scope/pkg` or `scope/pkg`), Glimpse uses it as provided.

## Slidev

After generating `slides.md`, Glimpse can start Slidev automatically using:

- `npx --yes @slidev/cli <file>`

## Author

- Zafer Kılıçaslan

## License

Licensed under Apache-2.0. See [LICENSE](./LICENSE).

All Go source files include Apache-2.0 header comments.
