# setup-ollama.ps1 — Install Ollama and pull a model for use with Glimpse (Windows)
#
# Usage (PowerShell, run as Administrator for the installer):
#   .\scripts\setup-ollama.ps1                    # installs default model (qwen2.5-coder:7b)
#   .\scripts\setup-ollama.ps1 llama3.2           # installs a specific model
#   .\scripts\setup-ollama.ps1 qwen2.5-coder:14b  # larger variant for better quality
#
# After running this script, start Glimpse and enter:
#   AI Model: local
#   Local LLM URL: http://localhost:11434

param(
    [string]$Model = "qwen2.5-coder:7b"
)

$ErrorActionPreference = "Stop"

$OLLAMA_URL = "http://localhost:11434"
$INSTALLER_URL = "https://ollama.com/download/OllamaSetup.exe"
$INSTALLER_PATH = "$env:TEMP\OllamaSetup.exe"

function Write-Info    { Write-Host "[glimpse] $args" -ForegroundColor Cyan }
function Write-Success { Write-Host "[glimpse] $args" -ForegroundColor Green }
function Write-Warn    { Write-Host "[glimpse] $args" -ForegroundColor Yellow }
function Write-Err     { Write-Host "[glimpse] ERROR: $args" -ForegroundColor Red; exit 1 }

# ── 1. Install Ollama ─────────────────────────────────────────────────────────
$ollamaCmd = Get-Command ollama -ErrorAction SilentlyContinue
if ($ollamaCmd) {
    Write-Success "Ollama is already installed."
} else {
    Write-Info "Downloading Ollama installer..."
    try {
        Invoke-WebRequest -Uri $INSTALLER_URL -OutFile $INSTALLER_PATH -UseBasicParsing
    } catch {
        Write-Err "Failed to download Ollama installer: $_"
    }

    Write-Info "Running Ollama installer (this may open a UAC prompt)..."
    Start-Process -FilePath $INSTALLER_PATH -ArgumentList "/S" -Wait
    Remove-Item $INSTALLER_PATH -ErrorAction SilentlyContinue

    # Refresh PATH so 'ollama' is available in this session.
    $env:Path = [System.Environment]::GetEnvironmentVariable("Path", "Machine") + ";" +
                [System.Environment]::GetEnvironmentVariable("Path", "User")

    $ollamaCmd = Get-Command ollama -ErrorAction SilentlyContinue
    if (-not $ollamaCmd) {
        Write-Warn "Ollama was installed but is not yet on PATH."
        Write-Warn "Please open a new PowerShell window and run this script again."
        exit 0
    }
    Write-Success "Ollama installed."
}

# ── 2. Start Ollama server (background, if not already running) ───────────────
$serverRunning = $false
try {
    $resp = Invoke-WebRequest -Uri $OLLAMA_URL -UseBasicParsing -TimeoutSec 2 -ErrorAction SilentlyContinue
    if ($resp.StatusCode -lt 500) { $serverRunning = $true }
} catch {}

if ($serverRunning) {
    Write-Success "Ollama server is already running at $OLLAMA_URL"
} else {
    Write-Info "Starting Ollama server in the background..."
    Start-Process -FilePath "ollama" -ArgumentList "serve" -WindowStyle Hidden
    # Wait up to 10 s for the server to become ready.
    $ready = $false
    for ($i = 1; $i -le 10; $i++) {
        Start-Sleep -Seconds 1
        try {
            $r = Invoke-WebRequest -Uri $OLLAMA_URL -UseBasicParsing -TimeoutSec 1 -ErrorAction SilentlyContinue
            if ($r.StatusCode -lt 500) { $ready = $true; break }
        } catch {}
    }
    if (-not $ready) {
        Write-Err "Ollama server did not start in time. Run 'ollama serve' manually in a separate window."
    }
    Write-Success "Ollama server started."
}

# ── 3. Pull the model ─────────────────────────────────────────────────────────
Write-Info "Pulling model '$Model' — this may take a few minutes on first run..."
& ollama pull $Model
if ($LASTEXITCODE -ne 0) {
    Write-Err "Failed to pull model '$Model'."
}
Write-Success "Model '$Model' is ready."

# ── 4. Usage hint ─────────────────────────────────────────────────────────────
Write-Host ""
Write-Host "════════════════════════════════════════" -ForegroundColor White
Write-Host "  Ollama is ready for Glimpse!" -ForegroundColor Green
Write-Host "════════════════════════════════════════" -ForegroundColor White
Write-Host ""
Write-Host "  Start Glimpse and enter the following when prompted:" -ForegroundColor White
Write-Host ""
Write-Host "    AI Model:      " -ForegroundColor Cyan -NoNewline; Write-Host "local"
Write-Host "    Local LLM URL: " -ForegroundColor Cyan -NoNewline; Write-Host $OLLAMA_URL
Write-Host ""
Write-Host "  Or use a specific model name:" -ForegroundColor White
Write-Host "    AI Model:      " -ForegroundColor Cyan -NoNewline; Write-Host "local/$Model"
Write-Host ""
Write-Host "  To keep Ollama running: " -ForegroundColor Cyan -NoNewline; Write-Host "ollama serve"
Write-Host ""
