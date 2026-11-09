package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestCampaignPathsBlockKitFooterBeforeWrite(t *testing.T) {
	posts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodGet {
			posts++
			fmt.Fprint(w, `{"data":{"id":"ok"}}`)
			return
		}
		switch r.URL.Path {
		case "/api/templates/template-1", "/api/campaign-messages/message-1/template":
			fmt.Fprint(w, `{"data":{"id":"template-1","type":"template","attributes":{"html":"<a href=\"{{ unsubscribe_url }}\">Unsubscribe</a>"}}}`)
		case "/api/campaigns/campaign-1/campaign-messages":
			fmt.Fprint(w, `{"data":[{"id":"message-1","type":"campaign-message"}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv("KLAVIYO_BASE_URL", server.URL)
	t.Setenv("KLAVIYO_API_KEY", "test-key")
	previous := registeredPlatformSource
	t.Cleanup(func() { registeredPlatformSource = previous })
	registeredPlatformSource = nil

	t.Run("assign template", func(t *testing.T) {
		cmd := newCampaignMessageAssignTemplatePromotedCmd(&rootFlags{asJSON: true})
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs([]string{"--data-id", "message-1", "--data-type", "campaign-message", "--data-relationships-template", `{"data":{"id":"template-1","type":"template"}}`})
		if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "campaign compliance preflight failed") {
			t.Fatalf("error=%v", err)
		}
	})
	t.Run("send campaign", func(t *testing.T) {
		body, err := os.CreateTemp(t.TempDir(), "send-body-*.json")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := body.WriteString(`{"data":{"id":"campaign-1","type":"campaign-send-job"}}`); err != nil {
			t.Fatal(err)
		}
		if _, err := body.Seek(0, 0); err != nil {
			t.Fatal(err)
		}
		old := os.Stdin
		os.Stdin = body
		defer func() { os.Stdin = old; body.Close() }()
		cmd := newCampaignSendJobsSendCampaignCmd(&rootFlags{asJSON: true})
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs([]string{"--stdin"})
		if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "campaign compliance preflight failed") {
			t.Fatalf("error=%v", err)
		}
	})
	if posts != 0 {
		t.Fatalf("made %d write requests before compliance passed", posts)
	}
}

func TestCampaignComplianceChecksEveryEmailMessageAndPage(t *testing.T) {
	var seen []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.URL.RequestURI())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/campaigns/campaign-1/campaign-messages":
			if r.URL.Query().Get("page[cursor]") == "next" {
				fmt.Fprint(w, `{"data":[{"id":"message-2","attributes":{"definition":{"channel":"email"}}}]}`)
			} else {
				fmt.Fprintf(w, `{"data":[{"id":"message-1","attributes":{"definition":{"channel":"email"}}}],"links":{"next":%q}}`, serverURLForRequest(r)+"/api/campaigns/campaign-1/campaign-messages?page[cursor]=next")
			}
		case "/api/campaign-messages/message-1/template":
			if r.URL.Query().Get("fields[template]") != "html" {
				t.Errorf("unsupported field query: %s", r.URL.RawQuery)
			}
			fmt.Fprint(w, `{"data":{"id":"template-1","attributes":{"html":"<a href=\"{% unsubscribe_link %}\">Unsubscribe</a>"}}}`)
		case "/api/campaign-messages/message-2/template":
			fmt.Fprint(w, `{"data":{"id":"template-2","attributes":{"html":"<a href=\"{{ unsubscribe_url }}\">Unsubscribe</a>"}}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv("KLAVIYO_BASE_URL", server.URL)
	t.Setenv("KLAVIYO_API_KEY", "test-key")
	previous := registeredPlatformSource
	t.Cleanup(func() { registeredPlatformSource = previous })
	registeredPlatformSource = nil
	c, err := (&rootFlags{}).newClient()
	if err != nil {
		t.Fatal(err)
	}
	if err := requireCampaignCompliance(c, "campaign-1"); err == nil || !strings.Contains(err.Error(), "message-2") {
		t.Fatalf("expected second message to fail, got %v; requests=%v", err, seen)
	}
}

func serverURLForRequest(r *http.Request) string { return "http://" + r.Host }

func TestTemplateWritePathsWarnInJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"id":"template-1","type":"template"}}`)
	}))
	defer server.Close()
	t.Setenv("KLAVIYO_BASE_URL", server.URL)
	t.Setenv("KLAVIYO_API_KEY", "test-key")
	previous := registeredPlatformSource
	t.Cleanup(func() { registeredPlatformSource = previous })
	registeredPlatformSource = nil
	for _, tc := range []struct {
		name    string
		makeCmd func(*rootFlags) *cobra.Command
		args    []string
		body    string
	}{
		{"create", newTemplatesCreateCmd, []string{"--stdin"}, `{"data":{"type":"template","attributes":{"html":"<a href=\"{{ unsubscribe_url }}\">Unsubscribe</a>"}}}`},
		{"update", newTemplatesUpdateCmd, []string{"template-1", "--stdin"}, `{"data":{"type":"template","attributes":{"html":"<a href=\"{{ unsubscribe_url }}\">Unsubscribe</a>"}}}`},
		{"clone", newTemplateClonePromotedCmd, []string{"--template-id", "template-1"}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			old := os.Stdin
			if tc.body != "" {
				file, err := os.CreateTemp(t.TempDir(), "body-*.json")
				if err != nil {
					t.Fatal(err)
				}
				file.WriteString(tc.body)
				file.Seek(0, 0)
				os.Stdin = file
				defer func() { os.Stdin = old; file.Close() }()
			}
			var stdout, stderr bytes.Buffer
			cmd := tc.makeCmd(&rootFlags{asJSON: true})
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)
			cmd.SetArgs(tc.args)
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			var out map[string]any
			if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
				t.Fatal(err)
			}
			if len(out["warnings"].([]any)) == 0 || !strings.Contains(stderr.String(), "warning:") {
				t.Fatalf("stdout=%s stderr=%s", stdout.String(), stderr.String())
			}
		})
	}
}

func TestTemplateCloneReadsSourceAndSendsValidBody(t *testing.T) {
	var posted map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			if r.URL.Path != "/api/templates/template-1" || r.URL.Query().Get("fields[template]") != "html" {
				t.Errorf("unexpected source request: %s", r.URL.String())
			}
			fmt.Fprint(w, `{"data":{"id":"template-1","attributes":{"html":"<a href=\"{% unsubscribe_link %}\">Unsubscribe</a>"}}}`)
			return
		}
		if r.URL.Path != "/api/template-clone" {
			t.Errorf("unexpected write: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&posted); err != nil {
			t.Error(err)
		}
		fmt.Fprint(w, `{"data":{"id":"template-2","type":"template"}}`)
	}))
	defer server.Close()
	t.Setenv("KLAVIYO_BASE_URL", server.URL)
	t.Setenv("KLAVIYO_API_KEY", "test-key")
	previous := registeredPlatformSource
	t.Cleanup(func() { registeredPlatformSource = previous })
	registeredPlatformSource = nil
	var stdout, stderr bytes.Buffer
	cmd := newTemplateClonePromotedCmd(&rootFlags{asJSON: true})
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--template-id", "template-1", "--name", "Clone"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stdout.String(), "warnings") || strings.Contains(stderr.String(), "warning:") {
		t.Fatalf("valid source unexpectedly warned: stdout=%s stderr=%s", stdout.String(), stderr.String())
	}
	if stringFromMapPath(posted, "data.id") != "template-1" || stringFromMapPath(posted, "data.type") != "template" || stringFromMapPath(posted, "data.attributes.name") != "Clone" {
		t.Fatalf("invalid clone request: %#v", posted)
	}
}

func TestReplacePathParamEscapesSegments(t *testing.T) {
	if got := replacePathParam("/api/templates/{id}", "id", "a/b"); got != "/api/templates/a%2Fb" {
		t.Fatalf("got %q", got)
	}
	if got := replacePathParam("/api/templates/{id}", "id", ".."); got != "/api/templates/%2E%2E" {
		t.Fatalf("got %q", got)
	}
}
