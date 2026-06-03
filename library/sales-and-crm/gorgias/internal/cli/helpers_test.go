package cli

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/gorgias/internal/client"
)

// classifyAPIError must switch on the typed *client.APIError.StatusCode, not
// on substring matching of err.Error(). These tests pin that contract — a
// future change to APIError.Error()'s format will not silently strip the
// classification.

func TestClassifyAPIError_TypedStatusDrivesClassification(t *testing.T) {
	cases := []struct {
		status   int
		wantCode int // exit code
	}{
		{401, 4}, // authErr
		{403, 4}, // authErr
		{404, 3}, // notFoundErr
		{429, 7}, // rateLimitErr
		{409, 5}, // apiErr (without --idempotent)
		{500, 5}, // apiErr default
	}
	for _, c := range cases {
		t.Run("status_"+itoa(c.status), func(t *testing.T) {
			apiE := &client.APIError{Method: "GET", Path: "/test", StatusCode: c.status, Body: "{}"}
			classified := classifyAPIError(apiE, &rootFlags{})
			gotCode := ExitCode(classified)
			if gotCode != c.wantCode {
				t.Errorf("status %d: want exit code %d, got %d (err: %v)", c.status, c.wantCode, gotCode, classified)
			}
		})
	}
}

func TestClassifyAPIError_Wrapped(t *testing.T) {
	// Wrapping with %w must preserve typed classification.
	apiE := &client.APIError{Method: "GET", Path: "/x", StatusCode: 401, Body: "{}"}
	wrapped := errors.New("higher-level: " + apiE.Error())
	// Wrapping that DOESN'T use %w should fall to default, NOT misclassify on string-match.
	classified := classifyAPIError(wrapped, &rootFlags{})
	if ExitCode(classified) != 5 {
		t.Errorf("unwrapped string-wrapped error must hit the default branch (apiErr=5), got %d", ExitCode(classified))
	}
}

func TestClassifyDeleteError_404IgnoreMissingIsNoop(t *testing.T) {
	apiE := &client.APIError{Method: "DELETE", Path: "/x/1", StatusCode: 404, Body: "{}"}
	got := classifyDeleteError(apiE, &rootFlags{ignoreMissing: true})
	if got != nil {
		t.Errorf("with --ignore-missing, 404 must collapse to nil; got %v", got)
	}
}

func TestClassifyDeleteError_404WithoutIgnoreMissingIsError(t *testing.T) {
	apiE := &client.APIError{Method: "DELETE", Path: "/x/1", StatusCode: 404, Body: "{}"}
	got := classifyDeleteError(apiE, &rootFlags{})
	if got == nil {
		t.Fatal("without --ignore-missing, 404 must be an error")
	}
	if ExitCode(got) != 3 {
		t.Errorf("404 without ignore-missing: want notFoundErr=3, got %d", ExitCode(got))
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var digits [10]byte
	i := len(digits)
	for n > 0 {
		i--
		digits[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		digits[i] = '-'
	}
	return string(digits[i:])
}

// Tests for compactFields / compactListFields / compactObjectFields /
// rawCellValue — the output pipeline that B14 (silent data loss on
// single-object responses) and B12 (scientific-notation IDs in CSV)
// both lived in. These functions previously had no direct test; the
// only verification was running shipcheck against a live tenant.

func TestCompactObjectFields_PreservesAllFields(t *testing.T) {
	// Single-object responses must NOT strip description/body/content —
	// the caller asked for this one record by id and wants every field.
	obj := map[string]any{
		"id":          float64(1234),
		"name":        "Example",
		"description": "Multi-paragraph prose the agent needs to see.",
		"body":        "Long body text.",
		"content":     "Content payload.",
	}
	out := compactObjectFields(obj)
	var parsed map[string]any
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, k := range []string{"id", "name", "description", "body", "content"} {
		if _, ok := parsed[k]; !ok {
			t.Errorf("compactObjectFields stripped %q from single-object response (B14 regression)", k)
		}
	}
}

func TestCompactListFields_StripsProseFromListItems(t *testing.T) {
	// List-context compaction CAN strip prose fields — that's the
	// token-efficiency intent of --compact for lists.
	items := []map[string]any{
		{"id": float64(1), "name": "a", "status": "open", "description": "long..."},
		{"id": float64(2), "name": "b", "status": "open", "description": "longer..."},
		{"id": float64(3), "name": "c", "status": "closed", "description": "longest..."},
	}
	out := compactListFields(items)
	var parsed []map[string]any
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(parsed) != 3 {
		t.Fatalf("expected 3 items, got %d", len(parsed))
	}
	for i, item := range parsed {
		if _, ok := item["description"]; ok {
			t.Errorf("item %d retained 'description' in list compaction (token-efficiency regression)", i)
		}
		if _, ok := item["id"]; !ok {
			t.Errorf("item %d missing 'id' after list compaction", i)
		}
	}
}

func TestUnwrapListEnvelope_GorgiasShape(t *testing.T) {
	// Gorgias list responses are {object, uri, data:[...], meta:{...}}.
	// unwrapListEnvelope must return the inner data[] for downstream
	// compactors/CSV/plain renderers to work on items, not the envelope.
	raw := json.RawMessage(`{"object":"list","uri":"/api/tags","data":[{"id":1},{"id":2}],"meta":{"next_cursor":null}}`)
	inner, rewrap, ok := unwrapListEnvelope(raw)
	if !ok {
		t.Fatal("expected envelope to unwrap")
	}
	var items []map[string]any
	if err := json.Unmarshal(inner, &items); err != nil {
		t.Fatalf("inner not an array: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	// rewrap should preserve the envelope shape with a substituted data
	wrapped := rewrap(json.RawMessage(`[{"id":99}]`))
	if !strings.Contains(string(wrapped), `"id":99`) {
		t.Errorf("rewrap dropped substituted data: %s", string(wrapped))
	}
	if !strings.Contains(string(wrapped), `"next_cursor"`) {
		t.Errorf("rewrap dropped meta: %s", string(wrapped))
	}
}

func TestUnwrapListEnvelope_NonEnvelopeReturnsFalse(t *testing.T) {
	for _, in := range []string{`{"id":1}`, `[1,2,3]`, `"plain string"`, `{"data":"not-an-array"}`} {
		_, _, ok := unwrapListEnvelope(json.RawMessage(in))
		if ok {
			t.Errorf("unwrapListEnvelope should return false for %q", in)
		}
	}
}

func TestRawCellValue_IntegralFloatNoScientificNotation(t *testing.T) {
	// JSON numbers decode as float64; fmt.Sprintf("%v", 123456789.0) emits
	// "1.23456789e+08". B12 fixed this for CSV/plain output.
	cases := map[string]any{
		"123456789": float64(123456789),
		"1066937":   float64(1066937),
		"0":         float64(0),
		"":          nil,
		"hello":     "hello",
		"true":      true,
		"false":     false,
		`{"k":"v"}`: map[string]any{"k": "v"},
		`["a","b"]`: []any{"a", "b"},
	}
	for want, in := range cases {
		got := rawCellValue(in)
		if got != want {
			t.Errorf("rawCellValue(%v): want %q, got %q", in, want, got)
		}
	}
}

func TestRawCellValue_NestedFloatIDNotScientific(t *testing.T) {
	// Nested objects render via json.Marshal — which formats integral
	// floats as plain integers (not scientific notation). This is the
	// B12 fix that lets --csv on tickets list emit usable customer IDs.
	in := map[string]any{
		"customer": map[string]any{
			"id":   float64(987654321),
			"name": "Test",
		},
	}
	got := rawCellValue(in)
	if strings.Contains(got, "e+") {
		t.Errorf("nested float ID rendered in scientific notation: %s", got)
	}
	if !strings.Contains(got, `"id":987654321`) {
		t.Errorf("expected nested id as plain integer, got: %s", got)
	}
}

func TestRawCellValue_NonIntegralFloatPreservesDecimal(t *testing.T) {
	got := rawCellValue(3.14)
	if got != "3.14" {
		t.Errorf("non-integral float: want %q, got %q", "3.14", got)
	}
}

func TestCompactFields_EnvelopeAwareTopLevelArray(t *testing.T) {
	// compactFields must handle bare arrays AND Gorgias envelopes.
	bare := json.RawMessage(`[{"id":1,"description":"long"},{"id":2,"description":"long"}]`)
	out := compactFields(bare)
	if strings.Contains(string(out), `"description"`) {
		t.Errorf("bare list: description not stripped: %s", string(out))
	}

	envelope := json.RawMessage(`{"object":"list","data":[{"id":1,"description":"long"},{"id":2,"description":"long"}],"meta":{}}`)
	out = compactFields(envelope)
	if strings.Contains(string(out), `"description"`) {
		t.Errorf("envelope list: description not stripped: %s", string(out))
	}
	if !strings.Contains(string(out), `"object":"list"`) {
		t.Errorf("envelope: outer envelope dropped after compaction: %s", string(out))
	}
}

func TestCompactFields_SingleObjectPreservesAllFields(t *testing.T) {
	single := json.RawMessage(`{"id":1,"name":"x","description":"keep this","body":"keep this"}`)
	out := compactFields(single)
	for _, want := range []string{`"description"`, `"body"`, `"keep this"`} {
		if !strings.Contains(string(out), want) {
			t.Errorf("compactFields stripped %s from single-object body (B14 regression): %s", want, string(out))
		}
	}
}

// XDG path helpers — the feedback ledger and profile store must follow
// the XDG Base Directory spec so they coexist with other CLI tooling
// and respect $XDG_*_HOME overrides (used by sandboxed test runners).
func TestXDGStateHome_RespectsEnvOverride(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/tmp/xdg-state-override")
	if got := xdgStateHome(); got != "/tmp/xdg-state-override" {
		t.Errorf("xdgStateHome: want override, got %q", got)
	}
}

func TestXDGStateHome_FallsBackToHomeLocalState(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("HOME", "/tmp/fake-home")
	got := xdgStateHome()
	if !strings.HasSuffix(got, "/.local/state") {
		t.Errorf("xdgStateHome fallback should end in /.local/state, got %q", got)
	}
}

func TestXDGConfigHome_RespectsEnvOverride(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg-config-override")
	if got := xdgConfigHome(); got != "/tmp/xdg-config-override" {
		t.Errorf("xdgConfigHome: want override, got %q", got)
	}
}

func TestXDGDataHome_RespectsEnvOverride(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/tmp/xdg-data-override")
	if got := xdgDataHome(); got != "/tmp/xdg-data-override" {
		t.Errorf("xdgDataHome: want override, got %q", got)
	}
}

// Legacy ~/.gorgias-pp-cli/<name> files written before the XDG migration
// must move to the new XDG path on first read, otherwise upgrading users
// silently lose their feedback ledger or saved profiles.
func TestMigrateLegacyDotfile_MovesWhenNewMissing(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	legacyDir := tmpHome + "/.gorgias-pp-cli"
	if err := os.MkdirAll(legacyDir, 0o700); err != nil {
		t.Fatal(err)
	}
	legacyFile := legacyDir + "/feedback.jsonl"
	wantBytes := []byte(`{"text":"legacy"}` + "\n")
	if err := os.WriteFile(legacyFile, wantBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	newPath := t.TempDir() + "/feedback.jsonl"

	migrateLegacyDotfile("feedback.jsonl", newPath)

	got, err := os.ReadFile(newPath)
	if err != nil {
		t.Fatalf("new path should exist after migration: %v", err)
	}
	if string(got) != string(wantBytes) {
		t.Errorf("migrated body mismatch: want %q, got %q", wantBytes, got)
	}
	if _, err := os.Stat(legacyFile); !os.IsNotExist(err) {
		t.Errorf("legacy file should be removed by Rename: %v", err)
	}
}

func TestMigrateLegacyDotfile_NoOpWhenNewExists(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	legacyDir := tmpHome + "/.gorgias-pp-cli"
	if err := os.MkdirAll(legacyDir, 0o700); err != nil {
		t.Fatal(err)
	}
	legacyFile := legacyDir + "/feedback.jsonl"
	if err := os.WriteFile(legacyFile, []byte(`{"text":"OLD"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	newPath := t.TempDir() + "/feedback.jsonl"
	if err := os.WriteFile(newPath, []byte(`{"text":"NEW"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	migrateLegacyDotfile("feedback.jsonl", newPath)

	got, err := os.ReadFile(newPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"text":"NEW"}` {
		t.Errorf("new path should NOT be overwritten by legacy; got %q", got)
	}
	if _, err := os.Stat(legacyFile); err != nil {
		t.Errorf("legacy file should be untouched when new exists: %v", err)
	}
}

func TestMigrateLegacyDotfile_NoOpWhenLegacyMissing(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	newPath := t.TempDir() + "/feedback.jsonl"
	// Neither legacy nor new file exists. Function must not panic or error.
	migrateLegacyDotfile("feedback.jsonl", newPath)
	if _, err := os.Stat(newPath); !os.IsNotExist(err) {
		t.Errorf("new path should still not exist after a no-op migrate: %v", err)
	}
}

// parseBodyJSONField must accept BOTH structured JSON (for object/array
// body fields) AND plain string enums (for `status`, `priority`, etc.).
// Pre-fix, this helper was strict-JSON-only, which broke `--status open`.
// Post-iter-3, malformed `{...}` / `[...]` shapes return an error rather
// than silently passing as a raw string — the user almost certainly meant
// JSON, and a clear local error beats a 400 from Gorgias.
func TestParseBodyJSONField_StructuredAndScalarBothWork(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want any
	}{
		{"object", `{"id":1,"email":"example-email-value"}`, map[string]any{"id": float64(1), "email": "example-email-value"}},
		{"array", `[{"id":1},{"id":2}]`, []any{map[string]any{"id": float64(1)}, map[string]any{"id": float64(2)}}},
		{"json-string", `"open"`, "open"},
		{"bare-enum-string", `open`, "open"}, // the common --status open case
		{"bare-string-with-spaces", `high priority`, "high priority"},
		{"json-number", `42`, float64(42)},
		{"json-bool", `true`, true},
		{"json-null", `null`, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := parseBodyJSONField("field", c.raw)
			if err != nil {
				t.Fatalf("parseBodyJSONField returned error on permissive parse: %v", err)
			}
			gotJSON, _ := json.Marshal(got)
			wantJSON, _ := json.Marshal(c.want)
			if string(gotJSON) != string(wantJSON) {
				t.Errorf("parseBodyJSONField(%q): want %s, got %s", c.raw, wantJSON, gotJSON)
			}
		})
	}
}

// Pins the new contract: a structured-looking shape that fails JSON parse
// must error rather than fall through. The user almost always typed
// malformed JSON, not a bare string starting with `{`.
func TestParseBodyJSONField_MalformedStructuredErrors(t *testing.T) {
	cases := []string{
		`{"foo": 1`,
		`[1, 2`,
		`{ "tags": ['a'] }`,
	}
	for _, in := range cases {
		_, err := parseBodyJSONField("channels", in)
		if err == nil {
			t.Errorf("parseBodyJSONField(%q): expected error on malformed JSON shape, got nil", in)
		}
	}
}
