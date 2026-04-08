package main

// httpserver.go — local HTTP file server for Chromecast LAN streaming.
//
// Chromecast (Google Home) is a network device: it cannot read file:// paths.
// It fetches audio by making an HTTP request to a URL on the local network.
// This embedded server starts on a random available port, serving local music
// files by absolute path so the Chromecast can stream them directly.

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
)

var (
	localServerPort int
	localServerIP   string
)

// startLocalFileServer binds to a random free port on the LAN interface,
// starts serving files, and stores the IP:port for URL generation.
// Called once from main() before the MCP server starts.
func startLocalFileServer() error {
	listener, err := net.Listen("tcp", ":0") // :0 → OS picks a free port
	if err != nil {
		return fmt.Errorf("local file server: cannot bind: %w", err)
	}
	localServerPort = listener.Addr().(*net.TCPAddr).Port
	localServerIP = getLANIP()

	mux := http.NewServeMux()
	mux.HandleFunc("/localfile", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Query().Get("path")
		// Basic safety: reject empty paths and traversal attempts
		if path == "" || strings.Contains(path, "..") {
			http.Error(w, "bad path", http.StatusBadRequest)
			return
		}
		// Set CORS header so Chromecast won't reject the response
		w.Header().Set("Access-Control-Allow-Origin", "*")
		http.ServeFile(w, r, path)
	})

	go func() {
		_ = http.Serve(listener, mux)
	}()
	return nil
}

// getLANIP returns the machine's first non-loopback IPv4 address —
// the address reachable by Chromecast on the same Wi-Fi.
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
