// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto

package cli

import (
	"context"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/uber-jobs/internal/store"
	"github.com/mvanhorn/printing-press-library/library/productivity/uber-jobs/internal/uberjobs"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newNovelCheckCmd(flags))
	})
}

// checkRow is the liveness answer for one posting id.
type checkRow struct {
	ID        string  `json:"id"`
	Status    string  `json:"status"`
	Title     *string `json:"title"`
	URL       *string `json:"url"`
	PostedOn  *string `json:"posted_on"`
	FirstSeen *string `json:"first_seen"`
	LastSeen  *string `json:"last_seen"`
	ClosedOn  *string `json:"closed_on"`
	AsOf      *string `json:"as_of"`
	Basis     string  `json:"basis"`
}

// checkHistory is what the local store knows about one id.
type checkHistory struct {
	posting   uberjobs.Posting
	firstSeen string
	lastSeen  string
	closedOn  *string
}

func newNovelCheckCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var maxAge string
	cmd := &cobra.Command{
		Use:   "check <id>...",
		Short: "Report whether each known posting id is still open, closed, never seen or unknown, with the date it closed",
		Long: strings.Trim(`
Report the status of known posting ids in one call: open, closed, never_seen, or unknown

Pass ids as arguments, or '-' to read them from stdin (whitespace or commas
between ids). open means the posting is listed; closed means a complete scan no
longer lists a posting this store saw before, with closed_on when the store
recorded the day; never_seen means a complete scan does not list it and the store
never saw it; unknown means no complete scan can answer. A refused or partial
read never reports closed.

auto answers from the local store when its last complete sync is newer than
--max-age, and otherwise reads the whole corpus live (two requests). live always
reads live; local never does. When the careers site refuses, the same ids are
read from Uber's Oracle candidate-experience API.

Exit codes: 2 usage, 5 API or content error, 6 DNS or transport failure,
7 refused (403, 429, or a challenge; never retried).`, "\n"),
		Example: strings.Trim(`
  uber-jobs-pp-cli check 302906 160425 --json
  uber-jobs-pp-cli check - --data-source local < tracked-ids.txt
  uber-jobs-pp-cli check 301235 --data-source live --agent`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "auto",
			"pp:happy-args":  "id=301235",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			ds, err := resolveDataSource(cmd, flags, "auto", "live", "local")
			if err != nil {
				return err
			}
			age, err := parseMaxAge(maxAge)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			now := nowUTC()
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, describeCheck(ctx, flags, ds, dbPath, age, now, args))
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("at least one posting id is required, or '-' to read ids from stdin"))
			}
			ids, err := parsePostingIDs(args, cmd.InOrStdin())
			if err != nil {
				return err
			}
			if len(ids) == 0 {
				return usageErr(fmt.Errorf("no posting ids were given"))
			}
			resolved, last, err := resolveLocalFirst(ctx, ds, dbPath, age, now)
			if err != nil {
				return err
			}
			env, rows, err := runCheck(ctx, flags, ds, resolved, dbPath, ids, last, now)
			if err != nil {
				return err
			}
			env.Results = rows
			env.Hits, env.Returned = len(rows), len(rows)
			return printEnvelope(cmd, flags, env, func(w io.Writer) error {
				tw := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
				fmt.Fprintln(tw, "ID\tSTATUS\tCLOSED ON\tTITLE")
				for _, r := range rows {
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", r.ID, r.Status, deref(r.ClosedOn, "-"), deref(r.Title, ""))
				}
				if err := tw.Flush(); err != nil {
					return err
				}
				fmt.Fprintf(w, "\nsource: %s\n", env.Meta.Source)
				if env.Meta.Note != "" {
					fmt.Fprintln(w, "note:", env.Meta.Note)
				}
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&maxAge, "max-age", "24h", "With --data-source auto, the oldest local sync that may answer (24h, 3d); older reads live")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path (default: the CLI data dir)")
	addDataSourceFlag(cmd, "auto", "Where to read: auto (the local store when its last complete sync is fresh, else live), live, or local")
	return cmd
}

func parseMaxAge(raw string) (time.Duration, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	d, err := uberjobs.PostedWithin(raw)
	if err != nil {
		return 0, usageErr(fmt.Errorf("invalid --max-age %q: use a duration such as 24h or 3d", raw))
	}
	return d, nil
}

// describeCheck renders the real source a check would read, for --dry-run.
func describeCheck(ctx context.Context, flags *rootFlags, ds, dbPath string, age time.Duration, now time.Time, args []string) string {
	what := "the given ids"
	if len(args) == 1 && args[0] == "-" {
		what = "ids read from stdin"
	} else if len(args) > 0 {
		what = strings.Join(args, ", ")
	}
	resolved, last, err := resolveLocalFirst(ctx, ds, dbPath, age, now)
	if err != nil {
		return "check " + what
	}
	if resolved == "local" {
		at := "no complete sync yet"
		if last != nil {
			at = "last complete sync " + last.FinishedAt
		}
		return fmt.Sprintf("answer %s from the local store at %s (%s)", what, uberDBPath(dbPath), at)
	}
	desc, err := describeRead(baseURLFor(flags), uberjobs.Filters{})
	if err != nil {
		return "check " + what
	}
	return fmt.Sprintf("%s, then answer %s", desc, what)
}

// runCheck answers ids from the resolved source. auto falls back to the local
// store (any age) when the live read fails, and reports unknown rather than
// closed whenever no complete scan covers an id.
func runCheck(ctx context.Context, flags *rootFlags, ds, resolved, dbPath string, ids []string, last *uberjobs.SyncRun, now time.Time) (uberjobs.Envelope, []checkRow, error) {
	env := uberjobs.NewEnvelope("check", "", []checkRow{}, 0)
	history, err := loadCheckHistory(ctx, dbPath)
	if err != nil {
		return env, nil, err
	}
	if resolved == "local" {
		return env, checkLocal(&env, ids, history, last), nil
	}
	c, err := newUberClient(flags)
	if err != nil {
		return env, nil, err
	}
	read, rerr := readLive(ctx, c, uberjobs.Filters{})
	if rerr != nil {
		if ds == "auto" && (uberjobs.IsRefusal(rerr) || uberjobs.IsTransport(rerr)) && last != nil {
			rows := checkLocal(&env, ids, history, last)
			env.Meta.Fallback = true
			env.Meta.FallbackFrom = rerr.Error()
			env.Meta.Note = "the live read failed, so these answers come from the local store; " + env.Meta.Note
			return env, rows, nil
		}
		return env, nil, uberErr(rerr)
	}
	env.Meta.Source = read.Source
	env.Meta.Fallback = read.Fallback
	env.Meta.FallbackFrom = read.FallbackFrom
	env.Meta.Requests = c.Requests()
	env.Meta.Cache = read.Cache
	env.Scanned = len(read.Postings)
	env.ScanCapHit = read.ScanCapHit
	complete := read.Complete && !read.ScanCapHit
	env.Meta.Complete = complete
	listed := make(map[string]uberjobs.Posting, len(read.Postings))
	for _, p := range read.Postings {
		listed[p.ID] = p
	}
	asOf := now.Format(time.RFC3339)
	basis := "live read of the whole corpus from " + read.Source
	if !complete {
		basis = "partial live read from " + read.Source
		env.Meta.Note = "the live read was incomplete, so ids it did not list are unknown, not closed; " + read.Note
	}
	rows := make([]checkRow, 0, len(ids))
	for _, id := range ids {
		row := checkRow{ID: id, AsOf: &asOf, Basis: basis}
		h, known := history[id]
		if known {
			row.FirstSeen, row.LastSeen = strPtrOrNil(h.firstSeen), strPtrOrNil(h.lastSeen)
		}
		if p, ok := listed[id]; ok {
			fillCheckPosting(&row, p)
			row.Status = "open"
			rows = append(rows, row)
			continue
		}
		switch {
		case !complete:
			row.Status = "unknown"
		case known:
			fillCheckPosting(&row, h.posting)
			row.Status = "closed"
			row.ClosedOn = h.closedOn
		default:
			row.Status = "never_seen"
		}
		rows = append(rows, row)
	}
	return env, rows, nil
}

// checkLocal answers ids from the store's complete-scan history.
func checkLocal(env *uberjobs.Envelope, ids []string, history map[string]checkHistory, last *uberjobs.SyncRun) []checkRow {
	env.Meta.Source = uberjobs.SourceLocal
	env.Meta.Complete = last != nil
	env.Scanned = len(history)
	env.Meta.Note = localFirstNote("local", last)
	rows := make([]checkRow, 0, len(ids))
	var asOf *string
	basis := "no complete sync in the local store"
	if last != nil {
		at := last.FinishedAt
		asOf = &at
		basis = "local store, last complete sync " + last.FinishedAt
	}
	for _, id := range ids {
		row := checkRow{ID: id, AsOf: asOf, Basis: basis}
		h, known := history[id]
		if known {
			fillCheckPosting(&row, h.posting)
			row.FirstSeen, row.LastSeen = strPtrOrNil(h.firstSeen), strPtrOrNil(h.lastSeen)
		}
		switch {
		case last == nil:
			row.Status = "unknown"
		case known && h.closedOn != nil:
			row.Status, row.ClosedOn = "closed", h.closedOn
		case known:
			row.Status = "open"
		default:
			row.Status = "never_seen"
		}
		rows = append(rows, row)
	}
	return rows
}

// loadCheckHistory reads every stored posting without creating the store.
func loadCheckHistory(ctx context.Context, dbPath string) (map[string]checkHistory, error) {
	out := map[string]checkHistory{}
	_, err := withStoreRO(ctx, dbPath, func(s *store.Store) error {
		stored, err := uberjobs.LoadPostings(ctx, s.DB(), false)
		if isMissingTable(err) {
			return nil
		}
		if err != nil {
			return err
		}
		for _, sp := range stored {
			out[sp.ID] = checkHistory{posting: sp.Posting, firstSeen: sp.FirstSeen, lastSeen: sp.LastSeen, closedOn: sp.ClosedOn}
		}
		return nil
	})
	return out, err
}

func fillCheckPosting(row *checkRow, p uberjobs.Posting) {
	row.Title = strPtrOrNil(p.Title)
	row.URL = strPtrOrNil(p.URL)
	row.PostedOn = p.PostedOn
}

func strPtrOrNil(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return &s
}
