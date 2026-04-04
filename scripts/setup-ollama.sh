#!/usr/bin/env bash
# setup-ollama.sh — Install Ollama and pull a model for use with Glimpse (Mac / Linux)
#
# Usage:
#   ./scripts/setup-ollama.sh                    # installs default model (qwen2.5-coder:7b)
#   ./scripts/setup-ollama.sh llama3.2           # installs a specific model
#   ./scripts/setup-ollama.sh qwen2.5-coder:14b  # larger variant for better quality
#
# After running this script, start Glimpse and enter:
#   AI Model: local
#   Local LLM URL: http://localhost:11434

set -euo pipefail

MODEL="${1:-qwen2.5-coder:7b}"
OLLAMA_URL="http://localhost:11434"

# ── Colors ────────────────────────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
YELLOW='\033[1;33m'
BOLD='\033[1m'
RESET='\033[0m'

info()    { echo -e "${CYAN}[glimpse]${RESET} $*"; }
success() { echo -e "${GREEN}[glimpse]${RESET} $*"; }
warn()    { echo -e "${YELLOW}[glimpse]${RESET} $*"; }
error()   { echo -e "${RED}[glimpse] ERROR:${RESET} $*" >&2; exit 1; }

# ── 1. Install Ollama ─────────────────────────────────────────────────────────
if command -v ollama &>/dev/null; then
    success "Ollama is already installed: $(ollama --version 2>/dev/null || echo 'version unknown')"
else
    info "Installing Ollama..."
    if [[ "$(uname)" == "Darwin" ]]; then
        if command -v brew &>/dev/null; then
            brew install ollama
        else
            curl -fsSL https://ollama.com/install.sh | sh
        fi
    else
        curl -fsSL https://ollama.com/install.sh | sh
    fi
    success "Ollama installed."
fi

# ── 2. Start Ollama server (background, if not already running) ───────────────
if curl -sf "${OLLAMA_URL}" &>/dev/null; then
    success "Ollama server is already running at ${OLLAMA_URL}"
else
    info "Starting Ollama server in the background..."
    ollama serve &>/dev/null &
    OLLAMA_PID=$!
    # Wait up to 10 s for the server to be ready.
    for i in $(seq 1 10); do
        if curl -sf "${OLLAMA_URL}" &>/dev/null; then
            success "Ollama server started (PID ${OLLAMA_PID})"
            break
        fi
        sleep 1
        if [[ $i -eq 10 ]]; then
            error "Ollama server did not start in time. Run 'ollama serve' manually."
        fi
    done
fi

# ── 3. Pull the model ─────────────────────────────────────────────────────────
info "Pulling model '${BOLD}${MODEL}${RESET}${CYAN}' — this may take a few minutes on first run..."
ollama pull "${MODEL}"
success "Model '${MODEL}' is ready."

# ── 4. Usage hint ─────────────────────────────────────────────────────────────
echo ""
echo -e "${BOLD}════════════════════════════════════════${RESET}"
echo -e "${GREEN}  Ollama is ready for Glimpse!${RESET}"
echo -e "${BOLD}════════════════════════════════════════${RESET}"
echo ""
echo -e "  Start Glimpse and enter the following when prompted:"
echo ""
echo -e "    ${CYAN}AI Model:${RESET}      local"
echo -e "    ${CYAN}Local LLM URL:${RESET} ${OLLAMA_URL}"
echo ""
echo -e "  Or use a specific model name:"
echo -e "    ${CYAN}AI Model:${RESET}      local/${MODEL}"
echo ""
echo -e "  To keep Ollama running in a separate terminal: ${CYAN}ollama serve${RESET}"
echo ""
