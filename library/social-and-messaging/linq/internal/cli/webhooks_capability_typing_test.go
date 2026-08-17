package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWebhooksAddEventReadModifyWritesFullEventArray(t *testing.T) {
	var putBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v3/webhook-events":
			_, _ = io.WriteString(w, `{"events":["message.received","chat.typing_indicator.started","chat.typing_indicator.stopped"]}`)
		case "/v3/webhook-subscriptions/sub_123":
			switch r.Method {
			case http.MethodGet:
				_, _ = io.WriteString(w, `{"id":"sub_123","target_url":"https://example.com/api/linq/inbound","subscribed_events":["message.received"]}`)
			case http.MethodPut:
				if err := json.NewDecoder(r.Body).Decode(&putBody); err != nil {
					t.Fatalf("decode PUT body: %v", err)
				}
				_, _ = io.WriteString(w, `{"id":"sub_123","subscribed_events":["message.received","chat.typing_indicator.started","chat.typing_indicator.stopped"]}`)
			default:
				t.Fatalf("unexpected method %s", r.Method)
			}
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	t.Setenv("LINQ_BASE_URL", server.URL)
	t.Setenv("LINQ_API_KEY", "test-token")

	out, err := runCommandWithIO(nil, "webhooks", "add-event", "sub_123", "chat.typing_indicator.started", "chat.typing_indicator.stopped", "--json")
	if err != nil {
		t.Fatalf("webhooks add-event failed: %v\n%s", err, out)
	}
	events := stringSliceFromAny(putBody["subscribed_events"])
	want := []string{"chat.typing_indicator.started", "chat.typing_indicator.stopped", "message.received"}
	if !sameStringSet(events, want) {
		t.Fatalf("PUT subscribed_events = %#v, want %#v", events, want)
	}
	if !strings.Contains(string(out), webhookUpdateEventSemantics) {
		t.Fatalf("output should document update semantics, got %s", out)
	}
}

func TestCapabilityCheckRoutesWithoutEchoingAddress(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode capability body: %v", err)
		}
		handleCheck, ok := body["handle_check"].(map[string]any)
		if !ok || handleCheck["address"] != "+15551234567" {
			t.Fatalf("unexpected capability body %#v", body)
		}
		switch r.URL.Path {
		case "/v3/capability/check_imessage":
			_, _ = io.WriteString(w, `{"address":"+15551234567","available":false}`)
		case "/v3/capability/check_rcs":
			_, _ = io.WriteString(w, `{"address":"+15551234567","available":true}`)
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	t.Setenv("LINQ_BASE_URL", server.URL)
	t.Setenv("LINQ_API_KEY", "test-token")

	out, err := runCommandWithIO(nil, "capability", "check", "+15551234567", "--json")
	if err != nil {
		t.Fatalf("capability check failed: %v\n%s", err, out)
	}
	if strings.Contains(string(out), "+15551234567") {
		t.Fatalf("capability output leaked full address: %s", out)
	}
	if !strings.Contains(string(out), `"channel": "rcs"`) {
		t.Fatalf("expected rcs route, got %s", out)
	}
}

func TestCapabilityCheckMarksRCSNotCheckedForIMessage(t *testing.T) {
	rcsCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v3/capability/check_imessage":
			_, _ = io.WriteString(w, `{"available":true}`)
		case "/v3/capability/check_rcs":
			rcsCalled = true
			_, _ = io.WriteString(w, `{"available":true}`)
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	t.Setenv("LINQ_BASE_URL", server.URL)
	t.Setenv("LINQ_API_KEY", "test-token")

	out, err := runCommandWithIO(nil, "capability", "check", "+15551234567", "--json")
	if err != nil {
		t.Fatalf("capability check failed: %v\n%s", err, out)
	}
	if rcsCalled {
		t.Fatalf("RCS should not be queried when iMessage is already available")
	}
	if !strings.Contains(string(out), `"rcs_checked": false`) || !strings.Contains(string(out), `"rcs_available": null`) {
		t.Fatalf("expected RCS to be marked not checked, got %s", out)
	}
}

func TestWebhooksRemoveEventRefusesToEmptySubscription(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v3/webhook-events":
			_, _ = io.WriteString(w, `{"events":["message.received"]}`)
		case "/v3/webhook-subscriptions/sub_123":
			if r.Method == http.MethodPut {
				t.Fatalf("remove-event should not PUT an empty subscribed_events array")
			}
			_, _ = io.WriteString(w, `{"id":"sub_123","subscribed_events":["message.received"]}`)
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	t.Setenv("LINQ_BASE_URL", server.URL)
	t.Setenv("LINQ_API_KEY", "test-token")

	out, err := runCommandWithIO(nil, "webhooks", "remove-event", "sub_123", "message.received", "--json")
	if err == nil {
		t.Fatalf("expected remove-event to refuse empty set, got %s", out)
	}
	if !strings.Contains(err.Error(), "subscribed_events empty") {
		t.Fatalf("expected empty-set error, got %v", err)
	}
}

func TestTypingWatchFiltersCapturedTypingEvents(t *testing.T) {
	input := strings.NewReader(strings.Join([]string{
		`{"event":"message.received","data":{"chat_id":"ch_skip"}}`,
		`{"event":"chat.typing_indicator.started","data":{"chat_id":"ch_123"}}`,
		`{"type":"chat.typing_indicator.stopped","chat_id":"ch_123"}`,
	}, "\n"))
	out, err := runCommandWithIO(input, "typing", "watch", "--json")
	if err != nil {
		t.Fatalf("typing watch failed: %v\n%s", err, out)
	}
	if strings.Contains(string(out), "ch_skip") {
		t.Fatalf("typing watch should filter non-typing events: %s", out)
	}
	if strings.Count(string(out), "chat.typing_indicator.") != 2 {
		t.Fatalf("expected two typing events, got %s", out)
	}
}

func runCommandWithIO(stdin io.Reader, args ...string) ([]byte, error) {
	cmd := RootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if stdin != nil {
		cmd.SetIn(stdin)
	}
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.Bytes(), err
}

func stringSliceFromAny(value any) []string {
	items, _ := value.([]any)
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.(string))
	}
	return out
}
