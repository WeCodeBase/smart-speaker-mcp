package main

import (
	"context"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// ── toJSON ────────────────────────────────────────────────────────────────────

func TestToJSON_SimpleMap(t *testing.T) {
	got := toJSON(map[string]string{"key": "value"})
	if !strings.Contains(got, `"key"`) || !strings.Contains(got, `"value"`) {
		t.Errorf("toJSON() = %q, expected key/value", got)
	}
}

func TestToJSON_Nil(t *testing.T) {
	got := toJSON(nil)
	if got != "null" {
		t.Errorf("toJSON(nil) = %q, want null", got)
	}
}

func TestToJSON_Struct(t *testing.T) {
	type sample struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}
	got := toJSON(sample{Name: "Claude", Age: 1})
	if !strings.Contains(got, `"name"`) || !strings.Contains(got, `"Claude"`) {
		t.Errorf("toJSON(struct) = %q, missing expected fields", got)
	}
}

func TestToJSON_IndentedOutput(t *testing.T) {
	got := toJSON(map[string]int{"a": 1})
	if !strings.Contains(got, "\n") {
		t.Errorf("toJSON() output is not indented: %q", got)
	}
}

// ── strArg / floatArg ─────────────────────────────────────────────────────────

func makeRequest(args map[string]any) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments,omitempty"`
			Meta      *struct {
				ProgressToken mcp.ProgressToken `json:"progressToken,omitempty"`
			} `json:"_meta,omitempty"`
		}{
			Arguments: args,
		},
	}
}

func TestStrArg_ExistingKey(t *testing.T) {
	req := makeRequest(map[string]any{"device_name": "Living Room"})
	if got := strArg(req, "device_name"); got != "Living Room" {
		t.Errorf("strArg = %q, want Living Room", got)
	}
}

func TestStrArg_MissingKey(t *testing.T) {
	req := makeRequest(map[string]any{})
	if got := strArg(req, "missing_key"); got != "" {
		t.Errorf("strArg(missing) = %q, want empty", got)
	}
}

func TestStrArg_WrongType(t *testing.T) {
	req := makeRequest(map[string]any{"count": 42})
	if got := strArg(req, "count"); got != "" {
		t.Errorf("strArg(int value) = %q, want empty", got)
	}
}

func TestFloatArg_ExistingKey(t *testing.T) {
	req := makeRequest(map[string]any{"level": float64(75)})
	if got := floatArg(req, "level"); got != 75.0 {
		t.Errorf("floatArg = %f, want 75.0", got)
	}
}

func TestFloatArg_MissingKey(t *testing.T) {
	req := makeRequest(map[string]any{})
	if got := floatArg(req, "missing"); got != 0.0 {
		t.Errorf("floatArg(missing) = %f, want 0.0", got)
	}
}

// ── resolveDevice ─────────────────────────────────────────────────────────────

func TestResolveDevice_UsesConfigDefault(t *testing.T) {
	saved := saveCfg()
	defer restoreCfg(saved)

	cfg.DefaultDevice = "Config Speaker"

	req := makeRequest(map[string]any{})
	name, err := resolveDevice(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "Config Speaker" {
		t.Errorf("name = %q, want Config Speaker", name)
	}
}

func TestResolveDevice_ArgOverridesConfig(t *testing.T) {
	saved := saveCfg()
	defer restoreCfg(saved)

	cfg.DefaultDevice = "Config Speaker"

	req := makeRequest(map[string]any{"device_name": "Arg Speaker"})
	name, err := resolveDevice(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "Arg Speaker" {
		t.Errorf("name = %q, want Arg Speaker", name)
	}
}

func TestResolveDevice_NoDeviceReturnsError(t *testing.T) {
	saved := saveCfg()
	defer restoreCfg(saved)

	cfg.DefaultDevice = ""

	req := makeRequest(map[string]any{})
	if _, err := resolveDevice(req); err == nil {
		t.Error("expected error when no device and no default, got nil")
	}
}

// ── handlePlay – argument validation ──────────────────────────────────────────

func TestHandlePlay_NoQueryOrURL(t *testing.T) {
	saved := saveCfg()
	defer restoreCfg(saved)

	cfg.DefaultDevice = "Test Speaker"

	req := makeRequest(map[string]any{})
	result, err := handlePlay(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "❌") {
		t.Errorf("expected error result, got: %q", text)
	}
}

func TestHandlePlay_NoDefaultDevice(t *testing.T) {
	saved := saveCfg()
	defer restoreCfg(saved)

	cfg.DefaultDevice = ""

	req := makeRequest(map[string]any{"query": "some song"})
	result, err := handlePlay(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "❌") {
		t.Errorf("expected error about missing device, got: %q", text)
	}
}

// ── handleGetConfig ───────────────────────────────────────────────────────────

func TestHandleGetConfig_ReturnsJSON(t *testing.T) {
	saved := saveCfg()
	defer restoreCfg(saved)

	cfg = Config{
		YtDlpPath:     "/usr/local/bin/yt-dlp",
		DefaultDevice: "Test Device",
		DefaultSource: "local",
		MusicDir:      "/tmp/music",
	}

	req := makeRequest(map[string]any{})
	result, err := handleGetConfig(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "default_device") {
		t.Errorf("config JSON missing default_device: %q", text)
	}
	if !strings.Contains(text, "Test Device") {
		t.Errorf("config JSON missing device value: %q", text)
	}
}

// ── handleSetConfig ───────────────────────────────────────────────────────────

func TestHandleSetConfig_UpdatesFields(t *testing.T) {
	saved := saveCfg()
	defer restoreCfg(saved)

	tmp := t.TempDir()
	origFile := configFile
	origDir := configDir
	configFile = tmp + "/config.json"
	configDir = tmp
	defer func() {
		configFile = origFile
		configDir = origDir
	}()

	cfg = Config{}

	req := makeRequest(map[string]any{
		"default_device": "New Speaker",
		"default_source": "youtube",
		"music_dir":      "/new/music",
	})

	result, err := handleSetConfig(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "✅") {
		t.Errorf("expected success, got: %q", text)
	}
	if cfg.DefaultDevice != "New Speaker" {
		t.Errorf("DefaultDevice = %q, want New Speaker", cfg.DefaultDevice)
	}
	if cfg.DefaultSource != "youtube" {
		t.Errorf("DefaultSource = %q, want youtube", cfg.DefaultSource)
	}
	if cfg.MusicDir != "/new/music" {
		t.Errorf("MusicDir = %q, want /new/music", cfg.MusicDir)
	}
}
