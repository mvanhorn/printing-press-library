package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/conduyt-crm/internal/cliutil/testenv"
)

// PATCH(slack-settings-typed-body-578): POST /integrations/slack/settings distinguishes an omitted notifyEvents (keep the stored list) from an
// explicit [] (clear every alert group) from a non-array (400, the API owns that rule), and an omitted channel keeps the stored destination.
// The CLI must therefore send exactly the fields the user passed, and an explicit empty list must reach the wire as [] and not be dropped.
func runSlackSettings(t *testing.T, stdin string, args ...string) (body map[string]any, hits int, err error) {
	t.Helper()
	testenv.Isolate(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.Method != http.MethodPost || r.URL.Path != "/integrations/slack/settings" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		raw, _ := io.ReadAll(r.Body)
		body = map[string]any{}
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Errorf("body is not JSON: %q", raw)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"success":true}}`))
	}))
	defer srv.Close()
	t.Setenv("CONDUYT_CRM_BASE_URL", srv.URL)
	t.Setenv("CONDUYT_CRM_BEARER_AUTH", "test-token")
	cmd := newIntegrationsPostSlackSettingsCmd(&rootFlags{noCache: true})
	cmd.SilenceUsage = true
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetIn(strings.NewReader(stdin))
	cmd.SetArgs(args)
	err = cmd.Execute()
	return
}

func TestSlackSettingsBodyIsExactlyTheFlagsPassed(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"nothing passed keeps both", nil, `{}`},
		{"channel only keeps events", []string{"--channel", "#alerts"}, `{"channel":"#alerts"}`},
		{"empty channel clears the destination", []string{"--channel", ""}, `{"channel":""}`},
		{"events comma list", []string{"--notify-events", "a,b"}, `{"notifyEvents":["a","b"]}`},
		{"events repeated", []string{"--notify-events", "a", "--notify-events", "b"}, `{"notifyEvents":["a","b"]}`},
		{"explicit empty list clears every event", []string{"--notify-events", ""}, `{"notifyEvents":[]}`},
		{"explicit empty list with = form", []string{"--notify-events="}, `{"notifyEvents":[]}`},
		{"both", []string{"--channel", "C123", "--notify-events", "a"}, `{"channel":"C123","notifyEvents":["a"]}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			body, hits, err := runSlackSettings(t, "", c.args...)
			if err != nil || hits != 1 {
				t.Fatalf("err=%v hits=%d", err, hits)
			}
			got, _ := json.Marshal(body)
			if string(got) != c.want {
				t.Errorf("body = %s, want %s", got, c.want)
			}
		})
	}
}

func TestSlackSettingsStdinStillPassesThroughUntouched(t *testing.T) {
	// a non-array notifyEvents must reach the API unchanged so the API can answer 400; the CLI does not rewrite it
	for _, in := range []string{`{"notifyEvents":[]}`, `{"channel":"x"}`, `{"notifyEvents":"a"}`, `{"notifyEvents":null}`} {
		t.Run(in, func(t *testing.T) {
			body, hits, err := runSlackSettings(t, in, "--stdin")
			if err != nil || hits != 1 {
				t.Fatalf("err=%v hits=%d", err, hits)
			}
			got, _ := json.Marshal(body)
			if string(got) != in {
				t.Errorf("stdin %s became %s", in, got)
			}
		})
	}
}

func TestSlackSettingsHelpStatesTheThreeSemantics(t *testing.T) {
	cmd := newIntegrationsPostSlackSettingsCmd(&rootFlags{})
	for _, want := range []string{"omitted keeps the stored alert groups", "turns every alert group off", "refused by the API with 400"} {
		if !strings.Contains(cmd.Long, want) {
			t.Errorf("Long is missing %q", want)
		}
	}
	f := cmd.Flags().Lookup("notify-events")
	if f == nil || !strings.Contains(f.Usage, "Omit to keep the stored list") || !strings.Contains(f.Usage, "turn all alert groups off") || !strings.Contains(f.Usage, "refused with 400") {
		t.Errorf("--notify-events usage = %+v", f)
	}
	if cmd.Flags().Lookup("channel") == nil || cmd.Flags().Lookup("stdin") == nil {
		t.Error("--channel or --stdin missing")
	}
}

func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
