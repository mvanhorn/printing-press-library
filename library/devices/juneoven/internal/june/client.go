package june

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client talks to June's REST surface (token mint/refresh, status snapshot) for
// a paired identity. The identity's AccessToken is refreshed transparently on a
// 401 and persisted back to disk.
type Client struct {
	id   *Identity
	http *http.Client
}

// NewClient builds a REST client for the identity.
func NewClient(id *Identity) *Client {
	return &Client{id: id, http: &http.Client{Timeout: 15 * time.Second}}
}

type tokenResp struct {
	Success bool `json:"success"`
	Token   struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	} `json:"token"`
}

// Refresh re-registers the companion device to mint a fresh 7-day access token,
// persisting it to the identity file. This is what the app itself does; the
// OAuth refresh_token grant is rejected by June.
func (c *Client) Refresh(ctx context.Context) error {
	body, _ := json.Marshal(map[string]string{
		"password":         c.id.Password,
		"device_id":        c.id.DeviceID,
		"client_id":        clientID,
		"client_secret":    clientSecret,
		"device_type":      "companion",
		"device_name":      c.id.DeviceName,
		"platform":         "android",
		"version":          appVersion,
		"platform_version": "34",
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, APIBase+"/2/devices/register", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", UserAgent)
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("re-registering device: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("token refresh failed: HTTP %d", resp.StatusCode)
	}
	var tr tokenResp
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return fmt.Errorf("decoding token: %w", err)
	}
	c.id.AccessToken = tr.Token.AccessToken
	if tr.Token.RefreshToken != "" {
		c.id.RefreshToken = tr.Token.RefreshToken
	}
	return c.id.Save()
}

// Status fetches the oven's status snapshot, refreshing the token once on a 401.
func (c *Client) Status(ctx context.Context) (json.RawMessage, error) {
	url := fmt.Sprintf("%s/1/messaging/device/%s/status", MessagingBase, c.id.OvenID)
	raw, code, err := c.get(ctx, url)
	if err != nil {
		return nil, err
	}
	if code == http.StatusUnauthorized {
		if err := c.Refresh(ctx); err != nil {
			return nil, err
		}
		raw, code, err = c.get(ctx, url)
		if err != nil {
			return nil, err
		}
	}
	if code != http.StatusOK {
		return nil, fmt.Errorf("status request failed: HTTP %d", code)
	}
	return raw, nil
}

func (c *Client) get(ctx context.Context, url string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+c.id.AccessToken)
	req.Header.Set("User-Agent", UserAgent)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return raw, resp.StatusCode, nil
}

// StatusView is the trimmed, agent-friendly shape of an oven status snapshot.
type StatusView struct {
	ConnectionState string `json:"connection_state"`
	State           string `json:"state"`
	TargetF         *int   `json:"target_f,omitempty"`
	CookName        string `json:"cook_name,omitempty"`
}

// ParseStatus extracts the high-gravity fields from a raw status snapshot.
func ParseStatus(raw json.RawMessage) (StatusView, error) {
	var snap struct {
		ConnectionState string `json:"connection_state"`
		DeviceState     struct {
			Data struct {
				State string `json:"state"`
			} `json:"data"`
		} `json:"device_state"`
		CookPlan struct {
			Data struct {
				Food struct {
					Name string `json:"name"`
					Plan struct {
						Steps []struct {
							TemperatureCavity int `json:"temperature_cavity"`
						} `json:"steps"`
					} `json:"plan"`
				} `json:"food"`
			} `json:"data"`
		} `json:"cook_plan"`
	}
	if err := json.Unmarshal(raw, &snap); err != nil {
		return StatusView{}, err
	}
	v := StatusView{
		ConnectionState: snap.ConnectionState,
		State:           snap.DeviceState.Data.State,
		CookName:        snap.CookPlan.Data.Food.Name,
	}
	if steps := snap.CookPlan.Data.Food.Plan.Steps; len(steps) > 0 {
		f := MilliCToFahrenheit(steps[0].TemperatureCavity)
		v.TargetF = &f
	}
	return v, nil
}
