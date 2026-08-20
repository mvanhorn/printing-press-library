// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package slackanalytics

import (
	"regexp"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/slack/internal/cliutil"
)

// Slack does not return rendered message text. Every message body arrives in
// Slack's own wire markup, and printing it verbatim leaks internals a reader
// cannot decode:
//
//	user mention:      <@U04AB9XYZ>              or <@U04AB9XYZ|alice>
//	channel reference: <#C07QQ2L>                or <#C07QQ2L|general>
//	usergroup mention: <!subteam^SAZ94GDB8>      or <!subteam^SAZ94GDB8|@eng>
//	broadcast:         <!here> <!channel> <!everyone>
//	other specials:    <!date^1739980800^{date_short}|Feb 19, 2026>
//	link:              <https://example.com>     or <https://example.com|label>
//	entities:          &amp; &lt; &gt;
//
// Slack escapes `&`, `<`, and `>` inside author-typed text, so an unescaped
// `<` in a message body is always markup — which is why the entity decode
// happens after the angle-bracket pass, never before. Decoding first would
// turn a literal "&lt;@U123&gt;" the author actually typed into a mention
// that never existed.
//
// De-rendering is a presentation concern only. The raw body stays in the
// local mirror untouched so mention classification (ClassifyMention) and
// full-text search keep operating on Slack's own encoding.

// markupRE matches one Slack markup span. The body is captured without the
// surrounding angle brackets; a literal ">" can never appear inside a span
// because Slack escapes it in author text.
var markupRE = regexp.MustCompile(`<([^<>]*)>`)

// LabelLookup resolves a Slack entity ID to a human-readable label. The second
// return is false when the local mirror does not know the ID, which is normal:
// a mirror holding one channel's history still sees mentions of people and
// channels it never synced.
type LabelLookup func(id string) (string, bool)

// TextRenderer de-renders Slack markup into readable text. The zero value is
// usable: with no lookups wired, every ID degrades to its readable ID form
// (@U04AB9XYZ, #C07QQ2L) rather than staying wrapped in Slack syntax.
type TextRenderer struct {
	// User resolves a user ID (U…/W…/B…) to a display name.
	User LabelLookup
	// Channel resolves a conversation ID (C…/G…/D…) to a bare channel name,
	// without the leading "#".
	Channel LabelLookup
	// Usergroup resolves a usergroup ID (S…) to its handle. Usually nil:
	// usergroup names are not part of the default sync set, and the inline
	// label Slack embeds in the mention covers the common case.
	Usergroup LabelLookup
}

// Render converts one Slack message body into readable text.
//
// Passthrough guarantee: a body containing neither "<" nor "&" is returned
// byte-identical — same allocation, no trimming, no normalization. That is
// the overwhelmingly common case, so the fast path is also the cheap path.
// A body that does carry markup or entities is additionally run through
// cliutil.CleanText, which decodes entities and trims surrounding whitespace.
func (r TextRenderer) Render(text string) string {
	if !strings.ContainsAny(text, "<&") {
		return text
	}
	replaced := markupRE.ReplaceAllStringFunc(text, func(span string) string {
		return r.renderSpan(span[1 : len(span)-1])
	})
	return cliutil.CleanText(replaced)
}

// RenderSnippet de-renders and then trims to at most max runes, the shape
// every preview field wants. Rendering first matters: truncating raw markup
// can cut a span in half and strand "<@U04AB9" in the output.
func (r TextRenderer) RenderSnippet(text string, max int) string {
	return Snippet(r.Render(text), max)
}

// renderSpan turns the inside of one <…> span into readable text. Anything
// unrecognized is returned re-wrapped in its brackets: leaving an unknown
// construct visible is better than silently deleting message content.
func (r TextRenderer) renderSpan(body string) string {
	if body == "" {
		return "<>"
	}
	label, hasLabel := "", false
	if pipe := strings.Index(body, "|"); pipe >= 0 {
		label, hasLabel = body[pipe+1:], true
		body = body[:pipe]
	}

	switch {
	case strings.HasPrefix(body, "@"):
		return "@" + resolveEntity(r.User, body[1:], label, hasLabel)

	case strings.HasPrefix(body, "#"):
		return "#" + resolveEntity(r.Channel, body[1:], label, hasLabel)

	case strings.HasPrefix(body, "!subteam^"):
		return "@" + strings.TrimPrefix(resolveEntity(r.Usergroup, body[len("!subteam^"):], label, hasLabel), "@")

	case body == "!here", body == "!channel", body == "!everyone":
		return "@" + body[1:]

	case strings.HasPrefix(body, "!"):
		// Every other special (<!date^…>, future additions) carries its own
		// rendered fallback after the pipe. Without one there is nothing
		// readable to show, so the raw form stays visible.
		if hasLabel && strings.TrimSpace(label) != "" {
			return label
		}
		return "<" + body + ">"

	case isLinkTarget(body):
		return renderLink(body, label, hasLabel)
	}

	if hasLabel {
		return "<" + body + "|" + label + ">"
	}
	return "<" + body + ">"
}

// resolveEntity picks the friendliest name available for an entity ID:
// the local mirror first (it is current), then the label Slack inlined at
// author time, then the bare ID.
func resolveEntity(lookup LabelLookup, id, label string, hasLabel bool) string {
	id = strings.TrimSpace(id)
	if lookup != nil && id != "" {
		if name, ok := lookup(id); ok && strings.TrimSpace(name) != "" {
			return strings.TrimSpace(name)
		}
	}
	if hasLabel && strings.TrimSpace(label) != "" {
		return strings.TrimSpace(label)
	}
	if id == "" {
		return "unknown"
	}
	return id
}

// linkSchemes are the URL schemes Slack auto-links inside message text.
var linkSchemes = []string{"http://", "https://", "mailto:", "tel:", "skype:"}

func isLinkTarget(body string) bool {
	for _, scheme := range linkSchemes {
		if strings.HasPrefix(body, scheme) {
			return true
		}
	}
	return false
}

// renderLink keeps the target visible. A terminal cannot carry a hyperlink the
// way Slack does, so dropping the URL in favour of its label would destroy the
// only actionable part of the message. The label is shown alongside unless it
// is redundant with the target.
func renderLink(target, label string, hasLabel bool) string {
	display := strings.TrimPrefix(strings.TrimPrefix(target, "mailto:"), "tel:")
	label = strings.TrimSpace(label)
	if !hasLabel || label == "" || label == target || label == display {
		return display
	}
	// Slack auto-labels bare links with a truncated, scheme-stripped form of
	// the URL itself ("example.com/very/long…"); that adds nothing next to
	// the full target.
	bare := display
	for _, scheme := range linkSchemes {
		bare = strings.TrimPrefix(bare, scheme)
	}
	if stem := strings.TrimSuffix(label, "…"); stem != "" && strings.HasPrefix(bare, stem) {
		return display
	}
	return label + " (" + display + ")"
}
