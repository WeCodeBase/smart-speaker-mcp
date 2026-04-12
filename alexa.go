package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	alexaAPIBase   = "https://api.amazonalexa.com"
	amazonTokenURL = "https://api.amazon.com/auth/o2/token"
	amazonAuthURL  = "https://www.amazon.com/ap/oa"
	alexaScope     = "alexa::all profile"
)

// ── Types ─────────────────────────────────────────────────────────────────────

type AlexaDevice struct {
	SerialNumber string `json:"serialNumber"`
	DeviceType   string `json:"deviceType"`
	Name         string `json:"accountName"`
	CustomerID   string `json:"deviceOwnerCustomerId"`
	Online       bool   `json:"online"`
}

// ── Auth ──────────────────────────────────────────────────────────────────────

// AlexaAuthURL returns the Amazon OAuth URL for the user to visit in a browser.
func AlexaAuthURL() (string, error) {
	if cfg.Alexa.ClientID == "" {
		return "", fmt.Errorf("alexa.client_id not set in config — run set_config first")
	}
	params := url.Values{
		"client_id":     {cfg.Alexa.ClientID},
		"scope":         {alexaScope},
		"response_type": {"code"},
		"redirect_uri":  {"https://localhost"},
	}
	return amazonAuthURL + "?" + params.Encode(), nil
}

// AlexaExchangeCode exchanges an auth code (from OAuth redirect) for tokens.
func AlexaExchangeCode(code string) error {
	resp, err := http.PostForm(amazonTokenURL, url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {code},
		"client_id":    {cfg.Alexa.ClientID},
		"client_secret": {cfg.Alexa.ClientSecret},
		"redirect_uri": {"https://localhost"},
	})
	if err != nil {
		return fmt.Errorf("token exchange failed: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}
	if errMsg, ok := result["error"].(string); ok {
		return fmt.Errorf("amazon error: %s — %v", errMsg, result["error_description"])
	}
	cfg.Alexa.AccessToken, _ = result["access_token"].(string)
	cfg.Alexa.RefreshToken, _ = result["refresh_token"].(string)
	return saveConfig()
}

// alexaRefreshAccessToken refreshes the access token using the stored refresh token.
func alexaRefreshAccessToken() error {
	if cfg.Alexa.RefreshToken == "" {
		return fmt.Errorf("not authenticated — use alexa_auth to connect your Amazon account")
	}
	resp, err := http.PostForm(amazonTokenURL, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {cfg.Alexa.RefreshToken},
		"client_id":     {cfg.Alexa.ClientID},
		"client_secret": {cfg.Alexa.ClientSecret},
	})
	if err != nil {
		return fmt.Errorf("refresh failed: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}
	token, ok := result["access_token"].(string)
	if !ok {
		return fmt.Errorf("no access_token in response: %v", result)
	}
	cfg.Alexa.AccessToken = token
	return saveConfig()
}

// ── HTTP client ───────────────────────────────────────────────────────────────

func alexaRequest(method, path string, body io.Reader) (*http.Response, error) {
	doReq := func() (*http.Response, error) {
		req, err := http.NewRequest(method, alexaAPIBase+path, body)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+cfg.Alexa.AccessToken)
		req.Header.Set("Content-Type", "application/json")
		return (&http.Client{Timeout: 15 * time.Second}).Do(req)
	}

	resp, err := doReq()
	if err != nil {
		return nil, err
	}
	// Auto-refresh on 401
	if resp.StatusCode == 401 {
		resp.Body.Close()
		if refreshErr := alexaRefreshAccessToken(); refreshErr != nil {
			return nil, fmt.Errorf("token refresh failed: %w", refreshErr)
		}
		return doReq()
	}
	return resp, nil
}

// ── Device discovery ──────────────────────────────────────────────────────────

func alexaDiscoverDevices() ([]AlexaDevice, error) {
	resp, err := alexaRequest("GET", "/v1/devices", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("alexa API error %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Devices []AlexaDevice `json:"devices"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}
	return result.Devices, nil
}

func alexaGetDeviceByName(name string) (*AlexaDevice, error) {
	devices, err := alexaDiscoverDevices()
	if err != nil {
		return nil, err
	}
	for _, d := range devices {
		if strings.EqualFold(d.Name, name) {
			return &d, nil
		}
	}
	return nil, fmt.Errorf("Alexa device '%s' not found", name)
}

// ── Behaviors API (play, volume, pause, resume) ───────────────────────────────

func alexaSendBehavior(device *AlexaDevice, nodeType string, extra map[string]any) error {
	payload := map[string]any{
		"deviceType":         device.DeviceType,
		"deviceSerialNumber": device.SerialNumber,
		"locale":             "en-US",
		"customerId":         device.CustomerID,
	}
	for k, v := range extra {
		payload[k] = v
	}

	seqNode := map[string]any{
		"@type":            "com.amazon.alexa.behaviors.model.OpaquePayloadOperationNode",
		"type":             nodeType,
		"operationPayload": payload,
	}
	sequence := map[string]any{
		"@type":     "com.amazon.alexa.behaviors.model.Sequence",
		"startNode": seqNode,
	}
	seqJSON, _ := json.Marshal(sequence)

	reqBody, _ := json.Marshal(map[string]any{
		"behaviorId":   "PREVIEW",
		"sequenceJson": string(seqJSON),
		"status":       "ENABLED",
	})

	resp, err := alexaRequest("POST", "/v1/behaviors/preview", strings.NewReader(string(reqBody)))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 && resp.StatusCode != 202 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("alexa behaviors error %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func alexaPlayMusic(device *AlexaDevice, query string) error {
	return alexaSendBehavior(device, "Alexa.Music.PlaySearchPhrase", map[string]any{
		"searchPhrase": query,
	})
}

func alexaSetVolume(device *AlexaDevice, level int) error {
	return alexaSendBehavior(device, "Alexa.DeviceControls.Volume", map[string]any{
		"value": fmt.Sprintf("%d", level),
	})
}

func alexaPause(device *AlexaDevice) error {
	return alexaSendBehavior(device, "Alexa.Media.Pause", nil)
}

func alexaResume(device *AlexaDevice) error {
	return alexaSendBehavior(device, "Alexa.Media.Resume", nil)
}

func alexaStop(device *AlexaDevice) error {
	return alexaSendBehavior(device, "Alexa.Media.Stop", nil)
}

// alexaGetStatus fetches the now-playing state for a device.
func alexaGetStatus(device *AlexaDevice) (map[string]any, error) {
	path := fmt.Sprintf("/api/np/player?deviceSerialNumber=%s&deviceType=%s&screenWidth=1",
		device.SerialNumber, device.DeviceType)
	resp, err := alexaRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		// Fall back to basic device info if player state isn't available
		return map[string]any{
			"device": device.Name,
			"online": device.Online,
			"note":   "detailed playback state not available",
		}, nil
	}
	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}
	return result, nil
}
