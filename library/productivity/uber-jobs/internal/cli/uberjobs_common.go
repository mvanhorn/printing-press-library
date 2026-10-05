// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

// Shared plumbing for the hand-written uber-jobs commands: the paced sibling
// client, the local store, exit-code mapping, and the single output envelope.

package cli

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/uber-jobs/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/uber-jobs/internal/config"
	"github.com/mvanhorn/printing-press-library/library/productivity/uber-jobs/internal/store"
	"github.com/mvanhorn/printing-press-library/library/productivity/uber-jobs/internal/uberjobs"
)

// ExitTransport is the documented exit code for DNS and transport failures,
// distinct from usage (2), not found (3), auth (4), API (5), rate limit or
// refusal (7), and config (10).
const ExitTransport = 6

func transportErr(err error) error { return &cliError{code: ExitTransport, err: err} }

// uberErr maps sibling-client errors onto the CLI's typed exit codes.
func uberErr(err error) error {
	if err == nil {
		return nil
	}
	var refusal *uberjobs.RefusalError
	var transport *uberjobs.TransportError
	var notFound *uberjobs.NotFoundError
	var content *uberjobs.ContentError
	var status *uberjobs.StatusError
	switch {
	case errors.As(err, &refusal):
		return rateLimitErr(fmt.Errorf("%w (exit %d: refused, not retried)", err, 7))
	case errors.As(err, &transport):
		return transportErr(err)
	case errors.Is(err, context.DeadlineExceeded):
		return transportErr(fmt.Errorf("request timed out: %w", err))
	case errors.As(err, &notFound):
		return notFoundErr(err)
	case errors.As(err, &content), errors.As(err, &status):
		return apiErr(err)
	}
	var ce *cliError
	if errors.As(err, &ce) {
		return err
	}
	return apiErr(err)
}

// uberStateDir is where the cross-process request gate and refusal log live.
func uberStateDir() string {
	if dir, err := cliutil.StateDir(); err == nil && dir != "" {
		return dir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state", "uber-jobs-pp-cli")
}

func uberCacheDir() string {
	if dir, err := cliutil.CacheDir(); err == nil && dir != "" {
		return dir
	}
	return ""
}

// newUberClient builds the paced sibling client. The base URL follows the
// generated config (UBER_JOBS_BASE_URL), so mock servers never reach Uber.
func newUberClient(flags *rootFlags) (*uberjobs.Client, error) {
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return nil, configErr(err)
	}
	c := uberjobs.NewClient(cfg.BaseURL, flags.timeout, uberStateDir(), uberCacheDir())
	c.NoCache = flags.noCache
	return c, nil
}

// addDataSourceFlag registers --data-source on one command, which owns its
// value. Never bind it to a shared field: pflag writes each default into the
// variable at registration, so a shared field would hold the default of
// whichever command registered last (sync's "live") for every command.
func addDataSourceFlag(cmd *cobra.Command, def, help string) {
	var ds string
	cmd.Flags().StringVar(&ds, "data-source", def, help)
}

// resolveDataSource reads this command's --data-source, validates it, and
// mirrors it into flags.dataSource for the generated agent-source helper.
func resolveDataSource(cmd *cobra.Command, flags *rootFlags, allowed ...string) (string, error) {
	raw, _ := cmd.Flags().GetString("data-source")
	ds := strings.ToLower(strings.TrimSpace(raw))
	if ds == "" {
		ds = "auto"
	}
	for _, a := range allowed {
		if ds == a {
			if flags != nil {
				flags.dataSource = ds
			}
			return ds, nil
		}
	}
	return "", usageErr(fmt.Errorf("--data-source %q is not supported here; use one of: %s", raw, strings.Join(allowed, ", ")))
}

func uberDBPath(dbPath string) string {
	if strings.TrimSpace(dbPath) != "" {
		return dbPath
	}
	return defaultDBPath("uber-jobs-pp-cli")
}

// openUberStore opens (and migrates) the local store.
func openUberStore(ctx context.Context, dbPath string) (*store.Store, *sql.DB, error) {
	path := uberDBPath(dbPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, nil, fmt.Errorf("creating store dir: %w", err)
	}
	s, err := store.OpenWithContext(ctx, path)
	if err != nil {
		return nil, nil, fmt.Errorf("opening local store: %w", err)
	}
	if err := uberjobs.EnsureSchema(ctx, s.DB()); err != nil {
		_ = s.Close()
		return nil, nil, err
	}
	return s, s.DB(), nil
}

// printEnvelope writes the one top-level envelope. Machine modes go through
// the generated output pipeline (--select, --csv, --compact) and keep every
// contract field, including description, under --agent/--compact.
func printEnvelope(cmd *cobra.Command, flags *rootFlags, env uberjobs.Envelope, human func(io.Writer) error) error {
	if env.Results == nil {
		env.Results = []any{}
	}
	if !wantsHumanTable(cmd.OutOrStdout(), flags) || human == nil {
		keep := append([]string{"meta", "hits", "returned", "scanned", "scan_cap_hit", "results"}, uberjobs.ContractFields...)
		keep = append(keep, uberRowKeys...)
		// --agent keeps its JSON and compaction, but not the generated agent
		// wrapper: that only recognises a bare {meta, results} pair and would
		// nest this envelope under a second, wrong meta.
		if flags.agent {
			flags.agent = false
			defer func() { flags.agent = true }()
		}
		return printJSONFilteredKeep(cmd.OutOrStdout(), env, flags, keep...)
	}
	return human(cmd.OutOrStdout())
}

// uberRowKeys are the non-posting row keys the hand-written commands emit;
// --agent/--compact keep every one of them.
var uberRowKeys = []string{
	// history and liveness (get --data-source local, check)
	"first_seen", "last_seen", "closed_on", "status", "as_of", "basis",
	// screen
	"verdict", "evidence",
	// stats
	"group", "label", "open", "posted_7d", "posted_30d", "opened_30d", "closed_30d",
	// save and searches
	"name", "filters", "created_at", "baseline_at", "last_advanced_at", "last_checked_at", "baseline_size", "reset", "deleted",
	// facets
	"facet", "value", "iso3", "teams",
	// new
	"complete", "baseline_established", "baseline_advanced", "current_count", "added_count", "removed_count",
	"added", "added_truncated", "removed", "reopened", "note",
	// sync
	"started_at", "finished_at", "scope", "total", "unique_count", "closed_marked", "inserted", "updated", "facets_updated", "facets_note",
}

// renderPostingsTable is the human view of a list of postings.
func renderPostingsTable(w io.Writer, env uberjobs.Envelope, rows []uberjobs.Posting) error {
	tw := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tPOSTED\tCOUNTRY\tTEAM\tTITLE")
	for _, p := range rows {
		posted := "-"
		if p.PostedOn != nil {
			posted = *p.PostedOn
		} else if p.PostedDateIsFloor {
			posted = "(old)"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", p.ID, posted, deref(p.CountryCode, "-"), deref(p.JobCategory, "-"), p.Title)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	fmt.Fprintf(w, "\n%d of %d postings (source: %s)\n", env.Returned, env.Hits, env.Meta.Source)
	if env.Meta.Note != "" {
		fmt.Fprintln(w, "note:", env.Meta.Note)
	}
	return nil
}

func deref(s *string, def string) string {
	if s == nil || *s == "" {
		return def
	}
	return *s
}

// sortPostings orders rows deterministically before --limit/--offset:
// recent = newest true posting date first (floor/undated last), ties by id
// descending; relevant = the site's own score order, ties by id.
func sortPostings(rows []uberjobs.Posting, mode string) {
	switch mode {
	case "relevant":
		sort.SliceStable(rows, func(i, j int) bool {
			si, sj := scoreOf(rows[i]), scoreOf(rows[j])
			if si != sj {
				return si > sj
			}
			return rows[i].ID > rows[j].ID
		})
	default:
		sort.SliceStable(rows, func(i, j int) bool {
			ti, tj := uberjobs.SortKey(rows[i]), uberjobs.SortKey(rows[j])
			if !ti.Equal(tj) {
				return ti.After(tj)
			}
			return rows[i].ID > rows[j].ID
		})
	}
}

func scoreOf(p uberjobs.Posting) float64 {
	if p.Score == nil {
		return 0
	}
	return *p.Score
}

// pageRows applies --offset then --limit after sorting.
func pageRows(rows []uberjobs.Posting, offset, limit int) []uberjobs.Posting {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(rows) {
		return []uberjobs.Posting{}
	}
	rows = rows[offset:]
	if limit > 0 && limit < len(rows) {
		rows = rows[:limit]
	}
	out := make([]uberjobs.Posting, len(rows))
	copy(out, rows)
	return out
}

func nowUTC() time.Time { return time.Now().UTC() }

// hasChildCommand reports whether parent already has a direct child named name.
func hasChildCommand(parent *cobra.Command, name string) bool {
	for _, c := range parent.Commands() {
		if c.Name() == name {
			return true
		}
	}
	return false
}

// secondaryCountryNote explains rows that match --country only through a
// secondary location: country_code is always the primary location.
func secondaryCountryNote(rows []uberjobs.Posting, countries []string) string {
	if len(countries) == 0 || len(rows) == 0 {
		return ""
	}
	want := make(map[string]bool, len(countries))
	for _, c := range countries {
		want[c] = true
	}
	n := 0
	for _, p := range rows {
		if p.CountryCode == nil || !want[*p.CountryCode] {
			n++
		}
	}
	if n == 0 {
		return ""
	}
	return fmt.Sprintf("%d of %d postings match --country through a secondary location; country_code is always the primary location (see locations[])", n, len(rows))
}
