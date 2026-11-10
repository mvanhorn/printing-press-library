package cli

// PATCH(reprint-637-required-on-changed): every REQUIRED scalar body field in tools-manifest.json, passed with its zero
// value ("" / 0 / 0.0 / false), appears in the body the command would send. The table is the manifest itself (one row
// per tool x required body param), so a reprint that brings back the generator's `!= ""` / `!= 0` guard fails here
// until .printing-press-patches/required-on-changed.pl is run again.

import (
	"encoding/json"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/conduyt-crm/internal/cliutil"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"unicode"

	"github.com/spf13/cobra"
)

type manifestParam struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Location string `json:"location"`
	Required bool   `json:"required"`
}

type manifestTool struct {
	Name   string          `json:"name"`
	Method string          `json:"method"`
	Path   string          `json:"path"`
	Params []manifestParam `json:"params"`
}

func kebabFlagName(param string) string {
	var b strings.Builder
	for i, r := range param {
		switch {
		case r == '_':
			b.WriteRune('-')
		case unicode.IsUpper(r):
			if i > 0 {
				b.WriteRune('-')
			}
			b.WriteRune(unicode.ToLower(r))
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// previewBody returns the top-level keys of the JSON body in a dry-run preview.
func previewBody(preview string) (map[string]any, bool) {
	_, rest, ok := strings.Cut(preview, "  Body:\n")
	if !ok {
		return nil, false
	}
	var body map[string]any
	if err := json.NewDecoder(strings.NewReader(rest)).Decode(&body); err != nil {
		return nil, false
	}
	return body, true
}

// requiredZeroValueExceptions: required body flags whose zero value is refused locally on purpose (each by its own
// ledger entry), so it is never serialized. Every other required scalar body flag must reach the body.
var requiredZeroValueExceptions = map[string]string{
	"conduyt-auth register --terms-accepted":                 "register-terms-accepted-601: --terms-accepted=false is a refusal",
	"contacts duplicate-proposals decide-contact --decision": "duplicate-proposals-look-board-95: decision must be one of not_sure, different, merge",
	"email send --to":         "email-send-live-keys-cli-324: hand-written required set (to, contact-id, subject) checked together",
	"email send --contact-id": "email-send-live-keys-cli-324: hand-written required set",
	"email send --subject":    "email-send-live-keys-cli-324: hand-written required set",
	// gocli1219: the two multipart file paths (call-flows post-audio --file, users avatar post-users-id --avatar) are no
	// longer exceptions: under 4.34.2 the --dry-run preview opens no file (it shows "@<path>"), so their zero value is
	// serialized like any other required flag; a real call still fails locally on the empty path.
}

// dryRunBody runs the command under the global --dry-run with extra flags and returns the previewed JSON body as text.
func dryRunBody(t *testing.T, cmd *cobra.Command, extra ...string) (string, error) {
	t.Helper()
	// gocli1219: under Printing Press 4.34.2 a user's --dry-run still enforces the OTHER required flags (only the
	// verify / dogfood harnesses waive them, requiredInputWaived). This probe sends one flag at a time, so it runs
	// as the dogfood harness does.
	t.Setenv(cliutil.DogfoodEnvVar, "1")
	root := newRootCmd(&rootFlags{})
	args := strings.Fields(strings.TrimPrefix(cmd.CommandPath(), root.Name()+" "))
	for i := 0; i < strings.Count(cmd.Use, "<"); i++ {
		args = append(args, "00000000-0000-4000-8000-00000000000"+string(rune('1'+i)))
	}
	args = append(append(args, extra...), "--dry-run", "--no-cache", "--yes")
	root.SetArgs(args)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	var runErr error
	preview := captureStderr(t, func() { runErr = root.Execute() })
	if runErr != nil {
		return "", runErr
	}
	body, ok := previewBody(preview)
	if !ok {
		return "", nil
	}
	b, _ := json.Marshal(body)
	return string(b), nil
}

func TestEveryRequiredBodyFlagSerializesItsZeroValue(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "tools-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Tools []manifestTool `json:"tools"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	byOp := map[string]*cobra.Command{}
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		if m, p := c.Annotations["pp:method"], c.Annotations["pp:path"]; m != "" && p != "" {
			if _, seen := byOp[m+" "+p]; !seen {
				byOp[m+" "+p] = c
			}
		}
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(newRootCmd(&rootFlags{}))

	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":{}}`)
	}))
	defer srv.Close()
	t.Setenv("CONDUYT_CRM_BASE_URL", srv.URL)
	t.Setenv("CONDUYT_CRM_BEARER_AUTH", "test-token")
	t.Setenv("CONDUYT_CRM_CONFIG", filepath.Join(t.TempDir(), "config.toml"))
	t.Setenv("CONDUYT_CRM_NO_LEARN", "true")
	t.Setenv("HOME", t.TempDir())

	zero := map[string]string{"string": "", "int": "0", "integer": "0", "number": "0", "float": "0", "bool": "false", "boolean": "false"}
	checked, skipped := 0, 0
	var unmapped []string
	for _, tool := range manifest.Tools {
		cmd := byOp[tool.Method+" "+tool.Path]
		if cmd == nil || tool.Method == "GET" {
			continue
		}
		for _, p := range tool.Params {
			z, scalar := zero[p.Type]
			if !p.Required || p.Location != "body" || !scalar || strings.Contains(p.Name, ".") {
				continue
			}
			flagName := kebabFlagName(p.Name)
			if cmd.Flags().Lookup(flagName) == nil {
				unmapped = append(unmapped, tool.Name+"."+p.Name)
				continue
			}
			key := cmd.CommandPath() + " --" + flagName
			if _, skip := requiredZeroValueExceptions[strings.TrimPrefix(key, "conduyt-crm-pp-cli ")]; skip {
				// an exception never drops the value silently: it refuses locally, also under --dry-run
				if _, err := dryRunBody(t, cmd, "--"+flagName+"="+z); err == nil {
					t.Errorf("%s=%q --dry-run: documented as refused, but it previewed a request", key, z)
				}
				skipped++
				continue
			}
			with, errWith := dryRunBody(t, cmd, "--"+flagName+"="+z)
			if errWith != nil {
				t.Errorf("%s=%q --dry-run: %v", key, z, errWith)
				continue
			}
			without, _ := dryRunBody(t, cmd)
			if with == without {
				t.Errorf("%s=%q: the required body field %q is not in the body (%s)", key, z, p.Name, with)
			}
			checked++
		}
	}
	if n := atomic.LoadInt32(&hits); n != 0 {
		t.Errorf("--dry-run sent %d request(s)", n)
	}
	sort.Strings(unmapped)
	t.Logf("checked %d required scalar body params, %d documented exceptions; %d without a same-named flag (renamed or hand-shaped): %v", checked, skipped, len(unmapped), unmapped)
	if checked < 200 {
		t.Fatalf("only %d required body params checked: the manifest walk is broken", checked)
	}
}

// Required inputs the manifest walk cannot see: the whole-body --body-json of the seven anyOf/oneOf bodies (the manifest
// lists no body params for them) and the hand-shaped typed set of emails create-template. An empty --body-json is
// refused locally (it used to send {}); an empty --body / --channel / --name is sent as "".
func TestRequiredFlagsOutsideTheManifestWalk(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":{}}`)
	}))
	defer srv.Close()
	t.Setenv("CONDUYT_CRM_BASE_URL", srv.URL)
	t.Setenv("CONDUYT_CRM_BEARER_AUTH", "test-token")
	t.Setenv("CONDUYT_CRM_CONFIG", filepath.Join(t.TempDir(), "config.toml"))
	t.Setenv("CONDUYT_CRM_NO_LEARN", "true")
	t.Setenv("HOME", t.TempDir())
	root := newRootCmd(&rootFlags{})
	find := func(path ...string) *cobra.Command {
		c, _, err := root.Find(path)
		if err != nil || c == root {
			t.Fatalf("%v: %v", path, err)
		}
		return c
	}
	for _, path := range [][]string{
		{"ai", "post-tasks"},
		{"automations", "post-validate"},
		{"batch-operations", "post"},
		{"chat", "post-agents-me-inbox-ack"},
		{"chat", "post-agents-me-inbox-claim"},
		{"conduyt-auth", "post-signup-events"},
		{"emails", "create-template"},
	} {
		c := find(path...)
		if _, err := dryRunBody(t, c, "--body-json="); err == nil {
			t.Errorf("%v --body-json= --dry-run previewed a request, want a local refusal", path)
		}
		if _, err := dryRunBody(t, c, "--body-json={}"); err != nil {
			t.Errorf("%v --body-json={} --dry-run: %v", path, err)
		}
	}
	tmpl := find("emails", "create-template")
	for _, flag := range []string{"body", "channel", "name"} {
		args := map[string]string{"body": "--body=hello", "channel": "--channel=email", "name": "--name=t1"}
		args[flag] = "--" + flag + "="
		got, err := dryRunBody(t, tmpl, args["body"], args["channel"], args["name"])
		if err != nil {
			t.Fatalf("emails create-template %s --dry-run: %v", args[flag], err)
		}
		key := flag
		if !strings.Contains(got, `"`+key+`":""`) {
			t.Errorf("emails create-template %s: body %s, want %q sent as empty", args[flag], got, key)
		}
	}
	if n := atomic.LoadInt32(&hits); n != 0 {
		t.Errorf("--dry-run sent %d request(s)", n)
	}
}
