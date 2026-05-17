// Package chromecast wraps the go-chromecast library with the small subset
// of operations smart-speaker-mcp needs: mDNS discovery and playback control
// (play, pause, resume, stop, set volume, get status).
//
// It has no dependencies on other internal packages — callers pass in the
// URL of media to play, leaving media resolution to a higher layer.
package chromecast

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/vishen/go-chromecast/application"
	castdns "github.com/vishen/go-chromecast/dns"
)

// Device is a minimal view of a Chromecast device on the network.
type Device struct {
	Name string
	Host string
	Port int
	UUID string
}

// ── Discovery ─────────────────────────────────────────────────────────────────

// DiscoverDevices scans the local Wi-Fi via mDNS for Chromecast / Google Home
// / Nest devices, blocking for at most timeoutSecs.
func DiscoverDevices(timeoutSecs int) ([]Device, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSecs)*time.Second)
	defer cancel()

	entryChan, err := castdns.DiscoverCastDNSEntries(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("discovery error: %w", err)
	}

	var devices []Device
	for entry := range entryChan {
		devices = append(devices, Device{
			Name: entry.GetName(),
			Host: entry.GetAddr(),
			Port: entry.GetPort(),
			UUID: entry.GetUUID(),
		})
	}
	return devices, nil
}

// connect opens a fresh chromecast app session against the named device.
// Caller must Close(false) when done.
func connect(name string) (*application.Application, error) {
	devices, err := DiscoverDevices(8)
	if err != nil {
		return nil, err
	}
	for _, d := range devices {
		if strings.EqualFold(d.Name, name) {
			app := application.NewApplication(application.WithCacheDisabled(false))
			if err := app.Start(d.Host, d.Port); err != nil {
				return nil, fmt.Errorf("failed to connect to %s: %w", name, err)
			}
			return app, nil
		}
	}
	return nil, fmt.Errorf("Chromecast device '%s' not found on network", name)
}

// ── Playback ──────────────────────────────────────────────────────────────────

// PlayMedia streams media at mediaURL on the named device. title and source
// are used only for the returned status message. startTime is the offset in
// seconds; 0 starts at the beginning.
func PlayMedia(deviceName, mediaURL, title, source string, startTime int) (string, error) {
	type result struct {
		msg string
		err error
	}
	ch := make(chan result, 1)

	go func() {
		app, err := connect(deviceName)
		if err != nil {
			ch <- result{"", err}
			return
		}
		defer app.Close(false)

		contentType := contentTypeFor(mediaURL, source)
		if err := app.Load(mediaURL, startTime, contentType, false, false, false); err != nil {
			ch <- result{"", fmt.Errorf("cast failed: %w", err)}
			return
		}
		msg := fmt.Sprintf("▶️  Now playing: '%s' [%s] on %s", title, source, deviceName)
		if startTime > 0 {
			msg += fmt.Sprintf(" (starting at %ds)", startTime)
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

func Pause(deviceName string) error {
	app, err := connect(deviceName)
	if err != nil {
		return err
	}
	defer app.Close(false)
	return app.Pause()
}

func Resume(deviceName string) error {
	app, err := connect(deviceName)
	if err != nil {
		return err
	}
	defer app.Close(false)
	return app.Unpause()
}

func Stop(deviceName string) error {
	app, err := connect(deviceName)
	if err != nil {
		return err
	}
	defer app.Close(false)
	return app.StopMedia()
}

func SetVolume(deviceName string, level float64) error {
	if level < 0 {
		level = 0
	}
	if level > 100 {
		level = 100
	}
	app, err := connect(deviceName)
	if err != nil {
		return err
	}
	defer app.Close(false)
	return app.SetVolume(float32(level / 100.0))
}

func GetStatus(deviceName string) (map[string]any, error) {
	app, err := connect(deviceName)
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

// ── Helpers ───────────────────────────────────────────────────────────────────

// contentTypeFor picks a Content-Type header based on the URL extension and
// optional source hint ("local" prefers audio/mpeg for unknown extensions).
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
		return "audio/mpeg"
	default:
		return "audio/mp4"
	}
}
