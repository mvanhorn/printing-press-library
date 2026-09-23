package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestCampaignCreatePreservesNestedRequestBody(t *testing.T) {
	expected := map[string]any{
		"data": map[string]any{
			"type": "campaign",
			"attributes": map[string]any{
				"name": "Contract test",
				"audiences": map[string]any{
					"included": []any{"list-1"},
					"excluded": []any{},
				},
				"campaign-messages": map[string]any{
					"email": map[string]any{"template_id": "template-1"},
				},
				"send_options": map[string]any{"use_smart_sending": false},
			},
		},
	}

	server := requestBodyServer(t, "/api/campaigns", expected)
	defer server.Close()
	t.Setenv("KLAVIYO_BASE_URL", server.URL)
	t.Setenv("KLAVIYO_API_KEY", "test-key")

	flags := rootFlags{asJSON: true}
	cmd := newCampaignsCreateCmd(&flags)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		"--data-type", "campaign",
		"--data-attributes-name", "Contract test",
		"--data-attributes-audiences", `{"included":["list-1"],"excluded":[]}`,
		"--data-attributes-campaign-messages", `{"email":{"template_id":"template-1"}}`,
		"--data-attributes-send-options", `{"use_smart_sending":false}`,
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute campaigns create: %v", err)
	}
}

func TestCampaignMessageAssignTemplatePreservesNestedRequestBody(t *testing.T) {
	expected := map[string]any{
		"data": map[string]any{
			"id":   "message-1",
			"type": "campaign-message",
			"relationships": map[string]any{
				"template": map[string]any{
					"data": map[string]any{
						"id":   "template-1",
						"type": "template",
					},
				},
			},
		},
	}

	server := requestBodyServer(t, "/api/campaign-message-assign-template", expected)
	defer server.Close()
	t.Setenv("KLAVIYO_BASE_URL", server.URL)
	t.Setenv("KLAVIYO_API_KEY", "test-key")

	flags := rootFlags{asJSON: true}
	cmd := newCampaignMessageAssignTemplatePromotedCmd(&flags)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		"--data-id", "message-1",
		"--data-type", "campaign-message",
		"--data-relationships-template", `{"data":{"id":"template-1","type":"template"}}`,
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute campaign-message-assign-template: %v", err)
	}
}

func requestBodyServer(t *testing.T, expectedPath string, expectedBody map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != expectedPath {
			t.Errorf("path = %s, want %s", r.URL.Path, expectedPath)
		}
		var got map[string]any
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decode request body: %v", err)
		} else if !reflect.DeepEqual(got, expectedBody) {
			t.Errorf("request body mismatch\ngot:  %s\nwant: %s", mustJSON(got), mustJSON(expectedBody))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"id":"ok","type":"campaign"}}`))
	}))
}

func mustJSON(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("<marshal error: %v>", err)
	}
	return string(data)
}
