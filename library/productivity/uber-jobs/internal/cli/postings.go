// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto

package cli

import (
	"context"
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
		addNovelCommandIfAbsent(root, newNovelPostingsCmd(flags))
	})
}

// postingFlags are the filter flags shared by postings, save, and screen.
type postingFlags struct {
	country      string
	baseQuery    string
	team         string
	subTeam      string
	contractType string
	workPattern  string
	postedWithin string
	descContains []string
	descExcludes []string
	sort         string
	limit        int
	offset       int
	dbPath       string
}

// registerPostingFlags declares the shared posting filter flags on cmd.
func registerPostingFlags(cmd *cobra.Command, pf *postingFlags, withPaging, withDescription bool) {
	cmd.Flags().StringVar(&pf.country, "country", "", "Market as an ISO3 code (GBR, USA, DEU); comma-separate several; matches any location of a posting")
	cmd.Flags().StringVar(&pf.baseQuery, "base-query", "", "Keyword search the careers site runs server-side (title, description, team labels)")
	cmd.Flags().StringVar(&pf.team, "team", "", "Team exactly as the site lists it (Engineer, Operations, Sales); see the facets command")
	cmd.Flags().StringVar(&pf.subTeam, "sub-team", "", "Sub-team exactly as the site lists it (Business Operations, Customer Support)")
	cmd.Flags().StringVar(&pf.contractType, "contract-type", "", "Contract type exactly as the site lists it (Full time)")
	cmd.Flags().StringVar(&pf.workPattern, "work-pattern", "", "Work pattern filter applied client-side (Regular, Intern, Fixed Term, Direct NCG Hire)")
	cmd.Flags().StringVar(&pf.postedWithin, "posted-within", "", "Only postings whose true posting date is within this window (24h, 7d, 2w)")
	if withDescription {
		cmd.Flags().StringArrayVar(&pf.descContains, "description-contains", nil, "Keep postings whose plain-text description contains this phrase (repeatable)")
		cmd.Flags().StringArrayVar(&pf.descExcludes, "description-not-contains", nil, "Drop postings whose plain-text description contains this phrase (repeatable)")
	}
	cmd.Flags().StringVar(&pf.sort, "sort", "recent", "Order: recent (newest true posting date first) or relevant (the site's own order)")
	if withPaging {
		cmd.Flags().IntVar(&pf.limit, "limit", 100, "Maximum postings to return after sorting (0 returns all)")
		cmd.Flags().IntVar(&pf.offset, "offset", 0, "Skip this many postings after sorting, for paging")
	}
	cmd.Flags().StringVar(&pf.dbPath, "db", "", "Local store path (default: the CLI data dir)")
}

func (pf *postingFlags) filters() (uberjobs.Filters, error) {
	f := uberjobs.Filters{
		Query:               strings.TrimSpace(pf.baseQuery),
		Team:                strings.TrimSpace(pf.team),
		SubTeam:             strings.TrimSpace(pf.subTeam),
		ContractType:        strings.TrimSpace(pf.contractType),
		WorkPattern:         strings.TrimSpace(pf.workPattern),
		PostedWithin:        strings.TrimSpace(pf.postedWithin),
		DescriptionContains: pf.descContains,
		DescriptionExcludes: pf.descExcludes,
		Sort:                strings.ToLower(strings.TrimSpace(pf.sort)),
	}
	if err := rejectBlankPhrases("--description-contains", pf.descContains); err != nil {
		return f, err
	}
	if err := rejectBlankPhrases("--description-not-contains", pf.descExcludes); err != nil {
		return f, err
	}
	if f.Sort == "" {
		f.Sort = "recent"
	}
	if f.Sort != "recent" && f.Sort != "relevant" {
		return f, usageErr(fmt.Errorf("--sort must be recent or relevant, got %q", pf.sort))
	}
	for _, c := range splitList(pf.country) {
		iso, _, ok := uberjobs.ResolveCountry(c)
		if !ok {
			return f, usageErr(fmt.Errorf("--country %q is not a known country: use an ISO3 code such as GBR, USA, DEU", c))
		}
		f.Countries = append(f.Countries, iso)
	}
	if _, err := uberjobs.PostedWithin(f.PostedWithin); err != nil {
		return f, usageErr(err)
	}
	if pf.limit < 0 || pf.offset < 0 {
		return f, usageErr(fmt.Errorf("--limit and --offset must be zero or positive"))
	}
	return f, nil
}

func newNovelPostingsCmd(flags *rootFlags) *cobra.Command {
	var pf postingFlags
	cmd := &cobra.Command{
		Use:   "postings",
		Short: "List Uber postings for an ISO3 market, newest first, in the single envelope a job tracker parses",
		Long: strings.Trim(`
List Uber careers postings with tracker-stable flags and fields.

The careers site has no date sort and pages unreliably, so this command reads
the whole matching set in one fresh page (a size probe plus one full page, two
requests), then sorts by the true posting date and applies --offset/--limit.
Output is always one JSON object: {meta:{source}, hits, returned, scanned,
scan_cap_hit, results:[...]}. hits counts every match before truncation.

When the careers site refuses, the same requisitions are read from Uber's
Oracle candidate-experience API instead and meta.source says oracle-ce;
description and job_category are null in that mode.

Exit codes: 2 usage, 5 API or content error, 6 DNS or transport failure,
7 refused (403, 429, or a challenge; never retried).`, "\n"),
		Example: strings.Trim(`
  uber-jobs-pp-cli postings --country GBR --limit 100 --offset 0 --sort recent --json --data-source live
  uber-jobs-pp-cli postings --country USA --base-query strategy --posted-within 7d --agent
  uber-jobs-pp-cli postings --country NLD --description-not-contains "fluent Dutch" --json`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "auto",
			"pp:happy-args":  "--country=GBR;--limit=5",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			f, err := pf.filters()
			if err != nil {
				return err
			}
			ds, err := resolveDataSource(cmd, flags, "auto", "live", "local")
			if err != nil {
				return err
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, describeSource(flags, ds, pf.dbPath, f))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			env, rows, err := gatherPostings(ctx, flags, ds, pf.dbPath, f, nowUTC())
			if err != nil {
				return err
			}
			sortPostings(rows, f.Sort)
			env.Meta.Note = joinNote(env.Meta.Note, secondaryCountryNote(rows, f.Countries))
			env.Hits = len(rows)
			page := pageRows(rows, pf.offset, pf.limit)
			env.Returned = len(page)
			env.Results = page
			return printEnvelope(cmd, flags, env, func(w io.Writer) error {
				return renderPostingsTable(w, env, page)
			})
		},
	}
	registerPostingFlags(cmd, &pf, true, true)
	addDataSourceFlag(cmd, "auto", "Where to read: auto (live, then the local store if the site is unreachable), live, or local")
	return cmd
}

// describeSource renders what a --dry-run would read.
func describeSource(flags *rootFlags, ds, dbPath string, f uberjobs.Filters) string {
	if ds == "local" {
		return "read postings from the local store at " + uberDBPath(dbPath)
	}
	desc, err := describeRead(baseURLFor(flags), f)
	if err != nil {
		return "read postings (invalid filters: " + err.Error() + ")"
	}
	return desc
}

// gatherPostings reads filtered postings from the chosen data source. auto
// reads live (with the Oracle fallback on refusal) and falls back to the
// local store only when both live paths fail and the store has rows.
func gatherPostings(ctx context.Context, flags *rootFlags, ds, dbPath string, f uberjobs.Filters, now time.Time) (uberjobs.Envelope, []uberjobs.Posting, error) {
	env := uberjobs.NewEnvelope("postings", "", nil, 0)
	if ds == "local" {
		local, err := localPostings(ctx, dbPath, f, now)
		if err != nil {
			return env, nil, err
		}
		env.Meta.Source = uberjobs.SourceLocal
		env.Meta.Complete = local.complete
		env.Meta.Note = local.note
		env.Scanned = local.scanned
		return env, local.rows, nil
	}
	c, err := newUberClient(flags)
	if err != nil {
		return env, nil, err
	}
	read, rerr := readLive(ctx, c, f)
	if rerr != nil {
		if ds == "auto" && (uberjobs.IsRefusal(rerr) || uberjobs.IsTransport(rerr)) {
			if local, lerr := localPostings(ctx, dbPath, f, now); lerr == nil && local.scanned > 0 {
				env.Meta.Source = uberjobs.SourceLocal
				env.Meta.Fallback = true
				env.Meta.FallbackFrom = rerr.Error()
				env.Meta.Note = "live read failed, so these rows come from the local store; " + local.note
				env.Scanned = local.scanned
				return env, local.rows, nil
			}
		}
		return env, nil, uberErr(rerr)
	}
	rows, err := applyClientFilters(read.Postings, f, now)
	if err != nil {
		return env, nil, err
	}
	env.Meta.Source = read.Source
	env.Meta.Fallback = read.Fallback
	env.Meta.FallbackFrom = read.FallbackFrom
	env.Meta.Complete = read.Complete
	env.Meta.Requests = read.Requests
	env.Meta.Note = read.Note
	env.Meta.Cache = read.Cache
	env.Scanned = len(read.Postings)
	env.ScanCapHit = read.ScanCapHit
	return env, rows, nil
}

// localResult is a filtered read from the local store.
type localResult struct {
	rows     []uberjobs.Posting
	scanned  int
	complete bool
	note     string
}

// localPostings filters the local store, read-only: a missing store is an
// empty one and is never created.
func localPostings(ctx context.Context, dbPath string, f uberjobs.Filters, now time.Time) (*localResult, error) {
	window, err := uberjobs.PostedWithin(f.PostedWithin)
	if err != nil {
		return nil, usageErr(err)
	}
	var stored []uberjobs.StoredPosting
	var last *uberjobs.SyncRun
	_, err = withStoreRO(ctx, dbPath, func(s *store.Store) error {
		var err error
		if stored, err = uberjobs.LoadPostings(ctx, s.DB(), true); err != nil {
			return err
		}
		last, err = uberjobs.LastFullSync(ctx, s.DB())
		return err
	})
	if err != nil && !isMissingTable(err) {
		return nil, err
	}
	res := &localResult{rows: make([]uberjobs.Posting, 0), scanned: len(stored)}
	for _, sp := range stored {
		if f.MatchLocal(sp.Posting, window, now) {
			res.rows = append(res.rows, sp.Posting)
		}
	}
	switch {
	case len(stored) == 0:
		res.note = "the local store is empty; run: uber-jobs-pp-cli sync"
	case last == nil:
		res.note = "no complete full sync recorded yet; local rows may be partial"
	default:
		res.complete = true
		res.note = fmt.Sprintf("from the local store, last complete sync %s; the keyword filter is a local substring match", last.FinishedAt)
	}
	return res, nil
}

// baseURLFor resolves the careers base URL the same way the client does.
func baseURLFor(flags *rootFlags) string {
	c, err := newUberClient(flags)
	if err != nil || c == nil {
		return uberjobs.DefaultBaseURL
	}
	return c.BaseURL
}

// rejectBlankPhrases refuses a blank phrase (an unset shell variable, say):
// it would match every description, keeping or dropping everything.
func rejectBlankPhrases(flag string, phrases []string) error {
	for _, p := range phrases {
		if strings.TrimSpace(p) == "" {
			return usageErr(fmt.Errorf("%s needs a non-empty phrase", flag))
		}
	}
	return nil
}
