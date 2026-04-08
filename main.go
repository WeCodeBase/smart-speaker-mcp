// Smart Speaker MCP Connector v3.0
// Controls Google Home and Amazon Alexa via the Model Context Protocol.
//
// Key features:
//   smart_play   — auto-detects device type + source from config; play now or schedule
//   gh_*         — direct Google Home / Chromecast controls
//   alexa_*      — direct Amazon Alexa controls
//   get_config / set_config — view and change runtime settings
//
// Config file : ~/.config/smart-speaker-mcp/config.json
// Env overrides: see config.go for the full list

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

// ── Helpers ───────────────────────────────────────────────────────────────────

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

// ── smart_play ────────────────────────────────────────────────────────────────
// Unified play command: reads defaults from config, auto-selects device type,
// chooses local vs cloud source, and optionally schedules for a future time.

func handleSmartPlay(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query := strArg(req, "query")
	directURL := strArg(req, "url")
	if query == "" && directURL == "" {
		return fail("provide query (song/artist) or url"), nil
	}

	// Device: arg > config default
	deviceName := strArg(req, "device_name")
	if deviceName == "" {
		deviceName = cfg.DefaultDevice
	}
	if deviceName == "" {
		return fail("no device specified and default_device is not set in config"), nil
	}

	// Device type: arg > config
	deviceType := strArg(req, "device_type")
	if deviceType == "" {
		deviceType = cfg.DeviceType
	}

	// Source: arg > config (influences resolveMedia path)
	source := strArg(req, "source")
	if source == "" {
		source = cfg.DefaultSource
	}

	startTime := int(floatArg(req, "start_time"))

	effectiveQuery := query
	if query == "" {
		effectiveQuery = directURL
	}

	// Play now
	if source == "youtube" {
		// Force YouTube — temporarily blank music dir so resolveMedia skips local
		saved := cfg.MusicDir
		cfg.MusicDir = ""
		defer func() { cfg.MusicDir = saved }()
	}

	m, err := resolveMedia(effectiveQuery, directURL, startTime)
	if err != nil {
		return fail("could not resolve media: " + err.Error()), nil
	}

	return playOnDeviceType(deviceName, deviceType, effectiveQuery, m)
}

// playOnDeviceType routes the play command to GH, Alexa, or both.
func playOnDeviceType(deviceName, deviceType, query string, m *MediaResult) (*mcp.CallToolResult, error) {
	switch strings.ToLower(deviceType) {

	case "alexa":
		device, err := alexaGetDeviceByName(deviceName)
		if err != nil {
			return fail("Alexa device not found: " + err.Error()), nil
		}
		if err := alexaPlayMusic(device, query); err != nil {
			return fail("Alexa play error: " + err.Error()), nil
		}
		return ok(fmt.Sprintf("▶️  Playing '%s' [alexa] on %s", query, deviceName)), nil

	case "both":
		var msgs []string
		// Google Home
		if msg, err := ghPlayMedia(deviceName, m); err == nil {
			msgs = append(msgs, msg)
		} else {
			msgs = append(msgs, "GH error: "+err.Error())
		}
		// Alexa
		if device, err := alexaGetDeviceByName(deviceName); err == nil {
			if err := alexaPlayMusic(device, query); err == nil {
				msgs = append(msgs, fmt.Sprintf("▶️  Playing on Alexa: %s", deviceName))
			} else {
				msgs = append(msgs, "Alexa error: "+err.Error())
			}
		} else {
			msgs = append(msgs, "Alexa: "+err.Error())
		}
		return ok(strings.Join(msgs, "\n")), nil

	default: // "google_home"
		msg, err := ghPlayMedia(deviceName, m)
		if err != nil {
			return fail(err.Error()), nil
		}
		return ok(msg), nil
	}
}

// ── Google Home handlers ──────────────────────────────────────────────────────

func handleGHDiscover(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	entries, err := ghDiscoverDevices(8)
	if err != nil {
		return fail(err.Error()), nil
	}
	if len(entries) == 0 {
		return ok("No Google Home / Chromecast devices found on this Wi-Fi."), nil
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

func handleGHPlay(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	deviceName := strArg(req, "device_name")
	query := strArg(req, "query")
	directURL := strArg(req, "url")
	startTime := int(floatArg(req, "start_time"))

	if deviceName == "" {
		return fail("device_name is required"), nil
	}
	if query == "" && directURL == "" {
		return fail("either query or url is required"), nil
	}
	if query == "" {
		query = directURL
	}

	m, err := resolveMedia(query, directURL, startTime)
	if err != nil {
		return fail("media not found: " + err.Error()), nil
	}
	msg, err := ghPlayMedia(deviceName, m)
	if err != nil {
		return fail(err.Error()), nil
	}
	return ok(msg), nil
}

func handleGHPause(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := ghPause(strArg(req, "device_name")); err != nil {
		return fail(err.Error()), nil
	}
	return ok("⏸  Paused on " + strArg(req, "device_name")), nil
}

func handleGHResume(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := ghResume(strArg(req, "device_name")); err != nil {
		return fail(err.Error()), nil
	}
	return ok("▶️  Resumed on " + strArg(req, "device_name")), nil
}

func handleGHStop(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := ghStop(strArg(req, "device_name")); err != nil {
		return fail(err.Error()), nil
	}
	return ok("⏹  Stopped on " + strArg(req, "device_name")), nil
}

func handleGHVolume(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	level := floatArg(req, "level")
	if err := ghSetVolume(strArg(req, "device_name"), level); err != nil {
		return fail(err.Error()), nil
	}
	return ok(fmt.Sprintf("🔊 Volume set to %.0f%% on %s", level, strArg(req, "device_name"))), nil
}

func handleGHStatus(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	status, err := ghGetStatus(strArg(req, "device_name"))
	if err != nil {
		return fail(err.Error()), nil
	}
	return ok(toJSON(status)), nil
}

// ── Alexa handlers ────────────────────────────────────────────────────────────

func handleAlexaAuth(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	authURL, err := AlexaAuthURL()
	if err != nil {
		return fail(err.Error()), nil
	}
	return ok(fmt.Sprintf(
		"Open this URL in your browser:\n\n%s\n\n"+
			"After authorising, copy the 'code' from the redirect URL and call alexa_auth_complete.",
		authURL,
	)), nil
}

func handleAlexaAuthComplete(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	code := strArg(req, "code")
	if code == "" {
		return fail("code is required"), nil
	}
	if err := AlexaExchangeCode(code); err != nil {
		return fail(err.Error()), nil
	}
	return ok("✅ Alexa connected! Tokens saved."), nil
}

func handleAlexaDiscover(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	devices, err := alexaDiscoverDevices()
	if err != nil {
		return fail(err.Error()), nil
	}
	if len(devices) == 0 {
		return ok("No Alexa devices found."), nil
	}
	return ok(toJSON(devices)), nil
}

func handleAlexaPlay(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	deviceName := strArg(req, "device_name")
	query := strArg(req, "query")
	device, err := alexaGetDeviceByName(deviceName)
	if err != nil {
		return fail(err.Error()), nil
	}
	if err := alexaPlayMusic(device, query); err != nil {
		return fail(err.Error()), nil
	}
	return ok(fmt.Sprintf("▶️  Playing '%s' on %s", query, deviceName)), nil
}

func handleAlexaPause(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	device, err := alexaGetDeviceByName(strArg(req, "device_name"))
	if err != nil {
		return fail(err.Error()), nil
	}
	if err := alexaPause(device); err != nil {
		return fail(err.Error()), nil
	}
	return ok("⏸  Paused on " + strArg(req, "device_name")), nil
}

func handleAlexaResume(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	device, err := alexaGetDeviceByName(strArg(req, "device_name"))
	if err != nil {
		return fail(err.Error()), nil
	}
	if err := alexaResume(device); err != nil {
		return fail(err.Error()), nil
	}
	return ok("▶️  Resumed on " + strArg(req, "device_name")), nil
}

func handleAlexaVolume(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	device, err := alexaGetDeviceByName(strArg(req, "device_name"))
	if err != nil {
		return fail(err.Error()), nil
	}
	level := int(floatArg(req, "level"))
	if err := alexaSetVolume(device, level); err != nil {
		return fail(err.Error()), nil
	}
	return ok(fmt.Sprintf("🔊 Volume set to %d%% on %s", level, strArg(req, "device_name"))), nil
}

// ── Local music diagnostic ────────────────────────────────────────────────────

func handleListLocalMusic(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	files, err := ListLocalMusic(50)
	if err != nil {
		return fail(err.Error()), nil
	}
	if len(files) == 0 {
		return ok(fmt.Sprintf("No audio files found in music_dir: %s", cfg.MusicDir)), nil
	}
	return ok(fmt.Sprintf("🎵 %d audio file(s) in %s:\n%s", len(files), cfg.MusicDir, strings.Join(files, "\n"))), nil
}

// ── Config handlers ───────────────────────────────────────────────────────────

func handleGetConfig(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	display := map[string]any{
		"ytdlp_path":     cfg.YtDlpPath,
		"music_dir":      cfg.MusicDir,
		"default_device": cfg.DefaultDevice,
		"device_type":    cfg.DeviceType,
		"default_source": cfg.DefaultSource,
		"alexa": map[string]any{
			"client_id":     cfg.Alexa.ClientID,
			"client_secret": maskSecret(cfg.Alexa.ClientSecret),
			"access_token":  maskSecret(cfg.Alexa.AccessToken),
			"refresh_token": maskSecret(cfg.Alexa.RefreshToken),
			"authenticated": cfg.Alexa.RefreshToken != "",
		},
		"env_overrides": map[string]string{
			"SMART_SPEAKER_MUSIC_DIR":      "music_dir",
			"SMART_SPEAKER_DEFAULT_DEVICE": "default_device",
			"SMART_SPEAKER_DEVICE_TYPE":    "device_type (google_home|alexa|both)",
			"SMART_SPEAKER_SOURCE":         "default_source (local|youtube|url)",
			"SMART_SPEAKER_YTDLP_PATH":     "ytdlp_path",
			"ALEXA_CLIENT_ID":              "alexa.client_id",
			"ALEXA_CLIENT_SECRET":          "alexa.client_secret",
		},
	}
	return ok(toJSON(display)), nil
}

func handleSetConfig(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if v := strArg(req, "ytdlp_path"); v != "" {
		cfg.YtDlpPath = v
	}
	if v := strArg(req, "music_dir"); v != "" {
		cfg.MusicDir = v
	}
	if v := strArg(req, "default_device"); v != "" {
		cfg.DefaultDevice = v
	}
	if v := strArg(req, "device_type"); v != "" {
		cfg.DeviceType = v
	}
	if v := strArg(req, "default_source"); v != "" {
		cfg.DefaultSource = v
	}
	if v := strArg(req, "alexa_client_id"); v != "" {
		cfg.Alexa.ClientID = v
	}
	if v := strArg(req, "alexa_client_secret"); v != "" {
		cfg.Alexa.ClientSecret = v
	}
	if err := saveConfig(); err != nil {
		return fail("could not save config: " + err.Error()), nil
	}
	return ok("✅ Config updated and saved."), nil
}

func maskSecret(s string) string {
	if len(s) < 8 {
		return "***"
	}
	return s[:4] + "****"
}

// ── Arg helpers ───────────────────────────────────────────────────────────────

func deviceArg() mcp.ToolOption {
	return mcp.WithString("device_name", mcp.Description("Friendly name of the device (uses default_device from config if omitted)"))
}
func queryArg() mcp.ToolOption {
	return mcp.WithString("query", mcp.Required(), mcp.Description("Song, artist, or search query"))
}
func levelArg() mcp.ToolOption {
	return mcp.WithNumber("level", mcp.Required(), mcp.Description("Volume level 0–100"))
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	if err := loadConfig(); err != nil {
		log.Fatalf("Config error: %v", err)
	}

	if err := startLocalFileServer(); err != nil {
		log.Printf("Warning: local file server failed to start: %v", err)
	}

	s := server.NewMCPServer("smart-speaker-mcp", "3.0.0", server.WithToolCapabilities(true))

	// ── Unified play (recommended) ────────────────────────────────────────────
	s.AddTool(mcp.NewTool("smart_play",
		mcp.WithDescription(
			"Play music now or at a scheduled time. "+
				"Auto-selects device type (google_home/alexa/both) and source (local/youtube) from config. "+
				"All args are optional if defaults are set in config."),
		mcp.WithString("query", mcp.Description("Song name or artist to search")),
		mcp.WithString("url", mcp.Description("Direct HTTP/HTTPS URL to stream (optional)")),
		mcp.WithString("device_name", mcp.Description("Device to play on (default: config.default_device)")),
		mcp.WithString("device_type", mcp.Description("google_home | alexa | both (default: config.device_type)")),
		mcp.WithString("source", mcp.Description("local | youtube | url (default: config.default_source)")),
		mcp.WithNumber("start_time", mcp.Description("Start at this many seconds into the track")),
	), handleSmartPlay)

	// ── Google Home tools ─────────────────────────────────────────────────────
	s.AddTool(mcp.NewTool("gh_discover_devices",
		mcp.WithDescription("Discover all Google Home / Chromecast devices on the local Wi-Fi"),
	), handleGHDiscover)

	s.AddTool(mcp.NewTool("gh_play_music",
		mcp.WithDescription("Play music on a specific Google Home device. Source priority: local → direct URL → YouTube."),
		mcp.WithString("device_name", mcp.Required(), mcp.Description("Friendly name of the Google Home device")),
		mcp.WithString("query", mcp.Description("Song name or artist")),
		mcp.WithString("url", mcp.Description("Direct HTTP/HTTPS URL to stream")),
		mcp.WithNumber("start_time", mcp.Description("Start at N seconds into the track")),
	), handleGHPlay)

	s.AddTool(mcp.NewTool("gh_pause",
		mcp.WithDescription("Pause playback on a Google Home device"),
		mcp.WithString("device_name", mcp.Required(), mcp.Description("Device name")),
	), handleGHPause)

	s.AddTool(mcp.NewTool("gh_resume",
		mcp.WithDescription("Resume playback on a Google Home device"),
		mcp.WithString("device_name", mcp.Required(), mcp.Description("Device name")),
	), handleGHResume)

	s.AddTool(mcp.NewTool("gh_stop",
		mcp.WithDescription("Stop playback on a Google Home device"),
		mcp.WithString("device_name", mcp.Required(), mcp.Description("Device name")),
	), handleGHStop)

	s.AddTool(mcp.NewTool("gh_set_volume",
		mcp.WithDescription("Set volume (0–100) on a Google Home device"),
		mcp.WithString("device_name", mcp.Required(), mcp.Description("Device name")),
		levelArg(),
	), handleGHVolume)

	s.AddTool(mcp.NewTool("gh_get_status",
		mcp.WithDescription("Get playback status of a Google Home device"),
		mcp.WithString("device_name", mcp.Required(), mcp.Description("Device name")),
	), handleGHStatus)

	// ── Alexa tools ───────────────────────────────────────────────────────────
	s.AddTool(mcp.NewTool("alexa_auth",
		mcp.WithDescription("Get Amazon OAuth URL to connect your Alexa account"),
	), handleAlexaAuth)

	s.AddTool(mcp.NewTool("alexa_auth_complete",
		mcp.WithDescription("Complete Alexa auth with the OAuth code from the redirect URL"),
		mcp.WithString("code", mcp.Required(), mcp.Description("The 'code' param from the Amazon redirect")),
	), handleAlexaAuthComplete)

	s.AddTool(mcp.NewTool("alexa_discover_devices",
		mcp.WithDescription("List all Echo / Alexa devices on your Amazon account"),
	), handleAlexaDiscover)

	s.AddTool(mcp.NewTool("alexa_play_music",
		mcp.WithDescription("Play music on an Alexa / Echo device"),
		mcp.WithString("device_name", mcp.Required(), mcp.Description("Device name")),
		queryArg(),
	), handleAlexaPlay)

	s.AddTool(mcp.NewTool("alexa_pause",
		mcp.WithDescription("Pause playback on an Alexa device"),
		mcp.WithString("device_name", mcp.Required(), mcp.Description("Device name")),
	), handleAlexaPause)

	s.AddTool(mcp.NewTool("alexa_resume",
		mcp.WithDescription("Resume playback on an Alexa device"),
		mcp.WithString("device_name", mcp.Required(), mcp.Description("Device name")),
	), handleAlexaResume)

	s.AddTool(mcp.NewTool("alexa_set_volume",
		mcp.WithDescription("Set volume (0–100) on an Alexa device"),
		mcp.WithString("device_name", mcp.Required(), mcp.Description("Device name")),
		levelArg(),
	), handleAlexaVolume)

	// ── Diagnostic + Config tools ─────────────────────────────────────────────
	s.AddTool(mcp.NewTool("list_local_music",
		mcp.WithDescription("List audio files in the configured local music directory"),
	), handleListLocalMusic)

	s.AddTool(mcp.NewTool("get_config",
		mcp.WithDescription("Show current config: devices, source, paths, Alexa status, and available env var overrides"),
	), handleGetConfig)

	s.AddTool(mcp.NewTool("set_config",
		mcp.WithDescription("Update config settings. Changes are saved to ~/.config/smart-speaker-mcp/config.json"),
		mcp.WithString("music_dir", mcp.Description("Path to local music library")),
		mcp.WithString("default_device", mcp.Description("Default device name for smart_play")),
		mcp.WithString("device_type", mcp.Description("google_home | alexa | both")),
		mcp.WithString("default_source", mcp.Description("local | youtube | url")),
		mcp.WithString("ytdlp_path", mcp.Description("Full path to yt-dlp binary")),
		mcp.WithString("alexa_client_id", mcp.Description("Amazon Developer app Client ID")),
		mcp.WithString("alexa_client_secret", mcp.Description("Amazon Developer app Client Secret")),
	), handleSetConfig)

	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("MCP server error: %v", err)
	}
}
