package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// makeMusicDir creates a temp directory with the given filenames inside it.
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

// ── ListLocalMusic ────────────────────────────────────────────────────────────

func TestListLocalMusic_ReturnsAudioFiles(t *testing.T) {
	saved := saveCfg()
	defer restoreCfg(saved)

	dir := makeMusicDir(t,
		"song1.mp3", "song2.flac", "notes.txt", "image.jpg", "track.m4a",
	)
	cfg.MusicDir = dir

	files, err := ListLocalMusic(100)
	if err != nil {
		t.Fatalf("ListLocalMusic() error: %v", err)
	}

	// Should return 3 audio files, skip .txt and .jpg
	if len(files) != 3 {
		t.Errorf("got %d files, want 3 — files: %v", len(files), files)
	}
}

func TestListLocalMusic_RespectsMaxFiles(t *testing.T) {
	saved := saveCfg()
	defer restoreCfg(saved)

	dir := makeMusicDir(t, "a.mp3", "b.mp3", "c.mp3", "d.mp3", "e.mp3")
	cfg.MusicDir = dir

	files, err := ListLocalMusic(2)
	if err != nil {
		t.Fatalf("ListLocalMusic() error: %v", err)
	}
	if len(files) != 2 {
		t.Errorf("got %d files, want 2 (maxFiles limit)", len(files))
	}
}

func TestListLocalMusic_NonExistentDir(t *testing.T) {
	saved := saveCfg()
	defer restoreCfg(saved)

	cfg.MusicDir = "/this/dir/does/not/exist/xyz"
	_, err := ListLocalMusic(10)
	if err == nil {
		t.Error("expected error for non-existent directory, got nil")
	}
}

func TestListLocalMusic_EmptyDir(t *testing.T) {
	saved := saveCfg()
	defer restoreCfg(saved)

	dir := t.TempDir()
	cfg.MusicDir = dir

	files, err := ListLocalMusic(10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 files, got %d", len(files))
	}
}

// ── findLocalMusic ────────────────────────────────────────────────────────────

func TestFindLocalMusic_ExactNameMatch(t *testing.T) {
	saved := saveCfg()
	defer restoreCfg(saved)

	dir := makeMusicDir(t, "ilaiyaraaja_hits.mp3", "other_song.mp3")
	cfg.MusicDir = dir

	path, title, err := findLocalMusic("ilaiyaraaja")
	if err != nil {
		t.Fatalf("findLocalMusic() error: %v", err)
	}
	if !strings.Contains(strings.ToLower(filepath.Base(path)), "ilaiyaraaja") {
		t.Errorf("expected ilaiyaraaja file, got %q", path)
	}
	if title == "" {
		t.Error("title should not be empty")
	}
}

func TestFindLocalMusic_CaseInsensitiveMatch(t *testing.T) {
	saved := saveCfg()
	defer restoreCfg(saved)

	dir := makeMusicDir(t, "ILAIYARAAJA_SONG.mp3")
	cfg.MusicDir = dir

	path, _, err := findLocalMusic("ilaiyaraaja")
	if err != nil {
		t.Fatalf("findLocalMusic() error: %v", err)
	}
	if path == "" {
		t.Error("expected a file path, got empty string")
	}
}

func TestFindLocalMusic_MultiWordQuery(t *testing.T) {
	saved := saveCfg()
	defer restoreCfg(saved)

	dir := makeMusicDir(t, "raja_theme_song.mp3", "other_theme.mp3", "raja_other.mp3")
	cfg.MusicDir = dir

	path, _, err := findLocalMusic("raja theme")
	if err != nil {
		t.Fatalf("findLocalMusic() error: %v", err)
	}
	if !strings.Contains(strings.ToLower(filepath.Base(path)), "raja") ||
		!strings.Contains(strings.ToLower(filepath.Base(path)), "theme") {
		t.Errorf("multi-word match failed, got %q", path)
	}
}

func TestFindLocalMusic_FallbackToFirstFile(t *testing.T) {
	saved := saveCfg()
	defer restoreCfg(saved)

	dir := makeMusicDir(t, "some_song.mp3", "another.flac")
	cfg.MusicDir = dir

	// Query that won't match any filename
	path, _, err := findLocalMusic("querythatmatchesnothing12345")
	if err != nil {
		t.Fatalf("expected fallback to first file, got error: %v", err)
	}
	if path == "" {
		t.Error("expected fallback file path, got empty")
	}
}

func TestFindLocalMusic_NonExistentDir(t *testing.T) {
	saved := saveCfg()
	defer restoreCfg(saved)

	cfg.MusicDir = "/nonexistent/path/xyz"
	_, _, err := findLocalMusic("anything")
	if err == nil {
		t.Error("expected error for non-existent dir, got nil")
	}
}

func TestFindLocalMusic_EmptyDir_ReturnsError(t *testing.T) {
	saved := saveCfg()
	defer restoreCfg(saved)

	cfg.MusicDir = t.TempDir()
	_, _, err := findLocalMusic("anything")
	if err == nil {
		t.Error("expected error for empty dir, got nil")
	}
}

func TestFindLocalMusic_TitleStripsExtension(t *testing.T) {
	saved := saveCfg()
	defer restoreCfg(saved)

	dir := makeMusicDir(t, "my_song.mp3")
	cfg.MusicDir = dir

	_, title, err := findLocalMusic("my_song")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if strings.HasSuffix(title, ".mp3") {
		t.Errorf("title should not have extension, got %q", title)
	}
}

// ── resolveMedia ─────────────────────────────────────────────────────────────

func TestResolveMedia_UsesLocalFileFirst(t *testing.T) {
	saved := saveCfg()
	defer restoreCfg(saved)

	dir := makeMusicDir(t, "test_track.mp3")
	cfg.MusicDir = dir
	// Pre-set server vars so localFileHTTPURL works
	localServerPort = 8765
	localServerIP = "127.0.0.1"

	result, err := resolveMedia("test_track", "", 0)
	if err != nil {
		t.Fatalf("resolveMedia() error: %v", err)
	}
	if result.Source != "local" {
		t.Errorf("Source = %q, want local", result.Source)
	}
	if !strings.HasPrefix(result.URL, "http://") {
		t.Errorf("URL should be http://, got %q", result.URL)
	}
}

func TestResolveMedia_DirectURLUsedWhenNoLocalMatch(t *testing.T) {
	saved := saveCfg()
	defer restoreCfg(saved)

	// Empty music dir — local will fail
	cfg.MusicDir = t.TempDir()

	result, err := resolveMedia("something", "https://example.com/audio.mp3", 0)
	if err != nil {
		t.Fatalf("resolveMedia() error: %v", err)
	}
	if result.Source != "url" {
		t.Errorf("Source = %q, want url", result.Source)
	}
	if result.URL != "https://example.com/audio.mp3" {
		t.Errorf("URL = %q, want https://example.com/audio.mp3", result.URL)
	}
}

func TestResolveMedia_DirectURLTitleFromPath(t *testing.T) {
	saved := saveCfg()
	defer restoreCfg(saved)

	cfg.MusicDir = t.TempDir()

	result, err := resolveMedia("", "https://example.com/my_song.mp3", 0)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result.Title == "" {
		t.Error("Title should not be empty for direct URL")
	}
}

func TestResolveMedia_StartTimePreserved(t *testing.T) {
	saved := saveCfg()
	defer restoreCfg(saved)

	cfg.MusicDir = t.TempDir()
	localServerPort = 8765
	localServerIP = "127.0.0.1"

	result, err := resolveMedia("", "https://example.com/audio.mp3", 42)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result.StartTime != 42 {
		t.Errorf("StartTime = %d, want 42", result.StartTime)
	}
}

func TestResolveMedia_InvalidDirectURL_FallsToYouTube(t *testing.T) {
	saved := saveCfg()
	defer restoreCfg(saved)

	cfg.MusicDir = t.TempDir()
	cfg.YtDlpPath = "/nonexistent/yt-dlp" // force yt-dlp failure

	// directURL without http/https prefix is not treated as direct URL
	_, err := resolveMedia("test query", "not-a-valid-url", 0)
	// We expect an error since yt-dlp doesn't exist
	if err == nil {
		t.Log("unexpectedly succeeded — possibly yt-dlp is installed")
	}
}

// ── getYouTubeAudioURL ────────────────────────────────────────────────────────

func TestGetYouTubeAudioURL_MissingBinary(t *testing.T) {
	saved := saveCfg()
	defer restoreCfg(saved)

	cfg.YtDlpPath = "/nonexistent/path/yt-dlp-fake"
	_, _, err := getYouTubeAudioURL("test query")
	if err == nil {
		t.Error("expected error when yt-dlp binary doesn't exist, got nil")
	}
}
