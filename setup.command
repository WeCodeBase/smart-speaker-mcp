#!/bin/bash
# ─────────────────────────────────────────────────────────────────────────────
# Smart Speaker MCP — One-Time Setup
# After this: use the VS Code task "🔄 Rebuild + Restart Claude" after edits.
# ─────────────────────────────────────────────────────────────────────────────

cd "$(dirname "$0")"
PROJECT_DIR="$(pwd)"
BINARY_PATH="$PROJECT_DIR/smart-speaker-mcp"
CONFIG_DIR="$HOME/Library/Application Support/Claude"
CONFIG_FILE="$CONFIG_DIR/claude_desktop_config.json"

echo ""
echo "╔════════════════════════════════════════════════════╗"
echo "║   Smart Speaker MCP — Setup                       ║"
echo "╚════════════════════════════════════════════════════╝"
echo ""

# ── Go ────────────────────────────────────────────────────────────────────────
echo "▶ Checking Go..."
if ! command -v go &>/dev/null; then
    echo "  ❌ Go not found. Install from https://go.dev/dl/"; exit 1
fi
echo "  ✅ $(go version)"

# ── yt-dlp ───────────────────────────────────────────────────────────────────
echo "▶ Checking yt-dlp..."
if ! command -v yt-dlp &>/dev/null; then
    pip3 install yt-dlp --quiet || pip install yt-dlp --quiet
fi
echo "  ✅ yt-dlp ready"

# ── Dependencies ──────────────────────────────────────────────────────────────
echo "▶ Downloading Go dependencies..."
go mod tidy && go mod download
echo "  ✅ Done"

# ── Build ─────────────────────────────────────────────────────────────────────
echo "▶ Building binary..."
go build -o smart-speaker-mcp .
if [ $? -ne 0 ]; then echo "  ❌ Build failed"; exit 1; fi
echo "  ✅ $BINARY_PATH"

# ── Create .env config if it doesn't exist ───────────────────────────────────
ENV_FILE="$HOME/.config/smart-speaker-mcp/.env"
mkdir -p "$HOME/.config/smart-speaker-mcp"
if [ ! -f "$ENV_FILE" ]; then
    echo "▶ Creating $ENV_FILE ..."
    cat > "$ENV_FILE" << 'ENVEOF'
# ─────────────────────────────────────────────────────────────────────────────
# Smart Speaker MCP — Environment Config
# Edit this file to change settings. Restart Claude Desktop to apply.
# ─────────────────────────────────────────────────────────────────────────────

# ── Playback defaults ─────────────────────────────────────────────────────────
SMART_SPEAKER_DEFAULT_DEVICE=Family Room speaker
SMART_SPEAKER_DEVICE_TYPE=google_home
SMART_SPEAKER_SOURCE=local

# ── Local music ───────────────────────────────────────────────────────────────
SMART_SPEAKER_MUSIC_DIR=~/sundar/songs

# ── yt-dlp ────────────────────────────────────────────────────────────────────
SMART_SPEAKER_YTDLP_PATH=/usr/local/bin/yt-dlp

# ── Amazon Alexa (fill in when ready) ────────────────────────────────────────
ALEXA_CLIENT_ID=
ALEXA_CLIENT_SECRET=
ALEXA_ACCESS_TOKEN=
ALEXA_REFRESH_TOKEN=
ALEXA_CUSTOMER_ID=
ENVEOF
    echo "  ✅ Created: $ENV_FILE"
else
    echo "  ✅ .env already exists: $ENV_FILE (not overwritten)"
fi
echo ""

# ── Claude Desktop config ─────────────────────────────────────────────────────
echo "▶ Registering with Claude Desktop..."
mkdir -p "$CONFIG_DIR"
python3 - <<PYEOF
import json, os
cfg = {}
if os.path.exists("$CONFIG_FILE"):
    try:
        cfg = json.load(open("$CONFIG_FILE"))
    except: pass
cfg.setdefault("mcpServers", {})
cfg["mcpServers"]["smart-speaker"] = {"command": "$BINARY_PATH", "args": []}
json.dump(cfg, open("$CONFIG_FILE", "w"), indent=2)
print("  ✅ Registered: $BINARY_PATH")
PYEOF

echo ""
echo "╔════════════════════════════════════════════════════╗"
echo "║  ✅ Setup complete!                                ║"
echo "║                                                    ║"
echo "║  After any code change, use VS Code task:          ║"
echo "║  👉 '🔄 Rebuild + Restart Claude'                  ║"
echo "║     (one click — builds & restarts automatically)  ║"
echo "╚════════════════════════════════════════════════════╝"
echo ""
