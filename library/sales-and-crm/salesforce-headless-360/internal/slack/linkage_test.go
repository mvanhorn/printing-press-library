package slack

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

type fakeSOQLClient struct {
	body json.RawMessage
	q    string
}

func (c *fakeSOQLClient) Get(path string, params map[string]string) (json.RawMessage, error) {
	if path != QueryPath {
		panic("unexpected path: " + path)
	}
	c.q = params["q"]
	return c.body, nil
}

func TestLinkageFetchHappyPath(t *testing.T) {
	c := &fakeSOQLClient{body: json.RawMessage(`{"totalSize":1,"done":true,"records":[{"EntityId":"001ACME0001","SlackChannelId":"C012ACME","WorkspaceId":"T012ACME"}]}`)}

	result, err := LinkageFetch(context.Background(), c, "001ACME0001", nil)
	if err != nil {
		t.Fatalf("LinkageFetch: %v", err)
	}
	if !result.Availability.Available {
		t.Fatalf("expected available: %+v", result.Availability)
	}
	if len(result.Rows) != 1 || result.Rows[0].ChannelID != "C012ACME" || result.Rows[0].WorkspaceID != "T012ACME" {
		t.Fatalf("unexpected rows: %#v", result.Rows)
	}
	if !strings.Contains(c.q, "SlackConversationRelation") || !strings.Contains(c.q, "'001ACME0001'") {
		t.Fatalf("unexpected query: %s", c.q)
	}
}

func TestLinkageFetchNoRowsUnavailable(t *testing.T) {
	c := &fakeSOQLClient{body: json.RawMessage(`{"totalSize":0,"done":true,"records":[]}`)}

	result, err := LinkageFetch(context.Background(), c, "001ACME0001", nil)
	if err != nil {
		t.Fatalf("LinkageFetch: %v", err)
	}
	if result.Availability.Available || result.Availability.Reason != "no_rows" {
		t.Fatalf("expected no_rows unavailable: %+v", result.Availability)
	}
	if len(result.Rows) != 0 {
		t.Fatalf("expected no rows: %#v", result.Rows)
	}
}

func TestLinkageFetchSupportsFixtureFieldNames(t *testing.T) {
	c := &fakeSOQLClient{body: json.RawMessage(`{"envelope":{"totalSize":1,"records":[{"RelatedRecordId":"001ACME0001","SlackConversationId":"C012ACME","SlackWorkspaceId":"T012ACME"}]}}`)}

	result, err := LinkageFetch(context.Background(), c, "001ACME0001", nil)
	if err != nil {
		t.Fatalf("LinkageFetch: %v", err)
	}
	if len(result.Rows) != 1 || result.Rows[0].ChannelID != "C012ACME" {
		t.Fatalf("unexpected fixture rows: %#v", result.Rows)
	}
}
