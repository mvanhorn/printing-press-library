// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored shared retrieval helpers — NOT generator output.
// pp:data-source local
//
// Shared read-side plumbing for the hand-built retrieval commands (quote,
// synthesize, filter, aggregate). Snips + episodes live in the generic
// `resources` table (resource_type='snips' / 'episodes') as JSON; these helpers
// open the mirror read-only, project compact rows, and print them.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/snipd/internal/snipd"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/snipd/internal/store"
)

// compactSnip is the agent-facing projection: everything worth ranking on except
// the (large) transcript, which is included only when explicitly requested.
type compactSnip struct {
	SnipID       string   `json:"snip_id"`
	Show         string   `json:"show"`
	EpisodeTitle string   `json:"episode_title"`
	Title        string   `json:"title"`
	Note         string   `json:"note,omitempty"`
	Quote        string   `json:"quote,omitempty"`
	Speaker      string   `json:"speaker,omitempty"`
	Start        string   `json:"start,omitempty"`
	Favorite     bool     `json:"favorite,omitempty"`
	Tags         []string `json:"tags,omitempty"`
	URL          string   `json:"url"`
	Transcript   string   `json:"transcript,omitempty"`
}

func toCompact(s snipd.Snip, withTranscript bool) compactSnip {
	c := compactSnip{
		SnipID:       s.SnipID,
		Show:         s.Show,
		EpisodeTitle: s.EpisodeTitle,
		Title:        s.Title,
		Note:         s.Note,
		Quote:        s.Quote,
		Speaker:      s.Speaker,
		Start:        s.Start,
		Favorite:     s.Favorite,
		Tags:         s.Tags,
		URL:          s.URL,
	}
	if withTranscript {
		c.Transcript = s.Transcript
	}
	return c
}

// openCorpusForRead opens the mirror read-only, honoring an explicit --db path.
// Returns (nil, path, nil) when the mirror does not exist yet, so callers can
// print a friendly "run pull first" hint instead of a raw open error.
func openCorpusForRead(ctx context.Context, dbPath string) (*store.Store, string, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("snipd-pp-cli")
	}
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil, dbPath, nil
	}
	st, err := store.OpenReadOnlyContext(ctx, dbPath)
	return st, dbPath, err
}

// emitNoCorpus writes the friendly missing-mirror message + an empty JSON array
// for machine callers, and returns nil (an empty local cache is not an error).
func emitNoCorpus(cmd *cobra.Command, flags *rootFlags, dbPath string) error {
	fmt.Fprintf(cmd.ErrOrStderr(), "no local snip corpus at %s\nrun: snipd-pp-cli pull\n", dbPath)
	if flags.asJSON || flags.agent {
		fmt.Fprintln(cmd.OutOrStdout(), "[]")
	}
	return nil
}

// querySnips runs a SELECT over the snips partition and unmarshals each row's
// JSON into a snipd.Snip. `where` is an optional extra predicate (already
// parameterized); `order` overrides the default ordering; limit<=0 = no limit.
func querySnips(ctx context.Context, st *store.Store, where, order string, args []any, limit int) ([]snipd.Snip, error) {
	q := "SELECT data FROM resources WHERE resource_type='snips'"
	if strings.TrimSpace(where) != "" {
		q += " AND " + where
	}
	if strings.TrimSpace(order) == "" {
		order = "json_extract(data,'$.episode_id'), json_extract(data,'$.start_seconds')"
	}
	q += " ORDER BY " + order
	if limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", limit)
	}
	rows, err := st.DB().QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()
	var out []snipd.Snip
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		var s snipd.Snip
		if err := json.Unmarshal([]byte(data), &s); err != nil {
			continue // skip an unparseable row rather than abort the whole read
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// unmarshalSnips converts the JSON blobs db.Search returns into snips.
func unmarshalSnips(raws []json.RawMessage) []snipd.Snip {
	out := make([]snipd.Snip, 0, len(raws))
	for _, raw := range raws {
		var s snipd.Snip
		if err := json.Unmarshal(raw, &s); err != nil {
			continue
		}
		out = append(out, s)
	}
	return out
}

// printCompact renders compact rows: machine JSON (with --select/--compact/--csv
// applied) or a scannable human list.
func printCompact(cmd *cobra.Command, flags *rootFlags, rows []compactSnip) error {
	if rows == nil {
		rows = []compactSnip{}
	}
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
	}
	if len(rows) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "no matching snips")
		return nil
	}
	w := cmd.OutOrStdout()
	for _, r := range rows {
		star := ""
		if r.Favorite {
			star = "★ "
		}
		fmt.Fprintf(w, "%s%s — %s\n", star, r.Title, r.Show)
		if r.Note != "" {
			fmt.Fprintf(w, "  %s\n", firstLine(r.Note, 140))
		}
		if r.URL != "" {
			fmt.Fprintf(w, "  %s\n", r.URL)
		}
	}
	fmt.Fprintf(w, "\n%d snip(s)\n", len(rows))
	return nil
}

func firstLine(s string, max int) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if len(s) > max {
		s = s[:max] + "…"
	}
	return strings.TrimPrefix(s, "- ")
}
