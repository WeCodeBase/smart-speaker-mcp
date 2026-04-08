package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var audioExtensions = map[string]bool{
	".mp3": true, ".m4a": true, ".flac": true,
	".wav": true, ".ogg": true, ".aac": true,
	".opus": true, ".wma": true,
}

// ── Local music search ────────────────────────────────────────────────────────

// ListLocalMusic returns up to maxFiles audio file names from cfg.MusicDir.
// Used for diagnostics / the list_local_music tool.
func ListLocalMusic(maxFiles int) ([]string, error) {
	musicDir := expandHome(cfg.MusicDir)
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

// findLocalMusic walks cfg.MusicDir looking for audio files whose names
// contain all words in the query (case-insensitive). If no exact match is
// found but audio files exist in the directory, the first available file is
// returned so artist-level queries ("Ilaiyaraaja") still play local music.
func findLocalMusic(query string) (filePath, title string, err error) {
	musicDir := expandHome(cfg.MusicDir)
	if _, statErr := os.Stat(musicDir); os.IsNotExist(statErr) {
		return "", "", fmt.Errorf("music directory not found: %s", musicDir)
	}

	words := strings.Fields(strings.ToLower(query))
	var firstAudio string // fallback: first audio file found, any name

	err = filepath.Walk(musicDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if !audioExtensions[ext] {
			return nil
		}

		// Remember the first audio file as a fallback
		if firstAudio == "" {
			firstAudio = path
		}

		// Check if all query words appear in the filename
		nameLower := strings.ToLower(info.Name())
		for _, word := range words {
			if !strings.Contains(nameLower, word) {
				return nil
			}
		}
		// All words matched — use this file
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

	// No exact match — play the first available audio file in the directory
	if firstAudio != "" {
		base := filepath.Base(firstAudio)
		return firstAudio, strings.TrimSuffix(base, filepath.Ext(base)), nil
	}

	return "", "", fmt.Errorf("no audio files found in %s", musicDir)
}

// ── YouTube search ────────────────────────────────────────────────────────────

// getYouTubeAudioURL uses yt-dlp to search YouTube and return a stream URL.
// Hard timeout of 30s so it never silently hangs the whole MCP call.
func getYouTubeAudioURL(query string) (streamURL, title string, err error) {
	ytdlp := cfg.YtDlpPath
	if ytdlp == "" {
		ytdlp = detectYtDlp()
	}
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

// ── Resolver: local first, direct URL second, YouTube fallback ────────────────

type MediaResult struct {
	URL       string
	Title     string
	Source    string // "local", "url", or "youtube"
	StartTime int    // seconds offset to begin playback
}

// resolveMedia determines the best audio source in priority order:
//  1. Local music library (cfg.MusicDir) — fastest, no network needed
//  2. Direct HTTP/HTTPS URL — if the caller passes one explicitly
//  3. YouTube via yt-dlp — last resort fallback
func resolveMedia(query, directURL string, startTime int) (*MediaResult, error) {
	// 1️⃣ Try local music first
	localPath, localTitle, localErr := findLocalMusic(query)
	if localErr == nil {
		return &MediaResult{
			URL:       localFileHTTPURL(localPath),
			Title:     localTitle,
			Source:    "local",
			StartTime: startTime,
		}, nil
	}

	// 2️⃣ Direct HTTP/HTTPS URL provided by caller
	if directURL != "" && (strings.HasPrefix(directURL, "http://") || strings.HasPrefix(directURL, "https://")) {
		title := directURL
		if idx := strings.LastIndex(directURL, "/"); idx >= 0 {
			title = directURL[idx+1:]
		}
		return &MediaResult{
			URL:       directURL,
			Title:     title,
			Source:    "url",
			StartTime: startTime,
		}, nil
	}

	// 3️⃣ Fall back to YouTube (with 30s timeout baked in)
	ytURL, ytTitle, ytErr := getYouTubeAudioURL(query)
	if ytErr != nil {
		return nil, fmt.Errorf("local: %v | youtube: %v", localErr, ytErr)
	}
	return &MediaResult{
		URL:       ytURL,
		Title:     ytTitle,
		Source:    "youtube",
		StartTime: startTime,
	}, nil
}
