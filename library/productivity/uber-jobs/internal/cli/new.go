// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/uber-jobs/internal/store"
	"github.com/mvanhorn/printing-press-library/library/productivity/uber-jobs/internal/uberjobs"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newNovelNewCmd(flags))
	})
}

// newAdded is a posting that entered a saved search since its last advance.
type newAdded struct {
	uberjobs.Posting
	Reopened bool `json:"reopened"`
}

// newRemoved is a posting that left a saved search. status is closed when
// the posting is no longer listed at all, removed when only this search lost it
// (or the local store cannot tell which).
type newRemoved struct {
	ID        string  `json:"id"`
	Title     *string `json:"title"`
	Status    string  `json:"status"`
	ClosedOn  *string `json:"closed_on"`
	FirstSeen string  `json:"first_seen"`
	LastSeen  string  `json:"last_seen"`
}

// newSearchRow is one saved search's result. baseline_size is the baseline
// the search holds after this run (unchanged when the run did not advance it).
type newSearchRow struct {
	ID                  string           `json:"id"`
	Name                string           `json:"name"`
	Filters             uberjobs.Filters `json:"filters"`
	Source              string           `json:"source"`
	Complete            bool             `json:"complete"`
	BaselineEstablished bool             `json:"baseline_established"`
	BaselineAdvanced    bool             `json:"baseline_advanced"`
	BaselineSize        int              `json:"baseline_size"`
	CurrentCount        int              `json:"current_count"`
	AddedCount          int              `json:"added_count"`
	RemovedCount        int              `json:"removed_count"`
	Added               []newAdded       `json:"added"`
	AddedTruncated      bool             `json:"added_truncated"`
	Removed             []newRemoved     `json:"removed"`
	Note                string           `json:"note"`
}

// newRead is one read of a saved search's membership.
type newRead struct {
	postings []uberjobs.Posting
	source   string
	// usable: every saved filter was applied exactly to data no older than the baseline.
	usable bool
	// complete: usable and covering every matching posting, so removals are real.
	complete     bool
	reason       string
	scanned      int
	capped       bool
	fallbackFrom string
	asOf         time.Time
}

func newNovelNewCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var all bool
	var limit int
	cmd := &cobra.Command{
		// The usage line names no positional: a saved-search name is
		// optional (--all runs every search), and the Long text and examples
		// show both forms. The press's live dogfood skips a command whose
		// usage carries a placeholder it cannot fill, so `new --all` (the
		// weekly check) would never run there.
		Use:   "new",
		Short: "See which Uber postings appeared in or closed from a saved search since its last complete scan",
		Long: strings.Trim(`
Diff a saved search, or every saved search with --all, against its baseline

Give one saved-search name (new uk-strategy), or --all for every saved search.

Each search is read fresh (a size probe plus one full page, two requests) and
its posting ids are compared with the ids it held at its last complete scan.
The first run takes the baseline and lists nothing. Later runs list postings
that were added (newest first; reopened marks a posting that came back) and
postings that left, with status closed when the local store saw them close.
Ids are compared rather than dates, so hours of edge-cache lag cannot hide a
posting. The baseline advances only after a complete read that applied every
saved filter exactly; otherwise baseline_advanced is false and the same changes
show again next time. --posted-within on a saved search only limits which added
postings are shown; a posting aging out of the window never counts as closed.

When the careers site refuses, the Oracle fallback answers searches it can
apply exactly (country filters only); with --data-source local the last complete
sync answers searches without a keyword. An unknown name exits not-found (3).
With --all, every search is read and diffed before any baseline is written.
If one fails, it is listed with its error (note "failed: ..."), the others are
still listed, no baseline advances in that run (so the listed changes show
again next time), and the command exits with the first failure's code.

Exit codes: 2 usage, 3 not found, 5 API or content error, 6 DNS or transport
failure, 7 refused (403, 429, or a challenge; never retried).`, "\n"),
		// The first example spells --all=true: the press's live dogfood
		// overlays pp:happy-args onto it, and only an explicit =value is
		// replaced in place (a bare --all would gain a stray "true" argument).
		Example: strings.Trim(`
  uber-jobs-pp-cli new --all=true --json
  uber-jobs-pp-cli new uk-strategy --agent
  uber-jobs-pp-cli new uk-strategy --data-source local --limit 20`, "\n"),
		Annotations: map[string]string{
			"mcp:local-write":    "true",
			"pp:data-source":     "auto",
			"pp:happy-args":      "--all",
			"pp:live-happy-path": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			ds, err := resolveDataSource(cmd, flags, "auto", "live", "local")
			if err != nil {
				return err
			}
			name := ""
			if len(args) >= 1 {
				name = strings.TrimSpace(args[0])
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, describeNew(ctx, flags, ds, dbPath, name, all))
			}
			if len(args) > 1 {
				return usageErr(fmt.Errorf("new takes one saved-search name, got %d; use --all for every search", len(args)))
			}
			if name != "" && all {
				return usageErr(fmt.Errorf("give a saved-search name or --all, not both"))
			}
			if limit < 0 {
				return usageErr(fmt.Errorf("--limit must be zero or positive"))
			}
			if name == "" && !all {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a saved-search name or --all is required (list them with: uber-jobs-pp-cli searches)"))
			}
			s, db, err := openUberStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer s.Close()
			var searches []uberjobs.SavedSearch
			if all {
				if searches, err = uberjobs.ListSearches(ctx, db); err != nil {
					return fmt.Errorf("listing saved searches: %w", err)
				}
			} else {
				sv, err := uberjobs.GetSearch(ctx, db, name)
				if err != nil {
					return fmt.Errorf("reading saved search %q: %w", name, err)
				}
				if sv == nil {
					return notFoundErr(fmt.Errorf("no saved search named %q; create it with: uber-jobs-pp-cli save %s --country GBR", name, name))
				}
				searches = []uberjobs.SavedSearch{*sv}
			}
			now := nowUTC()
			var c *uberjobs.Client
			// Every search is read and diffed before anything is written, so a
			// failure in one leaves every baseline where it was: the changes
			// listed now show again next run, whoever reads this output.
			plans := make([]newPlan, 0, len(searches))
			var firstErr error
			var failedNames []string
			for _, sv := range searches {
				plan, err := planNewSearch(ctx, flags, &c, db, ds, sv, limit, now)
				if err != nil {
					if !all {
						return err
					}
					if firstErr == nil {
						firstErr = err
					}
					failedNames = append(failedNames, sv.Name)
					plan.markFailed(err)
				}
				plans = append(plans, plan)
			}
			if len(failedNames) > 0 {
				holdPlans(plans, "saved search "+strings.Join(failedNames, ", ")+" failed in this run (fix it, or delete it with: uber-jobs-pp-cli searches --delete <name>)")
			}
			// Every write goes in one transaction: a store failure leaves
			// every baseline where it was, and every row keeps its listing.
			var writeErr error
			if err := commitPlans(ctx, db, plans); err != nil {
				writeErr = fmt.Errorf("writing the saved-search baselines: %w", err)
				if !all {
					return writeErr
				}
				holdPlans(plans, "the store write failed")
				if firstErr == nil {
					firstErr = writeErr
				}
			}
			rows := make([]newSearchRow, 0, len(plans))
			env := uberjobs.NewEnvelope("new", uberjobs.SourceLocal, rows, 0)
			env.Meta.Complete = firstErr == nil
			sources := map[string]bool{}
			for _, p := range plans {
				rows = append(rows, p.row)
				if p.failed {
					continue
				}
				sources[p.row.Source] = true
				env.Scanned += p.read.scanned
				env.ScanCapHit = env.ScanCapHit || p.read.capped
				env.Meta.Complete = env.Meta.Complete && p.row.Complete
				if p.read.fallbackFrom != "" && !env.Meta.Fallback {
					env.Meta.Fallback = true
					env.Meta.FallbackFrom = p.read.fallbackFrom
				}
			}
			switch len(sources) {
			case 0:
				if len(plans) == 0 {
					env.Meta.Note = "no saved searches; create one with: uber-jobs-pp-cli save <name> --country GBR"
				}
			case 1:
				for src := range sources {
					env.Meta.Source = src
				}
			default:
				env.Meta.Source = "mixed"
			}
			if c != nil {
				env.Meta.Requests = c.Requests()
			}
			env.Results = rows
			env.Hits, env.Returned = len(rows), len(rows)
			switch {
			case len(failedNames) > 0:
				env.Meta.Note = fmt.Sprintf("%d of %d saved searches failed (%s), so no baseline advanced in this run and the changes listed show again next run. First error: %v", len(failedNames), len(searches), strings.Join(failedNames, ", "), firstErr)
				if writeErr != nil {
					env.Meta.Note += fmt.Sprintf(" The store write also failed: %v", writeErr)
				}
			case writeErr != nil:
				env.Meta.Note = fmt.Sprintf("the store write failed, so no baseline advanced in this run and the changes listed show again next run: %v", writeErr)
			}
			if err := printEnvelope(cmd, flags, env, func(w io.Writer) error {
				return renderNew(w, env, rows)
			}); err != nil {
				return err
			}
			if firstErr != nil && flags.deliverBuf != nil {
				// Execute delivers only the output of a command that succeeded;
				// this partial result must still reach the --deliver sink.
				if derr := Deliver(flags.deliverSink, flags.deliverBuf.Bytes(), flags.compact); derr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: deliver to %s:%s failed: %v\n", flags.deliverSink.Scheme, flags.deliverSink.Target, derr)
				}
			}
			return firstErr
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "Run every saved search, in name order")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum added postings to list per search (0 lists all; added_count is never truncated)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path (default: the CLI data dir)")
	addDataSourceFlag(cmd, "auto", "Where to read: auto (live, then the local store if the site is unreachable), live, or local")
	return cmd
}

// membershipFilters are the saved filters that decide membership: everything
// except the display-only recency window and sort order.
func membershipFilters(f uberjobs.Filters) uberjobs.Filters {
	f.PostedWithin = ""
	f.Sort = ""
	return f
}

// newPlan is one saved search read and diffed, with its store write held
// back until every search in the run has been read.
type newPlan struct {
	row  newSearchRow
	read newRead
	// advances: commit writes the baseline, and row reports it advanced.
	advances bool
	// commit is the store write the read allows (the baseline advance, or
	// the check stamp), run inside the run's one transaction; nil when
	// nothing is written.
	commit func(context.Context, *sql.Tx) error
	// checked stamps the check without advancing; nil before a baseline.
	checked    func(context.Context, *sql.Tx) error
	priorSize  int
	firstTaken bool
	failed     bool
}

// markFailed turns the plan into the row for a search that failed.
func (p *newPlan) markFailed(err error) {
	p.failed, p.advances, p.commit = true, false, nil
	p.row.Complete, p.row.BaselineAdvanced, p.row.BaselineEstablished = false, false, false
	p.row.BaselineSize = p.priorSize
	p.row.AddedCount, p.row.RemovedCount, p.row.AddedTruncated = 0, 0, false
	p.row.Added, p.row.Removed = []newAdded{}, []newRemoved{}
	p.row.Note = "failed: " + err.Error()
}

// hold keeps an advancing plan from writing its baseline, for reason. It
// only stamps the check, so the changes it lists show again next run.
func (p *newPlan) hold(reason string) {
	p.advances, p.commit = false, p.checked
	p.row.BaselineAdvanced, p.row.BaselineEstablished = false, false
	p.row.BaselineSize = p.priorSize
	if p.firstTaken {
		p.row.Note = "no baseline taken because " + reason + "; the next run takes it"
		return
	}
	note := "baseline held because " + reason + ", so these changes show again next run"
	if p.row.AddedTruncated {
		note += fmt.Sprintf("; listing %d of %d added postings", len(p.row.Added), p.row.AddedCount)
	}
	p.row.Note = note
}

// holdPlans holds every plan that would advance a baseline.
func holdPlans(plans []newPlan, reason string) {
	for i := range plans {
		if p := &plans[i]; !p.failed && p.advances {
			p.hold(reason)
		}
	}
}

// newWriteTimeout bounds the store writes of one new run. They get their own
// deadline because the reads may have used up the command's --timeout, and a
// search that read fine must still record what it may.
const newWriteTimeout = 15 * time.Second

// commitPlans writes every plan's store change in one transaction, so a run
// advances all of its baselines or none.
func commitPlans(ctx context.Context, db *sql.DB, plans []newPlan) error {
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), newWriteTimeout)
	defer cancel()
	tx, err := db.BeginTx(wctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for i := range plans {
		if p := &plans[i]; !p.failed && p.commit != nil {
			if err := p.commit(wctx, tx); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

// runNewSearch reads, diffs and commits one saved search on its own.
func runNewSearch(ctx context.Context, flags *rootFlags, cp **uberjobs.Client, db *sql.DB, ds string, sv uberjobs.SavedSearch, limit int, now time.Time) (newSearchRow, newRead, error) {
	plan, err := planNewSearch(ctx, flags, cp, db, ds, sv, limit, now)
	if err != nil {
		return plan.row, plan.read, err
	}
	if err := commitPlans(ctx, db, []newPlan{plan}); err != nil {
		return plan.row, plan.read, err
	}
	return plan.row, plan.read, nil
}

// planNewSearch reads one saved search and diffs it against its baseline,
// writing nothing; the plan's commit does the write the read allows.
func planNewSearch(ctx context.Context, flags *rootFlags, cp **uberjobs.Client, db *sql.DB, ds string, sv uberjobs.SavedSearch, limit int, now time.Time) (newPlan, error) {
	plan := newPlan{priorSize: sv.BaselineSize}
	plan.row = newSearchRow{ID: sv.Name, Name: sv.Name, Filters: sv.Filters, BaselineSize: sv.BaselineSize, Added: []newAdded{}, Removed: []newRemoved{}}
	row := &plan.row
	membership := membershipFilters(sv.Filters)
	read, err := readForNew(ctx, flags, cp, db, ds, sv, membership, now)
	if err != nil {
		return plan, err
	}
	plan.read = read
	row.Source = read.source
	row.Complete = read.complete
	first := sv.BaselineAt == nil
	if !first {
		plan.checked = func(ctx context.Context, tx *sql.Tx) error {
			if err := uberjobs.MarkChecked(ctx, tx, sv.Name, now); err != nil {
				return fmt.Errorf("recording the check of %q: %w", sv.Name, err)
			}
			return nil
		}
	}
	if !read.usable {
		row.Note = read.reason
		if first {
			row.Note = "no baseline taken: " + read.reason
		}
		plan.commit = plan.checked
		return plan, nil
	}
	row.CurrentCount = len(read.postings)
	if first {
		if !read.complete {
			row.Note = "no baseline taken: " + read.reason
			return plan, nil
		}
		row.BaselineEstablished, row.BaselineAdvanced = true, true
		row.BaselineSize = len(read.postings)
		row.Note = fmt.Sprintf("baseline established with %d postings; later runs list what changed", len(read.postings))
		plan.advances, plan.firstTaken = true, true
		plan.commit = func(ctx context.Context, tx *sql.Tx) error {
			if err := searchUnchanged(ctx, tx, sv); err != nil {
				return err
			}
			if _, err := uberjobs.AdvanceMembershipTx(ctx, tx, sv.Name, read.postings, true, read.asOf); err != nil {
				return fmt.Errorf("taking the baseline of %q: %w", sv.Name, err)
			}
			return nil
		}
		return plan, nil
	}
	members, err := uberjobs.Members(ctx, db, sv.Name)
	if err != nil {
		return plan, fmt.Errorf("reading the baseline of %q: %w", sv.Name, err)
	}
	diff := uberjobs.DiffMembership(members, read.postings)
	window, err := uberjobs.PostedWithin(sv.Filters.PostedWithin)
	if err != nil {
		return plan, usageErr(fmt.Errorf("saved search %q: %w", sv.Name, err))
	}
	shown := make([]uberjobs.Posting, 0, len(diff.Added))
	for _, p := range diff.Added {
		if uberjobs.WithinWindow(p, window, now) {
			shown = append(shown, p)
		}
	}
	sortPostings(shown, "recent")
	row.AddedCount = len(shown)
	if limit > 0 && len(shown) > limit {
		shown = shown[:limit]
		row.AddedTruncated = true
	}
	for _, p := range shown {
		row.Added = append(row.Added, newAdded{Posting: p, Reopened: diff.Reopened[p.ID]})
	}
	if !read.complete {
		row.Note = read.reason + "; removals were not computed and the baseline was not advanced, so these additions will show again"
		plan.commit = plan.checked
		return plan, nil
	}
	if len(diff.Removed) > 0 {
		removed, err := removedStatuses(ctx, db, diff.Removed, membership, read)
		if err != nil {
			return plan, err
		}
		row.Removed = removed
		row.RemovedCount = len(removed)
	}
	row.BaselineAdvanced = true
	row.BaselineSize = len(read.postings)
	if row.AddedTruncated {
		row.Note = fmt.Sprintf("listing %d of %d added postings; raise --limit to see the rest", len(row.Added), row.AddedCount)
	}
	plan.advances = true
	plan.commit = func(ctx context.Context, tx *sql.Tx) error {
		if err := searchUnchanged(ctx, tx, sv); err != nil {
			return err
		}
		if _, err := uberjobs.AdvanceMembershipTx(ctx, tx, sv.Name, read.postings, false, read.asOf); err != nil {
			return fmt.Errorf("advancing the baseline of %q: %w", sv.Name, err)
		}
		return nil
	}
	return plan, nil
}

// readForNew reads a saved search's membership from the chosen source.
func readForNew(ctx context.Context, flags *rootFlags, cp **uberjobs.Client, db *sql.DB, ds string, sv uberjobs.SavedSearch, membership uberjobs.Filters, now time.Time) (newRead, error) {
	if ds == "local" {
		return localReadForNew(ctx, db, sv, membership, now)
	}
	if *cp == nil {
		c, err := newUberClient(flags)
		if err != nil {
			return newRead{}, err
		}
		*cp = c
	}
	read, err := readLive(ctx, *cp, membership)
	if err != nil {
		if ds == "auto" && (uberjobs.IsRefusal(err) || uberjobs.IsTransport(err)) {
			if lr, lerr := localReadForNew(ctx, db, sv, membership, now); lerr == nil {
				lr.fallbackFrom = err.Error()
				if !lr.usable {
					lr.reason = "the live read failed (" + err.Error() + ") and " + lr.reason
				}
				return lr, nil
			}
		}
		return newRead{}, uberErr(err)
	}
	r := newRead{source: read.Source, scanned: len(read.Postings), capped: read.ScanCapHit, fallbackFrom: read.FallbackFrom, asOf: now}
	if read.Source == uberjobs.SourceOracle {
		if membership.WorkPattern != "" || len(membership.DescriptionContains) > 0 || len(membership.DescriptionExcludes) > 0 {
			r.reason = "the careers site refused and the Oracle fallback carries no work pattern or description, so this search's filters cannot be applied"
			return r, nil
		}
		if strings.TrimSpace(membership.Query) != "" {
			r.reason = "the careers site refused and the Oracle fallback's keyword search is a different engine, so this search was not diffed"
			return r, nil
		}
	}
	rows, err := applyClientFilters(read.Postings, membership, now)
	if err != nil {
		return r, err
	}
	r.postings = rows
	r.usable = true
	r.complete = read.Complete && !read.ScanCapHit
	if !r.complete {
		r.reason = read.Note
		if r.reason == "" {
			r.reason = "the read was incomplete"
		}
	}
	return r, nil
}

// localReadForNew answers from the last complete sync. It refuses keyword
// searches (offline matching is approximate) and snapshots older than the
// search's last advance, because both would report changes that did not happen.
func localReadForNew(ctx context.Context, db *sql.DB, sv uberjobs.SavedSearch, membership uberjobs.Filters, now time.Time) (newRead, error) {
	r := newRead{source: uberjobs.SourceLocal}
	last, err := uberjobs.LastFullSync(ctx, db)
	if err != nil {
		return r, err
	}
	stored, err := uberjobs.LoadPostings(ctx, db, true)
	if err != nil {
		return r, err
	}
	r.scanned = len(stored)
	later := 0
	if last != nil {
		if later, err = uberjobs.SyncRunsAfter(ctx, db, last.ID); err != nil {
			return r, err
		}
	}
	switch {
	case last == nil:
		r.reason = "the local store has no complete sync; run: uber-jobs-pp-cli sync"
		return r, nil
	case strings.TrimSpace(membership.Query) != "":
		r.reason = "a keyword search cannot be applied exactly offline; run new against the live site"
		return r, nil
	case later > 0:
		// A keyword or fallback sync since then changed the open rows, so
		// they no longer match the complete sync this read would claim.
		r.reason = fmt.Sprintf("%d sync run(s) after the last complete sync (%s) changed the local store, so it no longer matches that snapshot; run a full sync first", later, last.FinishedAt)
		return r, nil
	case sv.LastAdvancedAt != nil && last.FinishedAt < *sv.LastAdvancedAt:
		r.reason = fmt.Sprintf("the last complete sync (%s) is older than this search's last advance (%s); run sync first", last.FinishedAt, *sv.LastAdvancedAt)
		return r, nil
	}
	asOf, err := time.Parse(time.RFC3339, last.FinishedAt)
	if err != nil {
		return r, fmt.Errorf("reading the last sync time %q: %w", last.FinishedAt, err)
	}
	r.postings = make([]uberjobs.Posting, 0)
	for _, sp := range stored {
		if membership.MatchLocal(sp.Posting, 0, now) {
			r.postings = append(r.postings, sp.Posting)
		}
	}
	r.usable, r.complete, r.asOf = true, true, asOf
	return r, nil
}

// searchUnchanged fails when the saved search changed after it was read: a
// concurrent save or searches --delete replaced it, or another new run
// advanced it. A commit then never writes members computed with filters the
// search no longer has, nor an older read over a newer baseline.
func searchUnchanged(ctx context.Context, tx *sql.Tx, sv uberjobs.SavedSearch) error {
	cur, err := uberjobs.GetSearch(ctx, tx, sv.Name)
	if err != nil {
		return fmt.Errorf("re-reading saved search %q: %w", sv.Name, err)
	}
	if cur == nil || filtersJSON(cur.Filters) != filtersJSON(sv.Filters) ||
		deref(cur.BaselineAt, "") != deref(sv.BaselineAt, "") ||
		deref(cur.LastAdvancedAt, "") != deref(sv.LastAdvancedAt, "") {
		return fmt.Errorf("saved search %q changed while new was reading it; run new again", sv.Name)
	}
	return nil
}

// removedStatuses labels members that left a search. With no filters the read
// was the whole corpus, so a missing posting is closed; otherwise only the
// local store's closed_on can say closed.
func removedStatuses(ctx context.Context, db *sql.DB, removed []uberjobs.Member, membership uberjobs.Filters, read newRead) ([]newRemoved, error) {
	stored, err := uberjobs.LoadPostings(ctx, db, false)
	if err != nil {
		return nil, fmt.Errorf("reading posting history: %w", err)
	}
	closedOn := make(map[string]*string, len(stored))
	for _, sp := range stored {
		closedOn[sp.ID] = sp.ClosedOn
	}
	wholeCorpus := filtersJSON(membership) == "{}"
	out := make([]newRemoved, 0, len(removed))
	for _, m := range removed {
		r := newRemoved{ID: m.PostingID, Title: m.Title, Status: "removed", FirstSeen: m.FirstSeen, LastSeen: m.LastSeen}
		switch {
		case closedOn[m.PostingID] != nil:
			r.Status, r.ClosedOn = "closed", closedOn[m.PostingID]
		case wholeCorpus:
			at := read.asOf.UTC().Format(time.RFC3339)
			r.Status, r.ClosedOn = "closed", &at
		}
		out = append(out, r)
	}
	return out, nil
}

// describeNew renders the real reads a new run would send, for --dry-run.
func describeNew(ctx context.Context, flags *rootFlags, ds, dbPath, name string, all bool) string {
	db := uberDBPath(dbPath)
	if all {
		var names []string
		_, _ = withStoreROList(ctx, dbPath, &names)
		if len(names) == 0 {
			return "diff every saved search in " + db + " (none saved yet) against a fresh read"
		}
		return fmt.Sprintf("for each of %d saved searches (%s): read it fresh and diff it against its baseline in %s", len(names), strings.Join(names, ", "), db)
	}
	if name == "" {
		return "diff a saved search against a fresh read"
	}
	sv, err := savedSearchRO(ctx, dbPath, name)
	if err != nil || sv == nil {
		return fmt.Sprintf("diff saved search %q (not found in %s) against a fresh read", name, db)
	}
	if ds == "local" {
		return fmt.Sprintf("diff saved search %q (baseline %d) against the last complete sync in %s", name, sv.BaselineSize, db)
	}
	desc, err := describeRead(baseURLFor(flags), membershipFilters(sv.Filters))
	if err != nil {
		return fmt.Sprintf("diff saved search %q against a fresh read", name)
	}
	return fmt.Sprintf("%s for saved search %q, then diff against its baseline of %d in %s", desc, name, sv.BaselineSize, db)
}

// withStoreROList lists saved-search names without creating the store.
func withStoreROList(ctx context.Context, dbPath string, names *[]string) (bool, error) {
	var list []uberjobs.SavedSearch
	ok, err := withStoreRO(ctx, dbPath, func(s *store.Store) error {
		got, err := uberjobs.ListSearches(ctx, s.DB())
		if isMissingTable(err) {
			return nil
		}
		list = got
		return err
	})
	for _, sv := range list {
		*names = append(*names, sv.Name)
	}
	return ok, err
}

// renderNew is the human view of new.
func renderNew(w io.Writer, env uberjobs.Envelope, rows []newSearchRow) error {
	if len(rows) == 0 {
		_, err := fmt.Fprintln(w, env.Meta.Note)
		return err
	}
	for i, r := range rows {
		if i > 0 {
			fmt.Fprintln(w)
		}
		switch {
		case strings.HasPrefix(r.Note, "failed: "):
			fmt.Fprintf(w, "%s: %s\n", r.Name, r.Note)
			continue
		case r.BaselineEstablished:
			fmt.Fprintf(w, "%s: baseline established with %d postings (source %s)\n", r.Name, r.CurrentCount, r.Source)
			continue
		default:
			fmt.Fprintf(w, "%s: %d added, %d left (source %s, baseline advanced: %v)\n", r.Name, r.AddedCount, r.RemovedCount, r.Source, r.BaselineAdvanced)
		}
		for _, a := range r.Added {
			mark := "+"
			if a.Reopened {
				mark = "+ (reopened)"
			}
			fmt.Fprintf(w, "  %s %s  %s  %s  %s\n", mark, a.ID, deref(a.PostedOn, "-"), deref(a.CountryCode, "-"), a.Title)
		}
		for _, rm := range r.Removed {
			fmt.Fprintf(w, "  - %s  %s %s  %s\n", rm.ID, rm.Status, deref(rm.ClosedOn, ""), deref(rm.Title, ""))
		}
		if r.Note != "" {
			fmt.Fprintln(w, "  note:", r.Note)
		}
	}
	if env.Meta.Note != "" {
		fmt.Fprintln(w, "\nnote:", env.Meta.Note)
	}
	return nil
}
