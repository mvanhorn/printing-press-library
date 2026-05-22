package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestQueryCrawlStatsSamplesUnionReturnsLatestMetadata(t *testing.T) {
	ctx := context.Background()
	st, err := Open(ctx, filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	olderPoll := time.Date(2026, 5, 21, 10, 0, 0, 0, time.UTC)
	newerPoll := time.Date(2026, 5, 22, 10, 0, 0, 0, time.UTC)
	if err := st.UpsertCrawlStatsSamples(ctx, []CrawlStatsSampleRow{
		{
			SiteURL:       "sc-domain:example.com",
			SampleURL:     "https://example.com/page",
			FileType:      "html",
			ResponseCode:  200,
			GooglebotType: "smartphone",
			FetchedAt:     olderPoll.Add(-time.Hour),
			SizeBytes:     100,
			ResponseMs:    50,
			PollAt:        olderPoll,
			RawJSON:       `{"poll":"older"}`,
		},
		{
			SiteURL:       "sc-domain:example.com",
			SampleURL:     "https://example.com/page",
			FileType:      "html",
			ResponseCode:  304,
			GooglebotType: "desktop",
			FetchedAt:     newerPoll.Add(-time.Hour),
			SizeBytes:     250,
			ResponseMs:    25,
			PollAt:        newerPoll,
			RawJSON:       `{"poll":"newer"}`,
		},
	}); err != nil {
		t.Fatalf("upsert samples: %v", err)
	}

	rows, err := st.QueryCrawlStatsSamplesUnion(ctx, "sc-domain:example.com", "", "", 0, 0)
	if err != nil {
		t.Fatalf("query union: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected one deduplicated URL, got %d", len(rows))
	}
	got := rows[0]
	if got.ResponseCode != 304 || got.GooglebotType != "desktop" || got.SizeBytes != 250 || got.RawJSON != `{"poll":"newer"}` {
		t.Fatalf("union returned stale metadata: %+v", got)
	}
	if !got.PollAt.Equal(newerPoll) {
		t.Fatalf("expected newest poll %s, got %s", newerPoll, got.PollAt)
	}
}
