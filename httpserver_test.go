package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── getLANIP ──────────────────────────────────────────────────────────────────

func TestGetLANIP_ReturnsValidIP(t *testing.T) {
	ip := getLANIP()
	if ip == "" {
		t.Error("getLANIP() returned empty string")
	}
	// Must be a dotted-decimal IPv4 address
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		t.Errorf("getLANIP() = %q, want dotted-decimal IPv4", ip)
	}
}

func TestGetLANIP_NeverReturnsEmpty(t *testing.T) {
	ip := getLANIP()
	if ip == "" {
		t.Error("getLANIP() returned empty; should fall back to 127.0.0.1")
	}
}

// ── localFileHTTPURL ──────────────────────────────────────────────────────────

func TestLocalFileHTTPURL_Format(t *testing.T) {
	localServerIP = "192.168.1.100"
	localServerPort = 54321

	url := localFileHTTPURL("/Users/test/music/song.mp3")

	if !strings.HasPrefix(url, "http://192.168.1.100:54321/localfile?path=") {
		t.Errorf("unexpected URL format: %q", url)
	}
}

func TestLocalFileHTTPURL_EncodesSpecialChars(t *testing.T) {
	localServerIP = "127.0.0.1"
	localServerPort = 8080

	url := localFileHTTPURL("/Users/test/my music/song name.mp3")

	// Spaces should be percent-encoded
	if strings.Contains(url, " ") {
		t.Errorf("URL contains unencoded space: %q", url)
	}
}

func TestLocalFileHTTPURL_ContainsPath(t *testing.T) {
	localServerIP = "127.0.0.1"
	localServerPort = 9000

	absPath := "/home/user/songs/track.flac"
	url := localFileHTTPURL(absPath)

	if !strings.Contains(url, "path=") {
		t.Errorf("URL missing path parameter: %q", url)
	}
	if !strings.Contains(url, "/localfile") {
		t.Errorf("URL missing /localfile endpoint: %q", url)
	}
}

// ── startLocalFileServer ──────────────────────────────────────────────────────

func TestStartLocalFileServer_BindsSuccessfully(t *testing.T) {
	// Reset port to 0 so a new one is assigned
	localServerPort = 0

	err := startLocalFileServer()
	if err != nil {
		t.Fatalf("startLocalFileServer() error: %v", err)
	}
	if localServerPort == 0 {
		t.Error("localServerPort was not set after startLocalFileServer()")
	}
	if localServerIP == "" {
		t.Error("localServerIP was not set after startLocalFileServer()")
	}
}

func TestLocalFileServer_ServesExistingFile(t *testing.T) {
	// Write a test file
	tmp := t.TempDir()
	content := "fake mp3 audio data for testing"
	testFile := filepath.Join(tmp, "test.mp3")
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Start a fresh server
	localServerPort = 0
	if err := startLocalFileServer(); err != nil {
		t.Fatalf("startLocalFileServer() error: %v", err)
	}
	localServerIP = "127.0.0.1" // use loopback for test

	url := localFileHTTPURL(testFile)
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("HTTP GET %s error: %v", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != content {
		t.Errorf("body = %q, want %q", string(body), content)
	}
}

func TestLocalFileServer_RejectsEmptyPath(t *testing.T) {
	localServerPort = 0
	if err := startLocalFileServer(); err != nil {
		t.Fatalf("startLocalFileServer() error: %v", err)
	}
	localServerIP = "127.0.0.1"

	url := fmt.Sprintf("http://127.0.0.1:%d/localfile", localServerPort)
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("HTTP GET error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("empty path: status = %d, want 400", resp.StatusCode)
	}
}

func TestLocalFileServer_RejectsPathTraversal(t *testing.T) {
	localServerPort = 0
	if err := startLocalFileServer(); err != nil {
		t.Fatalf("startLocalFileServer() error: %v", err)
	}
	localServerIP = "127.0.0.1"

	url := fmt.Sprintf("http://127.0.0.1:%d/localfile?path=../../etc/passwd", localServerPort)
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("HTTP GET error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("path traversal: status = %d, want 400", resp.StatusCode)
	}
}

func TestLocalFileServer_SetsCORSHeader(t *testing.T) {
	localServerPort = 0
	if err := startLocalFileServer(); err != nil {
		t.Fatalf("startLocalFileServer() error: %v", err)
	}
	localServerIP = "127.0.0.1"

	// Create a real file to serve
	tmp := t.TempDir()
	testFile := filepath.Join(tmp, "song.mp3")
	os.WriteFile(testFile, []byte("data"), 0644)

	url := localFileHTTPURL(testFile)
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("HTTP GET error: %v", err)
	}
	defer resp.Body.Close()

	cors := resp.Header.Get("Access-Control-Allow-Origin")
	if cors != "*" {
		t.Errorf("CORS header = %q, want *", cors)
	}
}
