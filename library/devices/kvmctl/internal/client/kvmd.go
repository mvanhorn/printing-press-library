// PATCH(library): KVMD semantic client operations preserved from Python kvmctl.
package client

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

var ErrCapabilityUnavailable = errors.New("capability unavailable")

func (c *Client) KVMDLogin(ctx context.Context, user, password string) (string, error) {
	data, _, err := c.PostFormWithParams(ctx, "/api/auth/login", nil, url.Values{"user": {user}, "passwd": {password}})
	if err != nil {
		return "", err
	}
	var v struct {
		Result struct {
			Token string `json:"token"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return "", fmt.Errorf("parse login response: %w", err)
	}
	if v.Result.Token == "" {
		return "", errors.New("login response missing token")
	}
	return v.Result.Token, nil
}

// KVMDOtgFunctions applies the Comet gadget configuration. The query
// parameters are intentional: this firmware ignores equivalent JSON bodies.
func (c *Client) KVMDOtgFunctions(ctx context.Context, enableKeyboard, enableMouse, enableMouseAlt, startCDROM, startFlash bool) error {
	params := map[string]string{
		"enable_keyboard":  fmt.Sprintf("%t", enableKeyboard),
		"enable_mouse":     fmt.Sprintf("%t", enableMouse),
		"enable_mouse_alt": fmt.Sprintf("%t", enableMouseAlt),
		"enable_camera":    "false",
		"enable_mic":       "false",
		"enable_mtp":       "false",
		"start_cdrom":      fmt.Sprintf("%t", startCDROM),
		"start_flash":      fmt.Sprintf("%t", startFlash),
	}
	_, _, err := c.PostWithParams(ctx, "/api/system/otg_functions", params, map[string]any{})
	return err
}

// KVMDRearmOTG re-enumerates the HID gadget without leaving virtual storage
// enabled. This is required by some TH41-3/Comet firmware combinations before
// the switch will recognize sequential hotkeys.
func (c *Client) KVMDRearmOTG(ctx context.Context) error {
	if err := c.KVMDOtgFunctions(ctx, true, true, true, true, true); err != nil {
		return fmt.Errorf("enable OTG gadget: %w", err)
	}
	select {
	case <-ctx.Done():
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = c.KVMDOtgFunctions(cleanupCtx, true, true, true, false, false)
		return ctx.Err()
	case <-time.After(8 * time.Second):
	}
	if err := c.KVMDOtgFunctions(ctx, true, true, true, false, false); err != nil {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = c.KVMDOtgFunctions(cleanupCtx, true, true, true, false, false)
		return fmt.Errorf("disable virtual storage: %w", err)
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(12 * time.Second):
	}
	return nil
}

// KVMDNudgeStreamer wakes ustreamer after a USB gadget re-enumeration.
func (c *Client) KVMDNudgeStreamer(ctx context.Context) error {
	_, _, err := c.PostWithParams(ctx, "/api/streamer/set_params", map[string]string{
		"desired_fps": "40",
		"quality":     "80",
	}, map[string]any{})
	return err
}

// KVMDSnapshot returns a fresh JPEG snapshot. A KVMD streamer may return 503
// until a stream WebSocket is held open and initialized, so retries remain
// bounded, context-aware, and inside the temporary read-only lease.
func (c *Client) KVMDSnapshot(ctx context.Context) ([]byte, error) {
	headers := map[string]string{"Accept": "image/jpeg", BinaryResponseHeader: "true"}
	data, err := c.GetWithHeadersNoCache(ctx, "/api/streamer/snapshot", nil, headers)
	if err == nil {
		return decodeKVMDSnapshot(data)
	}
	if !isKVMDStreamerUnavailable(err) {
		return nil, err
	}
	lease, leaseErr := c.OpenKVMDStream(ctx)
	if leaseErr != nil {
		return nil, fmt.Errorf("open KVMD stream lease: %w", leaseErr)
	}
	defer lease.Close()

	const maxAttempts = 8
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			delay := time.Duration(attempt) * 100 * time.Millisecond
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
			}
		}
		data, lastErr = c.GetWithHeadersNoCache(ctx, "/api/streamer/snapshot", nil, headers)
		if lastErr == nil {
			return decodeKVMDSnapshot(data)
		}
		if !isKVMDStreamerUnavailable(lastErr) {
			return nil, lastErr
		}
	}
	return nil, fmt.Errorf("snapshot remained unavailable after %d attempts: %w", maxAttempts, lastErr)
}

func decodeKVMDSnapshot(data []byte) ([]byte, error) {
	var envelope binaryResponseEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil || !envelope.PPBinary {
		return data, nil
	}
	if envelope.Encoding != "base64" || !strings.HasPrefix(strings.ToLower(envelope.ContentType), "image/") {
		return nil, errors.New("unexpected KVMD snapshot response envelope")
	}
	decoded, err := base64.StdEncoding.DecodeString(envelope.Data)
	if err != nil || len(decoded) != envelope.Bytes || len(decoded) == 0 {
		return nil, errors.New("invalid KVMD snapshot response envelope")
	}
	return decoded, nil
}

func isKVMDStreamerUnavailable(err error) bool {
	return err != nil && strings.Contains(err.Error(), "HTTP 503")
}

// OpenKVMDStream opens the authenticated stream prerequisite required by
// KVMD before snapshot reads. The caller must close the returned lease.
func (c *Client) OpenKVMDStream(ctx context.Context) (io.Closer, error) {
	if c == nil || c.Config == nil {
		return nil, errors.New("KVMD client configuration is required")
	}
	endpoint, err := url.Parse(c.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse KVMD URL: %w", err)
	}
	switch endpoint.Scheme {
	case "https":
		endpoint.Scheme = "wss"
	case "http":
		endpoint.Scheme = "ws"
	default:
		return nil, fmt.Errorf("unsupported KVMD URL scheme %q", endpoint.Scheme)
	}
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + "/api/ws"
	endpoint.RawQuery = "stream=1"

	token, err := c.authHeader(ctx)
	if err != nil {
		return nil, fmt.Errorf("get KVMD stream credentials: %w", err)
	}
	headers := http.Header{"Origin": []string{originForKVMD(endpoint)}}
	if token != "" {
		headers.Set("token", token)
	}
	dialer := websocket.Dialer{HandshakeTimeout: 5 * time.Second, TLSClientConfig: c.websocketTLSConfig()}
	conn, _, err := dialer.DialContext(ctx, endpoint.String(), headers)
	if err != nil {
		return nil, fmt.Errorf("open KVMD stream: %w", err)
	}
	return conn, nil
}

func (c *Client) websocketTLSConfig() *tls.Config {
	if c == nil || c.HTTPClient == nil {
		return nil
	}
	transport, ok := c.HTTPClient.Transport.(*http.Transport)
	if !ok || transport.TLSClientConfig == nil {
		return nil
	}
	config := transport.TLSClientConfig.Clone()
	// WebSocket requires HTTP/1.1; do not offer the HTTP transport's h2 ALPN.
	config.NextProtos = []string{"http/1.1"}
	return config
}

func originForKVMD(endpoint *url.URL) string {
	scheme := "http"
	if endpoint.Scheme == "wss" {
		scheme = "https"
	}
	return scheme + "://" + endpoint.Host
}

func (c *Client) KVMDCapabilities(ctx context.Context) (map[string]bool, error) {
	data, err := c.Get(ctx, "/api/info", nil)
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Result map[string]any `json:"result"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("parse info response: %w", err)
	}
	info := envelope.Result
	if info == nil {
		info = map[string]any{}
	}
	caps := map[string]bool{"hid": false, "stream": false, "ocr": false, "switch": false}
	if h, ok := info["hid"].(map[string]any); ok {
		caps["hid"] = h["enabled"] == true && (h["connected"] == nil || h["connected"] == true)
	}
	if _, ok := info["streamer"].(map[string]any); ok {
		caps["stream"] = true
	}
	if e, ok := info["extras"].(map[string]any); ok {
		if o, ok := e["ocr"].(map[string]any); ok {
			if o["enabled"] == true {
				if ls, ok := o["languages"].(map[string]any); ok {
					for k := range ls {
						if k != "--" {
							caps["ocr"] = true
							break
						}
					}
				}
			}
		}
		if s, ok := e["switch"].(map[string]any); ok {
			caps["switch"] = s["enabled"] == true
		}
	}
	return caps, nil
}

func requireKVMDCapability(caps map[string]bool, name string) error {
	if !caps[name] {
		return fmt.Errorf("%w: %s", ErrCapabilityUnavailable, name)
	}
	return nil
}

func (c *Client) KVMDKey(ctx context.Context, key string, down bool) error {
	if strings.TrimSpace(key) == "" {
		return errors.New("key must not be empty")
	}
	_, _, err := c.PostWithParams(ctx, "/api/hid/events/send_key", map[string]string{"key": key, "state": fmt.Sprintf("%t", down)}, map[string]any{})
	return err
}
func (c *Client) KVMDShortcut(ctx context.Context, keys string) error {
	parts := strings.Split(keys, ",")
	if strings.TrimSpace(keys) == "" {
		return errors.New("shortcut must contain one or more key names")
	}
	for _, p := range parts {
		if strings.TrimSpace(p) == "" {
			return errors.New("shortcut must contain one or more key names")
		}
	}
	_, _, err := c.PostWithParams(ctx, "/api/hid/events/send_shortcut", map[string]string{"keys": keys}, map[string]any{})
	return err
}
func (c *Client) KVMDMouseMove(ctx context.Context, x, y int) error {
	if x < -32768 || x > 32767 || y < -32768 || y > 32767 {
		return errors.New("mouse coordinates must be in -32768..32767")
	}
	_, _, err := c.PostWithParams(ctx, "/api/hid/events/send_mouse_move", map[string]string{"to_x": fmt.Sprint(x), "to_y": fmt.Sprint(y)}, map[string]any{})
	return err
}
func (c *Client) KVMDMouseButton(ctx context.Context, button string, state bool) error {
	if !map[string]bool{"left": true, "middle": true, "right": true, "up": true, "down": true}[button] {
		return fmt.Errorf("unsupported mouse button: %s", button)
	}
	_, _, err := c.PostWithParams(ctx, "/api/hid/events/send_mouse_button", map[string]string{"button": button, "state": fmt.Sprintf("%t", state)}, map[string]any{})
	return err
}
func (c *Client) KVMDMouseWheel(ctx context.Context, dx, dy int) error {
	if dx < -127 || dx > 127 || dy < -127 || dy > 127 {
		return errors.New("mouse wheel deltas must be in -127..127")
	}
	_, _, err := c.PostWithParams(ctx, "/api/hid/events/send_mouse_wheel", map[string]string{"delta_x": fmt.Sprint(dx), "delta_y": fmt.Sprint(dy)}, map[string]any{})
	return err
}
