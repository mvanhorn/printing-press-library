package datacloud

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

type fakeCoreClient struct {
	responses map[string]fakeResponse
	paths     []string
}

type fakeResponse struct {
	body   json.RawMessage
	status int
	err    error
}

func (c *fakeCoreClient) Post(path string, _ any) (json.RawMessage, int, error) {
	c.paths = append(c.paths, path)
	resp, ok := c.responses[path]
	if !ok {
		return nil, http.StatusNotFound, fmt.Errorf("not found")
	}
	return resp.body, resp.status, resp.err
}

func TestExchangeHappyPath(t *testing.T) {
	c := &fakeCoreClient{responses: map[string]fakeResponse{
		tokenPath: {status: http.StatusOK, body: json.RawMessage(`{"access_token":"offcore-token","instance_url":"https://example.c360a.salesforce.com","token_type":"Bearer","expires_in":3600}`)},
	}}

	token, availability, err := Exchange(context.Background(), c)
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	if !availability.Available {
		t.Fatalf("expected available: %+v", availability)
	}
	if token.AccessToken != "offcore-token" || token.InstanceURL == "" {
		t.Fatalf("unexpected token: %+v", token)
	}
}

func TestExchangeUnprovisionedIsUnavailable(t *testing.T) {
	c := &fakeCoreClient{responses: map[string]fakeResponse{
		tokenPath: {status: http.StatusForbidden, body: json.RawMessage(`[{"errorCode":"DATA_CLOUD_NOT_PROVISIONED"}]`), err: fmt.Errorf("forbidden")},
	}}

	token, availability, err := Exchange(context.Background(), c)
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	if availability.Available || availability.Reason != "unavailable" {
		t.Fatalf("expected unavailable, got token=%+v availability=%+v", token, availability)
	}
}

func TestExchangeFallsBackToMockRoute(t *testing.T) {
	c := &fakeCoreClient{responses: map[string]fakeResponse{
		tokenPath:     {status: http.StatusNotFound, err: fmt.Errorf("not found")},
		mockTokenPath: {status: http.StatusOK, body: json.RawMessage(`{"envelope":{"access_token":"mock-token","instance_url":"https://mock.example","token_type":"Bearer"}}`)},
	}}

	token, availability, err := Exchange(context.Background(), c)
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	if !availability.Available || token.AccessToken != "mock-token" {
		t.Fatalf("expected fallback token, got token=%+v availability=%+v", token, availability)
	}
	if len(c.paths) != 2 || c.paths[0] != tokenPath || c.paths[1] != mockTokenPath {
		t.Fatalf("unexpected paths: %#v", c.paths)
	}
}
