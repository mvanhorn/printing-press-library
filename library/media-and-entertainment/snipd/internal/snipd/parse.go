// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored Snipd source layer — NOT generator output.
// pp:data-source live
//
// parse.go ports experiment/parse_corpus.py: the Snipd export API renders each
// episode as markdown from a labelled-delimiter template (<<token>>value<</token>>
// with @@SNIP@@ separators), which makes the parse trivial and robust. This file
// turns one episode's rendered markdown into a normalized Episode + []Snip. The
// quote/speaker split mirrors experiment/build_1bis_import.py.
package snipd

import (
	"regexp"
	"strconv"
	"strings"
)

// EpisodeTokens / SnipTokens are the verified export-template allowlist. The
// server only renders these tokens; asking for anything else is rejected.
var EpisodeTokens = []string{
	"episode_title", "show_title", "show_author", "guests", "episode_publish_date",
	"episode_ai_description", "episode_description", "mentioned_books", "episode_duration",
	"episode_url", "show_url", "episode_image", "episode_export_date",
}

var SnipTokens = []string{
	"snip_favorite_star", "snip_title", "snip_url", "snip_tags", "snip_start_time",
	"snip_end_time", "snip_duration", "snip_audio_player", "snip_note", "snip_quote",
	"snip_transcript",
}

const snipSeparator = "@@SNIP@@"

// Episode is one podcast episode with its metadata. Serialized as the JSON stored
// under resource_type="episodes" in the local mirror.
type Episode struct {
	EpisodeID       string   `json:"episode_id"`
	Title           string   `json:"title"`
	Show            string   `json:"show"`
	Author          string   `json:"author"`
	Guests          []string `json:"guests"`
	PublishDate     string   `json:"publish_date"`
	AIDescription   string   `json:"ai_description"`
	Description     string   `json:"description"`
	MentionedBooks  []string `json:"mentioned_books"`
	Duration        string   `json:"duration"`
	DurationSeconds *int     `json:"duration_seconds"`
	URL             string   `json:"url"`
	ShowURL         string   `json:"show_url"`
	Image           string   `json:"image"`
	ExportDate      string   `json:"export_date"`
	SnipCount       int      `json:"snip_count"`
}

// Snip is one captured moment. Serialized as the JSON stored under
// resource_type="snips". note/quote/transcript are kept SEPARATE (the structural
// win over Readwise, which concatenates them into one blob).
type Snip struct {
	SnipID       string   `json:"snip_id"`
	EpisodeID    string   `json:"episode_id"`
	Show         string   `json:"show"`
	EpisodeTitle string   `json:"episode_title"`
	Favorite     bool     `json:"favorite"`
	Title        string   `json:"title"`
	Note         string   `json:"note"`
	Quote        string   `json:"quote"`
	Speaker      string   `json:"speaker,omitempty"`
	Transcript   string   `json:"transcript"`
	Start        string   `json:"start"`
	End          string   `json:"end"`
	Duration     string   `json:"duration"`
	StartSeconds *int     `json:"start_seconds"`
	Tags         []string `json:"tags"`
	URL          string   `json:"url"`
	AudioEmbed   string   `json:"audio_embed"`
}

var (
	fieldRegexCache = map[string]*regexp.Regexp{}
	uuidRE          = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
	guestLinkRE     = regexp.MustCompile(`\[([^\]]+)\]\([^)]*\)`)
	iframeSrcRE     = regexp.MustCompile(`src="([^"]+)"`)
	// snipSeparatorRE matches @@SNIP@@ only when it stands alone on its own line —
	// exactly how the export template emits it. Splitting on this instead of the
	// bare substring means the token appearing INLINE inside a note, quote, or
	// transcript is NOT mistaken for a snip boundary.
	snipSeparatorRE = regexp.MustCompile("(?m)^" + regexp.QuoteMeta(snipSeparator) + "$")
	// snipChunkStartRE recognizes the start of a real snip chunk: the export always
	// opens each snip with a snip_<token>=<< field. A post-split chunk that does NOT
	// start this way is a phantom produced by a standalone @@SNIP@@ line sitting
	// INSIDE a multiline value, so it is glued back rather than parsed as a snip.
	snipChunkStartRE = regexp.MustCompile(`\A\s*snip_[a-z_]+=<<`)
)

func init() {
	for _, t := range EpisodeTokens {
		fieldRegexCache[t] = compileFieldRE(t)
	}
	for _, t := range SnipTokens {
		fieldRegexCache[t] = compileFieldRE(t)
	}
}

func compileFieldRE(token string) *regexp.Regexp {
	// (?s) = DOTALL so a value may span newlines; non-greedy to stop at the
	// first closing delimiter.
	return regexp.MustCompile(`(?s)<<` + regexp.QuoteMeta(token) + `>>(.*?)<</` + regexp.QuoteMeta(token) + `>>`)
}

// fieldValue extracts the value rendered between <<token>>…<</token>>.
func fieldValue(token, text string) string {
	re := fieldRegexCache[token]
	if re == nil {
		re = compileFieldRE(token)
	}
	m := re.FindStringSubmatch(text)
	if m == nil {
		return ""
	}
	return strings.TrimSpace(m[1])
}

// uuidFrom pulls the first UUID out of a Snipd deep-link URL (the snip id).
func uuidFrom(url string) string {
	return uuidRE.FindString(url)
}

// hmsToSeconds parses "MM:SS" or "HH:MM:SS" into seconds; nil on anything else.
func hmsToSeconds(ts string) *int {
	ts = strings.TrimSpace(ts)
	if ts == "" {
		return nil
	}
	parts := strings.Split(ts, ":")
	nums := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			return nil
		}
		nums = append(nums, n)
	}
	var total int
	switch len(nums) {
	case 2:
		total = nums[0]*60 + nums[1]
	case 3:
		total = nums[0]*3600 + nums[1]*60 + nums[2]
	default:
		return nil
	}
	return &total
}

// parseGuests handles both markdown-link lists ([Name](url)) and plain comma lists.
func parseGuests(g string) []string {
	g = strings.TrimSpace(g)
	if g == "" {
		return []string{}
	}
	if names := guestLinkRE.FindAllStringSubmatch(g, -1); len(names) > 0 {
		out := make([]string, 0, len(names))
		for _, m := range names {
			if n := strings.TrimSpace(m[1]); n != "" {
				out = append(out, n)
			}
		}
		return out
	}
	return splitTrimComma(g)
}

func splitTrimComma(s string) []string {
	out := []string{}
	for _, p := range strings.Split(s, ",") {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// audioEmbed extracts the src="…" URL from a rendered audio-player iframe.
func audioEmbed(iframe string) string {
	m := iframeSrcRE.FindStringSubmatch(iframe)
	if m == nil {
		return ""
	}
	return m[1]
}

// parseQuote splits Snipd's rendered quote field into (quoteText, speaker).
// Format: lines prefixed with "> ", a "> — Name" attribution line, then optional
// context prose. Mirrors experiment/build_1bis_import.py:parse_quote.
func parseQuote(q string) (string, string) {
	if strings.TrimSpace(q) == "" {
		return "", ""
	}
	var qlines []string
	var speaker string
	for _, raw := range strings.Split(q, "\n") {
		line := strings.TrimRight(raw, " \t\r")
		t := strings.TrimSpace(strings.TrimLeft(line, "> "))
		if t == "" {
			continue
		}
		// Snipd renders attribution as an em-dash line ("> — Name") that directly
		// follows the quote, so the FIRST em-dash line wins. A "> - bullet" inside
		// the quote body must NOT be mistaken for a speaker — only "—" attributes.
		if strings.HasPrefix(t, "—") {
			if speaker == "" {
				speaker = strings.TrimSpace(strings.TrimLeft(t, "— "))
			}
			continue
		}
		if strings.HasPrefix(line, ">") {
			qlines = append(qlines, t)
		}
	}
	return strings.TrimSpace(strings.Join(qlines, " ")), speaker
}

// ParseEpisode turns one rendered episode markdown body into an Episode plus its
// Snips. episodeID comes from the export filename (<episode_id>_full_content.md).
func ParseEpisode(episodeID, raw string) (Episode, []Snip) {
	body := raw
	// Strip a leading YAML frontmatter block (--- … ---) if present.
	if strings.HasPrefix(raw, "---") {
		if parts := strings.SplitN(raw, "---", 3); len(parts) == 3 {
			body = parts[2]
		}
	}
	// Split on standalone @@SNIP@@ lines, then reattach any chunk that doesn't open
	// a real snip. That reattach is what makes the parse content-proof: a @@SNIP@@
	// alone on its own line INSIDE a multiline note/quote/transcript is glued back
	// (restoring the separator) instead of producing a phantom snip that shifts
	// field parsing. Every real snip chunk begins with a snip_<token>=<< field.
	rawChunks := snipSeparatorRE.Split(body, -1)
	chunks := make([]string, 0, len(rawChunks))
	for _, c := range rawChunks {
		if len(chunks) > 0 && !snipChunkStartRE.MatchString(c) {
			chunks[len(chunks)-1] += snipSeparator + c
			continue
		}
		chunks = append(chunks, c)
	}
	head := chunks[0]

	ep := Episode{
		EpisodeID:       episodeID,
		Title:           fieldValue("episode_title", head),
		Show:            fieldValue("show_title", head),
		Author:          fieldValue("show_author", head),
		Guests:          parseGuests(fieldValue("guests", head)),
		PublishDate:     fieldValue("episode_publish_date", head),
		AIDescription:   fieldValue("episode_ai_description", head),
		Description:     fieldValue("episode_description", head),
		MentionedBooks:  splitTrimComma(fieldValue("mentioned_books", head)),
		Duration:        fieldValue("episode_duration", head),
		DurationSeconds: hmsToSeconds(fieldValue("episode_duration", head)),
		URL:             fieldValue("episode_url", head),
		ShowURL:         fieldValue("show_url", head),
		ExportDate:      fieldValue("episode_export_date", head),
		SnipCount:       len(chunks) - 1,
	}
	if img := audioEmbed(fieldValue("episode_image", head)); img != "" {
		ep.Image = img
	} else {
		ep.Image = fieldValue("episode_image", head)
	}

	snips := make([]Snip, 0, len(chunks)-1)
	for _, sc := range chunks[1:] {
		surl := fieldValue("snip_url", sc)
		quote, speaker := parseQuote(fieldValue("snip_quote", sc))
		snips = append(snips, Snip{
			SnipID:       uuidFrom(surl),
			EpisodeID:    episodeID,
			Show:         ep.Show,
			EpisodeTitle: ep.Title,
			Favorite:     strings.TrimSpace(fieldValue("snip_favorite_star", sc)) != "",
			Title:        fieldValue("snip_title", sc),
			Note:         fieldValue("snip_note", sc),
			Quote:        quote,
			Speaker:      speaker,
			Transcript:   fieldValue("snip_transcript", sc),
			Start:        fieldValue("snip_start_time", sc),
			End:          fieldValue("snip_end_time", sc),
			Duration:     fieldValue("snip_duration", sc),
			StartSeconds: hmsToSeconds(fieldValue("snip_start_time", sc)),
			Tags:         splitTrimComma(fieldValue("snip_tags", sc)),
			URL:          surl,
			AudioEmbed:   audioEmbed(fieldValue("snip_audio_player", sc)),
		})
	}
	return ep, snips
}

// EpisodeTemplate / SnipTemplate build the request bodies for the export POST.
// They render each token as `token=<<token>>{{token}}<</token>>` so the server
// wraps the substituted value in the labelled delimiters ParseEpisode reads back.
func EpisodeTemplate() string {
	var b strings.Builder
	for _, t := range EpisodeTokens {
		b.WriteString(labelled(t))
		b.WriteString("\n")
	}
	b.WriteString("{{snips_section}}")
	return b.String()
}

func SnipTemplate() string {
	var b strings.Builder
	b.WriteString(snipSeparator)
	b.WriteString("\n")
	for i, t := range SnipTokens {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(labelled(t))
	}
	return b.String()
}

func labelled(token string) string {
	return token + "=<<" + token + ">>{{" + token + "}}<</" + token + ">>"
}
