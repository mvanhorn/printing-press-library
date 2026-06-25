package foxnews

import (
	"context"
	"testing"
	"time"
)

func TestResolveSectionDefaultAndAliases(t *testing.T) {
	got, err := ResolveSection("")
	if err != nil || got.ID != "latest" {
		t.Fatalf("empty: %#v, %v", got, err)
	}
	got, err = ResolveSection("videos")
	if err != nil || got.ID != "video" {
		t.Fatalf("videos alias: %#v, %v", got, err)
	}
	_, err = ResolveSection("nope")
	if err == nil {
		t.Fatal("expected error for unknown section")
	}
}

func TestFeedURL(t *testing.T) {
	section, _ := ResolveSection("politics")
	url := FeedURL(section, DefaultFeedBase)
	want := DefaultFeedBase + "/politics.xml"
	if url != want {
		t.Fatalf("got %q want %q", url, want)
	}
}

func TestParseFeedBody(t *testing.T) {
	section, _ := ResolveSection("latest")
	feed, err := parseFeedBody([]byte(verifyFixtureXML), section, FeedURL(section, DefaultFeedBase))
	if err != nil {
		t.Fatal(err)
	}
	if len(feed.Items) != 1 {
		t.Fatalf("items: %d", len(feed.Items))
	}
	if feed.Items[0].Title != "Sample Fox headline for verify" {
		t.Fatalf("title: %q", feed.Items[0].Title)
	}
	if feed.Items[0].Published.IsZero() {
		t.Fatal("expected parsed pubDate")
	}
}

func TestLimit(t *testing.T) {
	items := []Headline{{Title: "a"}, {Title: "b"}, {Title: "c"}}
	got := Limit(items, 2)
	if len(got) != 2 {
		t.Fatalf("got %d", len(got))
	}
}

func TestFetchVerifyMode(t *testing.T) {
	t.Setenv("PRINTING_PRESS_VERIFY", "1")
	section, _ := ResolveSection("sports")
	feed, err := Fetch(context.Background(), section, "", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if feed.Section != "sports" || len(feed.Items) != 1 {
		t.Fatalf("feed: %#v", feed)
	}
}
