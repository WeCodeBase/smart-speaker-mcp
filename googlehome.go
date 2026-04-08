package main

import (
	"fmt"
	"strings"
	"time"

	"context"

	"github.com/vishen/go-chromecast/application"
	castdns "github.com/vishen/go-chromecast/dns"
)

// contentTypeFor returns a suitable MIME type based on URL extension / source.
func contentTypeFor(url, source string) string {
	lower := strings.ToLower(url)
	switch {
	case strings.Contains(lower, ".mp3"):
		return "audio/mpeg"
	case strings.Contains(lower, ".m4a"), strings.Contains(lower, ".aac"):
		return "audio/aac"
	case strings.Contains(lower, ".flac"):
		return "audio/flac"
	case strings.Contains(lower, ".ogg"), strings.Contains(lower, ".opus"):
		return "audio/ogg"
	case strings.Contains(lower, ".wav"):
		return "audio/wav"
	case source == "local":
		return "audio/mpeg" // default for local files
	default:
		return "audio/mp4" // default for streams (YouTube, etc.)
	}
}

// ── Discovery ─────────────────────────────────────────────────────────────────

func ghDiscoverDevices(timeoutSecs int) ([]castdns.CastEntry, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSecs)*time.Second)
	defer cancel()

	entryChan, err := castdns.DiscoverCastDNSEntries(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("discovery error: %w", err)
	}

	var entries []castdns.CastEntry
	for entry := range entryChan {
		entries = append(entries, entry)
	}
	return entries, nil
}

func ghGetDevice(name string) (*application.Application, error) {
	entries, err := ghDiscoverDevices(8)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if strings.EqualFold(entry.GetName(), name) {
			app := application.NewApplication(application.WithCacheDisabled(false))
			if err := app.Start(entry.GetAddr(), entry.GetPort()); err != nil {
				return nil, fmt.Errorf("failed to connect to %s: %w", name, err)
			}
			return app, nil
		}
	}
	return nil, fmt.Errorf("Google Home device '%s' not found on network", name)
}

// ── Playback ──────────────────────────────────────────────────────────────────

func ghPlayMedia(deviceName string, media *MediaResult) (string, error) {
	type result struct {
		msg string
		err error
	}
	ch := make(chan result, 1)

	go func() {
		app, err := ghGetDevice(deviceName)
		if err != nil {
			ch <- result{"", err}
			return
		}
		defer app.Close(false)

		contentType := contentTypeFor(media.URL, media.Source)
		if err := app.Load(media.URL, media.StartTime, contentType, false, false, false); err != nil {
			ch <- result{"", fmt.Errorf("cast failed: %w", err)}
			return
		}
		msg := fmt.Sprintf("▶️  Now playing: '%s' [%s] on %s", media.Title, media.Source, deviceName)
		if media.StartTime > 0 {
			msg += fmt.Sprintf(" (starting at %ds)", media.StartTime)
		}
		ch <- result{msg, nil}
	}()

	select {
	case r := <-ch:
		return r.msg, r.err
	case <-time.After(20 * time.Second):
		return "", fmt.Errorf("timed out connecting to '%s' — is it on the same Wi-Fi?", deviceName)
	}
}

func ghPause(deviceName string) error {
	app, err := ghGetDevice(deviceName)
	if err != nil {
		return err
	}
	defer app.Close(false)
	return app.Pause()
}

func ghResume(deviceName string) error {
	app, err := ghGetDevice(deviceName)
	if err != nil {
		return err
	}
	defer app.Close(false)
	return app.Unpause()
}

func ghStop(deviceName string) error {
	app, err := ghGetDevice(deviceName)
	if err != nil {
		return err
	}
	defer app.Close(false)
	return app.StopMedia()
}

func ghSetVolume(deviceName string, level float64) error {
	if level < 0 {
		level = 0
	}
	if level > 100 {
		level = 100
	}
	app, err := ghGetDevice(deviceName)
	if err != nil {
		return err
	}
	defer app.Close(false)
	return app.SetVolume(float32(level / 100.0))
}

func ghGetStatus(deviceName string) (map[string]any, error) {
	app, err := ghGetDevice(deviceName)
	if err != nil {
		return nil, err
	}
	defer app.Close(false)
	castApp, castMedia, castVol := app.Status()
	return map[string]any{
		"device": deviceName,
		"app":    castApp,
		"media":  castMedia,
		"volume": castVol,
	}, nil
}
