package main

// webui.go — embedded HTTP control panel.
//
// This serves a single-page web UI that calls the same handler functions
// as the MCP tools, so anything Claude can do, you can do from a browser
// on your laptop or phone (same Wi-Fi).
//
// The UI is registered on the same HTTP server as the Chromecast file-stream
// route in httpserver.go, so there's only one listening port to manage.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// ── Public entry point ────────────────────────────────────────────────────────

// registerWebUI wires the UI HTML page and the JSON API onto the given mux.
// All routes here are prefixed with "/" or "/api/" so they don't collide with
// "/localfile" used by the Chromecast streaming server.
func registerWebUI(mux *http.ServeMux) {
	mux.HandleFunc("/", serveIndex)
	mux.HandleFunc("/api/play", apiPlay)
	mux.HandleFunc("/api/pause", apiSimple(handlePause))
	mux.HandleFunc("/api/resume", apiSimple(handleResume))
	mux.HandleFunc("/api/stop", apiSimple(handleStop))
	mux.HandleFunc("/api/volume", apiVolume)
	mux.HandleFunc("/api/devices", apiDevices)
	mux.HandleFunc("/api/status", apiStatus)
	mux.HandleFunc("/api/config", apiConfig)
	mux.HandleFunc("/api/local-music", apiLocalMusic)
}

// ── HTML page ─────────────────────────────────────────────────────────────────

func serveIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	fmt.Fprint(w, indexHTML)
}

// ── JSON API helpers ──────────────────────────────────────────────────────────

type mcpHandler func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// callTool builds a synthetic CallToolRequest from arguments and invokes the
// MCP handler, then extracts the text content from the result.
func callTool(h mcpHandler, args map[string]any) (string, error) {
	req := mcp.CallToolRequest{}
	req.Params.Arguments = args
	res, err := h(context.Background(), req)
	if err != nil {
		return "", err
	}
	return extractText(res), nil
}

// extractText walks the Content slice of a CallToolResult and concatenates
// any text payloads. Works with both value and pointer TextContent variants.
func extractText(res *mcp.CallToolResult) string {
	if res == nil {
		return ""
	}
	var sb strings.Builder
	for _, c := range res.Content {
		switch t := c.(type) {
		case mcp.TextContent:
			sb.WriteString(t.Text)
		case *mcp.TextContent:
			sb.WriteString(t.Text)
		}
	}
	return sb.String()
}

// argsFromQuery copies known string keys from URL params into an args map.
func argsFromQuery(r *http.Request, keys ...string) map[string]any {
	args := map[string]any{}
	q := r.URL.Query()
	for _, k := range keys {
		if v := q.Get(k); v != "" {
			args[k] = v
		}
	}
	return args
}

// apiSimple wraps a handler that takes only optional device_name.
// Used by pause / resume / stop.
func apiSimple(h mcpHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost && r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		args := argsFromQuery(r, "device_name")
		text, err := callTool(h, args)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"result": text})
	}
}

// ── Specific API handlers ─────────────────────────────────────────────────────

func apiPlay(w http.ResponseWriter, r *http.Request) {
	args := argsFromQuery(r, "query", "url", "device_name", "source")
	if _, hasQuery := args["query"]; !hasQuery {
		if _, hasURL := args["url"]; !hasURL {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "query or url is required"})
			return
		}
	}
	text, err := callTool(handlePlay, args)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"result": text})
}

func apiVolume(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	level := q.Get("level")
	if level == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "level is required (0–100)"})
		return
	}
	deviceName := q.Get("device_name")
	if deviceName == "" {
		deviceName = cfg.DefaultDevice
	}
	if deviceName == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "device_name is required (or set default_device in config)"})
		return
	}
	// set_volume expects level as a number — parse via JSON to coerce
	var levelNum float64
	if err := json.Unmarshal([]byte(level), &levelNum); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "level must be a number"})
		return
	}
	args := map[string]any{"device_name": deviceName, "level": levelNum}
	text, err := callTool(handleSetVolume, args)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"result": text})
}

func apiDevices(w http.ResponseWriter, r *http.Request) {
	text, err := callTool(handleDiscoverDevices, nil)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	// handleDiscoverDevices returns either a JSON array of devices or a plain message.
	var devices []map[string]any
	if err := json.Unmarshal([]byte(text), &devices); err != nil {
		// Not JSON — probably "No devices found". Surface raw text.
		writeJSON(w, http.StatusOK, map[string]any{"devices": []any{}, "message": text})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"devices": devices})
}

func apiStatus(w http.ResponseWriter, r *http.Request) {
	deviceName := r.URL.Query().Get("device_name")
	if deviceName == "" {
		deviceName = cfg.DefaultDevice
	}
	if deviceName == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "device_name required"})
		return
	}
	args := map[string]any{"device_name": deviceName}
	text, err := callTool(handleGetStatus, args)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	// Pass through as raw JSON if possible
	var parsed any
	if err := json.Unmarshal([]byte(text), &parsed); err == nil {
		writeJSON(w, http.StatusOK, map[string]any{"status": parsed})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"result": text})
}

func apiConfig(w http.ResponseWriter, r *http.Request) {
	text, err := callTool(handleGetConfig, nil)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	var parsed any
	_ = json.Unmarshal([]byte(text), &parsed)
	writeJSON(w, http.StatusOK, map[string]any{"config": parsed})
}

func apiLocalMusic(w http.ResponseWriter, r *http.Request) {
	text, err := callTool(handleListLocalMusic, nil)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"result": text})
}

// ── Embedded HTML page ────────────────────────────────────────────────────────

const indexHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Smart Speaker</title>
<style>
  :root { --accent: #3a7afe; --bg: #f4f5f7; --card: #fff; --text: #1c1e21; --muted: #6b7280; --ok: #16a34a; --err: #dc2626; --border: #e5e7eb; }
  * { box-sizing: border-box; }
  body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", system-ui, sans-serif; margin: 0; padding: 1.5em 1em; background: var(--bg); color: var(--text); }
  .wrap { max-width: 640px; margin: 0 auto; }
  h1 { margin: 0 0 0.2em; font-size: 1.4em; }
  .sub { color: var(--muted); margin-bottom: 1.5em; font-size: 0.9em; }
  .card { background: var(--card); padding: 1.2em 1.4em; border-radius: 14px; box-shadow: 0 1px 3px rgba(0,0,0,0.06); margin-bottom: 1em; }
  .card h2 { margin: 0 0 0.8em; font-size: 1.05em; }
  label { display: block; font-size: 0.78em; color: var(--muted); margin: 0.6em 0 0.25em; text-transform: uppercase; letter-spacing: 0.04em; }
  input[type=text], select { width: 100%; padding: 0.7em 0.8em; border: 1px solid var(--border); border-radius: 8px; font: inherit; background: white; }
  input[type=range] { width: 100%; }
  button { padding: 0.7em 1em; border: none; border-radius: 8px; font: inherit; cursor: pointer; background: var(--accent); color: white; transition: opacity 0.15s; }
  button:hover { opacity: 0.9; }
  button:disabled { opacity: 0.5; cursor: not-allowed; }
  button.secondary { background: #6b7280; }
  button.outline { background: transparent; color: var(--accent); border: 1px solid var(--accent); }
  .row { display: flex; gap: 0.5em; flex-wrap: wrap; margin-top: 0.8em; }
  .row > * { flex: 1; min-width: 80px; }
  .vol-row { display: flex; align-items: center; gap: 0.8em; }
  .vol-row input { flex: 1; }
  .vol-num { min-width: 3em; text-align: right; font-variant-numeric: tabular-nums; color: var(--muted); }
  .status { padding: 0.75em 1em; border-radius: 8px; margin-top: 1em; font-size: 0.9em; word-break: break-word; }
  .status.ok  { background: #ecfdf5; color: var(--ok); }
  .status.err { background: #fef2f2; color: var(--err); }
  ul { padding-left: 1.2em; margin: 0.5em 0 0; }
  ul li { margin: 0.25em 0; color: var(--muted); font-size: 0.9em; }
  .hint { font-size: 0.78em; color: var(--muted); margin-top: 0.4em; }
</style>
</head>
<body>
<div class="wrap">
  <h1>🎵 Smart Speaker</h1>
  <div class="sub">Web control for <code>smart-speaker-mcp</code></div>

  <div class="card">
    <h2>Play music</h2>
    <label for="query">Song / artist / search</label>
    <input id="query" type="text" placeholder="e.g. Ilaiyaraaja" autocomplete="off" />
    <div class="hint">Or paste a direct URL into the URL field below.</div>

    <label for="url">Direct URL (optional)</label>
    <input id="url" type="text" placeholder="https://..." autocomplete="off" />

    <label for="device">Device</label>
    <select id="device"><option value="">— default from config —</option></select>

    <label for="source">Source</label>
    <select id="source">
      <option value="">— default from config —</option>
      <option value="local">Local files</option>
      <option value="youtube">YouTube</option>
    </select>

    <div class="row">
      <button onclick="play()">▶ Play</button>
      <button class="secondary" onclick="ctl('pause')">⏸ Pause</button>
      <button class="secondary" onclick="ctl('resume')">▶ Resume</button>
      <button class="secondary" onclick="ctl('stop')">⏹ Stop</button>
    </div>
  </div>

  <div class="card">
    <h2>🔊 Volume</h2>
    <div class="vol-row">
      <input id="volume" type="range" min="0" max="100" value="50" oninput="vlabel.textContent = this.value + '%'" onchange="setVolume(this.value)" />
      <span id="vlabel" class="vol-num">50%</span>
    </div>
    <div class="hint">Applies to selected device (or default).</div>
  </div>

  <div class="card">
    <h2>📡 Devices</h2>
    <div class="row">
      <button class="outline" onclick="discover()">🔄 Re-scan</button>
      <button class="outline" onclick="loadConfig()">⚙ Config</button>
    </div>
    <ul id="devicelist"><li>Scanning…</li></ul>
  </div>

  <div id="status" class="status" hidden></div>
</div>

<script>
const $ = id => document.getElementById(id);

function status(msg, ok = true) {
  const s = $('status');
  s.hidden = false;
  s.textContent = msg;
  s.className = 'status ' + (ok ? 'ok' : 'err');
}

async function api(path, params = {}) {
  const qs = new URLSearchParams(params);
  try {
    const r = await fetch('/api/' + path + (qs.toString() ? '?' + qs : ''), { method: 'POST' });
    return await r.json();
  } catch (e) {
    return { error: e.message };
  }
}

async function play() {
  const query = $('query').value.trim();
  const url   = $('url').value.trim();
  if (!query && !url) return status('Enter a song or a URL', false);
  const params = {};
  if (query) params.query = query;
  if (url)   params.url = url;
  if ($('device').value) params.device_name = $('device').value;
  if ($('source').value) params.source = $('source').value;
  status('Sending play request…');
  const r = await api('play', params);
  status(r.result || r.error || 'OK', !r.error);
}

async function ctl(action) {
  const params = {};
  if ($('device').value) params.device_name = $('device').value;
  const r = await api(action, params);
  status(r.result || r.error || 'OK', !r.error);
}

async function setVolume(v) {
  const params = { level: v };
  if ($('device').value) params.device_name = $('device').value;
  const r = await api('volume', params);
  if (r.error) status(r.error, false);
}

async function discover() {
  $('devicelist').innerHTML = '<li>Scanning…</li>';
  const r = await fetch('/api/devices').then(r => r.json());
  const list = $('devicelist'); list.innerHTML = '';
  const sel  = $('device');
  // Preserve current selection if still present
  const cur = sel.value;
  sel.innerHTML = '<option value="">— default from config —</option>';
  if (r.devices && r.devices.length) {
    r.devices.forEach(d => {
      const li = document.createElement('li');
      li.textContent = d.name + ' — ' + d.host + ':' + d.port;
      list.appendChild(li);
      const opt = document.createElement('option');
      opt.value = d.name; opt.textContent = d.name;
      sel.appendChild(opt);
    });
    if (cur) sel.value = cur;
    status('Found ' + r.devices.length + ' device(s)', true);
  } else {
    list.innerHTML = '<li>' + (r.message || 'No devices found') + '</li>';
    status(r.message || 'No devices found', false);
  }
}

async function loadConfig() {
  const r = await fetch('/api/config').then(r => r.json());
  if (r.error) return status(r.error, false);
  status('Config: source=' + r.config.default_source + ', device=' + (r.config.default_device || '(none)') + ', music_dir=' + r.config.music_dir, true);
}

// Initial load
discover();
</script>
</body>
</html>
`
