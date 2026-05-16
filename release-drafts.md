# smart-speaker-mcp — Release Promotion Drafts

---

## 1. Reddit Post (r/ClaudeAI, r/homeautomation, r/googlehome)

**Title:**
> I built an open-source MCP connector to control Google Home & Alexa from Claude Desktop — now on the official MCP Registry

**Body:**
> Hey everyone! I just released **smart-speaker-mcp** — a free, open-source MCP connector that lets you control your Google Home and Amazon Alexa speakers directly from Claude Desktop using natural language.
>
> **What you can do:**
> - "Play Ilaiyaraaja on Family Room speaker"
> - "Pause the music"
> - "Set volume to 40% on Kitchen display"
> - "Stop the speaker"
>
> **How it works:**
> Claude Desktop talks to a lightweight Go binary over stdio (the Model Context Protocol). The binary discovers your Google Home devices over LAN using mDNS — no cloud, no API keys needed for Google Home. For Alexa, it uses Amazon's Behaviors API via OAuth2.
>
> Local MP3s are served to Chromecast via an embedded HTTP server (Chromecast can't read `file://` paths, so the binary spins up a tiny LAN server automatically).
>
> **Features:**
> - 23 MCP tools covering play, pause, resume, stop, volume, and status
> - Local music library support (MP3/FLAC/M4A)
> - YouTube streaming via yt-dlp fallback
> - Zero-code `.env` config — no JSON editing
> - One-click setup script (`bash setup.command`)
>
> **GitHub:** https://github.com/WeCodeBase/smart-speaker-mcp
>
> **MCP Registry listing:** https://registry.modelcontextprotocol.io/v0.1/servers/io.github.WeCodeBase%2Fsmart-speaker-mcp/versions/1.0.1
>
> It's my first MCP connector and I'd love feedback. Happy to help anyone set it up!

---

## 2. Medium Post

**Title:**
> I Built an MCP Connector to Control Smart Speakers with Claude — Here's How It Works (and How to Publish to the MCP Registry)

**Subtitle:**
> Using Go, mDNS, and the Model Context Protocol to talk to Google Home and Alexa with plain English — plus a battle-tested guide for publishing to the official MCP Server Registry

---

### Introduction

What if you could tell your AI assistant, "Play some Ilaiyaraaja on the Family Room speaker," and it just worked — no app switching, no voice wake words, no fumbling with your phone?

That's exactly what I built: **smart-speaker-mcp**, an open-source connector that bridges Claude Desktop with Google Home and Amazon Alexa speakers. You can control your speakers from the same conversation where you're writing emails, doing research, or planning your week.

Here's how it works, what I learned building it, and a complete walkthrough of publishing it to the official MCP Server Registry — including every error I hit along the way.

---

### What Is MCP?

The **Model Context Protocol (MCP)** is an open standard by Anthropic that lets Claude Desktop connect to external tools via a simple stdio interface. Claude talks to a local binary (any language) using JSON-RPC messages. The binary exposes "tools" — functions Claude can call like `gh_play_music` or `smart_stop`.

This means you can write a Go binary, register it in Claude Desktop's config, and Claude can use it like a native capability. No servers, no APIs, no latency beyond your LAN.

---

### Architecture

```
Claude Desktop
    │  stdio (MCP / JSON-RPC)
    ▼
smart-speaker-mcp (Go binary)
    ├── Local HTTP server  ──▶ Chromecast fetches audio over Wi-Fi
    ├── go-chromecast      ──▶ Google Home / Chromecast LAN control
    ├── Alexa Behaviors API──▶ Amazon Echo control (cloud)
    └── yt-dlp subprocess  ──▶ YouTube audio stream URLs
```

The binary starts when Claude Desktop launches and stays alive as a subprocess.

---

### The Tricky Parts

#### 1. Chromecast Can't Read Local Files

This caught me off guard. Chromecast is a network device — it fetches media over HTTP from a URL. You can't give it a `file:///Users/me/songs/song.mp3` path.

The fix: I embedded a lightweight HTTP server inside the binary that starts on a random LAN port at launch. When you play a local file, the binary resolves the LAN IP, builds a URL like `http://192.168.1.5:54231/localfile?path=/Users/me/songs/song.mp3`, and hands that to Chromecast. It fetches the file over Wi-Fi directly from your Mac.

```go
func startLocalFileServer() error {
    http.HandleFunc("/localfile", func(w http.ResponseWriter, r *http.Request) {
        path := r.URL.Query().Get("path")
        http.ServeFile(w, r, path)
    })
    listener, _ := net.Listen("tcp", ":0") // random port
    localServerPort = listener.Addr().(*net.TCPAddr).Port
    go http.Serve(listener, nil)
    return nil
}
```

#### 2. `app.Load()` Hangs Forever

The `go-chromecast` library has no internal timeout on device connection. If a device is unreachable, `app.Load()` blocks indefinitely — which causes Claude Desktop to hit its 60-second MCP timeout with no feedback.

The fix: wrap every play call in a goroutine with a hard 20-second deadline.

```go
done := make(chan error, 1)
go func() { done <- app.Load(...) }()
select {
case err := <-done:
    // handle result
case <-time.After(20 * time.Second):
    return fmt.Errorf("device did not respond within 20s")
}
```

#### 3. Claude Desktop Doesn't Support `cwd` in MCP Config

I initially tried `go run .` as the MCP command so users wouldn't need to pre-build a binary. It failed — Claude Desktop spawns the process without a working directory, so `go run .` can't find the `.go` files.

The fix: build a proper binary with `go build -o smart-speaker-mcp .` and point the config at the binary path. The setup script handles this automatically.

---

### The 23 Tools

| Category | Tools |
|---|---|
| **Unified** | `smart_play`, `smart_pause`, `smart_resume`, `smart_stop` |
| **Google Home** | `gh_discover_devices`, `gh_play_music`, `gh_pause`, `gh_resume`, `gh_stop`, `gh_set_volume`, `gh_get_status` |
| **Alexa** | `alexa_auth`, `alexa_auth_complete`, `alexa_discover_devices`, `alexa_play_music`, `alexa_pause`, `alexa_resume`, `alexa_stop`, `alexa_set_volume`, `alexa_get_status` |
| **Utilities** | `list_local_music`, `get_config`, `set_config` |

The `smart_*` tools are the most useful — they read your default device and source from `.env` so you can just say "pause" without specifying the device every time.

---

### Config: Zero JSON Required

All settings live in a single `.env` file at `~/.config/smart-speaker-mcp/.env`:

```bash
SMART_SPEAKER_DEFAULT_DEVICE=Family Room speaker
SMART_SPEAKER_DEVICE_TYPE=google_home
SMART_SPEAKER_SOURCE=youtube
SMART_SPEAKER_MUSIC_DIR=~/Music
SMART_SPEAKER_YTDLP_PATH=/usr/local/bin/yt-dlp
```

The binary loads this before JSON config, before shell env — so you get predictable, overridable settings without touching any code.

---

### Setup in 3 Steps

```bash
# 1. Clone
git clone https://github.com/WeCodeBase/smart-speaker-mcp.git
cd smart-speaker-mcp

# 2. One-click setup
bash setup.command

# 3. Restart Claude Desktop
```

The setup script builds the binary, creates your `.env` with sensible defaults, and registers the connector in Claude Desktop's config file automatically.

---

### Publishing to the Official MCP Server Registry

After building the connector, I wanted to list it on the official **MCP Server Registry** (`registry.modelcontextprotocol.io`) — the new searchable catalogue of MCP connectors.

What I didn't expect: five separate errors before it finally went live. Here's the honest account so you don't hit the same walls.

#### The Process (High Level)

1. Build a binary and upload it to your GitHub release
2. Install `mcp-publisher` CLI
3. Create a `server.json` metadata file
4. Authenticate via GitHub OAuth
5. Run `mcp-publisher publish`

Sounds simple. It wasn't.

#### Error 1 — npm 404

The first thing I tried, as suggested in several tutorials:

```
npm install -g @modelcontextprotocol/mcp-publisher
```

```
npm error 404 Not Found - GET https://registry.npmjs.org/@modelcontextprotocol%2Fmcp-publisher
```

The npm package simply doesn't exist. Use Homebrew instead:

```bash
brew install mcp-publisher
```

#### Error 2 — 422: Description Too Long

```
422 Unprocessable Entity: description must be at most 100 characters
```

My original description was 123 characters. The registry enforces a hard 100-character limit. I trimmed it to:

> "Control Google Home and Alexa speakers from Claude Desktop via natural language." (80 chars)

Lesson: count your characters before you publish.

#### Error 3 — 403: Namespace Permission Denied

```
403 Forbidden: you do not have permission to publish under io.github.WeCodeBase/*
```

This one was subtle. I was logged in correctly, but my membership in the **WeCodeBase** GitHub organisation was set to **Private**. The registry couldn't verify I owned the `io.github.WeCodeBase/*` namespace.

Fix: go to `https://github.com/orgs/WeCodeBase/people`, find your username, and set your membership to **Public**. Re-publish immediately after.

#### Error 4 — 400: Wrong SHA256 Field Name

```
400 Bad Request: packages[0].fileSha256 is required
```

I had the hash in the file, just under the wrong key name. The registry schema requires `"fileSha256"`, not `"sha256"`. One character of difference, one frustrating error.

```json
"sha256": "abc123..."      ← rejected
"fileSha256": "abc123..."  ← correct
```

#### Error 5 — 400: Duplicate Version (the --dry-run trap)

This one was the most surprising:

```
400 Bad Request: version 1.0.0 already exists
```

I had run `mcp-publisher publish --dry-run` earlier to test my setup. What I didn't know: **`--dry-run` actually publishes to the live registry**. It's not a preview — it submits for real. So when I ran the actual publish, version 1.0.0 was already taken.

Fix: bump the version to `1.0.1` in all three places in `server.json` (the top-level `"version"` field and both `"version"` fields inside the `"packages"` array).

#### Final Result

After fixing all five errors, the publish succeeded:

```
Publishing...
✓ Successfully published
✓ Server io.github.WeCodeBase/smart-speaker-mcp version 1.0.1
```

Live listing: https://registry.modelcontextprotocol.io/v0.1/servers/io.github.WeCodeBase%2Fsmart-speaker-mcp/versions/1.0.1

---

### What's Next

- **Playlist/queue support** — play a series of tracks in sequence
- **Multi-room sync** — broadcast to multiple devices at once
- **Linux support** — currently macOS only due to `osascript` usage in the dev workflow
- **Pre-built binaries** — so users don't need Go installed

---

### Try It

**GitHub:** https://github.com/WeCodeBase/smart-speaker-mcp

**MCP Registry:** https://registry.modelcontextprotocol.io/v0.1/servers/io.github.WeCodeBase%2Fsmart-speaker-mcp/versions/1.0.1

It's MIT licensed and I'd love contributions, bug reports, and feedback. If you set it up and run into issues, open an issue or drop a comment here.

If you're building your own MCP connectors, the `mark3labs/mcp-go` SDK made the stdio plumbing very straightforward — highly recommended as a starting point. And if you're publishing to the registry, hopefully this saves you an hour of head-scratching.

---

*Built with Go, go-chromecast, yt-dlp, and the Anthropic MCP SDK.*
*Released under the MIT License — free to use, fork, and improve.*
