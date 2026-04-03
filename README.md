# Glimpse ✨

[![Go Report Card](https://goreportcard.com/badge/github.com/Zafer83/glimpse)](https://goreportcard.com/report/github.com/Zafer83/glimpse)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Version](https://img.shields.io/github/go-mod/go-version/Zafer83/glimpse)](https://golang.org/doc/devel/release.html)

**Glimpse** is a powerful AI-driven CLI tool written in Go that automatically transforms your source code into professional [Slidev](https://sli.dev/) presentations. It uses Large Language Models to analyze your project's logic, highlight key snippets, and generate architectural visualizations.

---

## 🚀 Features

- **AI-Powered Analysis:** Leverages OpenAI (GPT-4o) to understand code intent and architectural patterns.
- **Developer-Centric:** Designed specifically for technical deep-dives and documentation.
- **Slidev Ready:** Generates `.md` files that work out-of-the-box with the Slidev ecosystem.
- **Smart Context Crawling:** Efficiently scans your project while ignoring `node_modules`, `.git`, and build artifacts.
- **Visual Diagrams:** Automatically generates **Mermaid.js** charts (sequence, flow, or class diagrams) based on your code.
- **Highly Configurable:** Use `.env` files or CLI flags to customize themes, models, and languages.

---

## 🛠️ Installation

### 1. Clone the repository
```bash
git clone [https://github.com/Zafer83/glimpse](https://github.com/Zafer83/glimpse)
cd glimpse
```

### 2. Install dependencies

``` bash
go mod tidy
```

## 3. Build the tool

``` bash
go build -o glimpse ./cmd/glimpse
```

------------------------------------------------------------------------

## ⚙️ Configuration

Glimpse uses environment variables for configuration. Create a `.env`
file in the root directory:

``` env
OPENAI_API_KEY=your_openai_api_key_here
GLIMPSE_THEME=seriph
GLIMPSE_MODEL=gpt-4o
GLIMPSE_LANGUAGE=en
GLIMPSE_OUTPUT=slides.md
```

### Configuration Options

  -----------------------------------------------------------------------------
Variable           Description                                    Default
  ------------------ ---------------------------------------------- -----------
OPENAI_API_KEY     Your OpenAI API Key (Required)                 \-

GLIMPSE_THEME      The Slidev theme to apply                      seriph

GLIMPSE_MODEL      OpenAI Model (e.g., gpt-4o, gpt-3.5-turbo)     gpt-4o

GLIMPSE_LANGUAGE   Output language for the slides (en, de, etc.)  en

GLIMPSE_OUTPUT     The filename for the generated Markdown        slides.md
-----------------------------------------------------------------------------

------------------------------------------------------------------------

## 📖 Usage

Point Glimpse to any directory containing source code (e.g., `./src`).

``` bash
# Using the compiled binary
./glimpse -path ./src -out my-presentation.md

# Using go run
go run ./cmd/glimpse/main.go -path ./src
```

### Viewing the Slides

Once Glimpse has generated your file, you can view it immediately using
Slidev:

``` bash
npx slidev slides.md
```

------------------------------------------------------------------------

## 📂 Project Structure

``` plaintext
glimpse/
├── cmd/glimpse/       # Entry point for the CLI
├── internal/
│   ├── ai/            # OpenAI API integration
│   ├── config/        # Environment and flag management
│   └── crawler/       # File system scanning logic
├── .env               # Local configuration (Git ignored)
└── README.md          # You are here!
```

------------------------------------------------------------------------

## 🛡️ Security & Privacy

Glimpse scans your local files but only sends the code content to the AI
provider (OpenAI) for analysis.

-   It automatically ignores sensitive folders like `.git` and
    `node_modules`.
-   **Note:** Ensure your `.env` file is never committed to version
    control.

------------------------------------------------------------------------

## 📄 License

This project is licensed under the Apache-2.0 license. See the `LICENSE` file
for details.

------------------------------------------------------------------------

## 🤝 Contributing

Contributions, issues, and feature requests are welcome! Feel free to check the [issues page](https://github.com/Zafer83/glimpse/issues).

To contribute, please follow these steps:

1. **Fork the Project**
2. **Create your Feature Branch**

    ``` bash
    git checkout -b feature/AmazingFeature
    ```
3. **Commit your Changes**

    ``` bash
    git commit -m 'Add some AmazingFeature'
    ```

4. **Push to the Branch**

    ``` bash
    git push origin feature/AmazingFeature
    ```

5. **Open a Pull Request**
