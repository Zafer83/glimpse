<br>
<p align="center">
  <a href="https://github.com/Zafer83/glimpse" target="_blank">
    <img src="https://raw.githubusercontent.com/Zafer83/glimpse/main/glimpse.png" alt="Glimpse" width="250"/>
  </a>
</p>

<p align="center">
  Glimpse is an AI-driven Go CLI that turns source code into Slidev presentations.
</p>

<p align="center">
  <a href="https://github.com/Zafer83/glimpse/releases">
    <img src="https://img.shields.io/github/v/release/Zafer83/glimpse?label=Latest%20Release&color=orange" alt="Latest Release">
  </a>
  <a href="https://goreportcard.com/report/github.com/Zafer83/glimpse">
    <img src="https://goreportcard.com/badge/github.com/Zafer83/glimpse" alt="Go Report Card">
  </a>
  <a href="https://opensource.org/licenses/Apache-2.0">
    <img src="https://img.shields.io/badge/License-Apache%202.0-blue.svg" alt="License: Apache-2.0">
  </a>
  <a href="https://golang.org/doc/devel/release.html">
    <img src="https://img.shields.io/github/go-mod/go-version/Zafer83/glimpse" alt="Go Version">
  </a>
</p>

## Features

- Interactive CLI flow (project path, theme, model, language, output file)
- Non-interactive mode via flags for CI/CD and scripts
- Multi-provider AI support:
  - OpenAI (`gpt-*`)
  - Gemini (`gemini-*`)
  - Anthropic (`claude-*`)
  - Local LLM (`local`, `local/<model>`, `ollama/<model>`)
- Slidev theme validation and auto-install for npm themes
- Progress bar with ETA during AI analysis
- Automatic Slidev startup option after generation
- Recursive source crawler with documentation-first prioritization

Terminal behavior:

- Linux/macOS: colored ANSI output enabled by default
- Windows release binaries: plain fallback mode enabled automatically

## Requirements

- Go (version from `go.mod`)
- `git`
- `node` + `npm` (required to run Slidev and auto-install themes)

For releases:

- `gh` (GitHub CLI)

## Installation

```bash
git clone https://github.com/Zafer83/glimpse
cd glimpse
go mod tidy
go build -o glimpse ./cmd/glimpse
```

## Usage

### Interactive mode

```bash
./glimpse
```

### Non-interactive mode (flags)

```bash
./glimpse \
  --path ./my-project \
  --model gpt-4o \
  --api-key "$OPENAI_API_KEY" \
  --theme seriph \
  --lang en \
  --output slides.md \
  --slidev
```

### CLI help

```bash
./glimpse --help
```

Current flags:

```text
-api-key string
      API key (or set GLIMPSE_API_KEY env)
-lang string
      Presentation language (default: de)
-local-url string
      Local LLM base URL (default: http://localhost:8080)
-model string
      AI model (e.g. gpt-4o, gemini-2.0-flash, local/qwen2.5-coder:7b)
-output string
      Output file name (default: slides.md)
-path string
      Project path to scan
-slidev
      Auto-start Slidev after generation (non-interactive)
-theme string
      Slidev theme (default: seriph)
-v
      Print version and exit
-version
      Print version and exit
```

### Flag reference

| Flag | Type | Default | Description |
|---|---|---|---|
| `--path` | string | *(none)* | Project directory to scan. Required for non-interactive mode. Supports relative paths and `~/`. |
| `--model` | string | `gemini-2.0-flash` | AI model identifier. |
| `--api-key` | string | *(none)* | Required for cloud models; optional for local models. |
| `--theme` | string | `seriph` | Slidev theme name/package. |
| `--lang` | string | `de` | Output language for generated slides. |
| `--output` | string | `slides.md` | Output markdown filename. |
| `--local-url` | string | `http://localhost:8080` | Base URL for local LLM servers. |
| `--slidev` | bool | `false` | Auto-start Slidev after generation (non-interactive mode). |
| `--version`, `-v` | bool | `false` | Print version and exit. |

## Model examples

- OpenAI: `gpt-4o`
- Gemini: `gemini-2.0-flash`
- Anthropic: `claude-3-5-sonnet-latest`
- Local:
  - `local` (auto-detects or defaults to `qwen2.5-coder:7b`)
  - `local/qwen2.5-coder:14b`
  - `ollama/llama3.2`

## Local LLM setup

If you choose a local model, Glimpse asks for `Local LLM URL`.

Recommended values:

- `http://localhost:8080` for OpenAI-compatible servers (e.g. `llama.cpp` server)
- `http://localhost:11434` for Ollama

Notes:

- Glimpse first tries OpenAI-compatible `/v1/chat/completions`.
- If that fails, it falls back to Ollama `/api/chat`.
- API key is optional in local mode.

### Quick Ollama setup

macOS / Linux:

```bash
./scripts/setup-ollama.sh
```

Windows (PowerShell as Admin):

```powershell
.\scripts\setup-ollama.ps1
```

## Slidev theme validation

Theme resolution behavior:

- Built-in direct names: `default`, `seriph`
- Short custom names are resolved as:
  - `@slidev/theme-<name>`
  - `slidev-theme-<name>`
- Full package names are accepted directly.
- If the package exists but is not installed locally, Glimpse installs it with:

```bash
npm install -D <theme-package>
```

## Quality gate (local + CI)

Run the same checks locally that CI runs:

```bash
./scripts/check.sh
```

This verifies:

- `gofmt` formatting (`gofmt -l` must be empty)
- `go test ./...`
- `go build ./cmd/glimpse`

GitHub Actions CI (`.github/workflows/ci.yml`) runs this gate on:

- pushes to `main`
- pull requests

## Build and release

### Local build

```bash
scripts/build.sh
```

Optional explicit version override:

```bash
scripts/build.sh 1.2.3
```

### Release script

```bash
scripts/release.sh <version|patch|minor|major>
```

Examples:

```bash
scripts/release.sh 1.0.0
scripts/release.sh patch
scripts/release.sh minor
scripts/release.sh major
```

What `scripts/release.sh` does:

1. Runs `scripts/check.sh`
2. Verifies you are on `main` with a clean working tree
3. Resolves release version (explicit or auto-bump from latest tag)
4. Cross-compiles binaries for:
   - `darwin/amd64`
   - `darwin/arm64`
   - `linux/amd64`
   - `linux/arm64`
   - `windows/amd64`
5. Generates checksums
6. Creates and pushes git tag
7. Creates GitHub release with artifacts

Artifacts are generated under:

- `dist/v<version>/glimpse-v<version>-darwin-amd64`
- `dist/v<version>/glimpse-v<version>-darwin-arm64`
- `dist/v<version>/glimpse-v<version>-linux-amd64`
- `dist/v<version>/glimpse-v<version>-linux-arm64`
- `dist/v<version>/glimpse-v<version>-windows-amd64.exe`
- `dist/v<version>/checksums.txt`

## Slidev startup

After generating slides, Glimpse can start Slidev via:

```bash
npx --yes @slidev/cli <slides-file>
```

## Environment variables

| Variable | Purpose |
|---|---|
| `GLIMPSE_API_KEY` | API key for cloud models (alternative to `--api-key`) |
| `UNSPLASH_IMAGE_URL` | URL template for image keyword expansion (`{keywords}` placeholder) |
| `NO_COLOR` | Disable colored output |
| `GLIMPSE_NO_ANSI` | Explicitly disable ANSI escape output |
| `FORCE_ANSI` | Force ANSI output on Windows (advanced use) |

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Success |
| `1` | Runtime error |
| `130` | Interrupted by user (`Ctrl+C`) |

## Author

- Zafer Kılıçaslan

## License

Licensed under Apache-2.0. See [LICENSE](./LICENSE).

All Go source files include Apache-2.0 header comments.
