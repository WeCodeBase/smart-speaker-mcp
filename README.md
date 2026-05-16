# smart-speaker-mcp

Control Chromecast / Google Home / Nest speakers from Claude (or any MCP client) via the [Model Context Protocol](https://modelcontextprotocol.io). Cross-platform — runs on macOS, Windows, and Linux.

Say things like:

- *"Play Ilaiyaraaja on the Family Room speaker"*
- *"Pause the music"*
- *"Set volume to 50% on the kitchen display"*
- *"What's playing right now?"*

---

## Install

> One line per OS. Downloads a prebuilt binary, ensures `yt-dlp` is present, creates the config file, and registers the MCP server with Claude Desktop. No Go toolchain required.

**macOS / Linux:**

```bash
curl -fsSL https://raw.githubusercontent.com/WeCodeBase/smart-speaker-mcp/main/install.sh | bash
```

**Windows (PowerShell):**

```powershell
irm https://raw.githubusercontent.com/WeCodeBase/smart-speaker-mcp/main/install.ps1 | iex
```

After install, **restart Claude Desktop**, then ask Claude: *"Discover my speakers."*

---

## Web control panel

The server hosts a small web UI on **http://localhost:8765** that lets you play music, control volume, and re-scan for devices from any browser on the same Wi-Fi (laptop or phone). It's running whenever the MCP server is running.

To override the port set `SMART_SPEAKER_WEB_PORT` in `.env`. If 8765 is taken, the server falls back to a random port and prints the chosen URL to stderr — visible in Claude Desktop's MCP server log.

---

## Tools exposed

| Tool | What it does |
|------|-------------|
| `discover_devices` | Scan Wi-Fi for Chromecast / Google Home / Nest devices |
| `play` | Play a song, artist, or direct URL on a speaker |
| `pause` / `resume` / `stop` | Playback controls |
| `set_volume` | Set volume 0–100 |
| `get_status` | Current playback state |
| `list_local_music` | List audio files in your music folder |
| `get_config` / `set_config` | View / update runtime configuration |

All playback tools accept `device_name` as an optional argument and fall back to `default_device` from your config when omitted.

---

## Configuration (`.env`)

Located at `~/.config/smart-speaker-mcp/.env` on macOS/Linux, or `%APPDATA%\smart-speaker-mcp\.env` on Windows.

```bash
# Default speaker (exact name shown by discover_devices)
SMART_SPEAKER_DEFAULT_DEVICE=Family Room speaker

# Audio source priority: local | youtube | url
SMART_SPEAKER_SOURCE=local

# Path to your local music folder (.mp3 / .flac / .m4a / .wav)
SMART_SPEAKER_MUSIC_DIR=~/Music

# Path to yt-dlp binary (auto-detected if blank)
SMART_SPEAKER_YTDLP_PATH=

# Port for the embedded web UI (default 8765)
SMART_SPEAKER_WEB_PORT=8765
```

Edit this file and restart Claude Desktop to apply, or use the `set_config` tool to update values without editing the file.

---

## What the installer puts where

| OS | Binary | `.env` config | Claude Desktop config |
|----|--------|---------------|----------------------|
| macOS | `~/.local/bin/smart-speaker-mcp` (or project dir if built locally) | `~/.config/smart-speaker-mcp/.env` | `~/Library/Application Support/Claude/claude_desktop_config.json` |
| Linux | `~/.local/bin/smart-speaker-mcp` | `~/.config/smart-speaker-mcp/.env` | `~/.config/Claude/claude_desktop_config.json` |
| Windows | `%LOCALAPPDATA%\Programs\smart-speaker-mcp\smart-speaker-mcp.exe` | `%APPDATA%\smart-speaker-mcp\.env` | `%APPDATA%\Claude\claude_desktop_config.json` |

---

## First-launch warnings (expected)

| OS | What you'll see | What to click |
|----|-----------------|---------------|
| macOS | "smart-speaker-mcp cannot be opened — unidentified developer" | System Settings → Privacy & Security → "Open Anyway". Or run `xattr -d com.apple.quarantine ~/.local/bin/smart-speaker-mcp` once. |
| Windows | SmartScreen: "Windows protected your PC" | "More info" → "Run anyway" |
| Windows | Defender Firewall prompt (local file server binds to LAN) | "Allow access" on Private networks |
| Linux | None typically | — |

These prompts appear because the binary isn't code-signed. Code-signing certificates cost $99–$400/yr and will be added if/when the project warrants it.

---

## Build from source (optional)

You only need this if you're modifying the code. Otherwise the installer above is faster.

**Prerequisites:** Go 1.21+, `yt-dlp`, Claude Desktop.

```bash
git clone https://github.com/WeCodeBase/smart-speaker-mcp.git
cd smart-speaker-mcp
```

### Initial setup (per platform)

| Platform | Command | What it does |
|----------|---------|--------------|
| macOS | `bash setup.command` | Builds, creates `.env`, registers with Claude Desktop |
| Linux | `bash setup.sh` | Same — Debian/Ubuntu tuned, falls back to dnf/pacman/pip |
| Windows (PowerShell) | `.\setup.ps1` | Same — uses winget for `yt-dlp` |
| Windows (cmd.exe) | `setup.bat` | Thin wrapper around `setup.ps1` |

### Rebuild after a code change

| Platform | Build only | Build + restart Claude |
|----------|------------|-----------------------|
| macOS / Linux | `go build .` *or* `make build` | `make rebuild` |
| Windows (PowerShell) | `go build .` | `go build . ; Stop-Process -Name Claude -Force ; Start-Process Claude` |
| Windows (Git Bash / WSL) | `make build` | `make rebuild` |
| VS Code (any platform) | — | `Ctrl+Shift+P` → Run Task → **🔄 Rebuild + Restart Claude** |

`make rebuild` auto-detects your OS and uses the right "restart Claude" command for it.

### Cross-compile all release targets

```bash
make build-all   # darwin-arm64, darwin-amd64, windows-amd64, linux-amd64, linux-arm64
```

---

## Releasing (maintainer notes)

Tagged releases are built and published automatically by `.github/workflows/release.yml`.

```bash
make tag VERSION=v4.0.0
```

The Action cross-compiles for all five OS/arch targets and attaches the binaries plus `SHA256SUMS.txt` to the GitHub Release.

---

## How it works

```
Claude Desktop  ──stdio (MCP)──▶  smart-speaker-mcp (Go binary, single static executable)
                                       ├── go-chromecast   ──▶ Chromecast / Google Home control over LAN
                                       ├── HTTP server     ──▶ Streams local files to Chromecast over Wi-Fi
                                       ├── HTTP server     ──▶ Web UI for browser/phone control
                                       └── yt-dlp          ──▶ YouTube audio stream URLs
```

Local audio files are served over a lightweight embedded HTTP server so Chromecast (a network device) can fetch them directly from your computer over Wi-Fi.

---

## Why no Alexa support?

Earlier versions tried. Amazon retired the Pre-built Routines API on 2026-05-13 and stopped accepting new AVS product registrations before that, so the original direct-control approach can no longer work for any new account. The remaining third-party paths (Voice Monkey, cookie-based reverse engineering of the Alexa app) all add either external service dependencies or brittle scrapers that break when Amazon updates their auth flow. We removed Alexa entirely in v4.0.0 to keep the project clean. If you need Echo control, [Voice Monkey](https://voicemonkey.io) is the cleanest standalone option today — and may return as an optional plugin to this project in a future version. See [CHANGELOG.md](CHANGELOG.md) for full background.

---

## License

MIT — Copyright (c) 2026 Sundara Senthil. See [LICENSE](LICENSE).
