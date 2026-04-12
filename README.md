# smart-speaker-mcp

Control Google Home and Amazon Alexa speakers through Claude using the [Model Context Protocol](https://modelcontextprotocol.io).

Say things like:
- *"Play Ilaiyaraaja on Family Room speaker"*
- *"Pause the music"*
- *"Set volume to 50% on Kitchen display"*
- *"Play Tamil hits on Family Room speaker"*

---

## Features

- **Google Home / Chromecast** — local LAN playback, no cloud required
- **Amazon Alexa** — cloud control via OAuth2
- **Local music library** — plays your MP3/FLAC/M4A files directly (default)
- **YouTube fallback** — streams via yt-dlp when no local file matches
- **Unified `smart_play`** — one command, reads defaults from config
- **`.env` config** — all settings in one file, no code changes needed

---

## Prerequisites

| Tool | Install |
|------|---------|
| [Go 1.21+](https://go.dev/dl/) | `brew install go` |
| [yt-dlp](https://github.com/yt-dlp/yt-dlp) | `brew install yt-dlp` |
| [Claude Desktop](https://claude.ai/download) | Download and install |

---

## Installation

```bash
# 1. Clone the repo
git clone https://github.com/WeCodeBase/smart-speaker-mcp.git
cd smart-speaker-mcp

# 2. Run one-time setup (builds binary, creates .env, registers with Claude Desktop)
bash setup.command

# 3. Restart Claude Desktop
```

That's it. Ask Claude: *"Discover Google Home devices"*

---

## Configuration

The `.env` file is created at `~/.config/smart-speaker-mcp/.env` on first run:

```bash
# Which speaker to use by default
SMART_SPEAKER_DEFAULT_DEVICE=Family Room speaker

# Device type: google_home | alexa | both
SMART_SPEAKER_DEVICE_TYPE=google_home

# Audio source: local | youtube | url
SMART_SPEAKER_SOURCE=local

# Path to your local music folder
SMART_SPEAKER_MUSIC_DIR=~/Music

# yt-dlp binary path (auto-detected if blank)
SMART_SPEAKER_YTDLP_PATH=/usr/local/bin/yt-dlp

# Amazon Alexa credentials (see Alexa Setup below)
ALEXA_CLIENT_ID=
ALEXA_CLIENT_SECRET=
ALEXA_ACCESS_TOKEN=
ALEXA_REFRESH_TOKEN=
ALEXA_CUSTOMER_ID=
```

Edit this file and run **`🔄 Rebuild + Restart Claude`** in VS Code to apply.

---

## Available Tools

### Unified
| Tool | What it does |
|------|-------------|
| `smart_play` | Play now or schedule — reads device/source from config |

### Google Home
| Tool | What it does |
|------|-------------|
| `gh_discover_devices` | Find all Google Home / Chromecast devices on Wi-Fi |
| `gh_play_music` | Play a song or stream on a specific device |
| `gh_pause` | Pause playback |
| `gh_resume` | Resume playback |
| `gh_stop` | Stop playback |
| `gh_set_volume` | Set volume 0–100 |
| `gh_get_status` | Get current playback status |

### Alexa
| Tool | What it does |
|------|-------------|
| `alexa_auth` | Get Amazon OAuth URL |
| `alexa_auth_complete` | Complete auth with the OAuth code |
| `alexa_discover_devices` | List Echo devices on your account |
| `alexa_play_music` | Play on an Echo device |
| `alexa_pause` / `alexa_resume` | Pause/resume |
| `alexa_set_volume` | Set volume 0–100 |

### Utilities
| Tool | What it does |
|------|-------------|
| `list_local_music` | List files in your music directory |
| `get_config` | Show current config + available env vars |
| `set_config` | Update config at runtime |

---

## Alexa Setup

1. Go to [developer.amazon.com](https://developer.amazon.com) → **Login with Amazon** → Create a new app
2. Set redirect URI to `https://localhost`
3. Copy **Client ID** and **Client Secret** into your `.env`
4. Ask Claude: *"Connect my Alexa account"* — it will give you a URL to open
5. Authorise, copy the `code` from the redirect URL, and say: *"Complete Alexa auth: \<code\>"*

---

## Development

After editing code, rebuild with one VS Code task:

```
⌘⇧P → Run Task → 🔄 Rebuild + Restart Claude
```

This builds the binary and restarts Claude Desktop automatically.

---

## How it works

```
Claude Desktop
    │  stdio (MCP)
    ▼
smart-speaker-mcp (Go binary)
    ├── Local HTTP server  ──► Chromecast fetches audio over LAN
    ├── go-chromecast      ──► Google Home / Chromecast control
    ├── Alexa Behaviors API──► Amazon Echo control (cloud)
    └── yt-dlp subprocess  ──► YouTube audio stream URLs
```

Local files are served over a lightweight embedded HTTP server so Chromecast (a network device) can fetch them directly from your Mac over Wi-Fi.

---

## License

MIT — see [LICENSE](LICENSE)
