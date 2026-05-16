// smart-speaker-mcp v4 — Control Google Home / Chromecast speakers from Claude
// via the Model Context Protocol.
//
// Tools exposed:
//   discover_devices  — scan Wi-Fi via mDNS for Chromecast / Google Home devices
//   play              — play a song / artist / direct URL on a speaker
//   pause / resume / stop — playback controls (uses default_device if omitted)
//   set_volume        — set volume 0–100
//   get_status        — query current playback state
//   list_local_music  — list audio files in the configured local music folder
//   get_config / set_config — view and update runtime configuration
//
// All tools are platform-agnostic — the same MCP server runs on macOS, Linux,
// and Windows. See README.md for installation and the embedded web UI.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// version is set at build time via -ldflags="-X main.version=...".
// Falls back to this string when built without ldflags (e.g. `go run`).
var version = "4.0.0"

// ── Tool-result helpers ───────────────────────────────────────────────────────

func toJSON(v any) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}

func ok(msg string) *mcp.CallToolResult   { return mcp.NewToolResultText(msg) }
func fail(msg string) *mcp.CallToolResult { return mcp.NewToolResultText("❌ " + msg) }

func strArg(req mcp.CallToolRequest, key string) string {
	v, _ := req.Params.Arguments[key].(string)
	return v
}

func floatArg(req mcp.CallToolRequest, key string) float64 {
	v, _ := req.Params.Arguments[key].(float64)
	return v
}

// resolveDevice returns the device name from the request or falls back to the
// configured default. Returns a clear error if neither is set.
func resolveDevice(req mcp.CallToolRequest) (string, error) {
	name := strArg(req, "device_name")
	if name == "" {
		name = cfg.DefaultDevice
	}
	if name == "" {
		return "", fmt.Errorf("device_name is required (or set default_device in config)")
	}
	return name, nil
}

// ── Tool handlers ─────────────────────────────────────────────────────────────

func handleDiscoverDevices(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	entries, err := ghDiscoverDevices(8)
	if err != nil {
		return fail(err.Error()), nil
	}
	if len(entries) == 0 {
		return ok("No Chromecast / Google Home devices found on this Wi-Fi."), nil
	}
	type info struct {
		Name string `json:"name"`
		Host string `json:"host"`
		Port int    `json:"port"`
		UUID string `json:"uuid"`
	}
	devices := make([]info, 0, len(entries))
	for _, e := range entries {
		devices = append(devices, info{e.GetName(), e.GetAddr(), e.GetPort(), e.GetUUID()})
	}
	return ok(toJSON(devices)), nil
}

func handlePlay(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query := strArg(req, "query")
	directURL := strArg(req, "url")
	if query == "" && directURL == "" {
		return fail("provide query (song/artist) or url"), nil
	}

	deviceName, err := resolveDevice(req)
	if err != nil {
		return fail(err.Error()), nil
	}

	source := strArg(req, "source")
	if source == "" {
		source = cfg.DefaultSource
	}
	startTime := int(floatArg(req, "start_time"))

	effectiveQuery := query
	if effectiveQuery == "" {
		effectiveQuery = directURL
	}

	// Force YouTube → temporarily blank music dir so resolveMedia skips local lookup.
	if source == "youtube" {
		saved := cfg.MusicDir
		cfg.MusicDir = ""
		defer func() { cfg.MusicDir = saved }()
	}

	media, err := resolveMedia(effectiveQuery, directURL, startTime)
	if err != nil {
		return fail("could not resolve media: " + err.Error()), nil
	}
	msg, err := ghPlayMedia(deviceName, media)
	if err != nil {
		return fail(err.Error()), nil
	}
	return ok(msg), nil
}

func handlePause(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	device, err := resolveDevice(req)
	if err != nil {
		return fail(err.Error()), nil
	}
	if err := ghPause(device); err != nil {
		return fail(err.Error()), nil
	}
	return ok("⏸  Paused on " + device), nil
}

func handleResume(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	device, err := resolveDevice(req)
	if err != nil {
		return fail(err.Error()), nil
	}
	if err := ghResume(device); err != nil {
		return fail(err.Error()), nil
	}
	return ok("▶️  Resumed on " + device), nil
}

func handleStop(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	device, err := resolveDevice(req)
	if err != nil {
		return fail(err.Error()), nil
	}
	if err := ghStop(device); err != nil {
		return fail(err.Error()), nil
	}
	return ok("⏹  Stopped on " + device), nil
}

func handleSetVolume(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	device, err := resolveDevice(req)
	if err != nil {
		return fail(err.Error()), nil
	}
	level := floatArg(req, "level")
	if err := ghSetVolume(device, level); err != nil {
		return fail(err.Error()), nil
	}
	return ok(fmt.Sprintf("🔊 Volume set to %.0f%% on %s", level, device)), nil
}

func handleGetStatus(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	device, err := resolveDevice(req)
	if err != nil {
		return fail(err.Error()), nil
	}
	status, err := ghGetStatus(device)
	if err != nil {
		return fail(err.Error()), nil
	}
	return ok(toJSON(status)), nil
}

func handleListLocalMusic(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	files, err := ListLocalMusic(50)
	if err != nil {
		return fail(err.Error()), nil
	}
	if len(files) == 0 {
		return ok(fmt.Sprintf("No audio files found in music_dir: %s", cfg.MusicDir)), nil
	}
	return ok(fmt.Sprintf("🎵 %d audio file(s) in %s:\n%s",
		len(files), cfg.MusicDir, strings.Join(files, "\n"))), nil
}

func handleGetConfig(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	display := map[string]any{
		"version":        version,
		"default_device": cfg.DefaultDevice,
		"default_source": cfg.DefaultSource,
		"music_dir":      cfg.MusicDir,
		"ytdlp_path":     cfg.YtDlpPath,
		"env_overrides": map[string]string{
			"SMART_SPEAKER_DEFAULT_DEVICE": "default_device",
			"SMART_SPEAKER_SOURCE":         "default_source (local|youtube|url)",
			"SMART_SPEAKER_MUSIC_DIR":      "music_dir",
			"SMART_SPEAKER_YTDLP_PATH":     "ytdlp_path",
		},
	}
	return ok(toJSON(display)), nil
}

func handleSetConfig(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if v := strArg(req, "default_device"); v != "" {
		cfg.DefaultDevice = v
	}
	if v := strArg(req, "default_source"); v != "" {
		cfg.DefaultSource = v
	}
	if v := strArg(req, "music_dir"); v != "" {
		cfg.MusicDir = v
	}
	if v := strArg(req, "ytdlp_path"); v != "" {
		cfg.YtDlpPath = v
	}
	if err := saveConfig(); err != nil {
		return fail("could not save config: " + err.Error()), nil
	}
	return ok("✅ Config updated and saved."), nil
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	if err := loadConfig(); err != nil {
		log.Fatalf("Config error: %v", err)
	}

	if err := startLocalFileServer(); err != nil {
		log.Printf("Warning: local file server failed to start: %v", err)
	}

	s := server.NewMCPServer("smart-speaker-mcp", version, server.WithToolCapabilities(true))

	deviceArg := mcp.WithString("device_name",
		mcp.Description("Speaker name (uses default_device from config if omitted)"))

	// Discovery
	s.AddTool(mcp.NewTool("discover_devices",
		mcp.WithDescription("Scan the local Wi-Fi via mDNS for Chromecast / Google Home / Nest devices. "+
			"Returns name, host, port, and UUID for each device found."),
	), handleDiscoverDevices)

	// Play
	s.AddTool(mcp.NewTool("play",
		mcp.WithDescription("Play a song, artist, or direct URL on a Chromecast / Google Home device. "+
			"Source priority: local file → direct URL → YouTube fallback. "+
			"All args except query/url are optional if defaults are set in config."),
		mcp.WithString("query", mcp.Description("Song name, artist, or search query")),
		mcp.WithString("url", mcp.Description("Direct HTTP/HTTPS URL to stream (alternative to query)")),
		deviceArg,
		mcp.WithString("source", mcp.Description("local | youtube | url (default: config.default_source)")),
		mcp.WithNumber("start_time", mcp.Description("Start at N seconds into the track")),
	), handlePlay)

	// Playback controls
	s.AddTool(mcp.NewTool("pause",
		mcp.WithDescription("Pause playback on a Chromecast / Google Home device."),
		deviceArg,
	), handlePause)

	s.AddTool(mcp.NewTool("resume",
		mcp.WithDescription("Resume playback on a Chromecast / Google Home device."),
		deviceArg,
	), handleResume)

	s.AddTool(mcp.NewTool("stop",
		mcp.WithDescription("Stop playback on a Chromecast / Google Home device."),
		deviceArg,
	), handleStop)

	s.AddTool(mcp.NewTool("set_volume",
		mcp.WithDescription("Set volume (0–100) on a Chromecast / Google Home device."),
		mcp.WithNumber("level", mcp.Required(), mcp.Description("Volume level 0–100")),
		deviceArg,
	), handleSetVolume)

	s.AddTool(mcp.NewTool("get_status",
		mcp.WithDescription("Get current playback status (now-playing, volume, state) of a device."),
		deviceArg,
	), handleGetStatus)

	// Diagnostics + config
	s.AddTool(mcp.NewTool("list_local_music",
		mcp.WithDescription("List audio files (.mp3 / .flac / .m4a / .wav) in the configured local music directory."),
	), handleListLocalMusic)

	s.AddTool(mcp.NewTool("get_config",
		mcp.WithDescription("Show current configuration: defaults, paths, version, and available env-var overrides."),
	), handleGetConfig)

	s.AddTool(mcp.NewTool("set_config",
		mcp.WithDescription("Update one or more config settings. Saved to ~/.config/smart-speaker-mcp/config.json."),
		mcp.WithString("default_device", mcp.Description("Default speaker name for play / pause / etc. when device_name is omitted")),
		mcp.WithString("default_source", mcp.Description("local | youtube | url")),
		mcp.WithString("music_dir", mcp.Description("Path to local music library")),
		mcp.WithString("ytdlp_path", mcp.Description("Full path to yt-dlp binary")),
	), handleSetConfig)

	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("MCP server error: %v", err)
	}
}
