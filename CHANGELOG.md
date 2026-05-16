# Changelog

All notable changes to **smart-speaker-mcp** will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [4.0.0] — 2026-05-12

First public release. Major cleanup that removes a non-working Alexa integration and collapses a duplicated tool surface into a single clean API.

### Removed (breaking)

- **All Alexa support.** Amazon retired the Pre-built Routines API on 2026-05-13 and stopped accepting new AVS product registrations earlier, so no third-party path can reliably control existing Echo devices via this codebase's original architecture. Voice Monkey was evaluated as an alternative but rejected for v4.0 to keep the project dependency-free; it may return as an optional plugin later.
- All `alexa_*` MCP tools (`alexa_play_music`, `alexa_pause`, `alexa_resume`, `alexa_stop`, `alexa_set_volume`, `alexa_get_status`, `alexa_discover_devices`, `alexa_auth`, `alexa_auth_complete`).
- `AlexaConfig` struct and the `Alexa` field on `Config`.
- All `ALEXA_*` and `VOICEMONKEY_*` environment variable overrides.
- `device_type` config field — there's only one supported backend now.
- `SMART_SPEAKER_DEVICE_TYPE` env var.
- The `smart_*` tool family (duplicated `gh_*`).
- The `gh_*` tool family (its features are now in the unprefixed names).

### Changed (breaking)

- **Tool naming.** Replaced 14 tools with a single clean 10-tool API. Migration:

  | Old | New |
  | --- | --- |
  | `gh_discover_devices` | `discover_devices` |
  | `gh_play_music`, `smart_play` | `play` |
  | `gh_pause`, `smart_pause` | `pause` |
  | `gh_resume`, `smart_resume` | `resume` |
  | `gh_stop`, `smart_stop` | `stop` |
  | `gh_set_volume` | `set_volume` |
  | `gh_get_status` | `get_status` |
  | `list_local_music` | `list_local_music` (unchanged) |
  | `get_config` / `set_config` | `get_config` / `set_config` (unchanged, simpler schema) |

- `set_config` no longer accepts `device_type`, `alexa_client_id`, `alexa_client_secret`, `voicemonkey_token`, `default_speaker_device`, or `routine_map`.
- Default `MusicDir` is now `~/Music` (was `~/sundar/songs`).

### Added

- `CHANGELOG.md` (this file).
- Build flag `-X main.version=...` so the version reported by the MCP server matches the release tag.

### Fixed

- `setup.command` now runs `go vet ./...` before building, catching syntax and import errors immediately instead of in the middle of `go build`.

---

## [3.x] and earlier

Pre-release iterations — not published. See git history for the development trail.
