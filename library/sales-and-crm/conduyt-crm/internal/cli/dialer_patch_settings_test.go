package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// PATCH /dialer/settings (app main ce2d3cdff, 2026-10-15): the typed body now carries wrapupAutoReleaseSeconds (0 = off, otherwise 30..3600).
// spec.yaml holds the typed schema, so the generated command takes --wrapup-auto-release-seconds. The API owns the 0-or-30..3600 rule (400).
func runDialerPatchSettings(t *testing.T, args ...string) (body map[string]any, hits int) {
	t.Helper()
	body, hits, err := runDialerPatchSettingsErr(t, "", args...)
	if err != nil {
		t.Fatalf("dialer patch-settings %v: %v", args, err)
	}
	return
}

// stdin non-empty: the args should include --stdin; the body is read from it.
func runDialerPatchSettingsErr(t *testing.T, stdin string, args ...string) (body map[string]any, hits int, err error) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.Method != "PATCH" || r.URL.Path != "/api/v1/dialer/settings" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		raw, _ := io.ReadAll(r.Body)
		body = map[string]any{}
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Errorf("body is not JSON: %q", raw)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{}}`))
	}))
	defer srv.Close()
	t.Setenv("CONDUYT_CRM_BASE_URL", srv.URL+"/api/v1")
	t.Setenv("CONDUYT_CRM_BEARER_AUTH", "test-token")
	t.Setenv("CONDUYT_CRM_CONFIG", filepath.Join(t.TempDir(), "config.toml"))
	cmd := newDialerPatchSettingsCmd(&rootFlags{noCache: true})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetIn(strings.NewReader(stdin))
	cmd.SetArgs(args)
	err = cmd.Execute()
	return
}

func TestDialerPatchSettingsWrapupSeconds(t *testing.T) {
	for _, v := range []string{"0", "30", "300", "3600"} {
		body, hits := runDialerPatchSettings(t, "--wrapup-auto-release-seconds", v)
		if hits != 1 {
			t.Fatalf("%s: %d requests", v, hits)
		}
		want := map[string]any{"0": 0.0, "30": 30.0, "300": 300.0, "3600": 3600.0}[v]
		if len(body) != 1 || body["wrapupAutoReleaseSeconds"] != want {
			t.Errorf("%s: body = %v, want only wrapupAutoReleaseSeconds=%v (0 must be sent, not dropped)", v, body, want)
		}
	}
}

func TestDialerPatchSettingsWrapupRidesWithTheOtherFields(t *testing.T) {
	body, _ := runDialerPatchSettings(t, "--pause-seconds", "10", "--advance-on", "timer", "--wrapup-auto-release-seconds", "120")
	if body["pauseSeconds"] != 10.0 || body["advanceOn"] != "timer" || body["wrapupAutoReleaseSeconds"] != 120.0 || len(body) != 3 {
		t.Errorf("body = %v", body)
	}
}

func TestDialerPatchSettingsWrapupFlagIsDocumented(t *testing.T) {
	f := newDialerPatchSettingsCmd(&rootFlags{}).Flags().Lookup("wrapup-auto-release-seconds")
	if f == nil {
		t.Fatal("flag missing")
	}
	if f.Usage != "Seconds of continuous wrap-up after which a rep returns to Available automatically: 0 = off, otherwise 30 to 3600 (default 300)" {
		t.Errorf("usage = %q", f.Usage)
	}
}

func TestToolsManifestCarriesWrapupSeconds(t *testing.T) {
	raw, err := os.ReadFile("../../tools-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Tools []struct {
			Name   string                                  `json:"name"`
			Params []struct{ Name, Type, Location string } `json:"params"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	for _, tool := range m.Tools {
		if tool.Name != "dialer_patch-settings" {
			continue
		}
		for _, p := range tool.Params {
			if p.Name == "wrapupAutoReleaseSeconds" && p.Type == "int" && p.Location == "body" {
				return
			}
		}
	}
	t.Error("dialer_patch-settings has no int body param wrapupAutoReleaseSeconds in tools-manifest.json")
}

// Hand-fix: the raw decimal text is validated before any request, on the flag path and on the --stdin path.
func TestDialerPatchSettingsWrapupInvalidFlagValuesSendNothing(t *testing.T) {
	for _, v := range []string{"29", "1", "3601", "99999", "100000", "-1", "-30", "12.5", "30.0", "1e2", "0x1e", "0X1E", "0b11", "030x", "+30", "", "  ", "abc", "3 0"} {
		_, hits, err := runDialerPatchSettingsErr(t, "", "--wrapup-auto-release-seconds", v)
		if err == nil {
			t.Errorf("%q: no error", v)
		} else if ExitCode(err) != 2 {
			t.Errorf("%q: exit code %d, want 2 (usage): %v", v, ExitCode(err), err)
		}
		if hits != 0 {
			t.Errorf("%q: %d requests sent", v, hits)
		}
	}
}

func TestDialerPatchSettingsWrapupStdinValidation(t *testing.T) {
	for _, v := range []string{"29", "3601", "-1", "12.5", "1e2", "1E2", "3e1", "30.0", `"30"`, `""`, "null", "true", "[30]", "0x1e"} {
		_, hits, err := runDialerPatchSettingsErr(t, `{"wrapupAutoReleaseSeconds": `+v+`}`, "--stdin")
		if err == nil {
			t.Errorf("stdin %s: no error", v)
		} else if ExitCode(err) != 2 {
			t.Errorf("stdin %s: exit code %d, want 2 (usage): %v", v, ExitCode(err), err)
		}
		if hits != 0 {
			t.Errorf("stdin %s: %d requests sent", v, hits)
		}
	}
	for _, v := range []string{"0", "30", "300", "3600"} {
		body, hits, err := runDialerPatchSettingsErr(t, `{"pauseSeconds": 5, "wrapupAutoReleaseSeconds": `+v+`}`, "--stdin")
		want, _ := strconv.Atoi(v)
		if err != nil || hits != 1 || body["wrapupAutoReleaseSeconds"] != float64(want) || body["pauseSeconds"] != 5.0 {
			t.Errorf("stdin %s: err=%v hits=%d body=%v", v, err, hits, body)
		}
	}
	// a body without the field is passed through untouched
	body, hits, err := runDialerPatchSettingsErr(t, `{"pauseSeconds": 7}`, "--stdin")
	if err != nil || hits != 1 || len(body) != 1 || body["pauseSeconds"] != 7.0 {
		t.Errorf("no-field stdin: err=%v hits=%d body=%v", err, hits, body)
	}
}

// Round 3: a malformed stdin body is a usage error (exit 2) before any client or request, not a late parse failure.
func TestDialerPatchSettingsMalformedStdinIsUsageErrorBeforeAnyRequest(t *testing.T) {
	for _, in := range []string{`{"wrapupAutoReleaseSeconds":0x1e}`, `{"wrapupAutoReleaseSeconds":}`, `{"wrapupAutoReleaseSeconds":30`, `not json`, ``, `[30]`} {
		_, hits, err := runDialerPatchSettingsErr(t, in, "--stdin")
		if err == nil {
			t.Errorf("stdin %q: no error", in)
		} else if ExitCode(err) != 2 {
			t.Errorf("stdin %q: exit code %d, want 2 (usage): %v", in, ExitCode(err), err)
		}
		if hits != 0 {
			t.Errorf("stdin %q: %d requests sent", in, hits)
		}
	}
}
