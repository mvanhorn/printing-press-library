package agent

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

type fakeEnrichmentClient struct {
	post map[string]fakeHTTPResult
	get  map[string]fakeHTTPResult
}

type fakeHTTPResult struct {
	body   json.RawMessage
	status int
	err    error
}

func (c fakeEnrichmentClient) Post(path string, _ any) (json.RawMessage, int, error) {
	result, ok := c.post[path]
	if !ok {
		return nil, http.StatusNotFound, fmt.Errorf("not found")
	}
	return result.body, result.status, result.err
}

func (c fakeEnrichmentClient) Get(path string, _ map[string]string) (json.RawMessage, error) {
	result, ok := c.get[path]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return result.body, result.err
}

func TestAssembleEnrichesDataCloudAndSlackLinkage(t *testing.T) {
	profileHTTP := &http.Client{Transport: agentRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		return agentJSONResponse(http.StatusOK, `{"unifiedProfileId":"up_mock_001"}`), nil
	})}
	c := fakeEnrichmentClient{
		post: map[string]fakeHTTPResult{
			"/services/a360/token": {status: http.StatusOK, body: json.RawMessage(`{"access_token":"offcore-token","instance_url":"https://data-cloud.test"}`)},
		},
		get: map[string]fakeHTTPResult{
			"/services/data/v63.0/query": {body: json.RawMessage(`{"totalSize":1,"records":[{"EntityId":"001ACME0001","SlackChannelId":"C012ACME","WorkspaceId":"T012ACME"}]}`)},
		},
	}

	bundle, err := Assemble(Manifest{Account: &Account{ID: "001ACME0001", Name: "Acme"}}, AssembleOptions{
		AccountHint:         "001ACME0001",
		SourcesUsed:         []string{"composite_graph"},
		EnrichmentClient:    c,
		DataCloudHTTPClient: profileHTTP,
	}, nil)
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	if bundle.Manifest.DataCloudProfile["unifiedProfileId"] != "up_mock_001" {
		t.Fatalf("missing Data Cloud profile: %#v", bundle.Manifest.DataCloudProfile)
	}
	if len(bundle.Manifest.SlackChannels) != 1 || bundle.Manifest.SlackChannels[0].ChannelID != "C012ACME" {
		t.Fatalf("missing Slack linkage: %#v", bundle.Manifest.SlackChannels)
	}
	wantSources := []string{"rest", "data_cloud", "slack_linkage"}
	if fmt.Sprint(bundle.Envelope.SourcesUsed) != fmt.Sprint(wantSources) {
		t.Fatalf("sources_used mismatch: got %#v want %#v", bundle.Envelope.SourcesUsed, wantSources)
	}
}

func TestAssembleRestOnlySkipsSlackHistoryWhenNoLinkage(t *testing.T) {
	slackCalls := 0
	httpClient := &http.Client{Transport: agentRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		slackCalls++
		return agentJSONResponse(http.StatusOK, `{}`), nil
	})}
	c := fakeEnrichmentClient{
		post: map[string]fakeHTTPResult{
			"/services/a360/token": {status: http.StatusForbidden, err: fmt.Errorf("forbidden")},
		},
		get: map[string]fakeHTTPResult{
			"/services/data/v63.0/query": {body: json.RawMessage(`{"totalSize":0,"records":[]}`)},
		},
	}

	bundle, err := Assemble(Manifest{Account: &Account{ID: "001ACME0001", Name: "Acme"}}, AssembleOptions{
		AccountHint:      "001ACME0001",
		SourcesUsed:      []string{"rest"},
		EnrichmentClient: c,
		SlackBotToken:    "xoxb-test",
		SlackHTTPClient:  httpClient,
	}, nil)
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	if fmt.Sprint(bundle.Envelope.SourcesUsed) != "[rest]" {
		t.Fatalf("expected REST-only sources, got %#v", bundle.Envelope.SourcesUsed)
	}
	if slackCalls != 0 {
		t.Fatalf("expected no Slack Web API calls without linkage, got %d", slackCalls)
	}
}

type agentRoundTripFunc func(*http.Request) (*http.Response, error)

func (f agentRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func agentJSONResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
