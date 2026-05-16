#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────────────────────
# smart-speaker-mcp — Linux installer (Debian/Ubuntu tuned, distro-agnostic)
#
# Downloads the right prebuilt binary for your CPU, ensures yt-dlp is present,
# creates the .env config, and registers the MCP server with Claude Desktop.
#
# Usage:
#   bash setup.sh                  # uses the latest GitHub Release
#   VERSION=v3.2.0 bash setup.sh   # pin a specific release
# ─────────────────────────────────────────────────────────────────────────────

set -euo pipefail

REPO="WeCodeBase/smart-speaker-mcp"
VERSION="${VERSION:-latest}"
INSTALL_DIR="$HOME/.local/bin"
BINARY_PATH="$INSTALL_DIR/smart-speaker-mcp"
CONFIG_DIR="$HOME/.config/smart-speaker-mcp"
ENV_FILE="$CONFIG_DIR/.env"
CLAUDE_CFG_DIR="$HOME/.config/Claude"
CLAUDE_CFG="$CLAUDE_CFG_DIR/claude_desktop_config.json"

cyan()  { printf "\033[36m%s\033[0m\n" "$*"; }
green() { printf "\033[32m%s\033[0m\n" "$*"; }
red()   { printf "\033[31m%s\033[0m\n" "$*" >&2; }
warn()  { printf "\033[33m%s\033[0m\n" "$*"; }

cyan "╔════════════════════════════════════════════════════╗"
cyan "║   smart-speaker-mcp — Linux setup                 ║"
cyan "╚════════════════════════════════════════════════════╝"
echo

# ── Detect arch ───────────────────────────────────────────────────────────────
case "$(uname -m)" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) red "Unsupported CPU: $(uname -m). Build from source instead."; exit 1 ;;
esac
ASSET="smart-speaker-mcp-linux-${ARCH}"
echo "▶ Detected: linux/${ARCH}"

# ── Resolve download URL ──────────────────────────────────────────────────────
if [ "$VERSION" = "latest" ]; then
  URL="https://github.com/${REPO}/releases/latest/download/${ASSET}"
else
  URL="https://github.com/${REPO}/releases/download/${VERSION}/${ASSET}"
fi

# ── Download binary ───────────────────────────────────────────────────────────
echo "▶ Downloading ${ASSET}..."
mkdir -p "$INSTALL_DIR"
if ! curl -fL -o "$BINARY_PATH" "$URL"; then
  red "Download failed: $URL"
  red "Check that the release exists: https://github.com/${REPO}/releases"
  exit 1
fi
chmod +x "$BINARY_PATH"
green "  ✅ Installed: $BINARY_PATH"
echo

# ── PATH hint (Claude Desktop uses absolute path, so this is just for CLI use) ─
case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *) warn "  ℹ  $INSTALL_DIR is not in your PATH. Add this to ~/.bashrc or ~/.zshrc:"
     warn "       export PATH=\"\$HOME/.local/bin:\$PATH\""
     echo ;;
esac

# ── yt-dlp ────────────────────────────────────────────────────────────────────
echo "▶ Checking yt-dlp..."
if command -v yt-dlp &>/dev/null; then
  green "  ✅ yt-dlp: $(command -v yt-dlp)"
else
  warn "  ℹ  yt-dlp not found — installing"
  if command -v apt-get &>/dev/null; then
    sudo apt-get update -qq && sudo apt-get install -y yt-dlp || pip3 install --user yt-dlp
  elif command -v dnf &>/dev/null; then
    sudo dnf install -y yt-dlp || pip3 install --user yt-dlp
  elif command -v pacman &>/dev/null; then
    sudo pacman -S --noconfirm yt-dlp || pip3 install --user yt-dlp
  else
    pip3 install --user yt-dlp || { red "Install yt-dlp manually then re-run."; exit 1; }
  fi
  green "  ✅ yt-dlp installed"
fi
echo

# ── .env config ───────────────────────────────────────────────────────────────
echo "▶ Setting up .env config at $ENV_FILE..."
mkdir -p "$CONFIG_DIR"
if [ -f "$ENV_FILE" ]; then
  green "  ✅ .env already exists (not overwritten)"
else
  YTDLP_PATH="$(command -v yt-dlp 2>/dev/null || echo /usr/local/bin/yt-dlp)"
  cat > "$ENV_FILE" <<ENVEOF
# ─────────────────────────────────────────────────────────────────────────────
# Smart Speaker MCP — Environment Config (Linux)
# Edit this file and restart Claude Desktop to apply.
# ─────────────────────────────────────────────────────────────────────────────

SMART_SPEAKER_DEFAULT_DEVICE=Family Room speaker
SMART_SPEAKER_SOURCE=local
SMART_SPEAKER_MUSIC_DIR=$HOME/Music
SMART_SPEAKER_YTDLP_PATH=$YTDLP_PATH
ENVEOF
  green "  ✅ Created: $ENV_FILE"
fi
echo

# ── Register with Claude Desktop ──────────────────────────────────────────────
echo "▶ Registering with Claude Desktop..."
mkdir -p "$CLAUDE_CFG_DIR"
if ! command -v python3 &>/dev/null; then
  red "python3 not found — needed to safely merge claude_desktop_config.json"
  red "Install python3 and re-run, or edit the file manually:"
  red "  $CLAUDE_CFG"
  red "Add: \"smart-speaker\": {\"command\": \"$BINARY_PATH\", \"args\": []}"
  exit 1
fi

python3 - <<PYEOF
import json, os
cfg_path = "$CLAUDE_CFG"
cfg = {}
if os.path.exists(cfg_path):
    try:
        with open(cfg_path) as f:
            cfg = json.load(f)
    except Exception as e:
        print(f"  ⚠  Existing config is not valid JSON ({e}); replacing.")
        cfg = {}
cfg.setdefault("mcpServers", {})
cfg["mcpServers"]["smart-speaker"] = {"command": "$BINARY_PATH", "args": []}
os.makedirs(os.path.dirname(cfg_path), exist_ok=True)
with open(cfg_path, "w") as f:
    json.dump(cfg, f, indent=2)
print(f"  ✅ Registered in: {cfg_path}")
PYEOF

echo
green "╔════════════════════════════════════════════════════════════════╗"
green "║  ✅ Setup complete!                                            ║"
green "║                                                                ║"
green "║  Next: restart Claude Desktop, then ask:                       ║"
green "║  👉 \"Discover Google Home devices\"                            ║"
green "║                                                                ║"
green "║  If you cloned the repo and want to rebuild from source later: ║"
green "║                                                                ║"
green "║  • macOS / Linux  →  make rebuild                              ║"
green "║  • Windows (PS)   →  make rebuild     (Git Bash / MSYS / WSL)  ║"
green "║                       …or:  go build .  +  restart Claude      ║"
green "║  • VS Code        →  Ctrl+Shift+P → Run Task → 🔄 Rebuild      ║"
green "║                                                                ║"
green "║  Just rebuild (no restart):  make build   /   go build .       ║"
green "║  Just restart Claude:        make restart-claude               ║"
green "╚════════════════════════════════════════════════════════════════╝"
