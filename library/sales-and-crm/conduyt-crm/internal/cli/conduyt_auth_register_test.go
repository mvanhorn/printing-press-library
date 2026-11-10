package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// POST /auth/register requires termsAccepted true (board #601). PATCH(register-terms-accepted-601): see .printing-press-patches.json.
func runRegister(t *testing.T, stdin string, args ...string) (body map[string]any, hits int, err error) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.Method != "POST" || r.URL.Path != "/api/v1/auth/register" {
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
	t.Setenv("CONDUYT_CRM_BASE_URL", srv.URL+"/api/v1")
	t.Setenv("CONDUYT_CRM_BEARER_AUTH", "test-token")
	t.Setenv("CONDUYT_CRM_CONFIG", filepath.Join(t.TempDir(), "config.toml"))
	cmd := newConduytAuthRegisterCmd(&rootFlags{noCache: true})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetIn(strings.NewReader(stdin))
	cmd.SetArgs(args)
	err = cmd.Execute()
	return
}

var registerBase = []string{"--email", "a@b.co", "--password", "pw123456", "--account-name", "Acme"}

func TestRegisterSendsTermsAcceptedTrue(t *testing.T) {
	body, hits, err := runRegister(t, "", append(append([]string{}, registerBase...), "--terms-accepted")...)
	if err != nil || hits != 1 {
		t.Fatalf("err=%v hits=%d", err, hits)
	}
	got, _ := json.Marshal(body)
	want := `{"accountName":"Acme","email":"a@b.co","password":"pw123456","termsAccepted":true}`
	if string(got) != want {
		t.Errorf("body = %s, want %s", got, want)
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

func TestRegisterHelpAndManifestStateTheTerms(t *testing.T) {
	cmd := newConduytAuthRegisterCmd(&rootFlags{})
	f := cmd.Flags().Lookup("terms-accepted")
	if f == nil || !strings.Contains(f.Usage, "Required") || !strings.Contains(cmd.Long, "requires --terms-accepted") {
		t.Errorf("--terms-accepted flag or Long missing: %+v", f)
	}
	raw, err := readRepoFile("tools-manifest.json")
	if err != nil || !strings.Contains(raw, `"name": "termsAccepted"`) {
		t.Errorf("tools-manifest.json lacks termsAccepted (err=%v)", err)
	}
}

func registerRequestBody(t *testing.T, file string) map[string]any {
	t.Helper()
	raw, err := readRepoFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("%s: %v", file, err)
	}
	op, ok := doc.Paths["/auth/register"]["post"].(map[string]any)
	if !ok {
		t.Fatalf("%s: POST /auth/register missing", file)
	}
	return op["requestBody"].(map[string]any)
}

func TestRegisterRequestBodyMatchesBetweenSpecAndUpstream(t *testing.T) {
	spec := registerRequestBody(t, "spec.yaml")
	upstream := registerRequestBody(t, "upstream/openapi.yaml")
	if !reflect.DeepEqual(spec, upstream) {
		t.Fatalf("requestBody differs between spec.yaml and upstream/openapi.yaml")
	}
	schema := spec["content"].(map[string]any)["application/json"].(map[string]any)["schema"].(map[string]any)
	req, _ := schema["required"].([]any)
	found := false
	for _, r := range req {
		if r == "termsAccepted" {
			found = true
		}
	}
	props, _ := schema["properties"].(map[string]any)
	ta, _ := props["termsAccepted"].(map[string]any)
	if !found || ta["type"] != "boolean" || ta["const"] != true {
		t.Errorf("termsAccepted is not a required const-true boolean: required=%v prop=%#v", req, ta)
	}
}
