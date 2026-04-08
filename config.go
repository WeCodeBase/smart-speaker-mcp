package main

// config.go — loads, saves, and applies env-var overrides for the MCP config.
//
// Config file location: ~/.config/smart-speaker-mcp/config.json
//
// Every field can be overridden by an environment variable — useful for
// running the MCP in Docker or CI without editing the JSON file.
//
// Environment variable map:
//   SMART_SPEAKER_MUSIC_DIR       → music_dir
//   SMART_SPEAKER_DEFAULT_DEVICE  → default_device
//   SMART_SPEAKER_DEVICE_TYPE     → device_type  (google_home | alexa | both)
//   SMART_SPEAKER_SOURCE          → default_source (local | youtube | url)
//   SMART_SPEAKER_YTDLP_PATH      → ytdlp_path
//   ALEXA_CLIENT_ID               → alexa.client_id
//   ALEXA_CLIENT_SECRET           → alexa.client_secret
//   ALEXA_ACCESS_TOKEN            → alexa.access_token
//   ALEXA_REFRESH_TOKEN           → alexa.refresh_token
//   ALEXA_CUSTOMER_ID             → alexa.customer_id

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ── Config structs ────────────────────────────────────────────────────────────

type AlexaConfig struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	CustomerID   string `json:"customer_id"`
}

type Config struct {
	// Paths
	YtDlpPath string `json:"ytdlp_path"`
	MusicDir  string `json:"music_dir"`

	// Playback defaults (used by smart_play when not specified in the call)
	DefaultDevice string `json:"default_device"` // e.g. "Family Room speaker"
	DeviceType    string `json:"device_type"`    // "google_home" | "alexa" | "both"
	DefaultSource string `json:"default_source"` // "local" | "youtube" | "url"

	// Alexa OAuth credentials
	Alexa AlexaConfig `json:"alexa"`
}

var cfg Config

var (
	configDir  = filepath.Join(os.Getenv("HOME"), ".config", "smart-speaker-mcp")
	configFile = filepath.Join(os.Getenv("HOME"), ".config", "smart-speaker-mcp", "config.json")
	dotEnvFile = filepath.Join(os.Getenv("HOME"), ".config", "smart-speaker-mcp", ".env")
)

// ── Load / Save ───────────────────────────────────────────────────────────────

func loadConfig() error {
	// 1. Load .env file first — sets os env vars so applyEnvOverrides() picks them up
	loadDotEnv(dotEnvFile)

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

	// Fill blank fields with defaults
	if cfg.YtDlpPath == "" {
		cfg.YtDlpPath = detectYtDlp()
	}
	if cfg.MusicDir == "" {
		cfg.MusicDir = filepath.Join(os.Getenv("HOME"), "sundar", "songs")
	}
	if cfg.DeviceType == "" {
		cfg.DeviceType = "google_home"
	}
	if cfg.DefaultSource == "" {
		cfg.DefaultSource = "local"
	}

	// Environment variables always win over the JSON file
	applyEnvOverrides()
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
		MusicDir:      filepath.Join(os.Getenv("HOME"), "sundar", "songs"),
		DefaultDevice: "",
		DeviceType:    "google_home",
		DefaultSource: "local",
	}
}

// applyEnvOverrides reads well-known environment variables and overlays them
// on top of whatever was loaded from the JSON config file.
func applyEnvOverrides() {
	if v := os.Getenv("SMART_SPEAKER_MUSIC_DIR"); v != "" {
		cfg.MusicDir = v
	}
	if v := os.Getenv("SMART_SPEAKER_DEFAULT_DEVICE"); v != "" {
		cfg.DefaultDevice = v
	}
	if v := os.Getenv("SMART_SPEAKER_DEVICE_TYPE"); v != "" {
		cfg.DeviceType = v
	}
	if v := os.Getenv("SMART_SPEAKER_SOURCE"); v != "" {
		cfg.DefaultSource = v
	}
	if v := os.Getenv("SMART_SPEAKER_YTDLP_PATH"); v != "" {
		cfg.YtDlpPath = v
	}
	// Alexa credentials via env (useful for keeping secrets out of the JSON file)
	if v := os.Getenv("ALEXA_CLIENT_ID"); v != "" {
		cfg.Alexa.ClientID = v
	}
	if v := os.Getenv("ALEXA_CLIENT_SECRET"); v != "" {
		cfg.Alexa.ClientSecret = v
	}
	if v := os.Getenv("ALEXA_ACCESS_TOKEN"); v != "" {
		cfg.Alexa.AccessToken = v
	}
	if v := os.Getenv("ALEXA_REFRESH_TOKEN"); v != "" {
		cfg.Alexa.RefreshToken = v
	}
	if v := os.Getenv("ALEXA_CUSTOMER_ID"); v != "" {
		cfg.Alexa.CustomerID = v
	}
}

// loadDotEnv reads a .env file and sets each KEY=VALUE pair as an OS
// environment variable (only if the variable is not already set by the shell).
// Lines starting with # are comments; blank lines are ignored.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return // file doesn't exist yet — that's fine
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
		// Only set if not already in the environment
		if os.Getenv(key) == "" {
			_ = os.Setenv(key, val)
		}
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// detectYtDlp tries common install locations across macOS / Linux.
func detectYtDlp() string {
	candidates := []string{
		"/usr/local/bin/yt-dlp",
		"/opt/homebrew/bin/yt-dlp",
		"/usr/bin/yt-dlp",
		"/home/linuxbrew/.linuxbrew/bin/yt-dlp",
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
