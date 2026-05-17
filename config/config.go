// Package config loads, persists, and exposes the runtime configuration for
// smart-speaker-mcp. A single Cfg value acts as the source of truth across
// all other internal packages — load it once at startup with Load().
package config

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config is the persisted configuration shape. Defaults populated by Load()
// when fields are blank; environment variables override the JSON file.
type Config struct {
	YtDlpPath     string `json:"ytdlp_path"`
	MusicDir      string `json:"music_dir"`
	DefaultDevice string `json:"default_device"`
	DefaultSource string `json:"default_source"`
}

// Cfg is the process-wide config singleton, populated by Load().
var Cfg Config

var (
	dir  = filepath.Join(os.Getenv("HOME"), ".config", "smart-speaker-mcp")
	file = filepath.Join(dir, "config.json")
	env  = filepath.Join(dir, ".env")
)

// Load reads .env and config.json, fills in defaults, and applies env overrides.
// Call this once at startup before any package accesses Cfg.
func Load() error {
	loadDotEnv(env)

	if _, err := os.Stat(file); os.IsNotExist(err) {
		Cfg = defaultConfig()
		if err := Save(); err != nil {
			return err
		}
	} else {
		data, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("cannot read config: %w", err)
		}
		if err := json.Unmarshal(data, &Cfg); err != nil {
			return fmt.Errorf("invalid config JSON: %w", err)
		}
	}

	if Cfg.YtDlpPath == "" {
		Cfg.YtDlpPath = detectYtDlp()
	}
	if Cfg.MusicDir == "" {
		Cfg.MusicDir = filepath.Join(os.Getenv("HOME"), "Music")
	}
	if Cfg.DefaultSource == "" {
		Cfg.DefaultSource = "local"
	}

	applyEnvOverrides()
	return nil
}

// Save writes the current Cfg back to disk as JSON.
func Save() error {
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(Cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(file, data, 0600)
}

// ── Internal helpers ──────────────────────────────────────────────────────────

func defaultConfig() Config {
	return Config{
		YtDlpPath:     detectYtDlp(),
		MusicDir:      filepath.Join(os.Getenv("HOME"), "Music"),
		DefaultDevice: "",
		DefaultSource: "local",
	}
}

func applyEnvOverrides() {
	if v := os.Getenv("SMART_SPEAKER_MUSIC_DIR"); v != "" {
		Cfg.MusicDir = v
	}
	if v := os.Getenv("SMART_SPEAKER_DEFAULT_DEVICE"); v != "" {
		Cfg.DefaultDevice = v
	}
	if v := os.Getenv("SMART_SPEAKER_SOURCE"); v != "" {
		Cfg.DefaultSource = v
	}
	if v := os.Getenv("SMART_SPEAKER_YTDLP_PATH"); v != "" {
		Cfg.YtDlpPath = v
	}
}

func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
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
		if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') ||
			(val[0] == '\'' && val[len(val)-1] == '\'')) {
			val = val[1 : len(val)-1]
		}
		if os.Getenv(key) == "" {
			_ = os.Setenv(key, val)
		}
	}
}

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
	return "yt-dlp"
}

// ExpandHome replaces a leading ~ with the user's home directory.
// Exported because media and webserver use it for path normalization.
func ExpandHome(path string) string {
	if len(path) > 0 && path[0] == '~' {
		return filepath.Join(os.Getenv("HOME"), path[1:])
	}
	return path
}

// ── Test hooks (used by config_test.go in this package) ──────────────────────

// SetPathsForTest overrides the config file and dir paths. Tests only.
func SetPathsForTest(d, f string) (restore func()) {
	od, of := dir, file
	dir, file = d, f
	return func() { dir, file = od, of }
}
