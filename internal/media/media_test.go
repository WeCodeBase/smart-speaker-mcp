package media

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WeCodeBase/smart-speaker-mcp/config"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func saveCfg() config.Config     { return config.Cfg }
func restoreCfg(c config.Config) { config.Cfg = c }

func makeMusicDir(t *testing.T, files ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, f := range files {
		path := filepath.Join(dir, f)
		if err := os.WriteFile(path, []byte("fake audio data"), 0644); err != nil {
			t.Fatalf("create test file %s: %v", f, err)
		}
	}
	return dir
}

// ── audioExtensions ───────────────────────────────────────────────────────────

func TestAudioExtensions_KnownFormats(t *testing.T) {
	exts := []string{".mp3", ".m4a", ".flac", ".wav", ".ogg", ".aac", ".opus", ".wma"}
	for _, ext := range exts {
		if !audioExtensions[ext] {
			t.Errorf("audioExtensions[%q] should be true", ext)
		}
	}
}

func TestAudioExtensions_NonAudioFormats(t *testing.T) {
	nonAudio := []string{".txt", ".jpg", ".pdf", ".mp4", ".exe", ""}
	for _, ext := range nonAudio {
		if audioExtensions[ext] {
			t.Errorf("audioExtensions[%q] should be false", ext)
		}
	}
}

// ── ListLocal ─────────────────────────────────────────────────────────────────

func TestListLocal_ReturnsAudioFiles(t *testing.T) {
	saved := saveCfg()
	defer restoreCfg(saved)

	dir := makeMusicDir(t, "song1.mp3", "song2.flac", "notes.txt", "image.jpg", "track.m4a")
	config.Cfg.MusicDir = dir

	files, err := ListLocal(100)
	if err != nil {
		t.Fatalf("ListLocal() error: %v", err)
	}
	if len(files) != 3 {
		t.Errorf("got %d files, want 3 — files: %v", len(files), files)
	}
}

func TestListLocal_RespectsMaxFiles(t *testing.T) {
	saved := saveCfg()
	defer restoreCfg(saved)

	dir := makeMusicDir(t, "a.mp3", "b.mp3", "c.mp3", "d.mp3", "e.mp3")
	config.Cfg.MusicDir = dir

	files, err := ListLocal(2)
	if err != nil {
		t.Fatalf("ListLocal() error: %v", err)
	}
	if len(files) != 2 {
		t.Errorf("got %d files, want 2 (maxFiles limit)", len(files))
	}
}

func TestListLocal_NonExistentDir(t *testing.T) {
	saved := saveCfg()
	defer restoreCfg(saved)

	config.Cfg.MusicDir = "/this/dir/does/not/exist/xyz"
	if _, err := ListLocal(10); err == nil {
		t.Error("expected error for non-existent directory, got nil")
	}
}

func TestListLocal_EmptyDir(t *testing.T) {
	saved := saveCfg()
	defer restoreCfg(saved)

	config.Cfg.MusicDir = t.TempDir()

	files, err := ListLocal(10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 files, got %d", len(files))
	}
}

// ── findLocal ─────────────────────────────────────────────────────────────────

func TestFindLocal_ExactNameMatch(t *testing.T) {
	saved := saveCfg()
	defer restoreCfg(saved)

	dir := makeMusicDir(t, "ilaiyaraaja_hits.mp3", "other_song.mp3")
	config.Cfg.MusicDir = dir

	path, title, err := findLocal("ilaiyaraaja")
	if err != nil {
		t.Fatalf("findLocal() error: %v", err)
	}
	if !strings.Contains(strings.ToLower(filepath.Base(path)), "ilaiyaraaja") {
		t.Errorf("expected ilaiyaraaja file, got %q", path)
	}
	if title == "" {
		t.Error("title should not be empty")
	}
}

func TestFindLocal_CaseInsensitiveMatch(t *testing.T) {
	saved := saveCfg()
	defer restoreCfg(saved)

	dir := makeMusicDir(t, "ILAIYARAAJA_SONG.mp3")
	config.Cfg.MusicDir = dir

	path, _, err := findLocal("ilaiyaraaja")
	if err != nil {
		t.Fatalf("findLocal() error: %v", err)
	}
	if path == "" {
		t.Error("expected a file path, got empty string")
	}
}

func TestFindLocal_MultiWordQuery(t *testing.T) {
	saved := saveCfg()
	defer restoreCfg(saved)

	dir := makeMusicDir(t, "raja_theme_song.mp3", "other_theme.mp3", "raja_other.mp3")
	config.Cfg.MusicDir = dir

	path, _, err := findLocal("raja theme")
	if err != nil {
		t.Fatalf("findLocal() error: %v", err)
	}
	if !strings.Contains(strings.ToLower(filepath.Base(path)), "raja") ||
		!strings.Contains(strings.ToLower(filepath.Base(path)), "theme") {
		t.Errorf("multi-word match failed, got %q", path)
	}
}

func TestFindLocal_FallbackToFirstFile(t *testing.T) {
	saved := saveCfg()
	defer restoreCfg(saved)

	dir := makeMusicDir(t, "some_song.mp3", "another.flac")
	config.Cfg.MusicDir = dir

	path, _, err := findLocal("querythatmatchesnothing12345")
	if err != nil {
		t.Fatalf("expected fallback to first file, got error: %v", err)
	}
	if path == "" {
		t.Error("expected fallback file path, got empty")
	}
}

func TestFindLocal_NonExistentDir(t *testing.T) {
	saved := saveCfg()
	defer restoreCfg(saved)

	config.Cfg.MusicDir = "/nonexistent/path/xyz"
	if _, _, err := findLocal("anything"); err == nil {
		t.Error("expected error for non-existent dir, got nil")
	}
}

func TestFindLocal_EmptyDir_ReturnsError(t *testing.T) {
	saved := saveCfg()
	defer restoreCfg(saved)

	config.Cfg.MusicDir = t.TempDir()
	if _, _, err := findLocal("anything"); err == nil {
		t.Error("expected error for empty dir, got nil")
	}
}

func TestFindLocal_TitleStripsExtension(t *testing.T) {
	saved := saveCfg()
	defer restoreCfg(saved)

	dir := makeMusicDir(t, "my_song.mp3")
	config.Cfg.MusicDir = dir

	_, title, err := findLocal("my_song")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if strings.HasSuffix(title, ".mp3") {
		t.Errorf("title should not have extension, got %q", title)
	}
}

// ── Resolve ───────────────────────────────────────────────────────────────────

func TestResolve_UsesLocalFileFirst(t *testing.T) {
	saved := saveCfg()
	defer restoreCfg(saved)

	dir := makeMusicDir(t, "test_track.mp3")
	config.Cfg.MusicDir = dir
	SetStreamingServer("127.0.0.1", 8765)

	result, err := Resolve("test_track", "", 0)
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if result.Source != "local" {
		t.Errorf("Source = %q, want local", result.Source)
	}
	if !strings.HasPrefix(result.URL, "http://") {
		t.Errorf("URL should be http://, got %q", result.URL)
	}
}

func TestResolve_DirectURLUsedWhenNoLocalMatch(t *testing.T) {
	saved := saveCfg()
	defer restoreCfg(saved)

	config.Cfg.MusicDir = t.TempDir()

	result, err := Resolve("something", "https://example.com/audio.mp3", 0)
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if result.Source != "url" {
		t.Errorf("Source = %q, want url", result.Source)
	}
	if result.URL != "https://example.com/audio.mp3" {
		t.Errorf("URL = %q, want https://example.com/audio.mp3", result.URL)
	}
}

func TestResolve_StartTimePreserved(t *testing.T) {
	saved := saveCfg()
	defer restoreCfg(saved)

	config.Cfg.MusicDir = t.TempDir()

	result, err := Resolve("", "https://example.com/audio.mp3", 42)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result.StartTime != 42 {
		t.Errorf("StartTime = %d, want 42", result.StartTime)
	}
}

// ── youtubeAudioURL ───────────────────────────────────────────────────────────

func TestYoutubeAudioURL_MissingBinary(t *testing.T) {
	saved := saveCfg()
	defer restoreCfg(saved)

	config.Cfg.YtDlpPath = "/nonexistent/path/yt-dlp-fake"
	if _, _, err := youtubeAudioURL("test query"); err == nil {
		t.Error("expected error when yt-dlp binary doesn't exist, got nil")
	}
}
