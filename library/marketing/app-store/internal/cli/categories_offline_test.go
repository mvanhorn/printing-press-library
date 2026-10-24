// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// `categories list` / `categories games` used to fetch apps.apple.com chart
// pages through the generated internal/client, which retries a 429 up to
// three times and then waits for its limiter, outside the App Store governor
// and the capture log. They now answer from the built-in genre table and send
// nothing.

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/client"
)

func TestCategoriesAnswerFromTheGenreTableWithoutANetworkRequest(t *testing.T) {
	// Every client either path could build answers through a recording
	// transport: the generated client (through a client hook) and the App
	// Store client (through the shared seam).
	generated := &fakeApple{handler: func(r *http.Request) fakeResponse {
		return fakeResponse{Status: http.StatusTooManyRequests, Header: http.Header{"Retry-After": {"13"}}}
	}}
	prevHooks := clientHooks
	clientHooks = append(append([]func(*client.Client) error{}, clientHooks...), func(c *client.Client) error {
		c.HTTPClient.Transport = generated
		return nil
	})
	t.Cleanup(func() { clientHooks = prevHooks })
	apple := &fakeApple{handler: func(r *http.Request) fakeResponse { return fakeResponse{Status: http.StatusTooManyRequests} }}
	useFakeAppStore(t, fakeAppStore{Transport: apple, CacheDir: t.TempDir()})

	decode := func(t *testing.T, args ...string) []categoryRow {
		t.Helper()
		out, stderr, err := runRootArgs(t, args...)
		if err != nil {
			t.Fatalf("%v: %v\n%s", args, err, stderr)
		}
		var rows []categoryRow
		if err := json.Unmarshal([]byte(out), &rows); err != nil {
			t.Fatalf("%v: not a JSON array: %v\n%s", args, err, out)
		}
		return rows
	}

	list := decode(t, "categories", "list", "--json")
	byID := map[string]categoryRow{}
	kids := map[string]string{}
	for _, r := range list {
		if r.AgeBandID != "" {
			kids[r.AgeBandID] = r.Name
			continue
		}
		if _, dup := byID[r.GenreID]; dup {
			t.Fatalf("genre %s listed twice", r.GenreID)
		}
		byID[r.GenreID] = r
	}
	for id, name := range map[string]string{"36": "All Apps", "6014": "Games", "6026": "Developer Tools", "6027": "Graphics & Design", "6000": "Business"} {
		if byID[id].Name != name {
			t.Fatalf("categories list lacks %s %s: %+v", id, name, byID[id])
		}
	}
	if len(byID) != 28 || len(kids) != 4 || kids["10001"] != "Kids Ages 6-8" || kids["0"] != "All Kids" {
		t.Fatalf("categories list = %d genres, kids bands %v", len(byID), kids)
	}
	if _, sub := byID["7012"]; sub {
		t.Fatal("game subgenres belong to 'categories games', not 'categories list'")
	}
	if got := byID["6026"].ChartURL; got != "https://apps.apple.com/us/iphone/charts/6026?chart=top-free" {
		t.Fatalf("chartUrl = %s", got)
	}

	games := decode(t, "categories", "games", "--country", "gb", "--json")
	names := map[string]string{}
	for _, r := range games {
		names[r.GenreID] = r.Name
		if !strings.HasPrefix(r.ChartURL, "https://apps.apple.com/gb/iphone/charts/"+r.GenreID+"?") {
			t.Fatalf("games row chartUrl %s does not follow --country gb", r.ChartURL)
		}
	}
	if len(games) != 19 || names["6014"] != "Games" || names["7003"] != "Casual" || names["7012"] != "Puzzle" || names["7019"] != "Word" {
		t.Fatalf("categories games = %d rows %v", len(games), names)
	}

	if n := len(generated.urls()) + len(apple.urls()); n != 0 {
		t.Fatalf("categories sent %d request(s): %v %v", n, generated.urls(), apple.urls())
	}

	// --select, the dry-run envelope and the argument check behave like
	// every other hand-built command.
	out, _, err := runRootArgs(t, "categories", "games", "--select", "genreId,name", "--json")
	if err != nil || strings.Contains(out, "chartUrl") || !strings.Contains(out, `"genreId": "7014"`) {
		t.Fatalf("--select: %v\n%s", err, out)
	}
	out, _, err = runRootArgs(t, "categories", "list", "--dry-run", "--json")
	if err != nil || !strings.Contains(out, `"dry_run":true`) {
		t.Fatalf("--dry-run: %v\n%s", err, out)
	}
	if _, _, err := runRootArgs(t, "categories", "list", "extra", "--json"); ExitCode(err) != 2 {
		t.Fatalf("stray argument exit = %d (%v), want 2", ExitCode(err), err)
	}
	for _, leaf := range []string{"list", "games"} {
		cmd, _, err := RootCmd().Find([]string{"categories", leaf})
		if err != nil || cmd.Name() != leaf || cmd.Annotations["pp:data-source"] != "computed" || cmd.Annotations["pp:endpoint"] != "" || cmd.Annotations["mcp:read-only"] != "true" {
			t.Fatalf("categories %s annotations = %v (%v)", leaf, cmd.Annotations, err)
		}
	}
}
