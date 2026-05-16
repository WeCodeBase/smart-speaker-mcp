package main

// httpserver.go — local HTTP server with two responsibilities:
//
//  1. /localfile  — Chromecast (Google Home) is a network device that cannot
//     read file:// paths. It fetches audio by HTTP from a URL on the local
//     network. This route streams local music files by absolute path so the
//     Chromecast can play them directly over Wi-Fi.
//
//  2. /  + /api/* — A web UI control panel (see webui.go). Lets you play,
//     pause, set volume, and discover devices from any browser on the same
//     Wi-Fi (laptop or phone).
//
// Both share one TCP listener / one port to keep things simple.

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
)

const (
	preferredWebPort = 8765 // try this first; fall back to a random port if taken
	webPortEnvVar    = "SMART_SPEAKER_WEB_PORT"
)

var (
	localServerPort int
	localServerIP   string
)

// startLocalFileServer binds the HTTP server, mounts both the Chromecast
// streaming route and the web UI routes, and announces the URL on stderr.
// Called once from main() before the MCP server starts.
func startLocalFileServer() error {
	listener, err := bindWebListener()
	if err != nil {
		return fmt.Errorf("local file server: cannot bind: %w", err)
	}
	localServerPort = listener.Addr().(*net.TCPAddr).Port
	localServerIP = getLANIP()

	mux := http.NewServeMux()
	mux.HandleFunc("/localfile", handleLocalFile)
	registerWebUI(mux) // routes from webui.go: /, /api/*

	go func() {
		_ = http.Serve(listener, mux)
	}()

	announceWebUI()
	return nil
}

// bindWebListener tries the user-preferred port first (env var or constant),
// then falls back to an OS-assigned random port if that's already in use.
func bindWebListener() (net.Listener, error) {
	preferred := preferredWebPort
	if v := os.Getenv(webPortEnvVar); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n < 65536 {
			preferred = n
		}
	}
	if l, err := net.Listen("tcp", fmt.Sprintf(":%d", preferred)); err == nil {
		return l, nil
	}
	// Fall back to a random free port
	return net.Listen("tcp", ":0")
}

// announceWebUI writes the control panel URL to stderr. stderr is safe with
// MCP — only stdout carries the JSON-RPC protocol stream. Claude Desktop will
// also surface these lines in its mcp-server-smart-speaker.log file.
func announceWebUI() {
	log.Printf("───────────────────────────────────────────────────────────")
	log.Printf("🎵 smart-speaker-mcp web UI — open in any browser to control playback:")
	log.Printf("    Local:    http://localhost:%d", localServerPort)
	if localServerIP != "" && localServerIP != "127.0.0.1" {
		log.Printf("    Network:  http://%s:%d   (use this from your phone)", localServerIP, localServerPort)
	}
	log.Printf("───────────────────────────────────────────────────────────")
}

// handleLocalFile serves a local audio file referenced by absolute path,
// for Chromecast streaming. Path traversal is rejected.
func handleLocalFile(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" || strings.Contains(path, "..") {
		http.Error(w, "bad path", http.StatusBadRequest)
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	http.ServeFile(w, r, path)
}

// getLANIP returns the machine's first non-loopback IPv4 address —
// the address reachable by Chromecast (and your phone) on the same Wi-Fi.
func getLANIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}
	for _, addr := range addrs {
		ipnet, ok := addr.(*net.IPNet)
		if !ok || ipnet.IP.IsLoopback() {
			continue
		}
		if ip4 := ipnet.IP.To4(); ip4 != nil {
			return ip4.String()
		}
	}
	return "127.0.0.1"
}

// localFileHTTPURL converts an absolute local file path to an HTTP URL
// that the Chromecast can fetch over the LAN.
func localFileHTTPURL(absPath string) string {
	return fmt.Sprintf("http://%s:%d/localfile?path=%s",
		localServerIP,
		localServerPort,
		url.QueryEscape(absPath),
	)
}
