// Package mcpserver wires the MCP tool handlers to the chromecast / media /
// webserver packages, and exposes a single Run() entrypoint called from main.
//
// Boot order:
//
//   1. config.Load()         — read .env + config.json
//   2. webserver.Start()     — bind HTTP server, get LAN IP + port
//   3. media.SetStreamingServer(ip, port) — so local-file URLs resolve correctly
//   4. registerWebUI(mux)    — mount the browser UI on the same HTTP server
//   5. server.NewMCPServer + AddTool x10 — register all tools
//   6. server.ServeStdio(...) — block on stdin, talk MCP to Claude
package mcpserver

import (
	"log"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/WeCodeBase/smart-speaker-mcp/config"
	"github.com/WeCodeBase/smart-speaker-mcp/internal/media"
	"github.com/WeCodeBase/smart-speaker-mcp/internal/webserver"
)

// Version is set at build time via -ldflags="-X mcpserver.Version=...".
var Version = "4.0.0"

// Run is the single entrypoint for the binary. main() should be one line:
//
//   func main() { mcpserver.Run() }
func Run() {
	if err := config.Load(); err != nil {
		log.Fatalf("Config error: %v", err)
	}

	web, err := webserver.Start()
	if err != nil {
		log.Printf("Warning: webserver failed to start: %v", err)
	} else {
		media.SetStreamingServer(web.IP, web.Port)
		registerWebUI(web.Mux)
	}

	s := server.NewMCPServer("smart-speaker-mcp", Version, server.WithToolCapabilities(true))
	registerTools(s)

	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("MCP server error: %v", err)
	}
}

// registerTools registers all MCP tools with the server. Defined here so
// the wiring is visible in one place; handler implementations live in
// handlers.go.
func registerTools(s *server.MCPServer) {
	deviceArg := mcp.WithString("device_name",
		mcp.Description("Speaker name (uses default_device from config if omitted)"))

	s.AddTool(mcp.NewTool("discover_devices",
		mcp.WithDescription("Scan the local Wi-Fi via mDNS for Chromecast / Google Home / Nest devices. "+
			"Returns name, host, port, and UUID for each device found."),
	), handleDiscoverDevices)

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
}
