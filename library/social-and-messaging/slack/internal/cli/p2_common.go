// Copyright 2026 erick-holm. Licensed under Apache-2.0. See LICENSE.

// This file is hand-built (NOT generator-emitted). It carries the shared
// helpers the 8 P2 transcendence verbs depend on: window-bound math over
// Slack epoch-string timestamps, the cross-source SQLite ATTACH plumbing
// (used by customer-intel-deep, dm-engagement, action-followthrough,
// goal-channel-pulse), and the <!subteam^S...> mention renderer. Pure
// logic lives here so it can be unit-tested without a live mirror.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// stringTrimNL trims a single leading+trailing newline from a literal —
// used so cobra Example strings can be written as readable raw literals.
func stringTrimNL(s string) string {
	return strings.Trim(s, "\n")
}

// auditCaller returns the identity recorded in m_audit_log rows the P2
// verbs append when they read DM/MPIM content. A cron/agent can set
// SLACK_PP_CALLER to attribute the read; otherwise it defaults to "cli".
func auditCaller() string {
	if c := strings.TrimSpace(os.Getenv("SLACK_PP_CALLER")); c != "" {
		return c
	}
	return "cli"
}

// windowBounds resolves a --window value (e.g. "14d", "7d", "24h") into a
// pair of Slack ts strings [since, until]. until is "now". An empty
// window yields an open-ended lower bound. Slack ts strings are
// zero-padded decimal seconds, so lexical comparison is chronological.
func windowBounds(window string) (since, until string, err error) {
	now := time.Now()
	until = fmt.Sprintf("%d.999999", now.Unix())
	if strings.TrimSpace(window) == "" {
		return "", until, nil
	}
	t, perr := parseSinceDuration(window)
	if perr != nil {
		return "", "", fmt.Errorf("invalid --window value %q: %w", window, perr)
	}
	since = fmt.Sprintf("%d.000000", t.Unix())
	return since, until, nil
}

// tsToTime parses a Slack epoch-string ts ("1747000000.001200") into a
// time.Time. A malformed ts yields the zero time so callers can render
// "" rather than crash.
func tsToTime(ts string) time.Time {
	if ts == "" {
		return time.Time{}
	}
	secStr := ts
	if i := strings.IndexByte(ts, '.'); i >= 0 {
		secStr = ts[:i]
	}
	sec, err := strconv.ParseInt(secStr, 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(sec, 0).UTC()
}

// subteamMentionRE matches a Slack subteam mention token. The display
// segment after the optional '|' is what Slack renders; group 2 captures
// it when present.
var subteamMentionRE = regexp.MustCompile(`<!subteam\^([A-Z0-9]+)(?:\|@?([^>]+))?>`)

// renderSubteamMentions rewrites every <!subteam^S012|@handle> token in s
// to a readable "@handle". When the token carries no inline display
// segment, the id->handle map (from ListUsergroups) is consulted; an
// unknown id degrades to "@<id>" so output is never an opaque token.
//
// This fixes the known weekly-digest mis-render bug where raw
// <!subteam^S...> IDs leaked into rendered digests.
func renderSubteamMentions(s string, handles map[string]string) string {
	if !strings.Contains(s, "<!subteam^") {
		return s
	}
	return subteamMentionRE.ReplaceAllStringFunc(s, func(tok string) string {
		m := subteamMentionRE.FindStringSubmatch(tok)
		if m == nil {
			return tok
		}
		id, inline := m[1], m[2]
		if inline != "" {
			return "@" + strings.TrimPrefix(inline, "@")
		}
		if h, ok := handles[id]; ok && h != "" {
			return "@" + strings.TrimPrefix(h, "@")
		}
		return "@" + id
	})
}

// siblingDBPath returns the on-disk path of a sibling pp-* CLI's SQLite
// mirror. The slack mirror lives at <parent>/slack-pp-cli/data.db; the
// sibling shares the same parent dir. On Windows %LOCALAPPDATA% is also
// probed because the asana/attio/fathom CLIs were observed to store
// their DB there. The first existing file wins; "" means "not found".
//
// SLACK_PP_SIBLING_DIR, when set, is the ONLY directory probed (looking
// for <dir>/<cli>/data.db). It lets tests pin sibling resolution to a
// hermetic temp dir so a developer's real pp-* mirrors do not leak in.
func siblingDBPath(cliName string) string {
	if pin := os.Getenv("SLACK_PP_SIBLING_DIR"); pin != "" {
		for _, f := range []string{"data.db", cliName + ".db", "mirror.db"} {
			p := filepath.Join(pin, cliName, f)
			if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
				return p
			}
		}
		return ""
	}

	candidates := []string{}

	// Same parent dir as the slack mirror (~/.local/share/<cli>/...).
	slackPath := defaultDBPath("slack-pp-cli")
	parent := filepath.Dir(filepath.Dir(slackPath))
	for _, f := range []string{"data.db", cliName + ".db"} {
		candidates = append(candidates, filepath.Join(parent, cliName, f))
	}

	// Windows LOCALAPPDATA fallback.
	if la := os.Getenv("LOCALAPPDATA"); la != "" {
		for _, f := range []string{"data.db", cliName + ".db", "mirror.db"} {
			candidates = append(candidates, filepath.Join(la, cliName, f))
		}
	}

	for _, p := range candidates {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p
		}
	}
	return ""
}

// attachedSource is one sibling DB the verb tried to ATTACH.
type attachedSource struct {
	name   string // logical name: "attio" | "asana" | "fathom"
	cli    string // sibling CLI name: "attio-pp-cli" | ...
	alias  string // SQL ATTACH alias
	path   string // resolved on-disk path; "" when missing
	usable bool   // true once ATTACH succeeded
}

// crossSource manages ATTACHing the sibling pp-* mirrors onto the slack
// store's DB handle for cross-database JOINs. It is deliberately
// defensive: a missing file or a failed ATTACH does not error — the
// source is simply marked unusable and recorded in MissingSources, so
// the verb can still emit the Slack-only portion.
type crossSource struct {
	db      *sql.DB
	sources []*attachedSource
}

// newCrossSource resolves and ATTACHes the requested sibling mirrors.
// skipMissing only changes messaging intent — degradation happens
// regardless because a hard failure on an absent sibling DB would defeat
// the verb. The caller must call detach when done.
func newCrossSource(ctx context.Context, db *sql.DB, want map[string]string) (*crossSource, error) {
	cs := &crossSource{db: db}
	for name, cli := range want {
		src := &attachedSource{name: name, cli: cli, alias: "x_" + name}
		src.path = siblingDBPath(cli)
		if src.path != "" {
			// ATTACH read-only so a verb can never mutate a sibling mirror.
			uri := "file:" + src.path + "?mode=ro"
			if _, err := db.ExecContext(ctx,
				fmt.Sprintf("ATTACH DATABASE '%s' AS %s", uri, src.alias)); err == nil {
				src.usable = true
			}
		}
		cs.sources = append(cs.sources, src)
	}
	return cs, nil
}

// detach DETACHes every successfully attached sibling DB. Best-effort —
// a DETACH failure is non-fatal (the connection is short-lived anyway).
func (cs *crossSource) detach(ctx context.Context) {
	for _, src := range cs.sources {
		if src.usable {
			_, _ = cs.db.ExecContext(ctx, "DETACH DATABASE "+src.alias)
		}
	}
}

// source returns the attached source by logical name, or nil.
func (cs *crossSource) source(name string) *attachedSource {
	for _, src := range cs.sources {
		if src.name == name {
			return src
		}
	}
	return nil
}

// missing returns the logical names of every requested source that could
// not be attached, sorted for stable JSON output.
func (cs *crossSource) missing() []string {
	var out []string
	for _, src := range cs.sources {
		if !src.usable {
			out = append(out, src.name)
		}
	}
	return out
}

// probeQuery runs a defensive cross-DB query against an attached sibling.
// If the sibling schema does not have the expected table/column the query
// errors; the caller treats that as "source unusable" and continues. The
// returned bool is false when the source was missing or the query failed.
func (cs *crossSource) probeQuery(ctx context.Context, name, query string, args ...any) (*sql.Rows, bool) {
	src := cs.source(name)
	if src == nil || !src.usable {
		return nil, false
	}
	rows, err := cs.db.QueryContext(ctx, query, args...)
	if err != nil {
		// Schema mismatch — demote the source so missing() reports it.
		src.usable = false
		return nil, false
	}
	return rows, true
}
