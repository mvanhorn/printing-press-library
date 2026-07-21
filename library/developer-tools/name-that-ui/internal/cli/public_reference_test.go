package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const translateFixture = `<html><body><table><thead><tr><th>The thing</th><th>AppKit</th><th>SwiftUI</th></tr></thead><tbody>
<tr><td><a href="/macos/button">Button</a><span class="block">Triggers an action</span></td><td><code>NSButton</code></td><td><code>Button</code></td></tr>
<tr><td><a href="/macos/checkbox">Checkbox</a></td><td><code>NSButton with switch type</code></td><td><code>Toggle + .toggleStyle(.checkbox)</code></td></tr>
</tbody></table></body></html>`

func runPublicReference(t *testing.T, args ...string) (map[string]any, error) {
	t.Helper()
	var flags rootFlags
	root := newRootCmd(&flags)
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs(append([]string{"--json", "--no-learn"}, args...))
	err := root.Execute()
	result := map[string]any{}
	if out.Len() > 0 {
		if decodeErr := json.Unmarshal(out.Bytes(), &result); decodeErr != nil {
			t.Fatalf("invalid JSON %q: %v", out.String(), decodeErr)
		}
	}
	return result, err
}

func TestTranslateExactFuzzyBidirectionalAndNoMatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/translate" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(translateFixture))
	}))
	defer server.Close()
	t.Setenv("NAME_THAT_UI_BASE_URL", server.URL)

	got, err := runPublicReference(t, "translate", "NSButton", "--from", "appkit", "--to", "swiftui")
	if err != nil {
		t.Fatal(err)
	}
	first := got["results"].([]any)[0].(map[string]any)
	if first["matched_field"] != "appkit" || first["match_type"] != "exact" || first["note"] != "Triggers an action" {
		t.Fatalf("appkit exact = %#v", first)
	}
	targets := first["targets"].([]any)
	if len(targets) != 1 || targets[0].(map[string]any)["framework"] != "swiftui" || targets[0].(map[string]any)["value"] != "Button" {
		t.Fatalf("swiftui target = %#v", targets)
	}

	got, err = runPublicReference(t, "translate", "Button", "--from", "swiftui", "--to", "appkit")
	if err != nil || got["results"].([]any)[0].(map[string]any)["matched_field"] != "swiftui" {
		t.Fatalf("swiftui-to-appkit = %#v, %v", got, err)
	}
	got, err = runPublicReference(t, "translate", "toggle checkbox", "--from", "swiftui")
	if err != nil || got["results"].([]any)[0].(map[string]any)["match_type"] != "fuzzy" {
		t.Fatalf("fuzzy = %#v, %v", got, err)
	}
	got, err = runPublicReference(t, "translate", "unmapped meteor button")
	if err != nil {
		t.Fatal(err)
	}
	if results, ok := got["results"].([]any); !ok || results == nil || len(results) != 0 {
		t.Fatalf("no match results = %#v", got["results"])
	}
}

func TestTranslateValidationLocalAndDryRun(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++ }))
	defer server.Close()
	t.Setenv("NAME_THAT_UI_BASE_URL", server.URL)

	_, err := runPublicReference(t, "translate")
	if err == nil || ExitCode(err) != 2 {
		t.Fatalf("missing term error = %v", err)
	}
	_, err = runPublicReference(t, "translate", "Button", "--from", "wrong")
	if err == nil || ExitCode(err) != 2 {
		t.Fatalf("bad --from error = %v", err)
	}
	_, err = runPublicReference(t, "translate", "Button", "--data-source", "local")
	if err == nil || !strings.Contains(err.Error(), "fetched directly") {
		t.Fatalf("local error = %v", err)
	}
	got, err := runPublicReference(t, "--dry-run", "translate", "Button")
	if err != nil || got["dry_run"] != true || calls != 0 {
		t.Fatalf("dry run = %#v, %v, calls=%d", got, err, calls)
	}
	if requests, ok := got["requests"].([]any); !ok || requests == nil || len(requests) != 1 {
		t.Fatalf("dry requests = %#v", got)
	}
}

func TestUpdatesMergeSinceUnknownPartialFailureAndDryRun(t *testing.T) {
	feed := `<rss><channel><item><title>Button article</title><link>https://example.test/button</link><pubDate>Mon, 20 Jul 2026 12:00:00 +0000</pubDate></item><item><title>Undated</title><link>https://example.test/undated</link></item></channel></rss>`
	sitemap := `<urlset><url><loc>https://example.test/button</loc><lastmod>2026-07-19</lastmod></url><url><loc>https://example.test/card</loc><lastmod>2026-07-21T00:00:00Z</lastmod></url><url><loc>https://example.test/old</loc><lastmod>2026-07-10</lastmod></url></urlset>`
	failFeed := false
	calls := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls[r.URL.Path]++
		switch r.URL.Path {
		case "/feed.xml":
			if failFeed {
				http.Error(w, "feed down", http.StatusBadGateway)
				return
			}
			_, _ = w.Write([]byte(feed))
		case "/sitemap.xml":
			_, _ = w.Write([]byte(sitemap))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv("NAME_THAT_UI_BASE_URL", server.URL)

	got, err := runPublicReference(t, "updates", "--since", "2026-07-20")
	if err != nil {
		t.Fatal(err)
	}
	entries := got["entries"].([]any)
	if len(entries) != 3 || entries[0].(map[string]any)["source_url"] != "https://example.test/card" || entries[1].(map[string]any)["source_kind"] != "feed" || entries[2].(map[string]any)["timestamp_known"] != false {
		t.Fatalf("merged entries = %#v", entries)
	}
	if warnings, ok := got["warnings"].([]any); !ok || warnings == nil || len(warnings) != 0 {
		t.Fatalf("warnings = %#v", got["warnings"])
	}
	failFeed = true
	got, err = runPublicReference(t, "updates", "--kind", "all")
	if err != nil || len(got["entries"].([]any)) != 3 || len(got["warnings"].([]any)) != 1 {
		t.Fatalf("partial result = %#v, %v", got, err)
	}
	before := calls["/feed.xml"] + calls["/sitemap.xml"]
	got, err = runPublicReference(t, "--dry-run", "updates", "--kind", "sitemap")
	if err != nil || got["dry_run"] != true || calls["/feed.xml"]+calls["/sitemap.xml"] != before {
		t.Fatalf("dry updates = %#v, %v, calls=%#v", got, err, calls)
	}
	if _, err := runPublicReference(t, "updates", "--since", "yesterday"); err == nil || ExitCode(err) != 2 {
		t.Fatalf("bad since error = %v", err)
	}
}

func TestUpdatesErrorsWhenBothSourcesFailAndRejectsMalformedXML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/feed.xml":
			_, _ = w.Write([]byte(`<rss>`))
		case "/sitemap.xml":
			http.Error(w, "down", http.StatusBadGateway)
		}
	}))
	defer server.Close()
	t.Setenv("NAME_THAT_UI_BASE_URL", server.URL)
	_, err := runPublicReference(t, "updates")
	if err == nil || !strings.Contains(err.Error(), "public feed") || !strings.Contains(err.Error(), "public sitemap") {
		t.Fatalf("both failures error = %v", err)
	}
}
