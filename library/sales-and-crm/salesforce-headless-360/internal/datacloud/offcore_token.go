package datacloud

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

const (
	tokenPath       = "/services/a360/token"
	mockTokenPath   = "/services/data/v63.0/connect/data-cloud/oauth2/token"
	sourceDataCloud = "data_cloud"
)

type CoreClient interface {
	Post(path string, body any) (json.RawMessage, int, error)
}

type OffcoreToken struct {
	AccessToken string `json:"access_token"`
	InstanceURL string `json:"instance_url"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

type Availability struct {
	Source    string
	Available bool
	Reason    string
	Warning   string
}

func Exchange(ctx context.Context, c CoreClient) (OffcoreToken, Availability, error) {
	_ = ctx
	token, availability, err := exchangePath(c, tokenPath)
	if err == nil || availability.Reason != "not_found" {
		return token, availability, err
	}
	return exchangePath(c, mockTokenPath)
}

func exchangePath(c CoreClient, path string) (OffcoreToken, Availability, error) {
	availability := Availability{Source: sourceDataCloud}
	if c == nil {
		availability.Reason = "no_client"
		return OffcoreToken{}, availability, nil
	}
	body, status, err := c.Post(path, map[string]string{})
	if status == http.StatusForbidden {
		availability.Reason = "unavailable"
		return OffcoreToken{}, availability, nil
	}
	if status == http.StatusNotFound {
		availability.Reason = "not_found"
		return OffcoreToken{}, availability, err
	}
	if err != nil {
		return OffcoreToken{}, availability, fmt.Errorf("data cloud token exchange: %w", err)
	}

	var token OffcoreToken
	if err := unmarshalEnvelope(body, &token); err != nil {
		return OffcoreToken{}, availability, fmt.Errorf("parse data cloud token: %w", err)
	}
	if token.AccessToken == "" || token.InstanceURL == "" {
		availability.Reason = "empty_token"
		return OffcoreToken{}, availability, nil
	}
	availability.Available = true
	return token, availability, nil
}

func unmarshalEnvelope(data json.RawMessage, v any) error {
	var wrapper struct {
		Envelope json.RawMessage `json:"envelope"`
	}
	if err := json.Unmarshal(data, &wrapper); err == nil && len(wrapper.Envelope) > 0 {
		return json.Unmarshal(wrapper.Envelope, v)
	}
	return json.Unmarshal(data, v)
}
