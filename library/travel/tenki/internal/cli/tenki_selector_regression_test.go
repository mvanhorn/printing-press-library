package cli

import (
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/tenki/internal/tenki"
)

func TestTenkiSelectorUsesRendererMatchSemantics(t *testing.T) {
	for _, test := range []struct {
		name       string
		places     []tenki.Place
		selector   string
		wantError  bool
		checkValue func(*testing.T, map[string]any)
	}{
		{
			name:     "anchored metadata",
			places:   []tenki.Place{{Name: "千代田区", URL: tenkiTokyo}},
			selector: "meta.metrics.http_requests",
			checkValue: func(t *testing.T, value map[string]any) {
				t.Helper()
				if len(value) != 1 || value["meta"].(map[string]any)["metrics"].(map[string]any)["http_requests"] != float64(1) {
					t.Fatalf("metadata projection leaked or lost fields: %#v", value)
				}
			},
		},
		{
			name:      "unanchored metadata is not a list envelope",
			places:    []tenki.Place{{Name: "千代田区", URL: tenkiTokyo}},
			selector:  "metrics.http_requests",
			wantError: true,
		},
		{
			name:      "whitespace-only selector",
			places:    []tenki.Place{{Name: "千代田区", URL: tenkiTokyo}},
			selector:  "   ",
			wantError: true,
		},
		{
			name:      "comma-only selector",
			places:    []tenki.Place{{Name: "千代田区", URL: tenkiTokyo}},
			selector:  " , , ",
			wantError: true,
		},
		{
			name:     "list envelope shorthand",
			places:   []tenki.Place{{Name: "千代田区", URL: tenkiTokyo}},
			selector: "name",
			checkValue: func(t *testing.T, value map[string]any) {
				t.Helper()
				rows := tenkiResult(t, value)["places"].([]any)
				if len(rows) != 1 || len(rows[0].(map[string]any)) != 1 || rows[0].(map[string]any)["name"] != "千代田区" {
					t.Fatalf("list-envelope shorthand lost: %#v", value)
				}
			},
		},
		{
			name:      "unrelated empty list cannot hide typo",
			places:    []tenki.Place{},
			selector:  "deliberately_missing",
			wantError: true,
		},
		{
			name:     "anchored empty list remains supported",
			places:   []tenki.Place{},
			selector: "results.places.name",
			checkValue: func(t *testing.T, value map[string]any) {
				t.Helper()
				rows := tenkiResult(t, value)["places"].([]any)
				if len(rows) != 0 {
					t.Fatalf("anchored empty result changed: %#v", value)
				}
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			fake := &tenkiFake{search: tenki.SearchResult{Status: "ok", Places: test.places}}
			value, stdout, _, err, _ := tenkiRun(t, fake, "places", "search", "--query", "東京", "--json", "--select", test.selector)
			if test.wantError {
				if ExitCode(err) != 2 || stdout != "" {
					t.Fatalf("invalid selector did not fail before output: exit=%d stdout=%q", ExitCode(err), stdout)
				}
				return
			}
			if err != nil || strings.Count(stdout, "\n") != 1 {
				t.Fatalf("valid selector failed: %v, stdout=%q", err, stdout)
			}
			test.checkValue(t, value)
		})
	}
}
