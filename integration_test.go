//go:build integration
// +build integration

// Integration tests require real hardware / network / binaries.
// Run with: go test -tags=integration -v ./...
//
// Prerequisites:
//   - A Google Home / Chromecast on the same Wi-Fi (for GH tests)
//   - yt-dlp installed (for YouTube tests)
//   - SMART_SPEAKER_DEFAULT_DEVICE set to your device name
//
// These tests are EXCLUDED from normal `go test ./...` runs.

package main

import (
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

// ── HTTP server: serve and fetch a real file ──────────────────────────────────

func TestIntegration_LocalFileServer_ServeAndFetch(t *testing.T) {
	testContent := "FAKE_MP3_CONTENT_FOR_INTEGRATION_TEST"
	testFile := t.TempDir() + "/integration_test.mp3"
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatal(err)
	}

	localServerPort = 0
	if err := startLocalFileServer(); err != nil {
		t.Fatalf("startLocalFileServer() error: %v", err)
	}
	localServerIP = "127.0.0.1"
	time.Sleep(50 * time.Millisecond) // let goroutine start

	url := localFileHTTPURL(testFile)
	t.Logf("Fetching: %s", url)

	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("HTTP GET error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != testContent {
		t.Errorf("body = %q, want %q", string(body), testContent)
	}
}

// ── YouTube resolution (requires yt-dlp installed) ───────────────────────────

func TestIntegration_YouTubeAudioURL(t *testing.T) {
	saved := saveCfg()
	defer restoreCfg(saved)

	cfg.YtDlpPath = detectYtDlp()
	if cfg.YtDlpPath == "yt-dlp" {
		t.Skip("yt-dlp not found at known paths — skipping YouTube integration test")
	}

	streamURL, title, err := getYouTubeAudioURL("Ilaiyaraaja Roja theme")
	if err != nil {
		t.Fatalf("getYouTubeAudioURL() error: %v", err)
	}
	if streamURL == "" {
		t.Error("stream URL is empty")
	}
	if title == "" {
		t.Error("title is empty")
	}
	preview := streamURL
	if len(preview) > 80 {
		preview = preview[:80] + "..."
	}
	t.Logf("Title: %s", title)
	t.Logf("URL:   %s", preview)
}

// ── Google Home: device discovery (requires device on LAN) ───────────────────

func TestIntegration_GoogleHome_DiscoverDevices(t *testing.T) {
	t.Log("Scanning for Google Home devices (8s)...")
	entries, err := ghDiscoverDevices(8)
	if err != nil {
		t.Fatalf("ghDiscoverDevices() error: %v", err)
	}
	if len(entries) == 0 {
		t.Skip("No Google Home devices found on this network")
	}
	for _, e := range entries {
		t.Logf("  Found: %s at %s:%d", e.GetName(), e.GetAddr(), e.GetPort())
	}
}

// ── Google Home: play local file (requires SMART_SPEAKER_DEFAULT_DEVICE) ─────

func TestIntegration_GoogleHome_PlayLocalFile(t *testing.T) {
	saved := saveCfg()
	defer restoreCfg(saved)

	deviceName := os.Getenv("SMART_SPEAKER_DEFAULT_DEVICE")
	if deviceName == "" {
		t.Skip("SMART_SPEAKER_DEFAULT_DEVICE not set — skipping play test")
	}

	testFile := t.TempDir() + "/test.mp3"
	os.WriteFile(testFile, []byte("fake mp3 for integration"), 0644)

	localServerPort = 0
	startLocalFileServer()
	localServerIP = getLANIP()

	media := &MediaResult{
		URL:    localFileHTTPURL(testFile),
		Title:  "Integration Test Track",
		Source: "local",
	}

	msg, err := ghPlayMedia(deviceName, media)
	if err != nil {
		t.Fatalf("ghPlayMedia() error: %v", err)
	}
	t.Logf("Result: %s", msg)

	time.Sleep(3 * time.Second)
	if err := ghStop(deviceName); err != nil {
		t.Logf("ghStop() warning: %v", err)
	}
}

// ── Config: full load/save cycle ─────────────────────────────────────────────

func TestIntegration_ConfigLoadSave_FullCycle(t *testing.T) {
	saved := saveCfg()
	defer restoreCfg(saved)

	tmp := t.TempDir()
	origFile := configFile
	origDir := configDir
	configFile = tmp + "/config.json"
	configDir = tmp
	defer func() {
		configFile = origFile
		configDir = origDir
	}()

	cfg = Config{
		DefaultDevice: "Integration Speaker",
		DeviceType:    "both",
		DefaultSource: "youtube",
	}
	if err := saveConfig(); err != nil {
		t.Fatalf("saveConfig() error: %v", err)
	}

	cfg = Config{}
	if err := loadConfig(); err != nil {
		t.Fatalf("loadConfig() error: %v", err)
	}

	if cfg.DefaultDevice != "Integration Speaker" {
		t.Errorf("DefaultDevice = %q, want Integration Speaker", cfg.DefaultDevice)
	}
	if cfg.DeviceType != "both" {
		t.Errorf("DeviceType = %q, want both", cfg.DeviceType)
	}
}
