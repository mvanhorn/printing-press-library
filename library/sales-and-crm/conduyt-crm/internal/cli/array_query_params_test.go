package cli

// PATCH(reprint-637-repeatable-array-query-params): every public query parameter whose schema is an array is a repeatable flag
// that is sent as one key per value. The table is spec.yaml itself, so a reprint that turns one back into a plain string flag
// (the generator's default) fails here.

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type arrayQueryParam struct{ method, path, name string }

func specArrayQueryParams(t *testing.T) []arrayQueryParam {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "spec.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	// resolve follows a $ref chain; a ref cycle ends the chain (nil) instead of looping.
	resolve := func(v any) map[string]any {
		m, _ := v.(map[string]any)
		seen := map[string]bool{}
		for m != nil {
			ref, ok := m["$ref"].(string)
			if !ok {
				break
			}
			if seen[ref] {
				return nil
			}
			seen[ref] = true
			var node any = doc
			for _, k := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
				nm, ok := node.(map[string]any)
				if !ok {
					return nil
				}
				node = nm[k]
			}
			m, _ = node.(map[string]any)
		}
		return m
	}
	// isArray reports an array schema, also through oneOf / anyOf / allOf branches and a type list. Cycle protection: a $ref
	// cycle ends in resolve, and the walk stops at depth 32 (a schema that composes itself through refs).
	var isArray func(v any, depth int) bool
	isArray = func(v any, depth int) bool {
		s := resolve(v)
		if s == nil || depth > 32 {
			return false
		}
		switch ty := s["type"].(type) {
		case string:
			if ty == "array" {
				return true
			}
		case []any:
			for _, x := range ty {
				if x == "array" {
					return true
				}
			}
		}
		for _, k := range []string{"oneOf", "anyOf", "allOf"} {
			branches, _ := s[k].([]any)
			for _, b := range branches {
				if isArray(b, depth+1) {
					return true
				}
			}
		}
		return false
	}
	var out []arrayQueryParam
	for path, item := range doc["paths"].(map[string]any) {
		pi := item.(map[string]any)
		shared, _ := pi["parameters"].([]any)
		for _, method := range []string{"get", "post", "put", "patch", "delete"} {
			op, ok := pi[method].(map[string]any)
			if !ok {
				continue
			}
			params, _ := op["parameters"].([]any)
			for _, p := range append(append([]any{}, params...), shared...) {
				pm := resolve(p)
				if pm == nil || pm["in"] != "query" {
					continue
				}
				if isArray(pm["schema"], 0) {
					out = append(out, arrayQueryParam{strings.ToUpper(method), path, pm["name"].(string)})
				}
			}
		}
	}
	return out
}

func findEndpointCmd(root *cobra.Command, method, path string) (*cobra.Command, []string) {
	var walk func(c *cobra.Command, chain []string) (*cobra.Command, []string)
	walk = func(c *cobra.Command, chain []string) (*cobra.Command, []string) {
		if c.Annotations["pp:method"] == method && c.Annotations["pp:path"] == path {
			return c, chain
		}
		for _, sub := range c.Commands() {
			if found, ch := walk(sub, append(append([]string{}, chain...), sub.Name())); found != nil {
				return found, ch
			}
		}
		return nil, nil
	}
	return walk(root, nil)
}

var camelBoundary = regexp.MustCompile(`([a-z0-9])([A-Z])`)

func TestEveryArrayQueryParamIsRepeatable(t *testing.T) {
	params := specArrayQueryParams(t)
	if len(params) < 9 {
		t.Fatalf("spec.yaml shows only %d array query parameters; the scan is blind", len(params))
	}
	for _, p := range params {
		flagName := strings.ToLower(camelBoundary.ReplaceAllString(strings.ReplaceAll(p.name, "_", "-"), "$1-$2"))
		root := newRootCmd(&rootFlags{})
		cmd, chain := findEndpointCmd(root, p.method, p.path)
		if cmd == nil {
			t.Errorf("%s %s: no command", p.method, p.path)
			continue
		}
		f := cmd.Flags().Lookup(flagName)
		if f == nil || f.Value.Type() != "stringArray" {
			t.Errorf("%s %s ?%s: flag --%s is %v, want a repeatable stringArray flag", p.method, p.path, p.name, flagName, f)
			continue
		}
		var got url.Values
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got = r.URL.Query()
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"data":[]}`)
		}))
		t.Setenv("CONDUYT_CRM_BASE_URL", srv.URL+"/api/v1")
		t.Setenv("CONDUYT_CRM_BEARER_AUTH", "test-token")
		t.Setenv("CONDUYT_CRM_CONFIG", filepath.Join(t.TempDir(), "config.toml"))
		t.Setenv("CONDUYT_CRM_NO_LEARN", "true")
		t.Setenv("HOME", t.TempDir())
		args := append(append([]string{}, chain...), "--data-source", "live", "--no-cache", "--json")
		for i := 0; i < strings.Count(cmd.Use, "<"); i++ {
			args = append(args, "p"+string(rune('1'+i)))
		}
		args = append(args, "--"+flagName, "v1", "--"+flagName, "v2,v3", "--"+flagName, `["v,4"]`)
		root.SetArgs(args)
		root.SetOut(io.Discard)
		root.SetErr(io.Discard)
		err := root.Execute()
		srv.Close()
		if err != nil {
			t.Errorf("%s %s ?%s: %v", p.method, p.path, p.name, err)
			continue
		}
		if vs := got[p.name]; strings.Join(vs, "|") != "v1|v2|v3|v,4" {
			t.Errorf("%s %s: ?%s sent %q, want one key per value [v1 v2 v3 v,4]", p.method, p.path, p.name, vs)
		}
	}
}

// PATCH(reprint-637-repeatable-array-query-params): the generator printed the deep-object parameter `cf[<key>]` as a literal
// --cf-key flag that sent `cf[<key>]=value`, which no API reads. It is not registered anywhere; the documented custom-field
// filter is --cf (fieldKey=value pairs or a JSON object).
func TestLiteralCfKeyFlagIsAbsent(t *testing.T) {
	root := newRootCmd(&rootFlags{})
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		if f := c.Flags().Lookup("cf-key"); f != nil {
			t.Errorf("%s registers --cf-key", c.CommandPath())
		}
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(root)
	for _, path := range []string{"/contacts", "/contacts/count", "/contacts/export"} {
		cmd, _ := findEndpointCmd(root, "GET", path)
		if cmd == nil || cmd.Flags().Lookup("cf") == nil {
			t.Errorf("GET %s: the documented --cf filter is missing", path)
		}
	}
	raw, err := os.ReadFile(filepath.Join("..", "..", "tools-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), `cf[<key>]`) || strings.Contains(string(raw), "cf[<key>]") {
		t.Errorf("tools-manifest.json still lists the literal cf[<key>] parameter")
	}
}
