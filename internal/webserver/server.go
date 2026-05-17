// Package webserver hosts the embedded HTTP server. It has two routes baked in:
//
//   GET /localfile?path=...  — streams a local audio file to a Chromecast (no
//                              listing, no traversal, paths must be absolute).
//   * Other routes           — added by callers via the *http.ServeMux returned
//                              from Start(). The mcpserver package mounts the
//                              web UI here.
//
// Start() also reports the bound IP and port back so other packages (notably
// media) can build URLs Chromecasts on the LAN can fetch.
package webserver

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
)

const (
	preferredPort = 8765
	portEnvVar    = "SMART_SPEAKER_WEB_PORT"
)

// Info describes a running webserver.
type Info struct {
	IP   string
	Port int
	Mux  *http.ServeMux
}

// Start binds the HTTP server (preferring SMART_SPEAKER_WEB_PORT, then 8765,
// then a random free port), registers the /localfile route, and returns the
// listener info plus a mux callers can attach extra routes to.
func Start() (*Info, error) {
	listener, err := bindListener()
	if err != nil {
		return nil, fmt.Errorf("webserver: cannot bind: %w", err)
	}

	info := &Info{
		IP:   detectLANIP(),
		Port: listener.Addr().(*net.TCPAddr).Port,
		Mux:  http.NewServeMux(),
	}
	info.Mux.HandleFunc("/localfile", handleLocalFile)

	go func() { _ = http.Serve(listener, info.Mux) }()

	announce(info)
	return info, nil
}

func bindListener() (net.Listener, error) {
	port := preferredPort
	if v := os.Getenv(portEnvVar); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n < 65536 {
			port = n
		}
	}
	if l, err := net.Listen("tcp", fmt.Sprintf(":%d", port)); err == nil {
		return l, nil
	}
	return net.Listen("tcp", ":0")
}

// announce writes the control panel URL to stderr — safe with MCP because
// only stdout carries the JSON-RPC protocol stream. Visible in Claude
// Desktop's mcp-server-smart-speaker.log.
func announce(info *Info) {
	log.Printf("───────────────────────────────────────────────────────────")
	log.Printf("🎵 smart-speaker-mcp web UI — open in any browser to control playback:")
	log.Printf("    Local:    http://localhost:%d", info.Port)
	if info.IP != "" && info.IP != "127.0.0.1" {
		log.Printf("    Network:  http://%s:%d   (use this from your phone)", info.IP, info.Port)
	}
	log.Printf("───────────────────────────────────────────────────────────")
}

// handleLocalFile serves a local audio file by absolute path, for Chromecast
// streaming. Path traversal (..) is rejected.
func handleLocalFile(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" || strings.Contains(path, "..") {
		http.Error(w, "bad path", http.StatusBadRequest)
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	http.ServeFile(w, r, path)
}

// detectLANIP returns the machine's first non-loopback IPv4 address.
func detectLANIP() string {
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
