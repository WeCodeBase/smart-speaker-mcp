package main

import (
	"context"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// ── maskSecret ────────────────────────────────────────────────────────────────

func TestMaskSecret_LongString(t *testing.T) {
	got := maskSecret("supersecrettoken")
	if got != "supe****" {
		t.Errorf("maskSecret(long) = %q, want supe****", got)
	}
}

func TestMaskSecret_ShortString(t *testing.T) {
	got := maskSecret("abc")
	if got != "***" {
		t.Errorf("maskSecret(short) = %q, want ***", got)
	}
}

func TestMaskSecret_EmptyString(t *testing.T) {
	got := maskSecret("")
	if got != "***" {
		t.Errorf("maskSecret('') = %q, want ***", got)
	}
}

func TestMaskSecret_ExactlyEightChars(t *testing.T) {
	got := maskSecret("12345678")
	if got != "1234****" {
		t.Errorf("maskSecret(8 chars) = %q, want 1234****", got)
	}
}

func TestMaskSecret_DoesNotExposeFull(t *testing.T) {
	secret := "mysupersecretpassword"
	got := maskSecret(secret)
	if got == secret {
		t.Error("maskSecret() should not return the full secret")
	}
	if strings.Contains(got, "supersecret") {
		t.Errorf("maskSecret() leaks too much: %q", got)
	}
}

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
	// Should be pretty-printed with newlines
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
	got := strArg(req, "device_name")
	if got != "Living Room" {
		t.Errorf("strArg = %q, want Living Room", got)
	}
}

func TestStrArg_MissingKey(t *testing.T) {
	req := makeRequest(map[string]any{})
	got := strArg(req, "missing_key")
	if got != "" {
		t.Errorf("strArg(missing) = %q, want empty", got)
	}
}

func TestStrArg_WrongType(t *testing.T) {
	req := makeRequest(map[string]any{"count": 42})
	got := strArg(req, "count")
	if got != "" {
		t.Errorf("strArg(int value) = %q, want empty", got)
	}
}

func TestFloatArg_ExistingKey(t *testing.T) {
	req := makeRequest(map[string]any{"level": float64(75)})
	got := floatArg(req, "level")
	if got != 75.0 {
		t.Errorf("floatArg = %f, want 75.0", got)
	}
}

func TestFloatArg_MissingKey(t *testing.T) {
	req := makeRequest(map[string]any{})
	got := floatArg(req, "missing")
	if got != 0.0 {
		t.Errorf("floatArg(missing) = %f, want 0.0", got)
	}
}

// ── handleSmartPlay – argument validation ─────────────────────────────────────

func TestHandleSmartPlay_NoQueryOrURL(t *testing.T) {
	saved := saveCfg()
	defer restoreCfg(saved)

	cfg.DefaultDevice = "Test Speaker"
	cfg.DeviceType = "google_home"

	req := makeRequest(map[string]any{})
	result, err := handleSmartPlay(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Content[0].(mcp.TextContent).Text, "❌") {
		t.Errorf("expected error result, got: %v", result)
	}
}

func TestHandleSmartPlay_NoDefaultDevice(t *testing.T) {
	saved := saveCfg()
	defer restoreCfg(saved)

	cfg.DefaultDevice = "" // No default
	cfg.DeviceType = "google_home"

	req := makeRequest(map[string]any{"query": "some song"})
	result, err := handleSmartPlay(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "❌") {
		t.Errorf("expected error about missing device, got: %q", text)
	}
}

// ── smartDeviceArgs ───────────────────────────────────────────────────────────

func TestSmartDeviceArgs_UsesConfigDefault(t *testing.T) {
	saved := saveCfg()
	defer restoreCfg(saved)

	cfg.DefaultDevice = "Config Speaker"
	cfg.DeviceType = "alexa"

	req := makeRequest(map[string]any{})
	name, dtype, err := smartDeviceArgs(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "Config Speaker" {
		t.Errorf("name = %q, want Config Speaker", name)
	}
	if dtype != "alexa" {
		t.Errorf("dtype = %q, want alexa", dtype)
	}
}

func TestSmartDeviceArgs_ArgOverridesConfig(t *testing.T) {
	saved := saveCfg()
	defer restoreCfg(saved)

	cfg.DefaultDevice = "Config Speaker"
	cfg.DeviceType = "google_home"

	req := makeRequest(map[string]any{
		"device_name": "Arg Speaker",
		"device_type": "alexa",
	})
	name, dtype, err := smartDeviceArgs(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "Arg Speaker" {
		t.Errorf("name = %q, want Arg Speaker", name)
	}
	if dtype != "alexa" {
		t.Errorf("dtype = %q, want alexa", dtype)
	}
}

func TestSmartDeviceArgs_NoDeviceReturnsError(t *testing.T) {
	saved := saveCfg()
	defer restoreCfg(saved)

	cfg.DefaultDevice = ""

	req := makeRequest(map[string]any{})
	_, _, err := smartDeviceArgs(req)
	if err == nil {
		t.Error("expected error when no device and no default, got nil")
	}
}

// ── handleGetConfig ───────────────────────────────────────────────────────────

func TestHandleGetConfig_ReturnsJSON(t *testing.T) {
	saved := saveCfg()
	defer restoreCfg(saved)

	cfg = Config{
		YtDlpPath:     "/usr/local/bin/yt-dlp",
		DefaultDevice: "Test Device",
		DeviceType:    "google_home",
		DefaultSource: "local",
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
	if !strings.Contains(text, "google_home") {
		t.Errorf("config JSON missing device type: %q", text)
	}
}

// ── handleSetConfig ───────────────────────────────────────────────────────────

func TestHandleSetConfig_UpdatesFields(t *testing.T) {
	saved := saveCfg()
	defer restoreCfg(saved)

	// Use temp config file
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
		"device_type":    "alexa",
		"default_source": "youtube",
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
	if cfg.DeviceType != "alexa" {
		t.Errorf("DeviceType = %q, want alexa", cfg.DeviceType)
	}
}
