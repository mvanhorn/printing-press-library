// Copyright 2026 USER and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"os"
	"testing"
)

const destinationHTMLFixture = `
<div class="section-inner s16" id="ID1">
  <div class="card-title altfont">Highlights</div>
  <div class="card-subtitle"><p>See the Statue of Liberty and Times Square.</p></div>
</div>
<div class="section-inner s99" id="events-and-toolkit">
  <p>Planning a watch party or public viewing event?</p>
  <p><a href="https://example.com/public-viewing.pdf">Review the public viewing guidance document</a>
  and visit <a href="https://publicviewing.fifa.org/public_viewing">FIFA's Public Viewing website</a>.</p>
</div>
<div class="section-inner footer"></div>
`

const fanEventsHTMLFixture = `
<div class="section-inner s27 contrast" id="fan-hub">
  <h2>NYNJ World Cup 26 Jersey Fan Hub</h2>
  <ul>
    <li>Sports Illustrated Stadium</li>
    <li>Harrison, NJ</li>
    <li>Select Dates June 11 - July 19, 2026</li>
    <li><a href="https://nynjfwc26.com/jersey/">Programming and ticketing details here</a></li>
  </ul>
  <img src="https://example.com/fan-hub.webp" />
  <p>A fan engagement experience.</p>
</div>
<div class="section-inner s27" id="fan-zone-brooklyn">
  <h2>BROOKLYN FAN ZONE</h2>
  <ul><li>Brooklyn Bridge Park</li><li>Brooklyn, NY</li><li>Select Dates June 13 - July 19, 2026</li></ul>
  <p>Matchday energy with food, music, and programming.</p>
</div>
<div class="section-inner s27" id="fan-zone-queens">
  <h2>NYNJ World Cup 26 Queens Group Stage HQ</h2>
  <ul><li>Flushing Meadows Corona Park</li><li>Queens, NY</li><li>June 11 - June 27, 2026</li></ul>
  <p>Group stage programming in Queens.</p>
</div>
<div class="section-inner s27" id="fan-zone-staten-island">
  <h2>NYNJ World Cup 26 Staten Island Fan Zone</h2>
  <ul><li>Empire Outlets</li><li>Staten Island, NY</li><li>June 29 - July 2, 2026</li></ul>
  <p>Waterfront fan programming.</p>
</div>
<div class="section-inner s27" id="fan-village-rockefeller">
  <h2>NYNJ World Cup 26 Fan Village Rockefeller Center</h2>
  <ul><li>Rockefeller Center</li><li>New York, NY</li><li>July 6 - 19, 2026</li></ul>
  <p>Fan village programming in Midtown.</p>
</div>
<div class="section-inner s15"></div>
`

func TestBuildPayloadExtractsCategories(t *testing.T) {
	data, err := buildPayload(fixtureOptions(t))
	if err != nil {
		t.Fatal(err)
	}

	categories := categoryCounts(data.Categories)
	if categories["Explore NYNJ"] != 1 {
		t.Fatalf("Explore NYNJ count = %d, want 1", categories["Explore NYNJ"])
	}
	if categories["Fan Experiences"] != 5 {
		t.Fatalf("Fan Experiences count = %d, want 5", categories["Fan Experiences"])
	}
	if categories["Watch Parties"] != 2 {
		t.Fatalf("Watch Parties count = %d, want 2", categories["Watch Parties"])
	}
}

func TestDateWindowFiltersOverlappingActivities(t *testing.T) {
	opts := fixtureOptions(t)
	opts.Categories = []string{"Fan Experiences", "Watch Parties"}
	opts.DateWindowStart = "2026-07-02"
	opts.DateWindowEnd = "2026-07-06"
	opts.ExcludeUndated = true

	data, err := buildPayload(opts)
	if err != nil {
		t.Fatal(err)
	}

	titles := map[string]bool{}
	for _, item := range data.Candidates {
		titles[item["title"].(string)] = true
		if item["type"] != "activity" {
			t.Fatalf("candidate %s type = %v, want activity", item["title"], item["type"])
		}
		if item["date_window_start"] == nil || item["date_window_end"] == nil {
			t.Fatalf("candidate %s missing parsed date window", item["title"])
		}
	}

	want := []string{
		"BROOKLYN FAN ZONE",
		"NYNJ World Cup 26 Fan Village Rockefeller Center",
		"NYNJ World Cup 26 Jersey Fan Hub",
		"NYNJ World Cup 26 Staten Island Fan Zone",
	}
	if len(titles) != len(want) {
		t.Fatalf("candidate count = %d, want %d: %#v", len(titles), len(want), titles)
	}
	for _, title := range want {
		if !titles[title] {
			t.Fatalf("missing title %q in %#v", title, titles)
		}
	}
	if titles["NYNJ World Cup 26 Queens Group Stage HQ"] {
		t.Fatal("Queens Group Stage HQ should not overlap July 2-6")
	}
}

func fixtureOptions(t *testing.T) sourceOptions {
	t.Helper()
	opts := defaultSourceOptions()
	opts.EventJSON = writeFixture(t, "event-*.json", `{"guid":"event-guid","name":"NYNJ World Cup Concierge"}`)
	opts.PromptsJSON = writeFixture(t, "prompts-*.json", `{"prompts":[{"guid":"watch-guid","order":1,"prompt_text":"Where can I watch matches if I don't have tickets?"}]}`)
	opts.DestinationHTML = writeFixture(t, "destination-*.html", destinationHTMLFixture)
	opts.FanEventsHTML = writeFixture(t, "fan-events-*.html", fanEventsHTMLFixture)
	return opts
}

func writeFixture(t *testing.T, pattern string, body string) string {
	t.Helper()
	file, err := os.CreateTemp("", pattern)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	t.Cleanup(func() { _ = os.Remove(file.Name()) })
	if _, err := file.WriteString(body); err != nil {
		t.Fatal(err)
	}
	return file.Name()
}

func categoryCounts(categories []categorySummary) map[string]int {
	counts := map[string]int{}
	for _, category := range categories {
		counts[category.Name] = category.Count
	}
	return counts
}
