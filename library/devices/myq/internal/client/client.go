package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"os"
	"time"
)

const (
	accountsEndpoint   = "https://accounts.myq-cloud.com/api/v6.0/accounts"
	devicesEndpointFmt = "https://devices.myq-cloud.com/api/v5.2/Accounts/%s/Devices"
	deviceEndpointFmt  = "https://devices.myq-cloud.com/api/v5.2/Accounts/%s/Devices/%s"
	actionsEndpointFmt = "https://account-devices-gdo.myq-cloud.com/api/v5.2/Accounts/%s/door_openers/%s/%s"
)

const (
	ActionClose = "close"
	ActionOpen  = "open"

	StateUnknown = "unknown"
	StateOpen    = "open"
	StateClosed  = "closed"
	StateStopped = "stopped"
)

var ErrNotLoggedIn = errors.New("not logged in")

type Client struct {
	Username     string
	Password     string
	Debug        bool
	ClientSecret string

	httpClient *http.Client
	token      string
	accounts   []*Account
}

type Account struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Device struct {
	Account      *Account
	SerialNumber string
	Type         string
	Name         string
	DoorState    string
}

func doRequest(client *http.Client, req *http.Request) (*http.Response, error) {
	return client.Do(req)
}

func New(username, password string, debug bool, timeout time.Duration, clientSecret string) *Client {
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	return &Client{
		Username:     username,
		Password:     password,
		Debug:        debug,
		ClientSecret: clientSecret,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *Client) Login(ctx context.Context) error {
	o, err := newOAuth(c.Debug, c.ClientSecret)
	if err != nil {
		return err
	}

	u, err := o.authorize(ctx)
	if err != nil {
		return err
	}
	u, err = o.login(ctx, u, c.Username, c.Password)
	if err != nil {
		return err
	}
	u, err = o.callback(ctx, u)
	if err != nil {
		return err
	}
	token, err := o.token(ctx, u)
	if err != nil {
		return err
	}

	c.token = token
	return nil
}

func (c *Client) Devices(ctx context.Context) ([]Device, error) {
	if err := c.fillAccounts(ctx); err != nil {
		return nil, err
	}

	var devices []Device
	for _, acct := range c.accounts {
		endpoint := fmt.Sprintf(devicesEndpointFmt, acct.ID)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}

		type item struct {
			SerialNumber string `json:"serial_number"`
			DeviceType   string `json:"device_type"`
			Name         string `json:"name"`
			State        struct {
				DoorState string `json:"door_state"`
			} `json:"state"`
		}

		var body struct {
			Items []item `json:"items"`
		}
		if err := c.apiRequestWithRetry(ctx, req, &body); err != nil {
			return nil, err
		}

		for i := range body.Items {
			devices = append(devices, Device{
				Account:      acct,
				SerialNumber: body.Items[i].SerialNumber,
				Type:         body.Items[i].DeviceType,
				Name:         body.Items[i].Name,
				DoorState:    body.Items[i].State.DoorState,
			})
		}
	}
	return devices, nil
}

func (c *Client) DeviceState(ctx context.Context, serialNumber string) (string, error) {
	if err := c.fillAccounts(ctx); err != nil {
		return "", err
	}

	for _, acct := range c.accounts {
		endpoint := fmt.Sprintf(deviceEndpointFmt, acct.ID, serialNumber)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return "", err
		}

		var body struct {
			State struct {
				DoorState string `json:"door_state"`
			} `json:"state"`
		}
		if err := c.apiRequestWithRetry(ctx, req, &body); err != nil {
			if isStatus(err, http.StatusNotFound) {
				continue
			}
			return "", err
		}
		return body.State.DoorState, nil
	}

	return "", fmt.Errorf("device %s not found", serialNumber)
}

func (c *Client) SetDoorState(ctx context.Context, serialNumber string, action string) error {
	if err := c.fillAccounts(ctx); err != nil {
		return err
	}

	for _, acct := range c.accounts {
		endpoint := fmt.Sprintf(actionsEndpointFmt, acct.ID, serialNumber, action)
		req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, nil)
		if err != nil {
			return err
		}
		var body struct{}
		if err := c.apiRequestWithRetry(ctx, req, &body); err != nil {
			if isStatus(err, http.StatusNotFound) {
				continue
			}
			return err
		}
		return nil
	}

	return fmt.Errorf("device %s not found", serialNumber)
}

func (c *Client) fillAccounts(ctx context.Context) error {
	if len(c.accounts) > 0 {
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, accountsEndpoint, nil)
	if err != nil {
		return err
	}

	var jsonResponse struct {
		Accounts []*Account `json:"accounts"`
	}
	if err := c.apiRequestWithRetry(ctx, req, &jsonResponse); err != nil {
		return err
	}
	c.accounts = jsonResponse.Accounts
	return nil
}

func (c *Client) apiRequestWithRetry(ctx context.Context, req *http.Request, target any) error {
	if err := c.apiRequest(ctx, req, target); err == ErrNotLoggedIn {
		if err := c.Login(ctx); err != nil {
			return err
		}
		return c.apiRequest(ctx, req, target)
	} else {
		return err
	}
}

func (c *Client) apiRequest(ctx context.Context, req *http.Request, target any) error {
	if req.Body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.doRequest(req.WithContext(ctx))
	if err != nil {
		return err
	}
	defer drain(resp.Body)

	switch resp.StatusCode {
	case http.StatusOK:
		return json.NewDecoder(resp.Body).Decode(target)
	case http.StatusNoContent, http.StatusAccepted:
		return nil
	case http.StatusUnauthorized:
		return ErrNotLoggedIn
	default:
		var errResp errorResponse
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
			errResp.Message = fmt.Sprintf("received HTTP status code %d", resp.StatusCode)
		}
		errResp.StatusCode = resp.StatusCode
		return &errResp
	}
}

func (c *Client) doRequest(req *http.Request) (*http.Response, error) {
	if c.Debug {
		if dump, err := httputil.DumpRequestOut(req, true); err == nil {
			fmt.Fprintln(os.Stderr, string(dump))
		}
	}

	resp, err := doRequest(c.httpClient, req)
	if err != nil {
		return nil, err
	}

	if c.Debug {
		if dump, err := httputil.DumpResponse(resp, true); err == nil {
			fmt.Fprintln(os.Stderr, string(dump))
		}
	}

	return resp, nil
}

type errorResponse struct {
	StatusCode int `json:"-"`

	Message     string
	Description string
}

func (e *errorResponse) Error() string {
	if e.Description != "" {
		return e.Message + ": " + e.Description
	}
	return e.Message
}

func isStatus(err error, code int) bool {
	e, ok := err.(*errorResponse)
	return ok && e.StatusCode == code
}

func drain(rc io.ReadCloser) {
	_, _ = io.Copy(io.Discard, rc)
	_ = rc.Close()
}
