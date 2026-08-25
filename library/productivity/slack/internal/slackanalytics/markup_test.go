// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package slackanalytics

import "testing"

// mirror is a stand-in for the locally synced users/channels maps.
func mirror(pairs map[string]string) LabelLookup {
	return func(id string) (string, bool) {
		name, ok := pairs[id]
		return name, ok
	}
}

func testRenderer() TextRenderer {
	return TextRenderer{
		User: mirror(map[string]string{
			"U04AB9XYZ": "Alice Adams",
			"U0EXAMPLE02": "GitHub",
			"W12ENT":    "Enterprise Ed",
		}),
		Channel: mirror(map[string]string{
			"C07QQ2L":  "general",
			"C0DEPLOY": "deploys",
		}),
		Usergroup: mirror(map[string]string{
			"SAZ94GDB8": "@eng",
		}),
	}
}

func TestTextRendererRender(t *testing.T) {
	r := testRenderer()
	cases := []struct {
		name string
		in   string
		want string
	}{
		// --- user mentions ---
		{"user mention resolved", "<@U04AB9XYZ> has joined the channel", "@Alice Adams has joined the channel"},
		{"user mention unresolved", "<@U99NOTSYNCED> shipped it", "@U99NOTSYNCED shipped it"},
		{"user mention inline label preferred over id", "<@U99NOTSYNCED|bob> shipped it", "@bob shipped it"},
		{"mirror beats inline label", "<@U04AB9XYZ|old-handle> replied", "@Alice Adams replied"},
		{"enterprise user id", "ping <@W12ENT>", "ping @Enterprise Ed"},

		// --- channel references ---
		{"channel ref resolved", "moved to <#C07QQ2L>", "moved to #general"},
		{"channel ref with label", "moved to <#C07QQ2L|general>", "moved to #general"},
		{"channel ref unresolved", "see <#C0NOPE>", "see #C0NOPE"},
		{"channel ref unresolved with label", "see <#C0NOPE|archive>", "see #archive"},

		// --- special mentions ---
		{"here", "<!here> standup in 5", "@here standup in 5"},
		{"channel broadcast", "<!channel> all hands", "@channel all hands"},
		{"everyone", "<!everyone> please read", "@everyone please read"},
		{"usergroup resolved", "<!subteam^SAZ94GDB8> please review", "@eng please review"},
		{"usergroup label fallback", "<!subteam^SNOPE|@design> please review", "@design please review"},
		{"usergroup bare fallback", "<!subteam^SNOPE> please review", "@SNOPE please review"},
		{"date special uses fallback text", "due <!date^1739980800^{date_short}|Feb 19, 2026>", "due Feb 19, 2026"},
		{"unknown special without fallback stays visible", "<!mystery^X1>", "<!mystery^X1>"},

		// --- links ---
		{"bare link", "Run: <https://example.com/runs/1>", "Run: https://example.com/runs/1"},
		{"labeled link keeps target", "see <https://example.com/pr/9|PR 9>", "see PR 9 (https://example.com/pr/9)"},
		{"label identical to target collapses", "<https://example.com|https://example.com>", "https://example.com"},
		{"slack autolabel prefix collapses", "<https://example.com/very/long/path|example.com/very/long…>", "https://example.com/very/long/path"},
		{"mailto strips scheme", "mail <mailto:ops@example.com|ops@example.com>", "mail ops@example.com"},
		{"tel strips scheme", "<tel:+15550001111>", "+15550001111"},

		// --- HTML entities ---
		{"ampersand", "ops &amp; eng", "ops & eng"},
		{"quoted block markers", "&gt; approved by captain", "> approved by captain"},
		{"escaped angle brackets are not markup", "use &lt;@U04AB9XYZ&gt; to mention", "use <@U04AB9XYZ> to mention"},

		// --- nested / multiple occurrences in one string ---
		{
			"many forms in one body",
			"<@U04AB9XYZ> moved <#C0DEPLOY|deploys> — <!here> see <https://ex.com/x|runbook> &amp; ping <@U99NOTSYNCED>",
			"@Alice Adams moved #deploys — @here see runbook (https://ex.com/x) & ping @U99NOTSYNCED",
		},
		{
			"repeated mention of the same user",
			"`<@U0EXAMPLE02> add tests` then `<@U0EXAMPLE02> refactor`",
			"`@GitHub add tests` then `@GitHub refactor`",
		},
		{
			"adjacent spans with no separator",
			"<@U04AB9XYZ><#C07QQ2L><!here>",
			"@Alice Adams#general@here",
		},

		// --- degenerate input ---
		{"empty string", "", ""},
		{"empty span", "a <> b", "a <> b"},
		{"unclosed span left alone", "a < b", "a < b"},
		{"non-markup angle pair left alone", "if a <b> c", "if a <b> c"},
		{"unknown scheme left alone", "<ftp://example.com>", "<ftp://example.com>"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := r.Render(tc.in); got != tc.want {
				t.Fatalf("Render(%q)\n got: %q\nwant: %q", tc.in, got, tc.want)
			}
		})
	}
}

// A body with no markup and no entities must survive untouched — no trimming,
// no whitespace collapsing, no reordering. Anything else would corrupt code
// blocks and pasted logs, which is most of what people put in Slack.
func TestRenderPassesPlainTextThroughByteIdentical(t *testing.T) {
	r := testRenderer()
	plain := []string{
		"deploy rollback finished",
		"  leading and trailing spaces preserved  ",
		"multi\nline\n\tbody with a tab",
		"emoji :rocket: and unicode — café … 日本語",
		"punctuation ! @ # $ % ^ * ( ) [ ] { } / \\ | ' \"",
		"",
	}
	for _, in := range plain {
		if got := r.Render(in); got != in {
			t.Errorf("Render(%q) = %q; want byte-identical passthrough", in, got)
		}
	}
}

// With no lookups wired the renderer must still de-render every form, falling
// back to readable IDs. This is the shape a caller gets before the mirror has
// ever synced users or channels.
func TestRenderWithoutResolversDegradesToReadableIDs(t *testing.T) {
	var r TextRenderer
	cases := map[string]string{
		"<@U04AB9XYZ> hi":       "@U04AB9XYZ hi",
		"in <#C07QQ2L>":         "in #C07QQ2L",
		"<!subteam^SAZ94GDB8>":  "@SAZ94GDB8",
		"<!here> ping":          "@here ping",
		"<https://example.com>": "https://example.com",
		"a &amp; b":             "a & b",
		"no markup at all":      "no markup at all",
	}
	for in, want := range cases {
		if got := r.Render(in); got != want {
			t.Errorf("zero-value Render(%q) = %q; want %q", in, got, want)
		}
	}
}

// A lookup that resolves to blank must not produce an empty "@" — it degrades
// exactly like an unknown ID.
func TestRenderIgnoresBlankLookupResults(t *testing.T) {
	r := TextRenderer{User: mirror(map[string]string{"U0BLANK": "   "})}
	if got, want := r.Render("<@U0BLANK>"), "@U0BLANK"; got != want {
		t.Fatalf("Render = %q; want %q", got, want)
	}
}

func TestRenderSnippet(t *testing.T) {
	r := testRenderer()
	// Rendering happens before truncation, so a span is never cut in half.
	got := r.RenderSnippet("<@U04AB9XYZ> says the deploy window is closed for the rest of the week", 24)
	if want := "@Alice Adams says the de…"; got != want {
		t.Fatalf("RenderSnippet = %q; want %q", got, want)
	}
	// Snippet's whitespace collapsing still applies to already-short bodies.
	if got, want := r.RenderSnippet("<!here>  spaced   out", 0), "@here spaced out"; got != want {
		t.Fatalf("RenderSnippet = %q; want %q", got, want)
	}
}

// De-rendering is presentation-only: the raw body still has to classify
// mentions, which depends on Slack's own encoding surviving in the store.
func TestRenderedTextIsNotUsedForMentionClassification(t *testing.T) {
	r := testRenderer()
	raw := "<@U04AB9XYZ> can you look?"
	if kind := ClassifyMention(raw, "U04AB9XYZ", nil, false); kind != MentionDirect {
		t.Fatalf("raw text should classify as a direct mention, got %q", kind)
	}
	if kind := ClassifyMention(r.Render(raw), "U04AB9XYZ", nil, false); kind != MentionNone {
		t.Fatalf("rendered text must not classify; got %q — classify before rendering", kind)
	}
}
