<a href="https://github.com/Zafer83/glimpse">
  <img src="https://raw.githubusercontent.com/Zafer83/glimpse/main/glimpse.png" alt="Glimpse Logo" width="300">
</a>

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

### Interactive Mode

Start Glimpse without Flags — das Programm fragt alle Eingaben interaktiv ab:

```bash
./glimpse
```

### Non-Interactive Mode (CLI Flags)

Alle Parameter können als Flags übergeben werden. Sobald `--path` gesetzt ist, läuft Glimpse vollständig non-interactive — ideal für Skripte, CI/CD und Automatisierung.

```bash
./glimpse --path ./my-project --model gpt-4o --api-key sk-... --theme seriph --lang en --output presentation.md --slidev
```

### `./glimpse --help`

Zeigt alle verfügbaren Flags:

```
Usage of glimpse:
  -api-key string
        API key (or set GLIMPSE_API_KEY env)
  -lang string
        Presentation language (default: de)
  -local-url string
        Local LLM base URL (default: http://localhost:11434)
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
  -v    Print version and exit
  -version
        Print version and exit
```

### Flag Reference

| Flag | Typ | Default | Beschreibung |
|------|-----|---------|--------------|
| `--path` | string | *(keiner)* | Pfad zum Projekt-Verzeichnis. **Pflicht** für Non-Interactive Mode. Unterstützt relative Pfade und `~/`. |
| `--model` | string | `gemini-2.0-flash` | AI-Modell. Siehe [Model examples](#model-examples) für gültige Werte. |
| `--api-key` | string | *(keiner)* | API-Key für Cloud-Modelle (OpenAI, Gemini, Anthropic). Alternativ via Umgebungsvariable `GLIMPSE_API_KEY`. Nicht benötigt für lokale Modelle. |
| `--theme` | string | `seriph` | Slidev-Theme. Offizielle Kurzform (`seriph`, `default`, `bricks`) oder voller npm-Paketname. |
| `--lang` | string | `de` | Sprache der generierten Präsentation (z.B. `de`, `en`, `fr`, `tr`). |
| `--output` | string | `slides.md` | Dateiname der erzeugten Slidev-Markdown-Datei. |
| `--local-url` | string | `http://localhost:11434` | Basis-URL des lokalen LLM-Servers (Ollama oder llama.cpp). |
| `--slidev` | bool | `false` | Startet Slidev automatisch nach der Generierung. Nur im Non-Interactive Mode relevant. |
| `--version`, `-v` | bool | `false` | Gibt die Version aus und beendet das Programm. |

### Beispiele

**Cloud-Modell mit OpenAI:**

```bash
./glimpse --path ~/Projects/my-app --model gpt-4o --api-key sk-abc123 --lang en
```

**Lokales Modell mit Ollama (kein API-Key nötig):**

```bash
./glimpse --path ./my-project --model local/qwen2.5-coder:7b
```

**Automatische Slidev-Präsentation starten:**

```bash
./glimpse --path ./my-project --model local --slidev --output demo.md
```

**API-Key über Umgebungsvariable:**

```bash
export GLIMPSE_API_KEY=sk-abc123
./glimpse --path ./my-project --model gpt-4o
```

**Version anzeigen:**

```bash
./glimpse --version
./glimpse -v
```

### CI/CD & Pipeline Integration

Glimpse lässt sich vollständig non-interaktiv in CI/CD-Pipelines einbinden.
Sobald `--path` gesetzt ist, werden keine interaktiven Eingaben erwartet.

**GitHub Actions Beispiel:**

```yaml
- name: Generate presentation
  env:
    GLIMPSE_API_KEY: ${{ secrets.OPENAI_API_KEY }}
  run: |
    ./glimpse --path . --model gpt-4o --theme seriph --lang en --output slides.md
```

**Shell-Skript Beispiel (mit Ollama):**

```bash
#!/bin/bash
set -e
./glimpse --path "$PROJECT_DIR" --model local --output slides.md
echo "Presentation generated: slides.md"
```

**Umgebungsvariablen:**

| Variable | Beschreibung |
|----------|-------------|
| `GLIMPSE_API_KEY` | API-Key (Alternative zu `--api-key` Flag) |
| `NO_COLOR` | Deaktiviert Farb-Ausgabe (Standard für CI) |
| `GLIMPSE_NO_ANSI` | Deaktiviert ANSI-Escape-Codes explizit |

**Exit-Codes:**

| Code | Bedeutung |
|------|-----------|
| `0` | Erfolgreich |
| `1` | Fehler (fehlender API-Key, Scan-Fehler, AI-Fehler) |
| `130` | Abbruch durch Benutzer (Ctrl+C) |

### Supported File Types

Glimpse erkennt und priorisiert automatisch:

**Dokumentation (höchste Priorität — 60% Budget):**

| Format | Endung |
|--------|--------|
| Markdown | `.md`, `.mdx` |
| Plain Text | `.txt`, `.rst` |
| Word | `.docx` (native Go-Extraktion) |
| PDF | `.pdf` (native Go-Extraktion) |

**Business Logic (35% Budget):**
`.go`, `.js`, `.ts`, `.jsx`, `.tsx`, `.py`, `.java`, `.rb`, `.php`, `.cs`, `.cpp`, `.c`, `.h`, `.rs`, `.sql`

**Support-Dateien (5% Budget):**
Tests, Configs, Migrations, Generated Code — werden automatisch klassifiziert und nachrangig behandelt.

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
  - `local` (defaults to `qwen2.5-coder:7b`)
  - `local/qwen2.5-coder:14b`

### Local server URL input

If you choose a local model, Glimpse asks for:

- `Local LLM URL (e.g. http://localhost:11434)`

For Ollama, use:

- `http://localhost:11434` (Glimpse uses `/api/chat`)

For `llama.cpp` OpenAI-compatible server, use:

- `http://localhost:8080` (Glimpse uses `/v1/chat/completions`)

## Local Mode (Ollama) — no API key required

Glimpse works fully offline with [Ollama](https://ollama.com). No OpenAI, Gemini, or Anthropic account needed.

### Quick setup

**Mac / Linux:**

```bash
./scripts/setup-ollama.sh
```

Install a specific model (optional — default is `qwen2.5-coder:7b`):

```bash
./scripts/setup-ollama.sh llama3.2
./scripts/setup-ollama.sh qwen2.5-coder:14b
```

**Windows (PowerShell, run as Administrator):**

```powershell
.\scripts\setup-ollama.ps1
```

Install a specific model:

```powershell
.\scripts\setup-ollama.ps1 llama3.2
.\scripts\setup-ollama.ps1 qwen2.5-coder:14b
```

The script will:
1. Install Ollama if it is not already installed
2. Start the Ollama server in the background
3. Download the chosen model
4. Print the exact values to enter in Glimpse

### Manual setup

If you prefer to set up Ollama yourself:

```bash
# 1. Install Ollama
# macOS:  brew install ollama  OR  curl -fsSL https://ollama.com/install.sh | sh
# Linux:  curl -fsSL https://ollama.com/install.sh | sh
# Windows: download from https://ollama.com/download

# 2. Start the server
ollama serve

# 3. Pull a model (in a separate terminal)
ollama pull qwen2.5-coder:7b
```

### Recommended models

| Model | Size | Best for |
|---|---|---|
| `qwen2.5-coder:7b` | ~4 GB | Code analysis — good default |
| `qwen2.5-coder:14b` | ~8 GB | Higher quality, needs more RAM |
| `llama3.2` | ~2 GB | Fast, lower RAM |
| `codellama:7b` | ~4 GB | Alternative code model |

### Glimpse settings for Ollama

When Glimpse prompts you, enter:

```
AI Model:      local
Local LLM URL: http://localhost:11434
API Key:        (leave empty)
```

Or use a specific model name:

```
AI Model:      local/qwen2.5-coder:7b
```

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
