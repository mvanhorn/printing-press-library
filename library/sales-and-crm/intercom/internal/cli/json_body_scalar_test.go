// Copyright 2026 Rob Zehner and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// Regression for patch json-body-scalars-keep-declared-type: body fields the
// API declares as integer, number, or boolean must reach the wire as JSON
// numbers/booleans, not as quoted strings.

func TestJSONBodyScalarCoercion(t *testing.T) {
	cases := []struct {
		kind, raw string
		want      string
		wantErr   bool
		omitted   bool
	}{
		{kind: "int", raw: "42", want: "42"},
		{kind: "int", raw: " -7 ", want: "-7"},
		{kind: "int", raw: "1.5", wantErr: true},
		{kind: "int", raw: "abc", wantErr: true},
		{kind: "number", raw: "0.75", want: "0.75"},
		{kind: "number", raw: "3", want: "3"},
		{kind: "number", raw: "NaN", wantErr: true},
		{kind: "bool", raw: "true", want: "true"},
		{kind: "bool", raw: "0", want: "false"},
		{kind: "bool", raw: "maybe", wantErr: true},
		{kind: "int", raw: "", wantErr: true},
		{kind: "int", raw: "null", want: "null"},
	}
	for _, tc := range cases {
		body := map[string]any{}
		err := setJSONBodyScalar(body, "help_center_id", "flag", tc.kind, tc.raw)
		if tc.wantErr {
			if err == nil {
				t.Errorf("%s %q: expected error", tc.kind, tc.raw)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%s %q: %v", tc.kind, tc.raw, err)
		}
		if tc.omitted {
			if _, ok := body["help_center_id"]; ok {
				t.Errorf("%s %q: expected key to be omitted", tc.kind, tc.raw)
			}
			continue
		}
		out, _ := json.Marshal(body)
		if got := string(out); got != `{"help_center_id":`+tc.want+`}` {
			t.Errorf("%s %q: wire = %s, want {\"f\":%s}", tc.kind, tc.raw, got, tc.want)
		}
	}
}

func jsonScalarFindCommand(root *cobra.Command, method, path string) *cobra.Command {
	if root.Annotations["pp:method"] == method && root.Annotations["pp:path"] == path {
		return root
	}
	for _, c := range root.Commands() {
		if found := jsonScalarFindCommand(c, method, path); found != nil {
			return found
		}
	}
	return nil
}

var jsonScalarQuotedRE = regexp.MustCompile(`"([a-z0-9][a-z0-9-]*)"`)

func jsonScalarPlaceholder(c *cobra.Command, name string) string {
	if f := c.Flags().Lookup(name); f != nil {
		switch f.Value.Type() {
		case "int", "int64", "int32", "float64", "float32", "uint", "count":
			return "1"
		case "bool":
			return "true"
		}
	}
	return "1"
}

// pathMatches compares a templated spec path to a request path segment by
// segment, from the end, so base-path prefixes are tolerated.
func jsonScalarPathMatches(tmpl, got string) bool {
	if i := strings.Index(tmpl, "://"); i >= 0 {
		rest := tmpl[i+3:]
		if j := strings.Index(rest, "/"); j >= 0 {
			tmpl = rest[j:]
		}
	}
	a := strings.Split(strings.Trim(tmpl, "/"), "/")
	b := strings.Split(strings.Trim(got, "/"), "/")
	if len(b) < len(a) {
		return false
	}
	b = b[len(b)-len(a):]
	for i := range a {
		if strings.Contains(a[i], "{") {
			continue
		}
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// jsonScalarCaptureStderr redirects os.Stderr (where --dry-run prints the request
// body) into buf until the returned restore func is called.
func jsonScalarCaptureStderr(t *testing.T, buf *strings.Builder) func() {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stderr
	os.Stderr = w
	done := make(chan struct{})
	go func() {
		b, _ := io.ReadAll(r)
		buf.Write(b)
		close(done)
	}()
	return func() {
		_ = w.Close()
		<-done
		os.Stderr = orig
	}
}

func jsonScalarContainsArg(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

func jsonScalarCommandPath(c *cobra.Command) []string {
	var parts []string
	for ; c != nil && c.HasParent(); c = c.Parent() {
		parts = append([]string{c.Name()}, parts...)
	}
	return parts
}

func TestJSONBodyScalarsSentWithDeclaredType(t *testing.T) {
	type tcase struct {
		method, path, flag, value string
		keyPath                   []string
		kind                      string
		extra                     []string
		dryRun                    bool
	}
	cases := []tcase{
		{method: "POST", path: "/ai/external_pages", flag: "source-id", value: "7", keyPath: []string{"source_id"}, kind: "int", extra: []string{}, dryRun: false},
		{method: "POST", path: "/articles", flag: "translated-content-ar-author-id", value: "7", keyPath: []string{"translated_content", "ar", "author_id"}, kind: "int", extra: []string{}, dryRun: false},
	}
	for _, tc := range cases {
		t.Run(tc.flag, func(t *testing.T) {
			var got map[string]any
			var hits int
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if !jsonScalarPathMatches(tc.path, r.URL.Path) {
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`{"accessToken":"t","token":"t","access_token":"t","expires_in":3600}`))
					return
				}
				hits++
				raw, _ := io.ReadAll(r.Body)
				dec := json.NewDecoder(strings.NewReader(string(raw)))
				dec.UseNumber()
				_ = dec.Decode(&got)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{}`))
			}))
			defer srv.Close()

			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
			t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
			t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
			t.Setenv("INTERCOM_ACCESS_TOKEN", "test-credential")
			t.Setenv("INTERCOM_BASE_URL", srv.URL)
			t.Setenv("INTERCOM_CONFIG", filepath.Join(home, "config.toml"))
			// Retry with placeholders for any other required flags (some
			// prints validate them in RunE) and confirm mutating commands.
			var supplied []string
			var lastErr error
			var lastStderr string
			for attempt := 0; attempt < 12 && hits == 0; attempt++ {
				root := RootCmd()
				target := jsonScalarFindCommand(root, tc.method, tc.path)
				if target == nil {
					t.Fatalf("no command annotated %s %s", tc.method, tc.path)
				}
				args := jsonScalarCommandPath(target)
				for _, tok := range strings.Fields(target.Use)[1:] {
					if strings.HasPrefix(tok, "<") {
						args = append(args, "1")
					}
				}
				args = append(args, "--"+tc.flag, tc.value)
				args = append(args, tc.extra...)
				args = append(args, supplied...)
				if tc.dryRun {
					args = append(args, "--dry-run")
				}
				root.SetArgs(args)
				var stdout, stderr strings.Builder
				root.SetOut(&stdout)
				root.SetErr(&stderr)
				var restore func()
				if tc.dryRun {
					restore = jsonScalarCaptureStderr(t, &stderr)
				}
				lastErr = root.Execute()
				if restore != nil {
					restore()
				}
				lastStderr = stderr.String()
				if tc.dryRun && lastErr == nil {
					if i := strings.Index(lastStderr, "Body:"); i >= 0 {
						dec := json.NewDecoder(strings.NewReader(lastStderr[i+len("Body:"):]))
						dec.UseNumber()
						if dec.Decode(&got) == nil {
							hits++
						}
					}
				}
				if hits > 0 || lastErr == nil {
					break
				}
				msg := lastErr.Error()
				if strings.Contains(msg, "required flag") {
					added := false
					for _, m := range jsonScalarQuotedRE.FindAllStringSubmatch(msg, -1) {
						name := strings.TrimPrefix(m[1], "--")
						if name == tc.flag || jsonScalarContainsArg(supplied, "--"+name+"="+jsonScalarPlaceholder(target, name)) {
							continue
						}
						supplied = append(supplied, "--"+name+"="+jsonScalarPlaceholder(target, name))
						added = true
					}
					if added {
						continue
					}
				}
				if strings.Contains(msg, "--yes") && !jsonScalarContainsArg(supplied, "--yes") {
					supplied = append(supplied, "--yes")
					continue
				}
				break
			}
			if hits == 0 {
				t.Fatalf("request never reached server (err=%v stderr=%s)", lastErr, lastStderr)
			}
			var cur any = got
			for _, k := range tc.keyPath {
				m, ok := cur.(map[string]any)
				if !ok {
					t.Fatalf("body %v has no object at %q", got, k)
				}
				cur = m[k]
			}
			switch tc.kind {
			case "bool":
				if _, ok := cur.(bool); !ok {
					t.Fatalf("%v = %#v (%T), want JSON boolean; body=%v", tc.keyPath, cur, cur, got)
				}
			default:
				n, ok := cur.(json.Number)
				if !ok || n.String() != tc.value {
					t.Fatalf("%v = %#v (%T), want JSON number %s; body=%v", tc.keyPath, cur, cur, tc.value, got)
				}
			}
		})
	}
}

func TestJSONBodyScalarNullOnlyForNullableFields(t *testing.T) {
	body := map[string]any{}
	if err := setJSONBodyScalar(body, "author_id", "author-id", "int", "null"); err == nil {
		t.Fatalf("null accepted for non-nullable author_id; body=%v", body)
	}
	body = map[string]any{}
	if err := setJSONBodyScalar(body, "help_center_id", "help-center-id", "int", "null"); err != nil {
		t.Fatalf("null rejected for nullable help_center_id: %v", err)
	}
	if v, ok := body["help_center_id"]; !ok || v != nil {
		t.Fatalf("help_center_id = %#v, want JSON null", v)
	}
}

func TestJSONBodyScalarRejectsExplicitBlank(t *testing.T) {
	for _, raw := range []string{"", "  \t "} {
		body := map[string]any{}
		if err := setJSONBodyScalar(body, "author_id", "author-id", "int", raw); err == nil {
			t.Errorf("%q: expected error for blank author ID", raw)
		}
		if _, ok := body["author_id"]; ok {
			t.Errorf("%q: blank author ID was added to body", raw)
		}
	}
}

func TestInternalArticleUpdateRejectsBlankAuthorIDBeforeRequest(t *testing.T) {
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	t.Setenv("INTERCOM_ACCESS_TOKEN", "test-credential")
	t.Setenv("INTERCOM_BASE_URL", srv.URL)
	t.Setenv("INTERCOM_CONFIG", filepath.Join(home, "config.toml"))

	root := RootCmd()
	root.SetArgs([]string{"internal-articles", "update", "1", "--author-id", "", "--yes"})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "--author-id must not be empty") {
		t.Fatalf("error = %v, want blank author ID error", err)
	}
	if requests != 0 {
		t.Fatalf("blank author ID caused %d requests", requests)
	}
}
