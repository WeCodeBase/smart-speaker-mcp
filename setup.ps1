# ─────────────────────────────────────────────────────────────────────────────
# smart-speaker-mcp — Windows installer (PowerShell 5.1+ / 7+)
#
# Downloads the prebuilt Windows binary, ensures yt-dlp is present, creates
# the .env config, and registers the MCP server with Claude Desktop.
#
# Usage (Windows Terminal / PowerShell):
#   .\setup.ps1                    # latest release
#   .\setup.ps1 -Version v3.2.0    # pin a specific release
#
# If you see "execution policy" errors, run once:
#   Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass
# ─────────────────────────────────────────────────────────────────────────────

[CmdletBinding()]
param(
    [string]$Version = "latest",
    [string]$Repo    = "WeCodeBase/smart-speaker-mcp"
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

function Write-Cyan  { param($Msg) Write-Host $Msg -ForegroundColor Cyan }
function Write-Green { param($Msg) Write-Host $Msg -ForegroundColor Green }
function Write-Warn  { param($Msg) Write-Host $Msg -ForegroundColor Yellow }
function Write-Err   { param($Msg) Write-Host $Msg -ForegroundColor Red }

Write-Cyan "╔════════════════════════════════════════════════════╗"
Write-Cyan "║   smart-speaker-mcp — Windows setup               ║"
Write-Cyan "╚════════════════════════════════════════════════════╝"
Write-Host ""

# ── Paths ─────────────────────────────────────────────────────────────────────
$InstallDir = Join-Path $env:LOCALAPPDATA "Programs\smart-speaker-mcp"
$BinaryPath = Join-Path $InstallDir "smart-speaker-mcp.exe"
$ConfigDir  = Join-Path $env:APPDATA   "smart-speaker-mcp"
$EnvFile    = Join-Path $ConfigDir     ".env"
$ClaudeDir  = Join-Path $env:APPDATA   "Claude"
$ClaudeCfg  = Join-Path $ClaudeDir     "claude_desktop_config.json"

# ── Detect arch (Windows on ARM not currently supported) ──────────────────────
$arch = $env:PROCESSOR_ARCHITECTURE
if ($arch -ne "AMD64") {
    Write-Err "Unsupported CPU architecture: $arch (only AMD64/x64 is currently built)."
    Write-Err "Open an issue if you need ARM64 Windows support."
    exit 1
}
$Asset = "smart-speaker-mcp-windows-amd64.exe"
Write-Host "▶ Detected: windows/amd64"

# ── Resolve URL ───────────────────────────────────────────────────────────────
if ($Version -eq "latest") {
    $Url = "https://github.com/$Repo/releases/latest/download/$Asset"
} else {
    $Url = "https://github.com/$Repo/releases/download/$Version/$Asset"
}

# ── Download binary ───────────────────────────────────────────────────────────
Write-Host "▶ Downloading $Asset..."
New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
try {
    Invoke-WebRequest -Uri $Url -OutFile $BinaryPath -UseBasicParsing
} catch {
    Write-Err "Download failed: $Url"
    Write-Err "Check that the release exists: https://github.com/$Repo/releases"
    exit 1
}
Write-Green "  ✅ Installed: $BinaryPath"
Write-Host ""

# ── yt-dlp ────────────────────────────────────────────────────────────────────
Write-Host "▶ Checking yt-dlp..."
$ytdlpCmd = Get-Command yt-dlp -ErrorAction SilentlyContinue
if ($ytdlpCmd) {
    Write-Green "  ✅ yt-dlp: $($ytdlpCmd.Source)"
    $YtdlpPath = $ytdlpCmd.Source
} else {
    Write-Warn "  ℹ  yt-dlp not found — attempting install via winget"
    if (Get-Command winget -ErrorAction SilentlyContinue) {
        winget install --id yt-dlp.yt-dlp --accept-package-agreements --accept-source-agreements -h
        $ytdlpCmd = Get-Command yt-dlp -ErrorAction SilentlyContinue
        if ($ytdlpCmd) {
            $YtdlpPath = $ytdlpCmd.Source
            Write-Green "  ✅ yt-dlp installed: $YtdlpPath"
        } else {
            Write-Warn "  ⚠  yt-dlp install completed but binary not on PATH yet."
            Write-Warn "     Open a new terminal and re-run, or install manually:"
            Write-Warn "       https://github.com/yt-dlp/yt-dlp#installation"
            $YtdlpPath = "yt-dlp.exe"
        }
    } else {
        Write-Warn "  ⚠  winget not available. Install yt-dlp manually:"
        Write-Warn "       https://github.com/yt-dlp/yt-dlp#installation"
        $YtdlpPath = "yt-dlp.exe"
    }
}
Write-Host ""

# ── .env config ───────────────────────────────────────────────────────────────
Write-Host "▶ Setting up .env config at $EnvFile..."
New-Item -ItemType Directory -Force -Path $ConfigDir | Out-Null
if (Test-Path $EnvFile) {
    Write-Green "  ✅ .env already exists (not overwritten)"
} else {
    $musicDir = Join-Path $env:USERPROFILE "Music"
    $envContent = @"
# ─────────────────────────────────────────────────────────────────────────────
# Smart Speaker MCP — Environment Config (Windows)
# Edit this file and restart Claude Desktop to apply.
# ─────────────────────────────────────────────────────────────────────────────

SMART_SPEAKER_DEFAULT_DEVICE=Family Room speaker
SMART_SPEAKER_SOURCE=local
SMART_SPEAKER_MUSIC_DIR=$musicDir
SMART_SPEAKER_YTDLP_PATH=$YtdlpPath
"@
    Set-Content -Path $EnvFile -Value $envContent -Encoding UTF8
    Write-Green "  ✅ Created: $EnvFile"
}
Write-Host ""

# ── Register with Claude Desktop ──────────────────────────────────────────────
Write-Host "▶ Registering with Claude Desktop..."
New-Item -ItemType Directory -Force -Path $ClaudeDir | Out-Null

# Load existing config (preserve other MCP servers); start fresh if invalid
$cfg = $null
if (Test-Path $ClaudeCfg) {
    try {
        $cfg = Get-Content $ClaudeCfg -Raw | ConvertFrom-Json
    } catch {
        Write-Warn "  ⚠  Existing config is not valid JSON; replacing."
        $cfg = $null
    }
}
if ($null -eq $cfg) { $cfg = [pscustomobject]@{} }
if (-not ($cfg.PSObject.Properties.Name -contains "mcpServers")) {
    $cfg | Add-Member -NotePropertyName "mcpServers" -NotePropertyValue ([pscustomobject]@{})
}

# Build the smart-speaker entry
$entry = [pscustomobject]@{
    command = $BinaryPath
    args    = @()
}

# Upsert the entry (Add-Member -Force replaces existing)
$cfg.mcpServers | Add-Member -NotePropertyName "smart-speaker" -NotePropertyValue $entry -Force

# Write back, pretty-printed
$cfg | ConvertTo-Json -Depth 10 | Set-Content -Path $ClaudeCfg -Encoding UTF8
Write-Green "  ✅ Registered in: $ClaudeCfg"
Write-Host ""

Write-Green "╔══════════════════════════════════════════════════════════════════╗"
Write-Green "║  ✅ Setup complete!                                              ║"
Write-Green "║                                                                  ║"
Write-Green "║  Next: restart Claude Desktop, then ask:                         ║"
Write-Green "║   👉 ""Discover Google Home devices""                           ║"
Write-Green "║                                                                  ║"
Write-Green "║  First launch:                                                   ║"
Write-Green "║   • SmartScreen → ""More info"" → ""Run anyway""                ║"
Write-Green "║   • Defender Firewall → ""Allow access"" (Private networks)     ║"
Write-Green "║                                                                  ║"
Write-Green "║  Rebuild from source later (if you cloned the repo):             ║"
Write-Green "║                                                                  ║"
Write-Green "║   PowerShell:    go build . ; .\setup.ps1                        ║"
Write-Green "║   Git Bash/WSL:  make rebuild                                    ║"
Write-Green "║   VS Code:       Ctrl+Shift+P → Run Task → 🔄 Rebuild           ║"
Write-Green "║                                                                  ║"
Write-Green "║  Restart Claude only (no rebuild):                               ║"
Write-Green "║   Stop-Process -Name Claude -Force ; Start-Process Claude        ║"
Write-Green "╚══════════════════════════════════════════════════════════════════╝"
