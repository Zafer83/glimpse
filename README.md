# Glimpse ✨

[![Go Report Card](https://goreportcard.com/badge/github.com/Zafer83/glimpse)](https://goreportcard.com/report/github.com/Zafer83/glimpse)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Version](https://img.shields.io/github/go-mod/go-version/Zafer83/glimpse)](https://golang.org/doc/devel/release.html)

**Glimpse** is an AI-driven Go CLI that turns source code into professional [Slidev](https://sli.dev/) presentations.

## 🚀 Features

- Interactive CLI flow for all inputs (path, theme, model, language, output, API key)
- OpenAI and Gemini support
- Automatic Gemini model resolution/fallback when a requested model is unavailable
- Slidev-ready Markdown output with architecture-focused summaries and Mermaid diagrams
- Recursive code crawling with common folders ignored (`.git`, `node_modules`, build artifacts)

## 🛠️ Installation

### 1. Clone

```bash
git clone https://github.com/Zafer83/glimpse
cd glimpse
```

### 2. Install dependencies

```bash
go mod tidy
```

### 3. Build

```bash
go build -o glimpse ./cmd/glimpse
```

## 📖 Usage

Run the binary and answer prompts:

```bash
./glimpse
```

Or run directly with Go:

```bash
go run ./cmd/glimpse/main.go
```

The CLI asks for:

1. `Project Path`
2. `Slidev Theme`
3. `AI Model (gpt-4o or gemini-1.5-pro)`
4. `Language`
5. `File Name`
6. `API Key (OpenAI/Gemini)`

### Provider selection

- If model starts with `gemini` (or `models/gemini...`), Glimpse uses Gemini.
- Otherwise, Glimpse uses OpenAI.

### Path autocompletion

- In a standard terminal (`Terminal`, `iTerm`, etc.), `Project Path` supports `Tab` completion.
- In some embedded IDE terminals, line-editing is disabled automatically for stability.

## ✅ Example models

- OpenAI: `gpt-4o`
- Gemini: `gemini-1.5-pro`, `gemini-2.0-flash`

## 👀 View the generated slides

```bash
npx slidev slides.md
```

## 📂 Project structure

```text
glimpse/
├── cmd/glimpse/       # CLI entry point
├── internal/
│   ├── ai/            # OpenAI/Gemini integration
│   ├── config/        # Shared config struct
│   └── crawler/       # File system scanning
└── README.md
```

## 🛡️ Security & Privacy

- Glimpse scans local files and sends code content to the selected AI provider (OpenAI or Gemini).
- Do not commit API keys.

## 📄 License

This project is licensed under Apache-2.0. See [LICENSE](./LICENSE).

## 🤝 Contributing

Contributions and issues are welcome: [issues page](https://github.com/Zafer83/glimpse/issues).
