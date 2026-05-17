package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/WeCodeBase/smart-speaker-mcp/config"
	"github.com/WeCodeBase/smart-speaker-mcp/internal/chromecast"
	"github.com/WeCodeBase/smart-speaker-mcp/internal/media"
)

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

// resolveDevice returns the device name from the request or falls back to
// config.Cfg.DefaultDevice. Returns a clear error if neither is set.
func resolveDevice(req mcp.CallToolRequest) (string, error) {
	name := strArg(req, "device_name")
	if name == "" {
		name = config.Cfg.DefaultDevice
	}
	if name == "" {
		return "", fmt.Errorf("device_name is required (or set default_device in config)")
	}
	return name, nil
}

// ── Tool handlers ─────────────────────────────────────────────────────────────

func handleDiscoverDevices(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	devices, err := chromecast.DiscoverDevices(8)
	if err != nil {
		return fail(err.Error()), nil
	}
	if len(devices) == 0 {
		return ok("No Chromecast / Google Home devices found on this Wi-Fi."), nil
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
		source = config.Cfg.DefaultSource
	}
	startTime := int(floatArg(req, "start_time"))

	effectiveQuery := query
	if effectiveQuery == "" {
		effectiveQuery = directURL
	}

	// Force YouTube → blank music dir so Resolve skips local lookup.
	if source == "youtube" {
		saved := config.Cfg.MusicDir
		config.Cfg.MusicDir = ""
		defer func() { config.Cfg.MusicDir = saved }()
	}

	m, err := media.Resolve(effectiveQuery, directURL, startTime)
	if err != nil {
		return fail("could not resolve media: " + err.Error()), nil
	}
	msg, err := chromecast.PlayMedia(deviceName, m.URL, m.Title, m.Source, m.StartTime)
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
	if err := chromecast.Pause(device); err != nil {
		return fail(err.Error()), nil
	}
	return ok("⏸  Paused on " + device), nil
}

func handleResume(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	device, err := resolveDevice(req)
	if err != nil {
		return fail(err.Error()), nil
	}
	if err := chromecast.Resume(device); err != nil {
		return fail(err.Error()), nil
	}
	return ok("▶️  Resumed on " + device), nil
}

func handleStop(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	device, err := resolveDevice(req)
	if err != nil {
		return fail(err.Error()), nil
	}
	if err := chromecast.Stop(device); err != nil {
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
	if err := chromecast.SetVolume(device, level); err != nil {
		return fail(err.Error()), nil
	}
	return ok(fmt.Sprintf("🔊 Volume set to %.0f%% on %s", level, device)), nil
}

func handleGetStatus(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	device, err := resolveDevice(req)
	if err != nil {
		return fail(err.Error()), nil
	}
	status, err := chromecast.GetStatus(device)
	if err != nil {
		return fail(err.Error()), nil
	}
	return ok(toJSON(status)), nil
}

func handleListLocalMusic(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	files, err := media.ListLocal(50)
	if err != nil {
		return fail(err.Error()), nil
	}
	if len(files) == 0 {
		return ok(fmt.Sprintf("No audio files found in music_dir: %s", config.Cfg.MusicDir)), nil
	}
	return ok(fmt.Sprintf("🎵 %d audio file(s) in %s:\n%s",
		len(files), config.Cfg.MusicDir, strings.Join(files, "\n"))), nil
}

func handleGetConfig(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	display := map[string]any{
		"version":        Version,
		"default_device": config.Cfg.DefaultDevice,
		"default_source": config.Cfg.DefaultSource,
		"music_dir":      config.Cfg.MusicDir,
		"ytdlp_path":     config.Cfg.YtDlpPath,
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
		config.Cfg.DefaultDevice = v
	}
	if v := strArg(req, "default_source"); v != "" {
		config.Cfg.DefaultSource = v
	}
	if v := strArg(req, "music_dir"); v != "" {
		config.Cfg.MusicDir = v
	}
	if v := strArg(req, "ytdlp_path"); v != "" {
		config.Cfg.YtDlpPath = v
	}
	if err := config.Save(); err != nil {
		return fail("could not save config: " + err.Error()), nil
	}
	return ok("✅ Config updated and saved."), nil
}
