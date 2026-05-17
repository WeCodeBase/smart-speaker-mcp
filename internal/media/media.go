// Package media resolves a user query (song name, artist, or direct URL) into
// a streamable URL the chromecast package can play. Resolution order:
//
//  1. Local music library (config.Cfg.MusicDir) — fastest, no network.
//  2. Direct HTTP/HTTPS URL passed by the caller.
//  3. YouTube via yt-dlp — last-resort fallback.
//
// For local files, this package needs to know the address of the embedded
// HTTP streaming server (so Chromecasts can fetch the file over LAN).
// The webserver package calls SetStreamingServer(ip, port) at startup;
// before that, local resolution will still find the file but produce
// an unusable URL.
package media

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/WeCodeBase/smart-speaker-mcp/config"
)

// Result describes a resolved playable media item.
type Result struct {
	URL       string
	Title     string
	Source    string // "local" | "url" | "youtube"
	StartTime int
}

var audioExtensions = map[string]bool{
	".mp3": true, ".m4a": true, ".flac": true,
	".wav": true, ".ogg": true, ".aac": true,
	".opus": true, ".wma": true,
}

// ── Streaming-server hookup (set by webserver at startup) ────────────────────

var (
	streamIP   string
	streamPort int
)

// SetStreamingServer tells the media package where the embedded HTTP server
// is bound, so localFileURL() can construct URLs Chromecasts can fetch.
func SetStreamingServer(ip string, port int) {
	streamIP, streamPort = ip, port
}

func localFileURL(absPath string) string {
	return fmt.Sprintf("http://%s:%d/localfile?path=%s",
		streamIP, streamPort, url.QueryEscape(absPath),
	)
}

// ── Local music ───────────────────────────────────────────────────────────────

// ListLocal returns up to maxFiles audio filenames from config.Cfg.MusicDir.
// Used for diagnostics / the list_local_music MCP tool.
func ListLocal(maxFiles int) ([]string, error) {
	musicDir := config.ExpandHome(config.Cfg.MusicDir)
	if _, statErr := os.Stat(musicDir); os.IsNotExist(statErr) {
		return nil, fmt.Errorf("music directory not found: %s", musicDir)
	}
	var names []string
	_ = filepath.Walk(musicDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info.IsDir() {
			return nil
		}
		if audioExtensions[strings.ToLower(filepath.Ext(path))] {
			names = append(names, filepath.Base(path))
			if maxFiles > 0 && len(names) >= maxFiles {
				return filepath.SkipAll
			}
		}
		return nil
	})
	return names, nil
}

// findLocal walks config.Cfg.MusicDir for audio files matching all words in
// query (case-insensitive). If no exact match is found, returns the first
// audio file available — so artist-level queries still play something.
func findLocal(query string) (filePath, title string, err error) {
	musicDir := config.ExpandHome(config.Cfg.MusicDir)
	if _, statErr := os.Stat(musicDir); os.IsNotExist(statErr) {
		return "", "", fmt.Errorf("music directory not found: %s", musicDir)
	}

	words := strings.Fields(strings.ToLower(query))
	var firstAudio string

	err = filepath.Walk(musicDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if !audioExtensions[ext] {
			return nil
		}
		if firstAudio == "" {
			firstAudio = path
		}
		nameLower := strings.ToLower(info.Name())
		for _, word := range words {
			if !strings.Contains(nameLower, word) {
				return nil
			}
		}
		filePath = path
		title = strings.TrimSuffix(info.Name(), filepath.Ext(info.Name()))
		return filepath.SkipAll
	})

	if err != nil && err != filepath.SkipAll {
		return "", "", fmt.Errorf("walk error: %w", err)
	}
	if filePath != "" {
		return filePath, title, nil
	}
	if firstAudio != "" {
		base := filepath.Base(firstAudio)
		return firstAudio, strings.TrimSuffix(base, filepath.Ext(base)), nil
	}
	return "", "", fmt.Errorf("no audio files found in %s", musicDir)
}

// ── YouTube search ────────────────────────────────────────────────────────────

// youtubeAudioURL uses yt-dlp to search YouTube and return a (streamURL, title).
// Hard 30s timeout so it never silently hangs an MCP call.
func youtubeAudioURL(query string) (streamURL, title string, err error) {
	ytdlp := config.Cfg.YtDlpPath
	if ytdlp == "" {
		return "", "", fmt.Errorf("yt-dlp not found; set ytdlp_path in config")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, ytdlp,
		"--format", "bestaudio/best",
		"--get-url", "--get-title",
		"--no-playlist", "--quiet",
		"ytsearch1:"+query,
	)
	out, cmdErr := cmd.Output()
	if cmdErr != nil {
		if ctx.Err() != nil {
			return "", "", fmt.Errorf("yt-dlp timed out after 30s")
		}
		return "", "", fmt.Errorf("yt-dlp error: %w", cmdErr)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 {
		return "", "", fmt.Errorf("yt-dlp returned unexpected output")
	}
	return strings.TrimSpace(lines[1]), strings.TrimSpace(lines[0]), nil
}

// ── Resolver: local → URL → YouTube ──────────────────────────────────────────

// Resolve picks the best audio source in priority order. Pass an empty
// directURL if you don't have one.
func Resolve(query, directURL string, startTime int) (*Result, error) {
	// 1. Try local music
	if localPath, localTitle, err := findLocal(query); err == nil {
		return &Result{
			URL:       localFileURL(localPath),
			Title:     localTitle,
			Source:    "local",
			StartTime: startTime,
		}, nil
	} else {
		// Capture the local-error for the combined error message later
		defer func(localErr error) {
			_ = localErr // referenced below via closure
		}(err)
	}

	// 2. Direct URL
	if directURL != "" && (strings.HasPrefix(directURL, "http://") || strings.HasPrefix(directURL, "https://")) {
		title := directURL
		if idx := strings.LastIndex(directURL, "/"); idx >= 0 {
			title = directURL[idx+1:]
		}
		return &Result{
			URL:       directURL,
			Title:     title,
			Source:    "url",
			StartTime: startTime,
		}, nil
	}

	// 3. YouTube fallback
	ytURL, ytTitle, ytErr := youtubeAudioURL(query)
	if ytErr != nil {
		return nil, fmt.Errorf("could not resolve media — local + youtube both failed: %v", ytErr)
	}
	return &Result{
		URL:       ytURL,
		Title:     ytTitle,
		Source:    "youtube",
		StartTime: startTime,
	}, nil
}
