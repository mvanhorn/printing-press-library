// Copyright 2026 erick-holm. Licensed under Apache-2.0. See LICENSE.

// This file is hand-built (NOT generator-emitted). It carries the shared
// pure-logic helpers the 12 v1.1 novel verbs (digest, customer-intel,
// drift, dms-summary, dormant, attention, who-said, thread-summary, post,
// schedule, channel-find, user-find) depend on: window math over Slack ts
// strings, mirror DB open, message rendering with resolved author names,
// and the comp/HR redaction filter. The logic kept here is the genuinely
// reusable slice; per-verb behaviour stays in the verb's own file.

package cli

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/slack/internal/store"
)

// p1WindowDefault is the lookback applied when a verb's --window flag is
// left empty. A week matches the cadence of the weekly-digest workflows
// the verbs were specced for.
const p1WindowDefault = "7d"

// resolveWindowTS converts a --window duration string (7d, 24h, 1w, 30m)
// into the Slack-ts lower bound for [since, now] range queries. An empty
// string falls back to p1WindowDefault. The returned string is a Slack
// epoch-seconds ts ("1747000000.000000"), directly comparable against the
// ts column. The error names the bad flag so the caller can wrap it as a
// usage error.
func resolveWindowTS(window string) (string, error) {
	w := strings.TrimSpace(window)
	if w == "" {
		w = p1WindowDefault
	}
	t, err := parseSinceDuration(w)
	if err != nil {
		return "", fmt.Errorf("invalid --window %q: %w", window, err)
	}
	return unixToSlackTS(t), nil
}

// unixToSlackTS formats a time as a Slack ts string (decimal seconds,
// zero microseconds). Slack ts strings are zero-padded so lexical
// comparison is also chronological.
func unixToSlackTS(t time.Time) string {
	return fmt.Sprintf("%d.000000", t.Unix())
}

// slackTSToTime parses a Slack ts string back into a time.Time. A ts like
// "1747000000.001200" is unix seconds with a microsecond fraction; the
// fraction is dropped (whole-second resolution is enough for digests and
// drift windows). Returns the zero time on an unparseable input.
func slackTSToTime(ts string) time.Time {
	ts = strings.TrimSpace(ts)
	if ts == "" {
		return time.Time{}
	}
	whole := ts
	if i := strings.IndexByte(ts, '.'); i >= 0 {
		whole = ts[:i]
	}
	sec, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(sec, 0)
}

// nowSlackTS is the upper bound for window queries — the current instant
// as a Slack ts.
func nowSlackTS() string {
	return unixToSlackTS(time.Now())
}

// redactKeywords are the comp/HR-sensitive stems stripped from emitted
// message text when --redact-sensitivity is set. Matching is
// case-insensitive and substring-based ("renunc" catches "renuncia",
// "renunció"). Kept in sync with the digest-sensitive-content memory.
var redactKeywords = []string{
	"renunc", "comp", "salary", "base", "accelerator", "pip", "salió",
}

// redactLinePattern matches a whole whitespace-delimited token that
// contains any redact keyword, so the replacement removes the offending
// word rather than only the stem. Built once at package init.
var redactLinePattern = func() *regexp.Regexp {
	var alts []string
	for _, k := range redactKeywords {
		alts = append(alts, regexp.QuoteMeta(k))
	}
	return regexp.MustCompile(`(?i)\S*(?:` + strings.Join(alts, "|") + `)\S*`)
}()

// redactSensitive replaces every token containing a comp/HR keyword with
// "[redacted]". Used by digest, dms-summary, customer-intel and attention
// when --redact-sensitivity is passed, so team-shareable output never
// leaks compensation or HR-process detail.
func redactSensitive(text string) string {
	if text == "" {
		return text
	}
	return redactLinePattern.ReplaceAllString(text, "[redacted]")
}

// maybeRedact applies redactSensitive only when on is true.
func maybeRedact(text string, on bool) string {
	if !on {
		return text
	}
	return redactSensitive(text)
}

// openMirror opens the SQLite mirror at dbPath (or the default path when
// empty) and ensures the mirror schema. The caller owns Close.
func openMirror(ctx context.Context, dbPath string) (*store.Store, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("slack-pp-cli")
	}
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening local mirror %s: %w\nRun 'slack-pp-cli sync mirror' first to populate it.", dbPath, err)
	}
	if err := db.EnsureMirrorSchema(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// userNameResolver caches m_users lookups so a verb rendering many
// messages resolves each distinct author once. Build it with
// newUserNameResolver, then call name(id).
type userNameResolver struct {
	ctx   context.Context
	db    *store.Store
	cache map[string]string
}

func newUserNameResolver(ctx context.Context, db *store.Store) *userNameResolver {
	return &userNameResolver{ctx: ctx, db: db, cache: map[string]string{}}
}

// name returns the best human display name for a user id — real name,
// then display name, then handle — falling back to the raw id when the
// user is not mirrored.
func (r *userNameResolver) name(userID string) string {
	if userID == "" {
		return ""
	}
	if n, ok := r.cache[userID]; ok {
		return n
	}
	n := userID
	u, err := r.db.ResolveUser(r.ctx, userID)
	if err == nil {
		switch {
		case u.RealName != "":
			n = u.RealName
		case u.DisplayName != "":
			n = u.DisplayName
		case u.Name != "":
			n = u.Name
		}
	}
	r.cache[userID] = n
	return n
}

// channelLabel renders a channel for display: "#name" for named
// channels, the raw id otherwise (IM/MPIM channels have no real name).
func channelLabel(ch store.Channel) string {
	if ch.Name != "" && !ch.IsIM && !ch.IsMPIM {
		return "#" + ch.Name
	}
	if ch.Name != "" {
		return ch.Name
	}
	return ch.ID
}

// channelIDLabels builds an id -> display-label map for the given
// channels so verbs can label messages without a per-row resolve.
func channelIDLabels(channels []store.Channel) map[string]string {
	m := make(map[string]string, len(channels))
	for _, ch := range channels {
		m[ch.ID] = channelLabel(ch)
	}
	return m
}

// channelIDs extracts the id slice from a channel slice.
func channelIDs(channels []store.Channel) []string {
	ids := make([]string, 0, len(channels))
	for _, ch := range channels {
		ids = append(ids, ch.ID)
	}
	return ids
}

// resolveChannelArg resolves a user-supplied channel token via the mirror
// and returns a typed cliError (notFound / usage) on failure, so callers
// can `return` it directly.
func resolveChannelArg(ctx context.Context, db *store.Store, input string) (store.Channel, error) {
	ch, err := db.ResolveChannel(ctx, input)
	if err == nil {
		return ch, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return store.Channel{}, notFoundErr(fmt.Errorf("no channel matches %q in the local mirror — run 'slack-pp-cli sync mirror' or check the name", input))
	}
	// Ambiguous match — that's a usage problem the user must disambiguate.
	return store.Channel{}, usageErr(err)
}

// containsFold reports whether haystack contains needle, case-insensitive.
func containsFold(haystack, needle string) bool {
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}
