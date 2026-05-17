package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// ── test helpers ──────────────────────────────────────────────────────────────

func saveCfg() Config     { return Cfg }
func restoreCfg(c Config) { Cfg = c }

func clearEnv(keys ...string) func() {
	saved := make(map[string]string, len(keys))
	for _, k := range keys {
		saved[k] = os.Getenv(k)
		os.Unsetenv(k)
	}
	return func() {
		for k, v := range saved {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	}
}

// ── loadDotEnv ────────────────────────────────────────────────────────────────

func TestLoadDotEnv_BasicKeyValue(t *testing.T) {
	restore := clearEnv("TEST_KEY_A", "TEST_KEY_B")
	defer restore()

	tmp := t.TempDir()
	path := filepath.Join(tmp, ".env")
	os.WriteFile(path, []byte("TEST_KEY_A=hello\nTEST_KEY_B=world\n"), 0644)

	loadDotEnv(path)

	if got := os.Getenv("TEST_KEY_A"); got != "hello" {
		t.Errorf("TEST_KEY_A = %q, want hello", got)
	}
	if got := os.Getenv("TEST_KEY_B"); got != "world" {
		t.Errorf("TEST_KEY_B = %q, want world", got)
	}
}

func TestLoadDotEnv_DoubleQuotedValue(t *testing.T) {
	restore := clearEnv("TEST_QUOTED")
	defer restore()

	tmp := t.TempDir()
	path := filepath.Join(tmp, ".env")
	os.WriteFile(path, []byte(`TEST_QUOTED="hello world"`+"\n"), 0644)
	loadDotEnv(path)

	if got := os.Getenv("TEST_QUOTED"); got != "hello world" {
		t.Errorf("TEST_QUOTED = %q, want \"hello world\"", got)
	}
}

func TestLoadDotEnv_SingleQuotedValue(t *testing.T) {
	restore := clearEnv("TEST_SINGLE")
	defer restore()

	tmp := t.TempDir()
	path := filepath.Join(tmp, ".env")
	os.WriteFile(path, []byte("TEST_SINGLE='single quoted'\n"), 0644)
	loadDotEnv(path)

	if got := os.Getenv("TEST_SINGLE"); got != "single quoted" {
		t.Errorf("TEST_SINGLE = %q, want 'single quoted'", got)
	}
}

func TestLoadDotEnv_SkipsExistingEnvVar(t *testing.T) {
	os.Setenv("TEST_EXISTING", "original")
	defer os.Unsetenv("TEST_EXISTING")

	tmp := t.TempDir()
	path := filepath.Join(tmp, ".env")
	os.WriteFile(path, []byte("TEST_EXISTING=overwritten\n"), 0644)
	loadDotEnv(path)

	if got := os.Getenv("TEST_EXISTING"); got != "original" {
		t.Errorf("existing var was overwritten: got %q, want original", got)
	}
}

func TestLoadDotEnv_IgnoresCommentsAndBlankLines(t *testing.T) {
	restore := clearEnv("TEST_REAL")
	defer restore()

	tmp := t.TempDir()
	path := filepath.Join(tmp, ".env")
	content := "\n# this is a comment\n\nTEST_REAL=yes\n\n# another comment\n"
	os.WriteFile(path, []byte(content), 0644)
	loadDotEnv(path)

	if got := os.Getenv("TEST_REAL"); got != "yes" {
		t.Errorf("TEST_REAL = %q, want yes", got)
	}
}

func TestLoadDotEnv_NonExistentFile_NoPanic(t *testing.T) {
	loadDotEnv("/tmp/nonexistent-smart-speaker-test.env")
}

func TestLoadDotEnv_InvalidLines_AreSkipped(t *testing.T) {
	restore := clearEnv("TEST_VALID_LINE")
	defer restore()

	tmp := t.TempDir()
	path := filepath.Join(tmp, ".env")
	content := "NOEQUALSSIGN\n=NOKEY\nTEST_VALID_LINE=yes\n"
	os.WriteFile(path, []byte(content), 0644)
	loadDotEnv(path)

	if got := os.Getenv("TEST_VALID_LINE"); got != "yes" {
		t.Errorf("TEST_VALID_LINE = %q, want yes", got)
	}
}

// ── applyEnvOverrides ─────────────────────────────────────────────────────────

func TestApplyEnvOverrides_SmartSpeakerVars(t *testing.T) {
	saved := saveCfg()
	defer restoreCfg(saved)
	restore := clearEnv(
		"SMART_SPEAKER_MUSIC_DIR", "SMART_SPEAKER_DEFAULT_DEVICE",
		"SMART_SPEAKER_SOURCE", "SMART_SPEAKER_YTDLP_PATH",
	)
	defer restore()

	os.Setenv("SMART_SPEAKER_MUSIC_DIR", "/tmp/music")
	os.Setenv("SMART_SPEAKER_DEFAULT_DEVICE", "Living Room")
	os.Setenv("SMART_SPEAKER_SOURCE", "youtube")
	os.Setenv("SMART_SPEAKER_YTDLP_PATH", "/usr/bin/yt-dlp")

	Cfg = Config{}
	applyEnvOverrides()

	tests := []struct{ got, want, field string }{
		{Cfg.MusicDir, "/tmp/music", "MusicDir"},
		{Cfg.DefaultDevice, "Living Room", "DefaultDevice"},
		{Cfg.DefaultSource, "youtube", "DefaultSource"},
		{Cfg.YtDlpPath, "/usr/bin/yt-dlp", "YtDlpPath"},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s = %q, want %q", tt.field, tt.got, tt.want)
		}
	}
}

func TestApplyEnvOverrides_EmptyVarsDoNotOverwrite(t *testing.T) {
	saved := saveCfg()
	defer restoreCfg(saved)
	restore := clearEnv("SMART_SPEAKER_MUSIC_DIR")
	defer restore()

	Cfg = Config{MusicDir: "/original/path"}
	applyEnvOverrides()

	if Cfg.MusicDir != "/original/path" {
		t.Errorf("MusicDir was unexpectedly overwritten: got %q", Cfg.MusicDir)
	}
}

// ── ExpandHome ────────────────────────────────────────────────────────────────

func TestExpandHome_TildeReplaced(t *testing.T) {
	home := os.Getenv("HOME")
	got := ExpandHome("~/Music")
	want := filepath.Join(home, "Music")
	if got != want {
		t.Errorf("ExpandHome(~/Music) = %q, want %q", got, want)
	}
}

func TestExpandHome_AbsolutePathUnchanged(t *testing.T) {
	got := ExpandHome("/absolute/path")
	if got != "/absolute/path" {
		t.Errorf("ExpandHome(/absolute/path) = %q, want /absolute/path", got)
	}
}

func TestExpandHome_EmptyStringUnchanged(t *testing.T) {
	got := ExpandHome("")
	if got != "" {
		t.Errorf("ExpandHome('') = %q, want empty", got)
	}
}

// ── detectYtDlp ───────────────────────────────────────────────────────────────

func TestDetectYtDlp_ReturnsNonEmptyString(t *testing.T) {
	if result := detectYtDlp(); result == "" {
		t.Error("detectYtDlp() returned empty string")
	}
}

// ── defaultConfig ─────────────────────────────────────────────────────────────

func TestDefaultConfig_HasExpectedDefaults(t *testing.T) {
	c := defaultConfig()
	if c.DefaultSource != "local" {
		t.Errorf("DefaultSource = %q, want local", c.DefaultSource)
	}
	if c.MusicDir == "" {
		t.Error("MusicDir is empty in default config")
	}
}

// ── Save / load round-trip ───────────────────────────────────────────────────

func TestSave_RoundTrip(t *testing.T) {
	saved := saveCfg()
	defer restoreCfg(saved)

	tmp := t.TempDir()
	restore := SetPathsForTest(tmp, filepath.Join(tmp, "config.json"))
	defer restore()

	Cfg = Config{
		YtDlpPath:     "/test/yt-dlp",
		MusicDir:      "/test/music",
		DefaultDevice: "Test Speaker",
		DefaultSource: "youtube",
	}

	if err := Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("could not read saved config: %v", err)
	}

	var loaded Config
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("could not parse saved config: %v", err)
	}

	if loaded.DefaultDevice != "Test Speaker" {
		t.Errorf("round-trip DefaultDevice = %q, want Test Speaker", loaded.DefaultDevice)
	}
	if loaded.DefaultSource != "youtube" {
		t.Errorf("round-trip DefaultSource = %q, want youtube", loaded.DefaultSource)
	}
	if loaded.MusicDir != "/test/music" {
		t.Errorf("round-trip MusicDir = %q, want /test/music", loaded.MusicDir)
	}
}
