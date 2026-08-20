// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.
package snipd

import "testing"

func TestHMSToSeconds(t *testing.T) {
	cases := []struct {
		in   string
		want *int
	}{
		{"", nil},
		{"12:34", intp(754)},
		{"01:10:02", intp(4202)},
		{"garbage", nil},
		{"5", nil},
	}
	for _, tc := range cases {
		got := hmsToSeconds(tc.in)
		if (got == nil) != (tc.want == nil) {
			t.Fatalf("hmsToSeconds(%q) nil-ness = %v, want %v", tc.in, got, tc.want)
		}
		if got != nil && *got != *tc.want {
			t.Errorf("hmsToSeconds(%q) = %d, want %d", tc.in, *got, *tc.want)
		}
	}
}

func TestUUIDFrom(t *testing.T) {
	url := "https://share.snipd.com/snip/6a0839ae-1234-5678-9abc-def012345678"
	if got := uuidFrom(url); got != "6a0839ae-1234-5678-9abc-def012345678" {
		t.Errorf("uuidFrom = %q", got)
	}
	if got := uuidFrom("no uuid here"); got != "" {
		t.Errorf("uuidFrom(no uuid) = %q, want empty", got)
	}
}

func TestParseGuests(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", []string{}},
		{"[Noah Brier](https://x.com/noah)", []string{"Noah Brier"}},
		{"Alice, Bob , Carol", []string{"Alice", "Bob", "Carol"}},
		{"[A](u), [B](v)", []string{"A", "B"}},
	}
	for _, tc := range cases {
		got := parseGuests(tc.in)
		if len(got) != len(tc.want) {
			t.Fatalf("parseGuests(%q) = %v, want %v", tc.in, got, tc.want)
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("parseGuests(%q)[%d] = %q, want %q", tc.in, i, got[i], tc.want[i])
			}
		}
	}
}

func TestParseQuote(t *testing.T) {
	cases := []struct {
		name        string
		in          string
		wantQuote   string
		wantSpeaker string
	}{
		{"empty", "", "", ""},
		{
			"quote with speaker",
			"> AI is a thinking partner, not a replacement.\n> — Dan Shipper\n\nHe said this early on.",
			"AI is a thinking partner, not a replacement.",
			"Dan Shipper",
		},
		{
			"multiline quote no speaker",
			"> First line\n> second line",
			"First line second line",
			"",
		},
		{
			// A "> - bullet" in the body must NOT be read as the speaker;
			// only the em-dash line attributes.
			"bullet in body, em-dash attributes",
			"> Three points:\n> - one\n> two\n> — Ada",
			"Three points: - one two",
			"Ada",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q, s := parseQuote(tc.in)
			if q != tc.wantQuote {
				t.Errorf("quote = %q, want %q", q, tc.wantQuote)
			}
			if s != tc.wantSpeaker {
				t.Errorf("speaker = %q, want %q", s, tc.wantSpeaker)
			}
		})
	}
}

func TestParseEpisode(t *testing.T) {
	// Render a synthetic episode exactly as the export API would, using the
	// same templates the client sends.
	md := "" +
		"episode_title=<<episode_title>>Second Brain<</episode_title>>\n" +
		"show_title=<<show_title>>AI & I<</show_title>>\n" +
		"show_author=<<show_author>>Dan Shipper<</show_author>>\n" +
		"guests=<<guests>>[Noah Brier](https://x.com/noah)<</guests>>\n" +
		"episode_publish_date=<<episode_publish_date>>2026-05-13<</episode_publish_date>>\n" +
		"episode_ai_description=<<episode_ai_description>>A chat about second brains.<</episode_ai_description>>\n" +
		"episode_duration=<<episode_duration>>01:10:02<</episode_duration>>\n" +
		"episode_url=<<episode_url>>https://share.snipd.com/episode/40860548<</episode_url>>\n" +
		"@@SNIP@@\n" +
		"snip_favorite_star=<<snip_favorite_star>>⭐<</snip_favorite_star>>\n" +
		"snip_title=<<snip_title>>Thinking Mode Folder<</snip_title>>\n" +
		"snip_url=<<snip_url>>https://share.snipd.com/snip/6a0839ae-1111-2222-3333-444455556666<</snip_url>>\n" +
		"snip_tags=<<snip_tags>>ai, workflow<</snip_tags>>\n" +
		"snip_start_time=<<snip_start_time>>15:11<</snip_start_time>>\n" +
		"snip_end_time=<<snip_end_time>>16:37<</snip_end_time>>\n" +
		"snip_note=<<snip_note>>- point one\n- point two<</snip_note>>\n" +
		"snip_quote=<<snip_quote>>> Use a thinking mode folder.\n> — Dan Shipper<</snip_quote>>\n" +
		"snip_transcript=<<snip_transcript>>full transcript text here<</snip_transcript>>\n"

	ep, snips := ParseEpisode("40860548-aaaa-bbbb-cccc-ddddeeeeffff", md)

	if ep.Title != "Second Brain" {
		t.Errorf("title = %q", ep.Title)
	}
	if ep.Show != "AI & I" {
		t.Errorf("show = %q", ep.Show)
	}
	if ep.Author != "Dan Shipper" {
		t.Errorf("author = %q", ep.Author)
	}
	if len(ep.Guests) != 1 || ep.Guests[0] != "Noah Brier" {
		t.Errorf("guests = %v", ep.Guests)
	}
	if ep.DurationSeconds == nil || *ep.DurationSeconds != 4202 {
		t.Errorf("duration_seconds = %v", ep.DurationSeconds)
	}
	if ep.SnipCount != 1 {
		t.Errorf("snip_count = %d, want 1", ep.SnipCount)
	}
	if len(snips) != 1 {
		t.Fatalf("got %d snips, want 1", len(snips))
	}
	s := snips[0]
	if s.SnipID != "6a0839ae-1111-2222-3333-444455556666" {
		t.Errorf("snip_id = %q", s.SnipID)
	}
	if !s.Favorite {
		t.Errorf("favorite should be true")
	}
	if s.Title != "Thinking Mode Folder" {
		t.Errorf("snip title = %q", s.Title)
	}
	if s.Quote != "Use a thinking mode folder." {
		t.Errorf("quote = %q", s.Quote)
	}
	if s.Speaker != "Dan Shipper" {
		t.Errorf("speaker = %q", s.Speaker)
	}
	if s.Transcript != "full transcript text here" {
		t.Errorf("transcript = %q", s.Transcript)
	}
	if len(s.Tags) != 2 || s.Tags[0] != "ai" || s.Tags[1] != "workflow" {
		t.Errorf("tags = %v", s.Tags)
	}
	if s.StartSeconds == nil || *s.StartSeconds != 911 {
		t.Errorf("start_seconds = %v, want 911", s.StartSeconds)
	}
	if s.Show != "AI & I" || s.EpisodeTitle != "Second Brain" {
		t.Errorf("denormalized show/episode wrong: %q / %q", s.Show, s.EpisodeTitle)
	}
}

// TestParseEpisodeSeparatorInContentNotSplit guards the snip-boundary split: the
// @@SNIP@@ token appearing INSIDE a snip's content (not alone on its own line)
// must not be treated as a real boundary. A bare strings.Split would add a phantom
// snip and shift field parsing; the line-anchored split keeps it as one snip.
func TestParseEpisodeSeparatorInContentNotSplit(t *testing.T) {
	md := "" +
		"episode_title=<<episode_title>>Sep Test<</episode_title>>\n" +
		"episode_url=<<episode_url>>https://share.snipd.com/episode/1<</episode_url>>\n" +
		"@@SNIP@@\n" +
		"snip_title=<<snip_title>>Only Snip<</snip_title>>\n" +
		"snip_url=<<snip_url>>https://share.snipd.com/snip/6a0839ae-1111-2222-3333-444455556666<</snip_url>>\n" +
		"snip_note=<<snip_note>>talking about the @@SNIP@@ marker inline<</snip_note>>\n" +
		"snip_transcript=<<snip_transcript>>full transcript<</snip_transcript>>\n"

	ep, snips := ParseEpisode("11111111-2222-3333-4444-555566667777", md)
	if ep.SnipCount != 1 {
		t.Fatalf("snip_count = %d, want 1 (an inline @@SNIP@@ split the episode)", ep.SnipCount)
	}
	if len(snips) != 1 {
		t.Fatalf("got %d snips, want 1", len(snips))
	}
	if snips[0].Title != "Only Snip" {
		t.Errorf("snip title = %q, want %q (field parsing shifted?)", snips[0].Title, "Only Snip")
	}
	if !contains(snips[0].Note, "@@SNIP@@") {
		t.Errorf("note dropped the inline @@SNIP@@: %q", snips[0].Note)
	}
}

// TestParseEpisodeStandaloneSeparatorInContentNotSplit guards the deeper case: a
// @@SNIP@@ alone on its OWN line inside a multiline note. The chunk after it does
// not open a snip field, so it is glued back instead of becoming a phantom snip.
func TestParseEpisodeStandaloneSeparatorInContentNotSplit(t *testing.T) {
	md := "" +
		"episode_title=<<episode_title>>Standalone Test<</episode_title>>\n" +
		"episode_url=<<episode_url>>https://share.snipd.com/episode/2<</episode_url>>\n" +
		"@@SNIP@@\n" +
		"snip_favorite_star=<<snip_favorite_star>><</snip_favorite_star>>\n" +
		"snip_title=<<snip_title>>One Snip<</snip_title>>\n" +
		"snip_url=<<snip_url>>https://share.snipd.com/snip/6a0839ae-1111-2222-3333-444455556666<</snip_url>>\n" +
		"snip_note=<<snip_note>>line one\n@@SNIP@@\nline three<</snip_note>>\n" +
		"snip_transcript=<<snip_transcript>>full transcript<</snip_transcript>>\n"

	ep, snips := ParseEpisode("22222222-3333-4444-5555-666677778888", md)
	if ep.SnipCount != 1 {
		t.Fatalf("snip_count = %d, want 1 (a standalone @@SNIP@@ inside a note split the snip)", ep.SnipCount)
	}
	if len(snips) != 1 {
		t.Fatalf("got %d snips, want 1", len(snips))
	}
	if snips[0].Title != "One Snip" {
		t.Errorf("snip title = %q, want %q", snips[0].Title, "One Snip")
	}
	if !contains(snips[0].Note, "line one") || !contains(snips[0].Note, "line three") {
		t.Errorf("note lost content around the standalone separator: %q", snips[0].Note)
	}
}

func TestTemplatesRoundTrip(t *testing.T) {
	// The rendered form of a token must be parseable by fieldValue.
	ep := EpisodeTemplate()
	if !contains(ep, "<<episode_title>>") || !contains(ep, "{{snips_section}}") {
		t.Errorf("episode template missing markers:\n%s", ep)
	}
	sn := SnipTemplate()
	if !contains(sn, "@@SNIP@@") || !contains(sn, "<<snip_quote>>") {
		t.Errorf("snip template missing markers:\n%s", sn)
	}
}

func intp(n int) *int { return &n }

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
