// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/cliutil/testenv"
)

// TestNovelDedupeHelpWires smoke-tests that the dedupe command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelDedupeHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"dedupe", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("dedupe --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "dedupe"} {
		if !strings.Contains(help, want) {
			t.Fatalf("dedupe --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestNovelNormalizeDedupeKeys(t *testing.T) {
	if got := novelNormalizeURL("https://www.Example.com/path/?utm_source=1&page=2#frag"); got != "example.com/path?page=2" {
		t.Fatalf("normalized URL = %q, want example.com/path", got)
	}
	if got := novelNormalizeURL("https://example.com/path?page=1"); got != "example.com/path?page=1" {
		t.Fatalf("normalized URL = %q, want page query preserved", got)
	}
	if got := novelNormalizeTitle("  The   Same\tTitle "); got != "the same title" {
		t.Fatalf("normalized title = %q, want collapsed lowercase", got)
	}
}

func TestNovelDedupeClustersSuggestedKeeperPrefersReadingProgress(t *testing.T) {
	members := []novelDedupeMember{
		{Kind: "doc", ID: "d1", Title: "Same", URL: "https://example.com/a", ReadingProgress: 0},
		{Kind: "doc", ID: "d2", Title: "Same", URL: "http://www.example.com/a/", ReadingProgress: 0.5},
	}
	clusters := novelDedupeClusters(members, map[string]bool{"url": true})
	if len(clusters) != 1 {
		t.Fatalf("clusters len = %d, want 1", len(clusters))
	}
	if clusters[0].SuggestedKeep.ID != "d2" {
		t.Fatalf("suggested keeper = %+v, want progressed member d2", clusters[0].SuggestedKeep)
	}
}

func TestNovelDedupeClustersSuggestedKeeperPrefersHighlightCountAfterProgressTie(t *testing.T) {
	members := []novelDedupeMember{
		{Kind: "doc", ID: "d1", Title: "Same", URL: "https://example.com/a", ReadingProgress: 0.5, NumHighlights: 1},
		{Kind: "book", ID: "b1", Title: "Same", URL: "http://www.example.com/a/", ReadingProgress: 0.5, NumHighlights: 7},
	}
	clusters := novelDedupeClusters(members, map[string]bool{"url": true})
	if len(clusters) != 1 {
		t.Fatalf("clusters len = %d, want 1", len(clusters))
	}
	if clusters[0].SuggestedKeep.ID != "b1" {
		t.Fatalf("suggested keeper = %+v, want higher-highlight member b1", clusters[0].SuggestedKeep)
	}
}

func TestNovelDedupeClustersConfidenceAndJunkTitleFiltering(t *testing.T) {
	members := []novelDedupeMember{
		{Kind: "doc", ID: "junk-1", Title: "Page Not Found", Author: "Ada", URL: "https://example.com/missing-a"},
		{Kind: "doc", ID: "junk-2", Title: "page not found", Author: "Ada", URL: "https://example.org/missing-b"},
		{Kind: "doc", ID: "site-1", Title: "Reforge", Author: "Ada", URL: "https://example.com/reforge-a"},
		{Kind: "doc", ID: "site-2", Title: "Reforge", Author: "Ada", URL: "https://example.org/reforge-b"},
		{Kind: "doc", ID: "real-1", Title: "Retrieval Ranking Guide", Author: "Ada", URL: "https://example.com/guide-a"},
		{Kind: "doc", ID: "real-2", Title: "retrieval ranking guide", Author: "Ada", URL: "https://example.org/guide-b"},
		{Kind: "doc", ID: "url-1", Title: "Different Title One", Author: "Ada", URL: "https://example.net/article?utm_campaign=x&page=1"},
		{Kind: "doc", ID: "url-2", Title: "Different Title Two", Author: "Grace", URL: "http://www.example.net/article?page=1"},
	}
	clusters := novelDedupeClusters(members, map[string]bool{"url": true, "title": true})
	if len(clusters) != 2 {
		t.Fatalf("clusters = %+v, want one URL cluster and one real title cluster", clusters)
	}
	by := map[string]novelDedupeCluster{}
	for _, cluster := range clusters {
		by[cluster.By] = cluster
	}
	if by["url"].Confidence != "high" || by["url"].Key != "example.net/article?page=1" {
		t.Fatalf("url cluster = %+v, want high-confidence normalized URL", by["url"])
	}
	if by["title"].Confidence != "medium" || by["title"].Key != "retrieval ranking guide" {
		t.Fatalf("title cluster = %+v, want medium-confidence real title", by["title"])
	}
	for _, cluster := range clusters {
		for _, member := range cluster.Members {
			if member.ID == "junk-1" || member.ID == "junk-2" || member.ID == "site-1" || member.ID == "site-2" {
				t.Fatalf("clustered weak title member %s in %+v", member.ID, cluster)
			}
		}
	}
}

func TestNovelDedupeTitleClusterRequiresMatchingAuthor(t *testing.T) {
	members := []novelDedupeMember{
		{Kind: "doc", ID: "d1", Title: "Shared Real Title", Author: "Ada", URL: "https://example.com/a"},
		{Kind: "doc", ID: "d2", Title: "Shared Real Title", Author: "Grace", URL: "https://example.com/b"},
		{Kind: "book", ID: "b1", Title: "Same", URL: "http://www.example.com/a/", NumHighlights: 7},
	}
	clusters := novelDedupeClusters(members, map[string]bool{"title": true})
	if len(clusters) != 0 {
		t.Fatalf("clusters = %+v, want no title-only cluster across different authors", clusters)
	}
}

func TestSliceDDedupeKeepInterstitialsAndLimit(t *testing.T) {
	sliceDStore(t,
		"UPDATE documents SET title='Useful Article', author='Same', reading_progress=0, source_url='https://example.com/article?utm_source=x', saved_at='2020-01-01T00:00:00Z' WHERE id='doc-1'",
		"UPDATE documents SET title='Useful Article', author='Same', reading_progress=0, source_url='https://example.com/article', saved_at='2021-01-01T00:00:00Z' WHERE id='doc-3'",
		"UPDATE documents SET source_url='https://example.com/empty' WHERE id='doc-2'",
	)
	var view novelDedupeView
	sliceDView(t, runNovelCommand(t, "dedupe", "--limit", "1"), &view)
	if view.TotalClusters != 2 || len(view.Clusters) != 1 || !view.Truncated || !strings.Contains(view.Note, "--limit") || view.TotalDocuments != 6 {
		t.Fatalf("%+v", view)
	}
	sliceDView(t, runNovelCommand(t, "dedupe", "--limit", "0"), &view)
	if len(view.Clusters) != 2 || view.Truncated {
		t.Fatalf("%+v", view)
	}
	var keeper string
	for _, c := range view.Clusters {
		if c.Key == "example.com/article" {
			keeper = c.SuggestedKeep.ID
		}
	}
	if keeper != "doc-3" {
		t.Fatalf("keeper=%q clusters=%+v", keeper, view.Clusters)
	}
	cmd := newNovelDedupeCmd(&rootFlags{})
	members, _, err := novelLoadDedupeMembers(cmd, map[string]bool{"docs": true})
	if err != nil {
		t.Fatal(err)
	}
	var a, b novelDedupeMember
	for _, m := range members {
		if m.ID == "doc-1" {
			a = m
		}
		if m.ID == "doc-3" {
			b = m
		}
	}
	a.URL = "https://example.com/long-path"
	b.URL = "https://example.com/x"
	if novelSuggestedKeep([]novelDedupeMember{a, b}).ID != b.ID {
		t.Fatal("shorter URL not preferred")
	}
	a.URL = b.URL
	if novelSuggestedKeep([]novelDedupeMember{b, a}).ID != a.ID {
		t.Fatal("oldest save not preferred")
	}
	for _, title := range []string{"SIGN IN", "Log In", "Login", "Error 403", "403 Forbidden", "Access Denied", "Page Not Found", "404", "Just A Moment"} {
		a.Title = title
		b.Title = title
		a.Author = "Same"
		b.Author = "Same"
		a.URL = ""
		b.URL = ""
		if novelValidDedupeTitleKey(novelNormalizeTitle(title)) || len(novelDedupeClusters([]novelDedupeMember{a, b}, map[string]bool{"title": true})) != 0 {
			t.Fatalf("interstitial %q clustered", title)
		}
	}
}

func TestE3DedupeBookTimestampFallback(t *testing.T) {
	sliceDStore(t, "UPDATE books SET data=json_remove(data,'$.saved_at','$.created_at'),last_highlight_at='2021-01-02T00:00:00Z',updated='2022-01-01T00:00:00Z' WHERE id='1'", "UPDATE books SET data=json_remove(data,'$.saved_at','$.created_at'),last_highlight_at=NULL,updated='2023-01-01T00:00:00Z' WHERE id='2'")
	members, _, err := novelLoadDedupeMembers(newNovelDedupeCmd(&rootFlags{}), map[string]bool{"books": true})
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 2 {
		t.Fatal(members)
	}
	for _, m := range members {
		want := 2021
		if m.ID == "2" {
			want = 2023
		}
		if m.SavedAt.Year() != want {
			t.Fatalf("member=%+v want=%d", m, want)
		}
	}
}
