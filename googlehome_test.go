package main

import "testing"

// ── contentTypeFor ────────────────────────────────────────────────────────────

func TestContentTypeFor_MP3(t *testing.T) {
	got := contentTypeFor("http://example.com/song.mp3", "local")
	if got != "audio/mpeg" {
		t.Errorf("mp3: got %q, want audio/mpeg", got)
	}
}

func TestContentTypeFor_M4A(t *testing.T) {
	got := contentTypeFor("http://example.com/track.m4a", "youtube")
	if got != "audio/aac" {
		t.Errorf("m4a: got %q, want audio/aac", got)
	}
}

func TestContentTypeFor_AAC(t *testing.T) {
	got := contentTypeFor("http://example.com/track.aac", "url")
	if got != "audio/aac" {
		t.Errorf("aac: got %q, want audio/aac", got)
	}
}

func TestContentTypeFor_FLAC(t *testing.T) {
	got := contentTypeFor("http://example.com/track.flac", "local")
	if got != "audio/flac" {
		t.Errorf("flac: got %q, want audio/flac", got)
	}
}

func TestContentTypeFor_OGG(t *testing.T) {
	got := contentTypeFor("http://example.com/track.ogg", "url")
	if got != "audio/ogg" {
		t.Errorf("ogg: got %q, want audio/ogg", got)
	}
}

func TestContentTypeFor_OPUS(t *testing.T) {
	got := contentTypeFor("http://example.com/track.opus", "url")
	if got != "audio/ogg" {
		t.Errorf("opus: got %q, want audio/ogg", got)
	}
}

func TestContentTypeFor_WAV(t *testing.T) {
	got := contentTypeFor("http://example.com/track.wav", "local")
	if got != "audio/wav" {
		t.Errorf("wav: got %q, want audio/wav", got)
	}
}

func TestContentTypeFor_LocalSourceDefault(t *testing.T) {
	// No extension match, but source is local → default to audio/mpeg
	got := contentTypeFor("http://example.com/stream", "local")
	if got != "audio/mpeg" {
		t.Errorf("local default: got %q, want audio/mpeg", got)
	}
}

func TestContentTypeFor_YouTubeStreamDefault(t *testing.T) {
	// YouTube stream URLs don't have a clear audio extension
	got := contentTypeFor("https://googlevideo.com/videoplayback?itag=140&...", "youtube")
	if got != "audio/mp4" {
		t.Errorf("youtube default: got %q, want audio/mp4", got)
	}
}

func TestContentTypeFor_CaseInsensitiveExtension(t *testing.T) {
	// Extension detection should be case-insensitive
	got := contentTypeFor("http://example.com/track.MP3", "url")
	if got != "audio/mpeg" {
		t.Errorf("uppercase MP3: got %q, want audio/mpeg", got)
	}
}

func TestContentTypeFor_URLWithQueryParams(t *testing.T) {
	// Extension embedded in URL path with query params
	got := contentTypeFor("http://192.168.1.5:9000/localfile?path=/music/song.mp3&token=x", "local")
	if got != "audio/mpeg" {
		t.Errorf("URL with query params: got %q, want audio/mpeg", got)
	}
}
