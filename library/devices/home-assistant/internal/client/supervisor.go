// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// Supervisor is a separate, optional API with its own app-scoped bearer token.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

type CapabilityError struct {
	Surface string
	Status  int
	Detail  string
}

func (e *CapabilityError) Error() string {
	return fmt.Sprintf("capability unavailable: %s (%s)", e.Surface, e.Detail)
}

// SupervisorCall uses a user-configured direct Supervisor URL and an
// app-scoped SUPERVISOR_TOKEN. Home Assistant long-lived tokens deliberately
// do not grant this API, so the function refuses to pretend they do.
func (c *Client) SupervisorCall(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	base := strings.TrimRight(os.Getenv("HASS_SUPERVISOR_URL"), "/")
	token := strings.TrimSpace(os.Getenv("SUPERVISOR_TOKEN"))
	if base == "" || token == "" {
		return nil, &CapabilityError{Surface: "Supervisor API", Detail: "set HASS_SUPERVISOR_URL and SUPERVISOR_TOKEN from an authorized Home Assistant app"}
	}
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, base+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	hc := c.HTTPClient
	if hc == nil {
		hc = http.DefaultClient
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusNotImplemented {
		return nil, &CapabilityError{Surface: "Supervisor API", Status: resp.StatusCode, Detail: strings.TrimSpace(string(raw))}
	}
	if resp.StatusCode >= 400 {
		return nil, &APIError{Method: method, Path: path, StatusCode: resp.StatusCode, Body: string(raw)}
	}
	return json.RawMessage(raw), nil
}
