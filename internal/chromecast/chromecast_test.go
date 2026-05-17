package chromecast

import "testing"

func TestContentTypeFor_MP3(t *testing.T) {
	if got := contentTypeFor("http://example.com/song.mp3", ""); got != "audio/mpeg" {
		t.Errorf("got %q, want audio/mpeg", got)
	}
}

func TestContentTypeFor_M4A(t *testing.T) {
	if got := contentTypeFor("file:///tmp/song.m4a", ""); got != "audio/aac" {
		t.Errorf("got %q, want audio/aac", got)
	}
}

func TestContentTypeFor_FLAC(t *testing.T) {
	if got := contentTypeFor("/music/song.flac", ""); got != "audio/flac" {
		t.Errorf("got %q, want audio/flac", got)
	}
}

func TestContentTypeFor_OGG(t *testing.T) {
	if got := contentTypeFor("/music/song.ogg", ""); got != "audio/ogg" {
		t.Errorf("got %q, want audio/ogg", got)
	}
}

func TestContentTypeFor_WAV(t *testing.T) {
	if got := contentTypeFor("/music/song.wav", ""); got != "audio/wav" {
		t.Errorf("got %q, want audio/wav", got)
	}
}

func TestContentTypeFor_LocalSourceFallback(t *testing.T) {
	if got := contentTypeFor("/music/song.weird", "local"); got != "audio/mpeg" {
		t.Errorf("got %q, want audio/mpeg (local fallback)", got)
	}
}

func TestContentTypeFor_StreamFallback(t *testing.T) {
	if got := contentTypeFor("https://stream.example.com/x", "youtube"); got != "audio/mp4" {
		t.Errorf("got %q, want audio/mp4 (stream fallback)", got)
	}
}

func TestContentTypeFor_CaseInsensitive(t *testing.T) {
	if got := contentTypeFor("/MUSIC/SONG.MP3", ""); got != "audio/mpeg" {
		t.Errorf("got %q, want audio/mpeg (uppercase URL)", got)
	}
}
