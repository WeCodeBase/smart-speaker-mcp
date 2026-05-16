#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────────────────────
# smart-speaker-mcp — One-line installer for macOS / Linux
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/WeCodeBase/smart-speaker-mcp/main/install.sh | bash
#
# This script:
#   1. Detects the OS (macOS vs Linux)
#   2. Downloads the matching setup script (setup.command or setup.sh)
#   3. Runs it
#
# It does NOT clone the repo — the OS-specific setup script downloads only
# the prebuilt binary it needs from the latest GitHub Release.
# ─────────────────────────────────────────────────────────────────────────────

set -euo pipefail

REPO="WeCodeBase/smart-speaker-mcp"
RAW_BASE="https://raw.githubusercontent.com/${REPO}/main"

case "$(uname -s)" in
  Darwin)
    SCRIPT_NAME="setup.command"
    ;;
  Linux)
    SCRIPT_NAME="setup.sh"
    ;;
  *)
    echo "Unsupported OS: $(uname -s). Supported: macOS, Linux." >&2
    echo "For Windows, use install.ps1 from PowerShell instead." >&2
    exit 1
    ;;
esac

TMP_SCRIPT="$(mktemp -t smart-speaker-mcp-setup.XXXXXX)"
trap 'rm -f "$TMP_SCRIPT"' EXIT

echo "▶ Fetching ${SCRIPT_NAME} from ${REPO}..."
if ! curl -fsSL "${RAW_BASE}/${SCRIPT_NAME}" -o "$TMP_SCRIPT"; then
  echo "Download failed: ${RAW_BASE}/${SCRIPT_NAME}" >&2
  exit 1
fi

echo "▶ Running setup..."
echo
bash "$TMP_SCRIPT"
