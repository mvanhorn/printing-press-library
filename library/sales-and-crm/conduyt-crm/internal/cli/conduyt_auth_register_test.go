package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/conduyt-crm/internal/cliutil/testenv"
)

// PATCH(register-terms-accepted-601): POST /auth/register requires termsAccepted true. See .printing-press-patches/register-terms-accepted-601.json.
func runRegister(t *testing.T, stdin string, args ...string) (body map[string]any, hits int, err error) {
	t.Helper()
	testenv.Isolate(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.Method != http.MethodPost || r.URL.Path != "/auth/register" {
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
	cmd := newConduytAuthRegisterCmd(&rootFlags{noCache: true})
	cmd.SilenceUsage = true
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetIn(strings.NewReader(stdin))
	cmd.SetArgs(args)
	err = cmd.Execute()
	return
}

var registerBase = []string{"--email", "a@b.co", "--password", "pw123456", "--account-name", "Acme"}

func TestRegisterSendsTermsAcceptedTrue(t *testing.T) {
	for _, flag := range []string{"--terms-accepted", "--terms-accepted=true"} {
		body, hits, err := runRegister(t, "", append(append([]string{}, registerBase...), flag)...)
		if err != nil || hits != 1 {
			t.Fatalf("%s: err=%v hits=%d", flag, err, hits)
		}
		got, _ := json.Marshal(body)
		want := `{"accountName":"Acme","email":"a@b.co","password":"pw123456","termsAccepted":true}`
		if string(got) != want {
			t.Errorf("%s: body = %s, want %s", flag, got, want)
		}
	}
}

func TestRegisterWithoutTermsRefusesLocally(t *testing.T) {
	for _, extra := range [][]string{nil, {"--terms-accepted=false"}} {
		_, hits, err := runRegister(t, "", append(append([]string{}, registerBase...), extra...)...)
		if err == nil || hits != 0 {
			t.Fatalf("%v: err=%v hits=%d, want a local refusal and no request", extra, err, hits)
		}
		for _, want := range []string{"--terms-accepted", "Terms of Service", "Privacy Policy"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("%v: error %q lacks %q", extra, err, want)
			}
		}
	}
}

func TestRegisterPositionalArgumentIsRefusedWithoutARequest(t *testing.T) {
	// pflag parses `--terms-accepted false` as the bare flag (true) plus a positional "false": it must never create the account
	for _, extra := range [][]string{{"--terms-accepted", "false"}, {"--terms-accepted", "0"}, {"--terms-accepted", "no"}, {"--terms-accepted", "true"}, {"stray"}} {
		_, hits, err := runRegister(t, "", append(append([]string{}, registerBase...), extra...)...)
		if err == nil || hits != 0 {
			t.Fatalf("%v: err=%v hits=%d, want a local refusal and no request", extra, err, hits)
		}
		if !strings.Contains(err.Error(), "positional") || !strings.Contains(err.Error(), "--terms-accepted") {
			t.Errorf("%v: error %q does not name the flag syntax", extra, err)
		}
	}
}

func TestRegisterStdinPassesThroughUntouched(t *testing.T) {
	in := `{"email":"a@b.co","password":"pw123456","accountName":"Acme","termsAccepted":true}`
	body, hits, err := runRegister(t, in, "--stdin")
	if err != nil || hits != 1 {
		t.Fatalf("err=%v hits=%d", err, hits)
	}
	if got, _ := json.Marshal(body); string(got) != `{"accountName":"Acme","email":"a@b.co","password":"pw123456","termsAccepted":true}` {
		t.Errorf("stdin became %s", got)
	}
}

// The register operation must carry the same required const-true boolean in spec.yaml, tools-manifest.json and the command flags;
// a reprint from the old spec would drop the flag and every registration would answer 400.
func TestRegisterTermsAcceptedParityAcrossSpecManifestAndCommand(t *testing.T) {
	lines := strings.Split(readRepoFile(t, "spec.yaml"), "\n")
	start := -1
	for i, l := range lines {
		if l == "  /auth/register:" {
			start = i
			break
		}
	}
	if start < 0 {
		t.Fatal("spec.yaml: POST /auth/register missing")
	}
	var body []string
	in, has400 := false, false
	for _, l := range lines[start+1:] {
		if strings.TrimSpace(l) != "" && !strings.HasPrefix(l, "   ") {
			break // next path
		}
		switch strings.TrimSpace(l) {
		case "requestBody:":
			in = true
			continue
		case "responses:":
			in = false
		case "'400':", "\"400\":":
			has400 = true
		}
		if in {
			body = append(body, strings.TrimSpace(l))
		}
	}
	if !containsInOrder(body, []string{"required:", "- email", "- password", "- accountName", "- termsAccepted", "properties:"}) ||
		!containsInOrder(body, []string{"termsAccepted:", "type: boolean", "const: true"}) {
		t.Errorf("spec.yaml requestBody does not require termsAccepted as a const-true boolean:\n%s", strings.Join(body, "\n"))
	}
	if !has400 {
		t.Error("spec.yaml register operation lacks a 400 response")
	}

	var manifest any
	if err := json.Unmarshal([]byte(readRepoFile(t, "tools-manifest.json")), &manifest); err != nil {
		t.Fatal(err)
	}
	tool := findNamedObject(manifest, "conduyt-auth_register")
	if tool == nil {
		t.Fatal("tools-manifest.json: conduyt-auth_register missing")
	}
	var param map[string]any
	params, _ := tool["params"].([]any)
	for _, p := range params {
		if pm, _ := p.(map[string]any); pm["name"] == "termsAccepted" {
			param = pm
		}
	}
	if param == nil || param["type"] != "boolean" || param["location"] != "body" || param["required"] != true {
		t.Errorf("tools-manifest.json termsAccepted param = %#v, want a required boolean body param", param)
	}

	cmd := newConduytAuthRegisterCmd(&rootFlags{})
	f := cmd.Flags().Lookup("terms-accepted")
	if f == nil || f.Value.Type() != "bool" || !strings.Contains(f.Usage, "Required") || !strings.Contains(cmd.Long, "requires --terms-accepted") {
		t.Errorf("--terms-accepted flag or Long missing: %+v", f)
	}
}
