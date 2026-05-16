package main

// config.go — loads, saves, and applies env-var overrides for the MCP config.
//
// Config file location:
//   macOS / Linux: ~/.config/smart-speaker-mcp/config.json
//   Windows:       %APPDATA%\smart-speaker-mcp\config.json
//
// Every field can also be overridden by an environment variable, useful for
// running the MCP in Docker or CI without editing the JSON file.
//
// Environment variable map:
//   SMART_SPEAKER_MUSIC_DIR       → music_dir
//   SMART_SPEAKER_DEFAULT_DEVICE  → default_device
//   SMART_SPEAKER_SOURCE          → default_source (local | youtube | url)
//   SMART_SPEAKER_YTDLP_PATH      → ytdlp_path

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config is the full runtime settings shape persisted to config.json.
//
// Defaults are populated by loadConfig() if a field is blank, then
// applyEnvOverrides() lets environment variables win.
type Config struct {
	// Path to the yt-dlp binary used for YouTube playback. Auto-detected
	// from common install locations if blank.
	YtDlpPath string `json:"ytdlp_path"`

	// Path to the local music library searched by source=local.
	MusicDir string `json:"music_dir"`

	// Default speaker name used when a tool call doesn't specify one.
	// Must match the name shown by `discover_devices`.
	DefaultDevice string `json:"default_device"`

	// Default audio source when not specified: "local" | "youtube" | "url".
	DefaultSource string `json:"default_source"`
}

var cfg Config

var (
	configDir  = filepath.Join(os.Getenv("HOME"), ".config", "smart-speaker-mcp")
	configFile = filepath.Join(configDir, "config.json")
	dotEnvFile = filepath.Join(configDir, ".env")
)

// ── Load / Save ───────────────────────────────────────────────────────────────

// loadConfig reads .env and config.json, fills in defaults, and applies
// environment overrides. Called once at startup from main().
func loadConfig() error {
	loadDotEnv(dotEnvFile) // populates os.Getenv before applyEnvOverrides

	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		cfg = defaultConfig()
		if err := saveConfig(); err != nil {
			return err
		}
	} else {
		data, err := os.ReadFile(configFile)
		if err != nil {
			return fmt.Errorf("cannot read config: %w", err)
		}
		if err := json.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("invalid config JSON: %w", err)
		}
	}

	// Fill blanks with defaults
	if cfg.YtDlpPath == "" {
		cfg.YtDlpPath = detectYtDlp()
	}
	if cfg.MusicDir == "" {
		cfg.MusicDir = filepath.Join(os.Getenv("HOME"), "Music")
	}
	if cfg.DefaultSource == "" {
		cfg.DefaultSource = "local"
	}

	applyEnvOverrides() // env always wins
	return nil
}

func saveConfig() error {
	if err := os.MkdirAll(filepath.Dir(configFile), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configFile, data, 0600)
}

func defaultConfig() Config {
	return Config{
		YtDlpPath:     detectYtDlp(),
		MusicDir:      filepath.Join(os.Getenv("HOME"), "Music"),
		DefaultDevice: "",
		DefaultSource: "local",
	}
}

// applyEnvOverrides reads well-known env vars and overlays them on top of
// whatever was loaded from the JSON file. Empty env vars are ignored.
func applyEnvOverrides() {
	if v := os.Getenv("SMART_SPEAKER_MUSIC_DIR"); v != "" {
		cfg.MusicDir = v
	}
	if v := os.Getenv("SMART_SPEAKER_DEFAULT_DEVICE"); v != "" {
		cfg.DefaultDevice = v
	}
	if v := os.Getenv("SMART_SPEAKER_SOURCE"); v != "" {
		cfg.DefaultSource = v
	}
	if v := os.Getenv("SMART_SPEAKER_YTDLP_PATH"); v != "" {
		cfg.YtDlpPath = v
	}
}

// loadDotEnv reads a .env file and sets each KEY=VALUE pair as an OS
// environment variable, but only when the variable isn't already set by
// the shell (so explicit shell vars always win).
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return // file doesn't exist — that's fine
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.IndexByte(line, '=')
		if idx < 1 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		// Strip optional surrounding quotes
		if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') ||
			(val[0] == '\'' && val[len(val)-1] == '\'')) {
			val = val[1 : len(val)-1]
		}
		if os.Getenv(key) == "" {
			_ = os.Setenv(key, val)
		}
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// detectYtDlp tries common install locations across macOS / Linux / Windows.
func detectYtDlp() string {
	candidates := []string{
		"/usr/local/bin/yt-dlp",
		"/opt/homebrew/bin/yt-dlp",
		"/usr/bin/yt-dlp",
		"/home/linuxbrew/.linuxbrew/bin/yt-dlp",
		`C:\Program Files\yt-dlp\yt-dlp.exe`,
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "yt-dlp" // fall back to PATH
}

// expandHome replaces a leading ~ with the real home directory.
func expandHome(path string) string {
	if len(path) > 0 && path[0] == '~' {
		return filepath.Join(os.Getenv("HOME"), path[1:])
	}
	return path
}
