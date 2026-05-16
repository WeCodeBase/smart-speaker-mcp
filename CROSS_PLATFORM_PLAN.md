# Cross-Platform Distribution Plan — smart-speaker-mcp

**Goal**: Ship `smart-speaker-mcp` so it installs cleanly on the latest macOS, Windows (10/11), and Linux (Debian/Ubuntu) without requiring users to install Go.

**Approach**: Prebuilt binaries via GitHub Actions release workflow + per-OS install scripts that download the right binary and register it with Claude Desktop.

---

## 1. Why this is straightforward

Verified facts about the current code:

- **Pure Go.** `go.mod` has no CGO dependencies. `mark3labs/mcp-go` and `vishen/go-chromecast` are pure Go; transitive deps (`zeroconf`, `miekg/dns`, `logrus`, etc.) are also pure Go.
- **Stdlib networking only.** `httpserver.go` uses `net` + `net/http` — works identically on all three OSes. The "first non-loopback IPv4" trick in `getLANIP()` is portable.
- **No platform-specific syscalls** anywhere in the project.

Implication: `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build` (and equivalents) produces working binaries with zero source-code changes. The whole job is *packaging* and *install scripts*.

---

## 2. Deliverables

| File | Purpose |
|------|---------|
| `.github/workflows/release.yml` | Cross-compiles on every `v*` tag, attaches binaries to a GitHub Release |
| `setup.sh` | Linux installer (Debian/Ubuntu-tuned, but distro-agnostic where possible) |
| `setup.ps1` | Modern Windows installer (PowerShell 5.1+ / 7+) |
| `setup.bat` | Legacy Windows installer (cmd.exe) |
| `setup.command` | Existing macOS installer — minor hardening only |
| `README.md` | New "Install" section with one-line install command per OS |
| `Makefile` | Optional: `make build-all` for local cross-compile sanity check |
| `go.mod` (line 1 edit) | Fix module path: `github.com/sundaranatarajan/smart-speaker-mcp` → `github.com/WeCodeBase/smart-speaker-mcp` to match the actual repo |

---

## 3. GitHub Actions release workflow

**Trigger**: Push of any tag matching `v*` (e.g. `git tag v3.2.0 && git push --tags`).

**Job**: Single `ubuntu-latest` runner — pure Go means we don't need per-OS runners.

**Build matrix** (5 binaries):

| GOOS | GOARCH | Output filename |
|------|--------|-----------------|
| darwin | arm64 | `smart-speaker-mcp-darwin-arm64` |
| darwin | amd64 | `smart-speaker-mcp-darwin-amd64` |
| windows | amd64 | `smart-speaker-mcp-windows-amd64.exe` |
| linux | amd64 | `smart-speaker-mcp-linux-amd64` |
| linux | arm64 | `smart-speaker-mcp-linux-arm64` |

**Build flags**:
```
CGO_ENABLED=0
go build -trimpath -ldflags="-s -w -X main.version=$TAG" -o $OUTPUT .
```

(`-trimpath` for reproducibility, `-s -w` to strip debug symbols → ~30% smaller binaries.)

**Release step**: Use `softprops/action-gh-release@v2` to attach all 5 binaries to a GitHub Release. Auto-generate release notes from commits since the previous tag.

**Optional**: SHA-256 checksums file (`SHA256SUMS.txt`) for users who want to verify downloads.

---

## 4. Install scripts — what they do (common shape)

Each installer follows the same 5 steps; only the syntax differs per OS.

**Step 1 — Detect arch.** Pick the right binary URL based on `uname -m` / `$env:PROCESSOR_ARCHITECTURE`.

**Step 2 — Download binary.** From the latest GitHub Release (`https://github.com/WeCodeBase/smart-speaker-mcp/releases/latest/download/<binary>`). Drop into a stable per-user location:
- macOS: `~/.local/bin/smart-speaker-mcp` (already lives in PATH for most users)
- Linux: `~/.local/bin/smart-speaker-mcp`
- Windows: `%LOCALAPPDATA%\Programs\smart-speaker-mcp\smart-speaker-mcp.exe`

**Step 3 — Verify yt-dlp.** If missing, attempt install:
- macOS: `brew install yt-dlp`
- Debian/Ubuntu: `sudo apt install -y yt-dlp` (fall back to `pip3 install yt-dlp` if not in repos)
- Windows: `winget install yt-dlp` (PowerShell) or print install instructions (cmd.exe)

**Step 4 — Create `.env`.** Write the default `.env` to:
- macOS/Linux: `~/.config/smart-speaker-mcp/.env`
- Windows: `%APPDATA%\smart-speaker-mcp\.env`

(Don't overwrite if it exists.)

**Step 5 — Register with Claude Desktop.** Merge the `smart-speaker` entry into the JSON config:
- macOS: `~/Library/Application Support/Claude/claude_desktop_config.json`
- Linux: `~/.config/Claude/claude_desktop_config.json`
- Windows: `%APPDATA%\Claude\claude_desktop_config.json`

JSON merge (preserve other MCP servers) — implementation:
- macOS/Linux: inline Python (already installed), same approach as today's `setup.command`
- Windows PowerShell: native `ConvertFrom-Json` / `ConvertTo-Json`
- Windows .bat: shell out to `powershell -Command` for the JSON merge (cmd.exe can't parse JSON)

**Step 6 — Print next steps.** "Restart Claude Desktop. Then ask: 'Discover Google Home devices'."

---

## 5. Per-OS install one-liner (after release is published)

These are what go in the README:

**macOS / Linux:**
```bash
curl -fsSL https://raw.githubusercontent.com/WeCodeBase/smart-speaker-mcp/main/install.sh | bash
```

**Windows (PowerShell):**
```powershell
irm https://raw.githubusercontent.com/WeCodeBase/smart-speaker-mcp/main/install.ps1 | iex
```

(The repo-root `install.sh` / `install.ps1` are tiny wrappers that detect OS and run the right `setup.sh` / `setup.ps1`.)

---

## 6. Edge cases & how the plan handles them

**macOS Gatekeeper warning** ("cannot be opened because the developer cannot be verified"). Two paths:
1. Document the manual unblock (`xattr -d com.apple.quarantine ./smart-speaker-mcp`) in the README.
2. Long-term: code-sign with an Apple Developer ID ($99/yr). Out of scope for v1.

**Windows SmartScreen / Defender** prompt on first launch. Same trade-off:
1. Document "click 'Run anyway'" in README.
2. Long-term: code-sign with an EV certificate (~$200-400/yr). Out of scope for v1.

**Windows firewall prompt** when `httpserver.go` first binds. User clicks "Allow" once. Document in README.

**No `~/.local/bin` in PATH on some Linux distros.** Installer prints a warning and the exact line to add to `~/.bashrc` / `~/.zshrc` if needed. Claude Desktop launches binaries by absolute path so PATH doesn't actually matter for MCP — only matters if the user wants to run `smart-speaker-mcp` directly for debugging.

**Existing `claude_desktop_config.json` with other MCP servers.** Installer must *merge* the `smart-speaker` key, not overwrite the file. Plan uses JSON-aware merging (Python on macOS/Linux, PowerShell on Windows) — never string templating.

**Claude Desktop config path on Linux varies** by Claude Desktop version. Recent builds use `~/.config/Claude/`. Installer probes both `~/.config/Claude/` and `~/.config/claude/` (case-sensitive on Linux) and falls back to creating the canonical one.

**ARM64 Linux** (Raspberry Pi, AWS Graviton). Covered by the `linux-arm64` build in the matrix.

**ARM64 Windows** (Surface Pro X, Snapdragon laptops). Skipping for v1 — Windows-on-ARM market share is small and `go-chromecast` hasn't been tested there. Add later if anyone asks.

---

## 7. Testing plan

Before tagging the first release:

1. **Local cross-compile sanity check** — run `make build-all` on macOS, confirm all 5 binaries produced, verify `file` reports the right arch on each.
2. **macOS smoke test** — run `setup.command`, confirm Claude Desktop sees `smart-speaker` and `gh_discover_devices` returns devices.
3. **Linux smoke test** — Ubuntu 22.04 VM (UTM/Parallels), run `setup.sh`, smoke test the same way.
4. **Windows smoke test** — Windows 11 VM, run `setup.ps1` and `setup.bat` separately, smoke test.
5. **Tag** `v3.2.0`, push, verify the GitHub Action attaches all 5 binaries, then re-run installs from the release URL (not the local source) on each OS.

---

## 8. Effort estimate

Rough hours assuming the current codebase is the starting point:

| Task | Hours |
|------|-------|
| `release.yml` workflow | 1.5 |
| `setup.sh` | 2 |
| `setup.ps1` | 2.5 |
| `setup.bat` (mostly delegating to PowerShell) | 1 |
| README rewrite + per-OS sections | 1.5 |
| `Makefile` | 0.5 |
| Smoke tests on Linux + Windows VMs | 2-3 |
| **Total** | **~11 hours** |

Most of the time is in the Windows scripts (PowerShell JSON merging is fiddly) and the smoke tests. The Go side is essentially untouched.

---

## 9. Out of scope for v1 (deliberate)

- **Code signing** (macOS Developer ID, Windows EV cert). Worth doing once you're charging or distributing widely. Until then, README workaround is fine.
- **Homebrew tap / winget package / Snap / Flatpak**. Nice-to-have once the project has users; the install one-liner covers everyone in the meantime.
- **Auto-update**. Add later if users ask. For now, re-running the installer is the update path.
- **Telemetry / crash reporting**. Out of scope unless you specifically want it.

---

## 10. Open questions for you

1. ~~**Repo URL**~~ — **Resolved.** Confirmed `https://github.com/WeCodeBase/smart-speaker-mcp` from the local `.git/config`. All install URLs and the Actions workflow target this repo.
2. **Release version**: Bump to `v3.2.0` for the first cross-platform release? Current code says `3.1.0` in `main.go`.
3. **License/branding**: README currently says MIT, copyright not specified. Want to add a copyright line (e.g., `Copyright (c) 2026 WeCodeBase`)?

Once you've answered #2 and #3 (or said "looks good, ship it"), I'll start with `release.yml` + the `go.mod` path fix, then move outward to Linux and Windows.
