# ─────────────────────────────────────────────────────────────────────────────
# smart-speaker-mcp — One-line installer for Windows (PowerShell)
#
# Usage:
#   irm https://raw.githubusercontent.com/WeCodeBase/smart-speaker-mcp/main/install.ps1 | iex
#
# This script downloads setup.ps1 from the repo and runs it. The setup script
# in turn downloads the prebuilt binary from the latest GitHub Release.
# ─────────────────────────────────────────────────────────────────────────────

[CmdletBinding()]
param(
    [string]$Version = "latest"
)

$ErrorActionPreference = "Stop"
$Repo    = "WeCodeBase/smart-speaker-mcp"
$RawBase = "https://raw.githubusercontent.com/$Repo/main"
$Url     = "$RawBase/setup.ps1"

$tmp = Join-Path $env:TEMP ("smart-speaker-mcp-setup-{0}.ps1" -f ([guid]::NewGuid().ToString("N")))

try {
    Write-Host "▶ Fetching setup.ps1 from $Repo..."
    Invoke-WebRequest -Uri $Url -OutFile $tmp -UseBasicParsing

    Write-Host "▶ Running setup..."
    Write-Host ""
    & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $tmp -Version $Version
    exit $LASTEXITCODE
} finally {
    if (Test-Path $tmp) { Remove-Item $tmp -Force -ErrorAction SilentlyContinue }
}
