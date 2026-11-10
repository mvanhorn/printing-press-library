package cli

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// GET /contacts?email= (app main 318adf38d): exact, repeatable lookup on the primary and secondary email slots, up to 20.
// spec.yaml carries the param as an exploded form-style array, so the flag takes a comma list or a JSON array and the
// request carries one `email` query param per address (never one comma-joined value).
func runContactsList(t *testing.T, args ...string) (q url.Values, hits int) {
	t.Helper()
	q, hits, err := runContactsListErr(t, args...)
	if err != nil {
		t.Fatalf("contacts list %v: %v", args, err)
	}
	return
}

func runContactsListErr(t *testing.T, args ...string) (q url.Values, hits int, err error) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.Method != "GET" || r.URL.Path != "/api/v1/contacts" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		q = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[],"meta":{}}`))
	}))
	defer srv.Close()
	t.Setenv("CONDUYT_CRM_BASE_URL", srv.URL+"/api/v1")
	t.Setenv("CONDUYT_CRM_BEARER_AUTH", "test-token")
	t.Setenv("CONDUYT_CRM_CONFIG", filepath.Join(t.TempDir(), "config.toml"))
	cmd := newContactsListCmd(&rootFlags{noCache: true})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return
}

func TestContactsListEmailFlag(t *testing.T) {
	if newContactsListCmd(&rootFlags{}).Flags().Lookup("email") == nil {
		t.Fatal("contacts list must take --email")
	}
}

func TestContactsListEmailSendsRepeatedParams(t *testing.T) {
	want := []string{"a@x.com", "b@y.com"}
	for name, arg := range map[string]string{"comma list": "a@x.com,b@y.com", "json array": `["a@x.com","b@y.com"]`} {
		q, hits := runContactsList(t, "--email", arg, "--search", "smith")
		if hits != 1 {
			t.Fatalf("%s: hits=%d", name, hits)
		}
		if got := q["email"]; !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: email params = %q, want %q (one per address, not comma-joined)", name, got, want)
		}
		if q.Get("search") != "smith" {
			t.Fatalf("%s: search = %q", name, q.Get("search"))
		}
	}
}

func TestContactsListWithoutEmailSendsNone(t *testing.T) {
	q, _ := runContactsList(t)
	if _, ok := q["email"]; ok {
		t.Fatalf("no --email must send no email param, got %q", q["email"])
	}
}

// Round 2 (Codex): a repeated --email must keep every occurrence, and an explicitly supplied empty value must never
// become an unfiltered request.
func TestContactsListEmailRepeatedFlagKeepsEveryOccurrence(t *testing.T) {
	q, hits := runContactsList(t, "--email", "a@x.com", "--email", "b@y.com,c@z.com", "--email", `["d@w.com"]`)
	if hits != 1 {
		t.Fatalf("hits=%d", hits)
	}
	want := []string{"a@x.com", "b@y.com", "c@z.com", "d@w.com"}
	if !reflect.DeepEqual(q["email"], want) {
		t.Fatalf("email params = %q, want %q", q["email"], want)
	}
}

func TestContactsListEmailBlankInputsAreRejectedBeforeAnyRequest(t *testing.T) {
	for name, args := range map[string][]string{
		"empty":            {"--email", ""},
		"whitespace":       {"--email", "   "},
		"empty comma item": {"--email", "a@x.com,,b@y.com"},
		"trailing comma":   {"--email", "a@x.com,"},
		"empty array":      {"--email", "[]"},
		"blank json item":  {"--email", `["a@x.com"," "]`},
		"blank in repeat":  {"--email", "a@x.com", "--email", ""},
	} {
		_, hits, err := runContactsListErr(t, args...)
		if err == nil {
			t.Fatalf("%s: expected an error, got none", name)
		}
		if hits != 0 {
			t.Fatalf("%s: %d requests were sent, want none", name, hits)
		}
	}
}

func TestContactsListEmailTwentyOneDistinctValuesAreAllSent(t *testing.T) {
	var all []string
	for i := 0; i < 21; i++ {
		all = append(all, fmt.Sprintf("u%d@x.com", i))
	}
	args := []string{}
	for _, a := range all {
		args = append(args, "--email", a)
	}
	q, hits := runContactsList(t, args...)
	if hits != 1 || !reflect.DeepEqual(q["email"], all) {
		t.Fatalf("hits=%d, got %d email params, want all 21 sent (the server owns the 400)", hits, len(q["email"]))
	}
}

func TestContactsListEmailPlusIsEncodedAndRoundTrips(t *testing.T) {
	var raw string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[],"meta":{}}`))
	}))
	defer srv.Close()
	t.Setenv("CONDUYT_CRM_BASE_URL", srv.URL+"/api/v1")
	t.Setenv("CONDUYT_CRM_BEARER_AUTH", "test-token")
	t.Setenv("CONDUYT_CRM_CONFIG", filepath.Join(t.TempDir(), "config.toml"))
	cmd := newContactsListCmd(&rootFlags{noCache: true})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--email", "a+b@example.com"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(raw, "email=a%2Bb%40example.com") {
		t.Fatalf("raw query %q must carry the + as %%2B", raw)
	}
}

// PATCH(reprint-637-full-public-api): contacts list pages on per_page and sends limit only when --limit is passed (the API answers 400
// when limit and per_page differ).
func TestContactsListPagesOnPerPageNotLimit(t *testing.T) {
	for _, tc := range []struct {
		args      []string
		perPage   string
		limitSent bool
	}{
		{nil, "", false}, // gocli1219: 4.34.2 sends only the flags passed (retainCLIQueryParams); the API defaults per_page
		{[]string{"--per-page", "100"}, "100", false},
		{[]string{"--per-page", "100", "--limit", "100"}, "100", true},
	} {
		q, _ := runContactsList(t, tc.args...)
		if q.Get("per_page") != tc.perPage || q.Has("limit") != tc.limitSent {
			t.Errorf("%v sent %v, want per_page=%s and limit sent=%v", tc.args, q, tc.perPage, tc.limitSent)
		}
	}
}
