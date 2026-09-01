// PATCH(library): KVMD semantic client operations preserved from Python kvmctl.
package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
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
